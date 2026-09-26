"""Receipt domain model."""

from dataclasses import dataclass
from datetime import datetime
from enum import StrEnum


class ReceiptStatus(StrEnum):
    UPLOADED = "uploaded"
    QUEUED = "queued"
    PROCESSING = "processing"
    READY = "ready"
    FAILED = "failed"
    DELETED = "deleted"


@dataclass(frozen=True, slots=True)
class Receipt:
    id: int
    actor_user_id: int
    group_id: int
    filename: str
    content_type: str
    size_bytes: int
    object_key: str
    status: ReceiptStatus
    created_at: datetime
    updated_at: datetime
    deleted_at: datetime | None = None
    original_purged_at: datetime | None = None
