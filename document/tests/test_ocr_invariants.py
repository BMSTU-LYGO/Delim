from datetime import datetime
import unittest

from delim_document.ocr.confidence import build_ocr_result
from delim_document.ocr.provider import OCRLine
from delim_document.qr.fiscal import parse_fiscal_qr


def line(text: str, top: float) -> OCRLine:
    return OCRLine(
        text=text,
        confidence=0.9,
        bbox=((0, top), (200, top), (200, top + 10), (0, top + 10)),
    )


class OCRInvariantTest(unittest.TestCase):
    def test_empty_ocr_without_qr_is_valid(self) -> None:
        result = build_ocr_result((), None, None)
        self.assertEqual(result.items, ())
        self.assertIsNone(result.total_minor)
        self.assertIsNone(result.date)
        self.assertEqual(result.confidence, 0.0)

    def test_money_and_item_order_are_deterministic(self) -> None:
        lines = (
            line("Хлеб 50.00", 10),
            line("Молоко 70.00", 20),
            line("ИТОГ 120.00", 30),
        )
        first = build_ocr_result(lines, None, None)
        second = build_ocr_result(tuple(reversed(lines)), None, None)
        self.assertEqual([item.name for item in first.items], ["Хлеб", "Молоко"])
        self.assertEqual(first.items, second.items)
        self.assertIsInstance(first.total_minor, int)
        self.assertTrue(all(isinstance(item.amount_minor, int) for item in first.items))
        self.assertTrue(all(0.0 <= item.confidence <= 1.0 for item in first.items))
        self.assertTrue(0.0 <= first.confidence <= 1.0)

    def test_receipt_without_items_does_not_fail(self) -> None:
        result = build_ocr_result((line("ИТОГ 10.00", 10),), None, None)
        self.assertEqual(result.total_minor, 1000)
        self.assertEqual(result.items, ())

    def test_qr_only_supplies_primary_total_and_date(self) -> None:
        qr = parse_fiscal_qr("t=20250102T0304&s=42.50&fn=1&i=2&fp=3&n=1")
        result = build_ocr_result((), qr, "t=20250102T0304&s=42.50")
        self.assertEqual(result.total_minor, 4250)
        self.assertEqual(result.date, datetime(2025, 1, 2, 3, 4))
        self.assertEqual(result.currency, "RUB")
        self.assertTrue(0.0 <= result.confidence <= 1.0)


if __name__ == "__main__":
    unittest.main()
