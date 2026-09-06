"""Document service use cases."""

from __future__ import annotations

from contextlib import suppress
import asyncio
from collections.abc import AsyncIterator
from dataclasses import dataclass
from uuid import uuid4

from delim_document.config import UploadConfig
from delim_document.domain.job import DocumentJob
from delim_document.domain.receipt import Receipt, ReceiptStatus
from delim_document.export.csv import render_csv
from delim_document.export.models import ExportFormat, ExportRecord, ExportStatus, ReportRow
from delim_document.export.pdf import render_pdf
from delim_document.export.xlsx import render_xlsx
from delim_document.ocr.provider import OCRResult
from delim_document.repository.job import JobRepository
from delim_document.repository.export import ExportRepository
from delim_document.repository.ocr_result import OCRResultRepository
from delim_document.repository.receipt import ReceiptRepository
from delim_document.storage.minio import MinioStorage


ALLOWED_CONTENT_TYPES = frozenset({"image/jpeg", "image/png", "image/webp"})


class InvalidInputError(ValueError):
    """Raised when receipt input is not valid."""


class NotFoundError(LookupError):
    """Raised when an owned document resource does not exist."""


class ForbiddenError(PermissionError):
    """Raised when an actor cannot access a resource."""


class ConflictError(RuntimeError):
    """Raised when a resource state prevents an operation."""


class DependencyUnavailableError(RuntimeError):
    """Raised when PostgreSQL or object storage is unavailable."""


@dataclass(frozen=True, slots=True)
class CreateReceiptResult:
    receipt: Receipt
    job: DocumentJob


@dataclass(frozen=True, slots=True)
class OCRResultView:
    status: ReceiptStatus
    result: OCRResult | None


def validate_receipt_upload(
    content: bytes, content_type: str, config: UploadConfig
) -> None:
    if not content:
        raise InvalidInputError("receipt image is empty")
    if len(content) > config.max_size_bytes:
        raise InvalidInputError("receipt image exceeds the configured size limit")
    if content_type not in ALLOWED_CONTENT_TYPES:
        raise InvalidInputError("unsupported receipt content type")

    detected_type: str | None = None
    if len(content) >= 3 and content[:3] == b"\xff\xd8\xff":
        detected_type = "image/jpeg"
    elif len(content) >= 8 and content[:8] == b"\x89PNG\r\n\x1a\n":
        detected_type = "image/png"
    elif (
        len(content) >= 12
        and content[:4] == b"RIFF"
        and content[8:12] == b"WEBP"
    ):
        detected_type = "image/webp"

    if detected_type != content_type:
        raise InvalidInputError("receipt bytes do not match content type")


class DocumentService:
    def __init__(
        self,
        receipts: ReceiptRepository,
        jobs: JobRepository,
        results: OCRResultRepository,
        exports: ExportRepository,
        storage: MinioStorage,
        upload_config: UploadConfig,
    ) -> None:
        self._receipts = receipts
        self._jobs = jobs
        self._results = results
        self._exports = exports
        self._storage = storage
        self._upload_config = upload_config

    async def create_receipt(
        self,
        *,
        actor_user_id: int,
        group_id: int,
        filename: str,
        content_type: str,
        content: bytes,
    ) -> CreateReceiptResult:
        if actor_user_id <= 0:
            raise InvalidInputError("actor_user_id must be positive")
        if group_id <= 0:
            raise InvalidInputError("group_id must be positive")
        if not filename.strip():
            raise InvalidInputError("filename is required")
        validate_receipt_upload(content, content_type, self._upload_config)

        object_key = f"receipts/{group_id}/{uuid4()}"
        receipt: Receipt | None = None
        await self._storage.put_receipt(object_key, content, content_type)
        try:
            receipt = await self._receipts.create(
                actor_user_id=actor_user_id,
                group_id=group_id,
                filename=filename,
                content_type=content_type,
                size_bytes=len(content),
                object_key=object_key,
            )
            job = await self._jobs.create(receipt.id)
            queued = await self._receipts.mark_status(receipt.id, ReceiptStatus.QUEUED)
            if queued is None:
                raise RuntimeError("created receipt disappeared")
            return CreateReceiptResult(receipt=queued, job=job)
        except Exception:
            if receipt is not None:
                with suppress(Exception):
                    await self._receipts.soft_delete(receipt.id, actor_user_id)
            with suppress(Exception):
                await self._storage.delete_receipt(object_key)
            raise

    async def get_receipt(self, actor_user_id: int, receipt_id: int) -> Receipt:
        if actor_user_id <= 0 or receipt_id <= 0:
            raise InvalidInputError("actor_user_id and receipt_id must be positive")
        receipt = await self._receipts.get(receipt_id, actor_user_id)
        if receipt is None:
            raise NotFoundError("receipt not found")
        return receipt

    async def get_document_job(
        self, actor_user_id: int, job_id: int
    ) -> DocumentJob:
        if actor_user_id <= 0 or job_id <= 0:
            raise InvalidInputError("actor_user_id and job_id must be positive")
        job = await self._jobs.get(job_id, actor_user_id)
        if job is None:
            raise NotFoundError("document job not found")
        return job

    async def get_ocr_result(
        self, actor_user_id: int, receipt_id: int
    ) -> OCRResultView:
        receipt = await self.get_receipt(actor_user_id, receipt_id)
        if receipt.status is not ReceiptStatus.READY:
            return OCRResultView(status=receipt.status, result=None)
        result = await self._results.get_by_receipt(receipt_id, actor_user_id)
        if result is None:
            raise ConflictError("OCR result is not available")
        return OCRResultView(status=receipt.status, result=result)

    async def retry_receipt_ocr(
        self, actor_user_id: int, receipt_id: int
    ) -> DocumentJob:
        receipt = await self.get_receipt(actor_user_id, receipt_id)
        if receipt.status is not ReceiptStatus.FAILED:
            raise ConflictError("only a failed receipt can be retried")
        job = await self._jobs.create_retry(receipt_id, actor_user_id)
        if job is None:
            raise ConflictError("receipt retry is already active")
        return job

    async def create_export(
        self,
        actor_user_id: int,
        group_id: int,
        group_name: str,
        export_format: ExportFormat,
        rows: tuple[ReportRow, ...],
    ) -> ExportRecord:
        if actor_user_id <= 0 or group_id <= 0:
            raise InvalidInputError("actor_user_id and group_id must be positive")
        if not group_name.strip():
            raise InvalidInputError("group_name is required")
        extension = export_format.value.lower()
        filename = f"report.{extension}"
        record = await self._exports.create(
            actor_user_id, group_id, export_format, filename
        )
        processing = await self._exports.mark_processing(record.id)
        if processing is None:
            raise ConflictError("export cannot start")
        object_key = f"exports/{group_id}/{record.id}/{filename}"
        try:
            renderer = {
                ExportFormat.CSV: render_csv,
                ExportFormat.PDF: render_pdf,
                ExportFormat.XLSX: render_xlsx,
            }[export_format]
            content = await asyncio.to_thread(renderer, group_name, rows)
            content_type = {
                ExportFormat.CSV: "text/csv; charset=utf-8",
                ExportFormat.PDF: "application/pdf",
                ExportFormat.XLSX: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
            }[export_format]
            await self._storage.put_export(object_key, content, content_type)
            ready = await self._exports.mark_ready(record.id, object_key)
            if ready is None:
                raise ConflictError("export cannot be completed")
            return ready
        except Exception:
            with suppress(Exception):
                await self._storage.delete_export(object_key)
            with suppress(Exception):
                await self._exports.mark_failed(record.id, "render_failed")
            raise

    async def get_export(self, actor_user_id: int, export_id: int) -> ExportRecord:
        if actor_user_id <= 0 or export_id <= 0:
            raise InvalidInputError("actor_user_id and export_id must be positive")
        record = await self._exports.get(export_id, actor_user_id)
        if record is None:
            raise NotFoundError("export not found")
        return record

    async def download_export(
        self, actor_user_id: int, export_id: int
    ) -> AsyncIterator[bytes]:
        record = await self.get_export(actor_user_id, export_id)
        if record.status is not ExportStatus.READY or record.object_key is None:
            raise ConflictError("export is not ready")
        async for chunk in self._storage.stream_export(record.object_key):
            yield chunk

    async def delete_receipt(self, actor_user_id: int, receipt_id: int) -> None:
        if actor_user_id <= 0 or receipt_id <= 0:
            raise InvalidInputError("actor_user_id and receipt_id must be positive")
        receipt = await self._receipts.soft_delete(receipt_id, actor_user_id)
        if receipt is None:
            raise NotFoundError("receipt not found")
        await self._storage.delete_receipt(receipt.object_key)
