"""Document job repository."""

from __future__ import annotations

from typing import Any

import asyncpg

from delim_document.domain.job import DocumentJob, DocumentJobStatus, DocumentJobType


def _job(record: asyncpg.Record) -> DocumentJob:
    values: dict[str, Any] = dict(record)
    values["type"] = DocumentJobType(values["type"])
    values["status"] = DocumentJobStatus(values["status"])
    return DocumentJob(**values)


_COLUMNS = """
    id, receipt_id, type, status, attempts, error_code, error_message,
    created_at, started_at, finished_at
"""


class JobRepository:
    def __init__(self, pool: asyncpg.Pool) -> None:
        self._pool = pool

    async def create(
        self, receipt_id: int, job_type: DocumentJobType = DocumentJobType.OCR
    ) -> DocumentJob:
        record = await self._pool.fetchrow(
            f"""
            INSERT INTO document_jobs (receipt_id, type, status)
            VALUES ($1, $2, 'pending')
            RETURNING {_COLUMNS}
            """,
            receipt_id,
            job_type.value,
        )
        assert record is not None
        return _job(record)

    async def get(self, job_id: int, actor_user_id: int) -> DocumentJob | None:
        record = await self._pool.fetchrow(
            f"""
            SELECT {', '.join(f'j.{column.strip()}' for column in _COLUMNS.split(','))}
            FROM document_jobs AS j
            JOIN document_receipts AS r ON r.id = j.receipt_id
            WHERE j.id = $1 AND r.actor_user_id = $2 AND r.deleted_at IS NULL
            """,
            job_id,
            actor_user_id,
        )
        return _job(record) if record else None

    async def mark_processing(self, job_id: int) -> DocumentJob | None:
        record = await self._pool.fetchrow(
            f"""
            UPDATE document_jobs
            SET status = 'processing', attempts = attempts + 1,
                started_at = NOW(), finished_at = NULL,
                error_code = NULL, error_message = NULL
            WHERE id = $1
            RETURNING {_COLUMNS}
            """,
            job_id,
        )
        return _job(record) if record else None

    async def mark_completed(self, job_id: int) -> DocumentJob | None:
        record = await self._pool.fetchrow(
            f"""
            UPDATE document_jobs
            SET status = 'completed', finished_at = NOW(),
                error_code = NULL, error_message = NULL
            WHERE id = $1
            RETURNING {_COLUMNS}
            """,
            job_id,
        )
        return _job(record) if record else None

    async def mark_failed(
        self, job_id: int, error_code: str, error_message: str
    ) -> DocumentJob | None:
        record = await self._pool.fetchrow(
            f"""
            UPDATE document_jobs
            SET status = 'failed', error_code = $2, error_message = $3,
                finished_at = NOW()
            WHERE id = $1
            RETURNING {_COLUMNS}
            """,
            job_id,
            error_code,
            error_message,
        )
        return _job(record) if record else None
