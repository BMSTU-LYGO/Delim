"""Generate small synthetic (non-personal) OCR regression fixtures.

Produces PNG receipt images plus a JSON manifest consumed by
``document/tools/ocr_check.py``. All content is invented; no real personal
receipts are used. Run with Pillow + qrcode available:

    python document/tools/gen_ocr_fixtures.py
"""

from __future__ import annotations

from datetime import datetime
import json
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont

TESTDATA = Path(__file__).resolve().parents[1] / "testdata"

# Fiscal QR payloads accepted by delim_document.qr.fiscal. s= is the
# authoritative total. The mismatch fixture's QR total (42.50) intentionally
# differs from its printed OCR total (99.90).
GOOD_QR_PAYLOAD = "t=20250102T030400&s=42.50&fn=9250440300&i=51&fp=1a2b&type=1"
MISMATCH_QR_PAYLOAD = "t=20250102T030400&s=42.50&fn=9250440300&i=52&fp=2b3c&type=1"

MERCHANT = "МАГАЗИОН РУ"
FOOTER = "СПАСИБО ЗА ПОКУПКУ"
FONT_CANDIDATES = (
    "/System/Library/Fonts/Supplemental/Arial.ttf",
    "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
    "/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
)


def _font(size: int):
    for candidate in FONT_CANDIDATES:
        if Path(candidate).exists():
            try:
                return ImageFont.truetype(candidate, size)
            except OSError:
                continue
    return ImageFont.load_default()


def _receipt_lines(total: str = "42.50") -> list[str]:
    # Real items mixed with service lines that must never become items
    # (КАССОВЫЙ ЧЕК, НДС, ФН, ИТОГО, БЕЗНАЛИЧНЫМИ).
    return [
        "КАССОВЫЙ ЧЕК",
        "2025-01-02 03:04",
        "Хлеб белый 1x25.00 25.00",
        "Молоко 3.2% 1x17.50 17.50",
        "НДС 10% 3.86",
        f"ИТОГО {total}",
        f"БЕЗНАЛИЧНЫМИ {total}",
        "ФН 9250440300",
    ]


def _stub_lines(text_lines: list[str], confidence: float) -> list[dict]:
    # Synthetic bounding boxes with stable vertical ordering; the check only
    # needs geometry that the deterministic parser can sort.
    return [
        {"text": text, "confidence": confidence, "y": 100 + index * 34}
        for index, text in enumerate([MERCHANT, *text_lines, FOOTER])
    ]


def _render(text_lines: list[str], *, width: int = 620, height: int = 820,
            background: int = 255, text_color: int = 0, rotate: int = 0) -> Image.Image:
    image = Image.new("L", (width, height), background)
    draw = ImageDraw.Draw(image)
    font = _font(26)
    small = _font(20)
    y = 40
    draw.text((40, y), MERCHANT, font=_font(34), fill=text_color)
    y += 60
    for line in text_lines:
        draw.text((40, y), line, font=font, fill=text_color)
        y += 34
    draw.text((40, height - 60), FOOTER, font=small, fill=text_color)
    if rotate:
        image = image.rotate(rotate, expand=True, fillcolor=background)
    return image


def _paste_qr(image: Image.Image, payload: str) -> Image.Image:
    import qrcode  # only needed to (re)generate fixtures, not to run the check

    qr = qrcode.QRCode(box_size=6, border=4)
    qr.add_data(payload)
    qr.make(fit=True)
    module = qr.make_image().convert("L")
    canvas = image.convert("L").copy()
    canvas.paste(module, (canvas.width - module.width - 40, canvas.height - module.height - 40))
    return canvas


def main() -> None:
    TESTDATA.mkdir(parents=True, exist_ok=True)
    confidence = 0.93
    fixtures: dict[str, dict] = {}

    def record(name: str, lines: list[str], *, qr: str | None, rotate: int = 0,
               background: int = 255, text_color: int = 0,
               expected_total_minor: int, expect_mismatch: bool) -> None:
        image = _render(lines, rotate=rotate, background=background, text_color=text_color)
        if qr:
            image = _paste_qr(image, qr)
        image.save(TESTDATA / name)
        entry: dict = {
            "expect_qr": bool(qr),
            "stub_lines": _stub_lines(lines, confidence),
            "expected_total_minor": expected_total_minor,
            "expect_mismatch": expect_mismatch,
        }
        if qr:
            entry["qr_payload"] = qr
        fixtures[name] = entry

    base = _receipt_lines()
    record("good_ru.png", base, qr=None, expected_total_minor=4250, expect_mismatch=False)
    record("rotated.png", base, qr=None, rotate=90, expected_total_minor=4250, expect_mismatch=False)
    record("low_contrast.png", base, qr=None, background=220, text_color=140,
           expected_total_minor=4250, expect_mismatch=False)
    record("no_qr.png", base, qr=None, expected_total_minor=4250, expect_mismatch=False)
    record("good_qr.png", base, qr=GOOD_QR_PAYLOAD, expected_total_minor=4250, expect_mismatch=False)
    # QR total 4250 wins over the printed 9990 and flags the mismatch.
    record("qr_mismatch.png", _receipt_lines(total="99.90"), qr=MISMATCH_QR_PAYLOAD,
           expected_total_minor=4250, expect_mismatch=True)

    manifest = {
        "expected_date": datetime(2025, 1, 2, 3, 4).strftime("%Y-%m-%dT%H:%M:%S"),
        "total_minor": 4250,
        "mismatch_total_minor": 9990,
        "stub_line_confidence": confidence,
        "forbidden_item_substrings": [
            "КАССОВЫЙ", "НДС", "ФН", "ИТОГО", "БЕЗНАЛИЧНЫМИ", "СПАСИБО", "МАГАЗИОН",
        ],
        "fixtures": fixtures,
    }
    (TESTDATA / "manifest.json").write_text(
        json.dumps(manifest, ensure_ascii=False, indent=2), encoding="utf-8"
    )
    print(f"fixtures written to {TESTDATA}")


if __name__ == "__main__":
    main()
