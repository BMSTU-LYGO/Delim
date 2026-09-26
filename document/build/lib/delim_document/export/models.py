"""Normalized export input supplied by Gateway/Core."""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from enum import StrEnum


class ExportFormat(StrEnum):
    CSV = "CSV"
    PDF = "PDF"
    XLSX = "XLSX"


class ExportStatus(StrEnum):
    PENDING = "pending"
    PROCESSING = "processing"
    READY = "ready"
    FAILED = "failed"


@dataclass(frozen=True, slots=True)
class ReportRow:
    date: datetime
    description: str
    payer: str
    amount_minor: int
    currency: str
    note: str


@dataclass(frozen=True, slots=True)
class ExportRecord:
    id: int
    actor_user_id: int
    group_id: int
    format: ExportFormat
    status: ExportStatus
    object_key: str | None
    filename: str
    error_code: str | None
    created_at: datetime
    finished_at: datetime | None
    object_purged_at: datetime | None = None


def printable_money(amount_minor: int) -> str:
    sign = "-" if amount_minor < 0 else ""
    absolute = abs(amount_minor)
    return f"{sign}{absolute // 100}.{absolute % 100:02d}"
