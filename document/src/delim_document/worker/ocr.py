"""Background OCR job processing."""

from __future__ import annotations

from delim_document.domain.job import DocumentJob
from delim_document.domain.receipt import ReceiptStatus
from delim_document.image.decoder import decode_image
from delim_document.image.preprocess import preprocess_receipt
from delim_document.ocr.confidence import build_ocr_result
from delim_document.ocr.provider import OCRProvider, OCRResult
from delim_document.qr.fiscal import parse_fiscal_qr
from delim_document.qr.reader import ReceiptQRReader
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
    ) -> None:
        self._jobs = jobs
        self._receipts = receipts
        self._results = results
        self._storage = storage
        self._provider = provider
        self._qr_reader = ReceiptQRReader()
        self._max_attempts = max_attempts

    async def run_once(self) -> bool:
        job = await self._jobs.claim_pending(self._max_attempts)
        if job is None:
            return False
        await self._process(job)
        return True

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

        candidates: list[OCRResult] = []
        for image in (processed.normal, processed.enhanced, processed.binarized):
            lines = await self._provider.recognize(image)
            candidates.append(build_ocr_result(lines, fiscal_qr, qr.raw_payload))
        result = max(candidates, key=lambda candidate: candidate.confidence)

        saved = await self._results.replace_result(
            receipt.id, receipt.actor_user_id, result
        )
        if not saved:
            raise ValueError("receipt is unavailable for result persistence")
        await self._receipts.mark_status(receipt.id, ReceiptStatus.READY)
        await self._jobs.mark_completed(job.id)
