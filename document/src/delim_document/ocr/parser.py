"""Deterministic parsing of normalized receipt OCR lines."""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
import re
from typing import Generic, TypeVar

from delim_document.ocr.provider import BBox, OCRLine
from delim_document.qr.fiscal import FiscalReceiptQR, parse_money_minor


_SPACE_RE = re.compile(r"\s+")
_MONEY_TOKEN_RE = re.compile(r"(?<!\w)[0-9OО]+[.,][0-9OО]{2}(?!\w)")
_MERCHANT_EXCLUDED_RE = re.compile(
    r"\b(?:КАССОВЫЙ\s+ЧЕК|ИНН|ФН|ФД|ФП|КАССИР|СМЕНА)\b", re.IGNORECASE
)
_LETTER_RE = re.compile(r"[A-Za-zА-Яа-яЁё]")
_DATE_PATTERNS = (
    (re.compile(r"(?<!\d)(\d{2}\.\d{2}\.\d{4}\s+\d{2}:\d{2})(?!\d)"), "%d.%m.%Y %H:%M"),
    (re.compile(r"(?<!\d)(\d{2}-\d{2}-\d{4})(?!\d)"), "%d-%m-%Y"),
    (re.compile(r"(?<!\d)(\d{4}-\d{2}-\d{2})(?!\d)"), "%Y-%m-%d"),
)
_TOTAL_RE = re.compile(r"\b(?:ИТОГО|ИТОГ|К\s+ОПЛАТЕ|ВСЕГО)\b", re.IGNORECASE)
_AMOUNT_RE = re.compile(r"(?<!\d)(\d+(?:[ \u00a0]\d{3})*[.,]\d{2})(?!\d)")

T = TypeVar("T")


@dataclass(frozen=True, slots=True)
class ExtractedValue(Generic[T]):
    value: T
    confidence: float


@dataclass(frozen=True, slots=True)
class TotalExtraction:
    value: int
    confidence: float
    source: str
    mismatch: bool


@dataclass(frozen=True, slots=True)
class NormalizedLine:
    text: str
    raw_text: str
    confidence: float
    bbox: BBox

    @property
    def left(self) -> float:
        return min((point[0] for point in self.bbox), default=0.0)

    @property
    def top(self) -> float:
        return min((point[1] for point in self.bbox), default=0.0)

    @property
    def right(self) -> float:
        return max((point[0] for point in self.bbox), default=0.0)


def _fix_money_artifacts(text: str) -> str:
    def replace(match: re.Match[str]) -> str:
        return match.group(0).replace("O", "0").replace("О", "0")

    return _MONEY_TOKEN_RE.sub(replace, text)


def normalize_ocr_lines(lines: tuple[OCRLine, ...]) -> tuple[NormalizedLine, ...]:
    normalized: list[tuple[int, NormalizedLine]] = []
    for index, line in enumerate(lines):
        raw = line.text
        text = _fix_money_artifacts(_SPACE_RE.sub(" ", raw.strip()))
        if not text:
            continue
        normalized.append(
            (
                index,
                NormalizedLine(
                    text=text,
                    raw_text=raw,
                    confidence=max(0.0, min(1.0, line.confidence)),
                    bbox=line.bbox,
                ),
            )
        )
    normalized.sort(key=lambda entry: (entry[1].top, entry[1].left, entry[0]))
    return tuple(line for _, line in normalized)


def extract_merchant(
    lines: tuple[NormalizedLine, ...],
) -> ExtractedValue[str] | None:
    if not lines:
        return None
    upper_count = min(12, max(5, (len(lines) + 2) // 3))
    candidates: list[tuple[float, int, NormalizedLine]] = []
    for index, line in enumerate(lines[:upper_count]):
        text = line.text.strip(" -—:;|")
        if (
            len(text) < 2
            or _MERCHANT_EXCLUDED_RE.search(text)
            or not _LETTER_RE.search(text)
        ):
            continue
        letters = sum(character.isalpha() for character in text)
        if letters / len(text) < 0.45:
            continue
        score = line.confidence - index * 0.025
        candidates.append((score, -index, line))
    if not candidates:
        return None
    _, negative_index, selected = max(candidates, key=lambda entry: entry[:2])
    position_factor = 1.0 - min(0.2, (-negative_index) * 0.02)
    return ExtractedValue(
        value=selected.text.strip(" -—:;|"),
        confidence=max(0.0, min(1.0, selected.confidence * position_factor)),
    )


def extract_receipt_date(
    lines: tuple[NormalizedLine, ...],
    fiscal_qr: FiscalReceiptQR | None = None,
) -> ExtractedValue[datetime] | None:
    if fiscal_qr is not None and fiscal_qr.timestamp is not None:
        return ExtractedValue(value=fiscal_qr.timestamp, confidence=0.99)

    for line in lines:
        for expression, pattern in _DATE_PATTERNS:
            match = expression.search(line.text)
            if match is None:
                continue
            try:
                value = datetime.strptime(match.group(1), pattern)
            except ValueError:
                continue
            return ExtractedValue(
                value=value,
                confidence=max(0.0, min(1.0, line.confidence * 0.9)),
            )
    return None


def _extract_ocr_total(
    lines: tuple[NormalizedLine, ...],
) -> ExtractedValue[int] | None:
    candidates: list[tuple[float, int, int]] = []
    for index, line in enumerate(lines):
        if _TOTAL_RE.search(line.text) is None:
            continue
        matches = list(_AMOUNT_RE.finditer(line.text))
        for match in matches:
            amount = parse_money_minor(match.group(1).replace(" ", "").replace("\u00a0", ""))
            if amount is not None:
                candidates.append((line.confidence, index, amount))
    if not candidates:
        return None
    confidence, _, amount = max(candidates, key=lambda value: (value[0], value[1]))
    return ExtractedValue(value=amount, confidence=min(1.0, confidence * 0.95))


def extract_receipt_total(
    lines: tuple[NormalizedLine, ...],
    fiscal_qr: FiscalReceiptQR | None = None,
) -> TotalExtraction | None:
    ocr_total = _extract_ocr_total(lines)
    qr_total = fiscal_qr.total_minor if fiscal_qr is not None else None
    if qr_total is not None:
        mismatch = ocr_total is not None and ocr_total.value != qr_total
        return TotalExtraction(
            value=qr_total,
            confidence=0.75 if mismatch else 0.99,
            source="qr",
            mismatch=mismatch,
        )
    if ocr_total is None:
        return None
    return TotalExtraction(
        value=ocr_total.value,
        confidence=ocr_total.confidence,
        source="ocr",
        mismatch=False,
    )
