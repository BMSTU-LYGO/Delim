"""Evidence-based confidence calculation for structured OCR results."""

from __future__ import annotations

from statistics import fmean

from delim_document.ocr.parser import (
    extract_merchant,
    extract_receipt_date,
    extract_receipt_items,
    extract_receipt_total,
    normalize_ocr_lines,
)
from delim_document.ocr.provider import OCRLine, OCRResult
from delim_document.qr.fiscal import FiscalReceiptQR


def _clamp(value: float) -> float:
    return max(0.0, min(1.0, value))


def _item_total_consistency(item_total: int, receipt_total: int) -> float:
    if receipt_total == 0:
        return 1.0 if item_total == 0 else 0.0
    difference = abs(item_total - receipt_total)
    return _clamp(1.0 - difference / receipt_total)


def build_ocr_result(
    raw_lines: tuple[OCRLine, ...],
    fiscal_qr: FiscalReceiptQR | None,
    qr_raw: str | None,
) -> OCRResult:
    lines = normalize_ocr_lines(raw_lines)
    merchant = extract_merchant(lines)
    receipt_date = extract_receipt_date(lines, fiscal_qr)
    total = extract_receipt_total(lines, fiscal_qr)
    items = extract_receipt_items(lines)

    line_confidence = fmean(line.confidence for line in lines) if lines else 0.0
    item_confidence = fmean(item.confidence for item in items) if items else 0.0
    consistency = 0.0
    if total is not None and items:
        consistency = _item_total_consistency(
            sum(item.amount_minor for item in items), total.value
        )

    confidence = (
        line_confidence * 0.25
        + (merchant.confidence if merchant else 0.0) * 0.15
        + (receipt_date.confidence if receipt_date else 0.0) * 0.15
        + (total.confidence if total else 0.0) * 0.25
        + item_confidence * 0.10
        + consistency * 0.10
    )
    if fiscal_qr is not None:
        confidence += 0.05
    if total is not None and total.mismatch:
        confidence *= 0.7

    return OCRResult(
        merchant=merchant.value if merchant else None,
        merchant_confidence=merchant.confidence if merchant else 0.0,
        date=receipt_date.value if receipt_date else None,
        date_confidence=receipt_date.confidence if receipt_date else 0.0,
        total_minor=total.value if total else None,
        total_confidence=total.confidence if total else 0.0,
        currency="RUB" if fiscal_qr is not None else None,
        items=items,
        confidence=_clamp(confidence),
        raw_text="\n".join(line.raw_text for line in lines),
        raw_lines=raw_lines,
        qr_raw=qr_raw,
        total_mismatch=total.mismatch if total else False,
    )
