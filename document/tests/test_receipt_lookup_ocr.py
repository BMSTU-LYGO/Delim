from datetime import datetime
from decimal import Decimal
from pathlib import Path
import unittest

from delim_document.domain.job import DocumentJob, DocumentJobStatus, DocumentJobType
from delim_document.domain.receipt import Receipt, ReceiptStatus
from delim_document.qr.fiscal import parse_fiscal_qr
from delim_document.receipt_lookup import (
    ReceiptItem,
    ReceiptLookupError,
    ReceiptLookupResult,
)
from delim_document.worker.ocr import OCRWorker


QR_RAW = "t=20260923T1949&s=259.98&fn=7384440900957540&i=61373&fp=2016648272&n=1"
TESTDATA = Path(__file__).resolve().parents[1] / "testdata"


class _Lookup:
    def __init__(self, result=None, error: Exception | None = None) -> None:
        self.result = result
        self.error = error
        self.calls = 0

    async def lookup(self, image_bytes: bytes):
        self.calls += 1
        if self.error is not None:
            raise self.error
        return self.result


class _Provider:
    def __init__(self) -> None:
        self.calls = 0

    async def recognize(self, _image):
        self.calls += 1
        raise AssertionError("local OCR must not run after a successful lookup")


class _Storage:
    def __init__(self, content: bytes) -> None:
        self.content = content

    async def get_receipt(self, _object_key: str) -> bytes:
        return self.content


class _Receipts:
    def __init__(self, receipt: Receipt) -> None:
        self.receipt = receipt
        self.statuses: list[ReceiptStatus] = []

    async def get_for_processing(self, _receipt_id: int) -> Receipt:
        return self.receipt

    async def mark_status(self, _receipt_id: int, status: ReceiptStatus) -> None:
        self.statuses.append(status)


class _Results:
    def __init__(self) -> None:
        self.result = None

    async def replace_result(self, _receipt_id: int, _actor_id: int, result) -> bool:
        self.result = result
        return True


class _Jobs:
    def __init__(self) -> None:
        self.completed: list[int] = []

    async def mark_completed(self, job_id: int) -> None:
        self.completed.append(job_id)


def _worker(lookup: _Lookup) -> OCRWorker:
    return OCRWorker(
        None,  # type: ignore[arg-type]
        None,  # type: ignore[arg-type]
        None,  # type: ignore[arg-type]
        None,  # type: ignore[arg-type]
        None,  # type: ignore[arg-type]
        3,
        2,
        receipt_lookup=lookup,  # type: ignore[arg-type]
    )


class ReceiptLookupOCRTest(unittest.IsolatedAsyncioTestCase):
    async def test_real_qr_fixture_uses_lookup_and_bypasses_local_ocr(self) -> None:
        now = datetime.now()
        receipt = Receipt(
            id=7,
            actor_user_id=11,
            group_id=13,
            filename="proverkacheka_qr.png",
            content_type="image/png",
            size_bytes=1,
            object_key="receipts/7.png",
            status=ReceiptStatus.QUEUED,
            created_at=now,
            updated_at=now,
        )
        job = DocumentJob(
            id=17,
            receipt_id=receipt.id,
            type=DocumentJobType.OCR,
            status=DocumentJobStatus.PROCESSING,
            attempts=1,
            error_code=None,
            error_message=None,
            created_at=now,
            next_attempt_at=now,
            updated_at=now,
        )
        lookup = _Lookup(
            ReceiptLookupResult(
                total_minor=25998,
                seller_name="Магазин",
                seller_inn=None,
                retail_place_address=None,
                ticket_date="2026-09-23T19:49:00",
                items=(ReceiptItem("Товар", 25998, Decimal("1"), 25998),),
            )
        )
        provider = _Provider()
        receipts = _Receipts(receipt)
        results = _Results()
        jobs = _Jobs()
        worker = OCRWorker(
            jobs,  # type: ignore[arg-type]
            receipts,  # type: ignore[arg-type]
            results,  # type: ignore[arg-type]
            _Storage((TESTDATA / "proverkacheka_qr.png").read_bytes()),  # type: ignore[arg-type]
            provider,  # type: ignore[arg-type]
            3,
            2,
            receipt_lookup=lookup,  # type: ignore[arg-type]
        )

        await worker._process(job)

        self.assertEqual(lookup.calls, 1)
        self.assertEqual(provider.calls, 0)
        self.assertIsNotNone(results.result)
        self.assertEqual(results.result.qr_raw, QR_RAW)
        self.assertEqual(receipts.statuses, [ReceiptStatus.PROCESSING, ReceiptStatus.READY])
        self.assertEqual(jobs.completed, [job.id])

    async def test_verified_positions_become_ocr_result(self) -> None:
        lookup = _Lookup(
            ReceiptLookupResult(
                total_minor=25998,
                seller_name="Магазин",
                seller_inn="123",
                retail_place_address="Москва",
                ticket_date="2026-09-23T19:49:00",
                items=(
                    ReceiptItem("Товар", 12999, Decimal("2"), 25998),
                ),
            )
        )
        result = await _worker(lookup)._lookup_receipt(
            b"png", parse_fiscal_qr(QR_RAW), QR_RAW
        )

        self.assertIsNotNone(result)
        assert result is not None
        self.assertEqual(result.total_minor, 25998)
        self.assertEqual(result.merchant, "Магазин")
        self.assertEqual(result.items[0].quantity, Decimal("2"))
        self.assertEqual(result.items[0].unit_price_minor, 12999)
        self.assertEqual(result.qr_raw, QR_RAW)
        self.assertFalse(result.total_mismatch)

    async def test_lookup_failure_falls_back_to_local_ocr(self) -> None:
        lookup = _Lookup(error=ReceiptLookupError("unavailable"))
        result = await _worker(lookup)._lookup_receipt(
            b"png", parse_fiscal_qr(QR_RAW), QR_RAW
        )

        self.assertIsNone(result)
        self.assertEqual(lookup.calls, 1)

    async def test_incomplete_qr_does_not_send_image_to_third_party(self) -> None:
        lookup = _Lookup()
        result = await _worker(lookup)._lookup_receipt(
            b"png", parse_fiscal_qr("s=259.98"), "s=259.98"
        )

        self.assertIsNone(result)
        self.assertEqual(lookup.calls, 0)


if __name__ == "__main__":
    unittest.main()
