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
    created_at, next_attempt_at, updated_at, started_at, finished_at
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
                error_code = NULL, error_message = NULL, updated_at = NOW()
            WHERE id = $1
            RETURNING {_COLUMNS}
            """,
            job_id,
        )
        return _job(record) if record else None

    async def claim_pending(self, max_attempts: int) -> DocumentJob | None:
        record = await self._pool.fetchrow(
            f"""
            WITH candidate AS (
                SELECT j.id
                FROM document_jobs AS j
                JOIN document_receipts AS r ON r.id = j.receipt_id
                WHERE j.status = 'pending'
                  AND j.type = 'OCR'
                  AND j.next_attempt_at <= NOW()
                  AND j.attempts < $1
                  AND r.deleted_at IS NULL
                ORDER BY j.next_attempt_at, j.created_at
                FOR UPDATE OF j SKIP LOCKED
                LIMIT 1
            )
            UPDATE document_jobs AS j
            SET status = 'processing', attempts = j.attempts + 1,
                started_at = NOW(), finished_at = NULL,
                error_code = NULL, error_message = NULL, updated_at = NOW()
            FROM candidate
            WHERE j.id = candidate.id
            RETURNING {', '.join(f'j.{column.strip()}' for column in _COLUMNS.split(','))}
            """,
            max_attempts,
        )
        return _job(record) if record else None

    async def recover_stale(self, stale_after_minutes: int) -> int:
        result = await self._pool.execute(
            """
            UPDATE document_jobs
            SET status = 'pending', next_attempt_at = NOW(),
                started_at = NULL, finished_at = NULL,
                error_code = 'worker_recovered',
                error_message = 'processing interrupted; retry scheduled',
                updated_at = NOW()
            WHERE status = 'processing'
              AND started_at < NOW() - make_interval(mins => $1)
            """,
            stale_after_minutes,
        )
        return int(result.rsplit(" ", 1)[-1])

    async def mark_completed(self, job_id: int) -> DocumentJob | None:
        record = await self._pool.fetchrow(
            f"""
            UPDATE document_jobs
            SET status = 'completed', finished_at = NOW(),
                error_code = NULL, error_message = NULL, updated_at = NOW()
            WHERE id = $1
            RETURNING {_COLUMNS}
            """,
            job_id,
        )
        return _job(record) if record else None

    async def schedule_retry(
        self, job_id: int, delay_seconds: int, error_code: str, error_message: str
    ) -> DocumentJob | None:
        record = await self._pool.fetchrow(
            f"""
            UPDATE document_jobs
            SET status = 'pending',
                next_attempt_at = NOW() + make_interval(secs => $2),
                error_code = $3, error_message = $4,
                started_at = NULL, finished_at = NULL, updated_at = NOW()
            WHERE id = $1
            RETURNING {_COLUMNS}
            """,
            job_id,
            delay_seconds,
            error_code,
            error_message,
        )
        return _job(record) if record else None

    async def mark_failed(
        self, job_id: int, error_code: str, error_message: str
    ) -> DocumentJob | None:
        record = await self._pool.fetchrow(
            f"""
            UPDATE document_jobs
            SET status = 'failed', error_code = $2, error_message = $3,
                finished_at = NOW(), updated_at = NOW()
            WHERE id = $1
            RETURNING {_COLUMNS}
            """,
            job_id,
            error_code,
            error_message,
        )
        return _job(record) if record else None
