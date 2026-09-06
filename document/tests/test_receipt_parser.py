from datetime import datetime
from decimal import Decimal
import unittest

from delim_document.ocr.parser import (
    extract_merchant,
    extract_receipt_date,
    extract_receipt_items,
    extract_receipt_total,
    normalize_ocr_lines,
)
from delim_document.ocr.provider import OCRLine
from delim_document.qr.fiscal import parse_fiscal_qr, parse_money_minor


def line(text: str, confidence: float = 0.9, top: float = 0) -> OCRLine:
    return OCRLine(
        text=text,
        confidence=confidence,
        bbox=((0, top), (200, top), (200, top + 10), (0, top + 10)),
    )


class ReceiptParserTest(unittest.TestCase):
    def test_money_is_parsed_to_integer_minor_units(self) -> None:
        value = parse_money_minor("1234,56")
        self.assertEqual(value, 123456)
        self.assertIsInstance(value, int)

    def test_extracts_cyrillic_merchant_and_ignores_headers(self) -> None:
        lines = normalize_ocr_lines(
            (line("КАССОВЫЙ ЧЕК", top=0), line("ООО РОМАШКА", top=12), line("ИНН 1", top=24))
        )
        merchant = extract_merchant(lines)
        self.assertIsNotNone(merchant)
        assert merchant is not None
        self.assertEqual(merchant.value, "ООО РОМАШКА")

    def test_extracts_supported_date_formats(self) -> None:
        cases = (
            ("Дата 31.12.2025 23:59", datetime(2025, 12, 31, 23, 59)),
            ("Дата 31-12-2025", datetime(2025, 12, 31)),
            ("Дата 2025-12-31", datetime(2025, 12, 31)),
        )
        for text, expected in cases:
            with self.subTest(text=text):
                result = extract_receipt_date(normalize_ocr_lines((line(text),)))
                self.assertIsNotNone(result)
                assert result is not None
                self.assertEqual(result.value, expected)

    def test_detects_total_and_qr_mismatch(self) -> None:
        lines = normalize_ocr_lines((line("ИТОГО 100.00"),))
        result = extract_receipt_total(lines, parse_fiscal_qr("s=120.00"))
        self.assertIsNotNone(result)
        assert result is not None
        self.assertEqual(result.value, 12000)
        self.assertTrue(result.mismatch)
        self.assertEqual(result.source, "qr")

    def test_extracts_items_and_filters_service_lines(self) -> None:
        lines = normalize_ocr_lines(
            (
                line("Молоко 2 x 50.00 100.00", top=0),
                line("ФН 12345 1.00", top=12),
                line("ИТОГ 100.00", top=24),
            )
        )
        items = extract_receipt_items(lines)
        self.assertEqual(len(items), 1)
        self.assertEqual(items[0].name, "Молоко")
        self.assertEqual(items[0].quantity, Decimal("2"))
        self.assertEqual(items[0].unit_price_minor, 5000)
        self.assertEqual(items[0].amount_minor, 10000)


if __name__ == "__main__":
    unittest.main()
