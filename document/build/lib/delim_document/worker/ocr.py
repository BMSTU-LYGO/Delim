"""Background OCR job processing."""

from __future__ import annotations

import asyncio
from datetime import datetime
import time
import asyncpg
from minio.error import S3Error
import numpy as np
from urllib3.exceptions import HTTPError

from delim_document.domain.job import DocumentJob
from delim_document.domain.receipt import ReceiptStatus
from delim_document.image.decoder import ImageDecodeError, decode_image
from delim_document.image.preprocess import preprocess_receipt
from delim_document.metrics import Recorder, noop_recorder
from delim_document.ocr.confidence import build_ocr_result
from delim_document.ocr.provider import OCRItem, OCRLine, OCRProvider, OCRResult
from delim_document.qr.fiscal import FiscalReceiptQR, parse_fiscal_qr
from delim_document.qr.reader import ReceiptQRReader
from delim_document.receipt_lookup import (
    ProverkaChekaClient,
    ReceiptLookupError,
    ReceiptLookupResult,
)
from delim_document.repository.job import JobRepository
from delim_document.repository.ocr_result import OCRResultRepository
from delim_document.repository.receipt import ReceiptRepository
from delim_document.storage.minio import MinioStorage


class OCRWorker:
    def __init__(
        self,
        jobs: JobRepository,
        receipts: ReceiptRepository,
        results: OCRResultRepository,
        storage: MinioStorage,
        provider: OCRProvider,
        max_attempts: int,
        retry_base_seconds: int,
        recorder: Recorder | None = None,
        *,
        receipt_lookup: ProverkaChekaClient | None = None,
    ) -> None:
        self._jobs = jobs
        self._receipts = receipts
        self._results = results
        self._storage = storage
        self._provider = provider
        self._receipt_lookup = receipt_lookup
        self._qr_reader = ReceiptQRReader()
        self._max_attempts = max_attempts
        self._retry_base_seconds = retry_base_seconds
        self._recorder = recorder or noop_recorder()

    async def run(self, stop_event: asyncio.Event, poll_interval_ms: int) -> None:
        poll_seconds = poll_interval_ms / 1000
        while not stop_event.is_set():
            processed = await self.run_once()
            if processed:
                continue
            try:
                await asyncio.wait_for(stop_event.wait(), timeout=poll_seconds)
            except TimeoutError:
                pass

    async def run_once(self) -> bool:
        job = await self._jobs.claim_pending(self._max_attempts)
        if job is None:
            return False
        self._recorder.observe_ocr_job("processing")
        started = time.monotonic()
        result = "failed"
        try:
            await self._process(job)
            result = "completed"
        except ImageDecodeError:
            await self._fail(job, "invalid_image", "receipt image is invalid")
        except (asyncpg.PostgresError, S3Error, HTTPError, OSError):
            await self._retry_or_fail(
                job, "dependency_unavailable", "processing dependency is unavailable"
            )
        except OCRRuntimeError:
            await self._retry_or_fail(job, "ocr_runtime", "OCR processing failed")
        finally:
            duration = max(0.0, time.monotonic() - started)
            self._recorder.observe_ocr_duration(result, duration)
        self._recorder.observe_ocr_job(result)
        return True

    async def _fail(self, job: DocumentJob, code: str, message: str) -> None:
        await self._jobs.mark_failed(job.id, code, message)
        await self._receipts.mark_status(job.receipt_id, ReceiptStatus.FAILED)
        self._recorder.observe_ocr_job("failed")

    async def _retry_or_fail(
        self, job: DocumentJob, code: str, message: str
    ) -> None:
        if job.attempts >= self._max_attempts:
            await self._fail(job, code, message)
            return
        delay = self._retry_base_seconds * (2 ** max(0, job.attempts - 1))
        await self._jobs.schedule_retry(job.id, delay, code, message)
        await self._receipts.mark_status(job.receipt_id, ReceiptStatus.QUEUED)
        self._recorder.observe_ocr_job("pending")

    async def _recognize(self, image: np.ndarray) -> tuple[OCRLine, ...]:
        try:
            return await self._provider.recognize(image)
        except Exception as exc:
            raise OCRRuntimeError from exc

    async def _process(self, job: DocumentJob) -> None:
        receipt = await self._receipts.get_for_processing(job.receipt_id)
        if receipt is None:
            raise ValueError("receipt is unavailable for processing")
        await self._receipts.mark_status(receipt.id, ReceiptStatus.PROCESSING)

        image_bytes = await self._storage.get_receipt(receipt.object_key)
        original = decode_image(image_bytes)
        processed = preprocess_receipt(original)
        qr = self._qr_reader.read(
            original, processed.enhanced, processed.grayscale
        )
        fiscal_qr = parse_fiscal_qr(qr.raw_payload) if qr.raw_payload else None

        lookup_result = await self._lookup_receipt(
            image_bytes, fiscal_qr, qr.raw_payload
        )
        if lookup_result is not None:
            saved = await self._results.replace_result(
                receipt.id, receipt.actor_user_id, lookup_result
            )
            if not saved:
                raise ValueError("receipt is unavailable for result persistence")
            await self._receipts.mark_status(receipt.id, ReceiptStatus.READY)
            await self._jobs.mark_completed(job.id)
            return

        candidates: list[OCRResult] = []
        for image in (processed.normal, processed.enhanced, processed.binarized):
            lines = await self._recognize(image)
            candidates.append(build_ocr_result(lines, fiscal_qr, qr.raw_payload))
        result = max(candidates, key=lambda candidate: candidate.confidence)

        saved = await self._results.replace_result(
            receipt.id, receipt.actor_user_id, result
        )
        if not saved:
            raise ValueError("receipt is unavailable for result persistence")
        await self._receipts.mark_status(receipt.id, ReceiptStatus.READY)
        await self._jobs.mark_completed(job.id)

    async def _lookup_receipt(
        self,
        image_bytes: bytes,
        fiscal_qr: FiscalReceiptQR | None,
        qr_raw: str | None,
    ) -> OCRResult | None:
        if self._receipt_lookup is None or not _complete_fiscal_qr(fiscal_qr):
            return None
        try:
            receipt = await self._receipt_lookup.lookup(image_bytes)
        except ReceiptLookupError:
            return None
        return _lookup_to_ocr_result(receipt, fiscal_qr, qr_raw)


class OCRRuntimeError(RuntimeError):
    """A temporary PaddleOCR inference failure."""


def _complete_fiscal_qr(fiscal_qr: FiscalReceiptQR | None) -> bool:
    return bool(
        fiscal_qr
        and fiscal_qr.fiscal_drive_number
        and fiscal_qr.fiscal_document_number
        and fiscal_qr.fiscal_sign
    )


def _lookup_to_ocr_result(
    receipt: ReceiptLookupResult,
    fiscal_qr: FiscalReceiptQR | None,
    qr_raw: str | None,
) -> OCRResult:
    receipt_date = _parse_lookup_date(receipt.ticket_date)
    if receipt_date is None and fiscal_qr is not None:
        receipt_date = fiscal_qr.timestamp
    total_minor = receipt.total_minor
    if total_minor is None and fiscal_qr is not None:
        total_minor = fiscal_qr.total_minor
    qr_total = fiscal_qr.total_minor if fiscal_qr is not None else None
    total_mismatch = (
        total_minor is not None and qr_total is not None and total_minor != qr_total
    )
    items = tuple(
        OCRItem(
            name=item.name,
            quantity=item.quantity,
            unit_price_minor=item.price_minor,
            amount_minor=item.total_minor,
            confidence=1.0,
        )
        for item in receipt.items
    )
    return OCRResult(
        merchant=receipt.seller_name,
        merchant_confidence=1.0 if receipt.seller_name else 0.0,
        date=receipt_date,
        date_confidence=1.0 if receipt_date else 0.0,
        total_minor=total_minor,
        total_confidence=1.0 if total_minor is not None else 0.0,
        currency="RUB",
        items=items,
        confidence=0.99 if not total_mismatch else 0.85,
        raw_text="",
        raw_lines=(),
        qr_raw=qr_raw,
        total_mismatch=total_mismatch,
    )


def _parse_lookup_date(value: str | None) -> datetime | None:
    if not value:
        return None
    normalized = value.strip().replace("Z", "+00:00")
    try:
        return datetime.fromisoformat(normalized)
    except ValueError:
        return None
