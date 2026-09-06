"""Multipage Unicode PDF report renderer."""

from __future__ import annotations

from datetime import datetime, timezone
from html import escape
from io import BytesIO
from pathlib import Path

from reportlab.lib import colors
from reportlab.lib.pagesizes import A4, landscape
from reportlab.lib.styles import ParagraphStyle
from reportlab.lib.units import mm
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.platypus import Paragraph, SimpleDocTemplate, Spacer, Table, TableStyle

from delim_document.export.csv import HEADERS
from delim_document.export.models import ReportRow, printable_money


DEFAULT_FONT_PATH = Path("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf")
FONT_NAME = "DelimUnicode"


def render_pdf(
    group_name: str,
    rows: tuple[ReportRow, ...],
    font_path: Path = DEFAULT_FONT_PATH,
) -> bytes:
    if not font_path.is_file():
        raise RuntimeError("Unicode PDF font is unavailable")
    if FONT_NAME not in pdfmetrics.getRegisteredFontNames():
        pdfmetrics.registerFont(TTFont(FONT_NAME, str(font_path)))

    output = BytesIO()
    document = SimpleDocTemplate(
        output,
        pagesize=landscape(A4),
        leftMargin=12 * mm,
        rightMargin=12 * mm,
        topMargin=12 * mm,
        bottomMargin=12 * mm,
    )
    text_style = ParagraphStyle("DelimText", fontName=FONT_NAME, fontSize=8, leading=10)
    title_style = ParagraphStyle(
        "DelimTitle", fontName=FONT_NAME, fontSize=14, leading=18
    )
    content: list[object] = [
        Paragraph(f"Группа: {escape(group_name)}", title_style),
        Paragraph(
            "Сформировано: "
            + datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M UTC"),
            text_style,
        ),
        Spacer(1, 6 * mm),
    ]
    table_rows: list[list[Paragraph]] = [
        [Paragraph(header, text_style) for header in HEADERS]
    ]
    for row in rows:
        table_rows.append(
            [
                Paragraph(row.date.isoformat(sep=" ", timespec="seconds"), text_style),
                Paragraph(escape(row.description), text_style),
                Paragraph(escape(row.payer), text_style),
                Paragraph(printable_money(row.amount_minor), text_style),
                Paragraph(escape(row.currency), text_style),
                Paragraph(escape(row.note), text_style),
            ]
        )
    table = Table(
        table_rows,
        repeatRows=1,
        colWidths=(38 * mm, 58 * mm, 42 * mm, 28 * mm, 22 * mm, 70 * mm),
    )
    table.setStyle(
        TableStyle(
            [
                ("FONTNAME", (0, 0), (-1, -1), FONT_NAME),
                ("BACKGROUND", (0, 0), (-1, 0), colors.lightgrey),
                ("GRID", (0, 0), (-1, -1), 0.25, colors.grey),
                ("VALIGN", (0, 0), (-1, -1), "TOP"),
                ("LEFTPADDING", (0, 0), (-1, -1), 3),
                ("RIGHTPADDING", (0, 0), (-1, -1), 3),
            ]
        )
    )
    content.append(table)
    document.build(content)
    return output.getvalue()
