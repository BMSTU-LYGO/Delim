from datetime import UTC, datetime
from io import BytesIO

from openpyxl import load_workbook

from delim_document.export.models import ReportRow
from delim_document.export.xlsx import render_xlsx


def test_render_xlsx_accepts_timezone_aware_dates() -> None:
    content = render_xlsx(
        "Поездка",
        (
            ReportRow(
                date=datetime(2026, 9, 6, 12, 30, tzinfo=UTC),
                description="Обед",
                payer="Анна",
                amount_minor=12345,
                currency="RUB",
                note="подтверждено",
            ),
        ),
    )

    workbook = load_workbook(filename=BytesIO(content))
    sheet = workbook["Операции"]
    assert sheet["A4"].value == datetime(2026, 9, 6, 12, 30)
    assert sheet["B4"].value == "Обед"
