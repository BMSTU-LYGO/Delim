"""Parser for Russian fiscal receipt QR payloads."""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from decimal import Decimal, InvalidOperation
from urllib.parse import parse_qs


@dataclass(frozen=True, slots=True)
class FiscalReceiptQR:
    timestamp: datetime | None
    total_minor: int | None
    fiscal_drive_number: str | None
    fiscal_document_number: str | None
    fiscal_sign: str | None
    operation_type: int | None


def parse_money_minor(value: str) -> int | None:
    try:
        amount = Decimal(value.replace(",", "."))
    except InvalidOperation:
        return None
    minor = amount * 100
    if amount < 0 or minor != minor.to_integral_value():
        return None
    return int(minor)


def _parse_timestamp(value: str | None) -> datetime | None:
    if not value:
        return None
    pattern = {13: "%Y%m%dT%H%M", 15: "%Y%m%dT%H%M%S"}.get(len(value))
    if pattern is None:
        return None
    try:
        return datetime.strptime(value, pattern)
    except ValueError:
        return None


def _parse_operation(value: str | None) -> int | None:
    if value is None:
        return None
    try:
        operation = int(value)
    except ValueError:
        return None
    return operation if operation > 0 else None


def parse_fiscal_qr(payload: str) -> FiscalReceiptQR | None:
    values = parse_qs(payload.strip().lstrip("?"), keep_blank_values=False)
    recognized = {"t", "s", "fn", "i", "fp", "n"}.intersection(values)
    if not recognized:
        return None

    def first(key: str) -> str | None:
        entries = values.get(key)
        return entries[0].strip() if entries and entries[0].strip() else None

    total = first("s")
    return FiscalReceiptQR(
        timestamp=_parse_timestamp(first("t")),
        total_minor=parse_money_minor(total) if total is not None else None,
        fiscal_drive_number=first("fn"),
        fiscal_document_number=first("i"),
        fiscal_sign=first("fp"),
        operation_type=_parse_operation(first("n")),
    )
