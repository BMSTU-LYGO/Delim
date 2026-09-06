from datetime import datetime
import unittest

from delim_document.qr.fiscal import parse_fiscal_qr, parse_money_minor


class FiscalQRTest(unittest.TestCase):
    def test_parses_typical_payload_without_float_money(self) -> None:
        result = parse_fiscal_qr(
            "t=20250314T1930&s=1234.56&fn=9282440300123456&"
            "i=45678&fp=1234567890&n=1"
        )

        self.assertIsNotNone(result)
        assert result is not None
        self.assertEqual(result.timestamp, datetime(2025, 3, 14, 19, 30))
        self.assertEqual(result.total_minor, 123456)
        self.assertEqual(result.fiscal_document_number, "45678")
        self.assertEqual(result.operation_type, 1)

    def test_invalid_fields_are_optional(self) -> None:
        result = parse_fiscal_qr("t=bad&s=12.345&fn=123")

        self.assertIsNotNone(result)
        assert result is not None
        self.assertIsNone(result.timestamp)
        self.assertIsNone(result.total_minor)
        self.assertEqual(result.fiscal_drive_number, "123")

    def test_rejects_non_fiscal_payload(self) -> None:
        self.assertIsNone(parse_fiscal_qr("https://example.com"))
        self.assertEqual(parse_money_minor("0,01"), 1)


if __name__ == "__main__":
    unittest.main()
