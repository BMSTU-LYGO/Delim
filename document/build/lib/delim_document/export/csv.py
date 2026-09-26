"""UTF-8 CSV report renderer."""

from __future__ import annotations

import csv
from io import StringIO

from delim_document.export.models import ReportRow, printable_money


HEADERS = ("Дата", "Описание", "Плательщик", "Сумма", "Валюта", "Примечание")


def render_csv(group_name: str, rows: tuple[ReportRow, ...]) -> bytes:
    output = StringIO(newline="")
    writer = csv.writer(output)
    writer.writerow(("Группа", group_name))
    writer.writerow(())
    writer.writerow(HEADERS)
    for row in rows:
        writer.writerow(
            (
                row.date.isoformat(sep=" ", timespec="seconds"),
                row.description,
                row.payer,
                printable_money(row.amount_minor),
                row.currency,
                row.note,
            )
        )
    return output.getvalue().encode("utf-8")
