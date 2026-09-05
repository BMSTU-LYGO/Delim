"""Document service use cases."""

from __future__ import annotations

from contextlib import suppress
from dataclasses import dataclass
from uuid import uuid4

from delim_document.config import UploadConfig
from delim_document.domain.job import DocumentJob
from delim_document.domain.receipt import Receipt, ReceiptStatus
from delim_document.repository.job import JobRepository
from delim_document.repository.receipt import ReceiptRepository
from delim_document.storage.minio import MinioStorage


ALLOWED_CONTENT_TYPES = frozenset({"image/jpeg", "image/png", "image/webp"})


class InvalidInputError(ValueError):
    """Raised when receipt input is not valid."""


@dataclass(frozen=True, slots=True)
class CreateReceiptResult:
    receipt: Receipt
    job: DocumentJob


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
        storage: MinioStorage,
        upload_config: UploadConfig,
    ) -> None:
        self._receipts = receipts
        self._jobs = jobs
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
