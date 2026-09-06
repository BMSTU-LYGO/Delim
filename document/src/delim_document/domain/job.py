"""Document job domain model."""

from dataclasses import dataclass
from datetime import datetime
from enum import StrEnum


class DocumentJobType(StrEnum):
    OCR = "OCR"


class DocumentJobStatus(StrEnum):
    PENDING = "pending"
    PROCESSING = "processing"
    COMPLETED = "completed"
    FAILED = "failed"


@dataclass(frozen=True, slots=True)
class DocumentJob:
    id: int
    receipt_id: int
    type: DocumentJobType
    status: DocumentJobStatus
    attempts: int
    error_code: str | None
    error_message: str | None
    created_at: datetime
    next_attempt_at: datetime
    updated_at: datetime
    started_at: datetime | None = None
    finished_at: datetime | None = None
