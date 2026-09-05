"""PostgreSQL export metadata repository."""

from __future__ import annotations

from typing import Any

import asyncpg

from delim_document.export.models import ExportFormat, ExportRecord, ExportStatus


_COLUMNS = """
    id, actor_user_id, group_id, format, status, object_key, filename,
    error_code, created_at, finished_at
"""


def _export(record: asyncpg.Record) -> ExportRecord:
    values: dict[str, Any] = dict(record)
    values["format"] = ExportFormat(values["format"])
    values["status"] = ExportStatus(values["status"])
    return ExportRecord(**values)


class ExportRepository:
    def __init__(self, pool: asyncpg.Pool) -> None:
        self._pool = pool

    async def create(
        self,
        actor_user_id: int,
        group_id: int,
        export_format: ExportFormat,
        filename: str,
    ) -> ExportRecord:
        record = await self._pool.fetchrow(
            f"""
            INSERT INTO document_exports (
                actor_user_id, group_id, format, status, filename
            ) VALUES ($1, $2, $3, 'pending', $4)
            RETURNING {_COLUMNS}
            """,
            actor_user_id,
            group_id,
            export_format.value,
            filename,
        )
        assert record is not None
        return _export(record)

    async def mark_processing(self, export_id: int) -> ExportRecord | None:
        record = await self._pool.fetchrow(
            f"""
            UPDATE document_exports SET status = 'processing'
            WHERE id = $1 AND status = 'pending'
            RETURNING {_COLUMNS}
            """,
            export_id,
        )
        return _export(record) if record else None

    async def mark_ready(
        self, export_id: int, object_key: str
    ) -> ExportRecord | None:
        record = await self._pool.fetchrow(
            f"""
            UPDATE document_exports
            SET status = 'ready', object_key = $2, finished_at = NOW(),
                error_code = NULL
            WHERE id = $1 AND status = 'processing'
            RETURNING {_COLUMNS}
            """,
            export_id,
            object_key,
        )
        return _export(record) if record else None

    async def mark_failed(
        self, export_id: int, error_code: str
    ) -> ExportRecord | None:
        record = await self._pool.fetchrow(
            f"""
            UPDATE document_exports
            SET status = 'failed', error_code = $2, finished_at = NOW()
            WHERE id = $1
            RETURNING {_COLUMNS}
            """,
            export_id,
            error_code,
        )
        return _export(record) if record else None

    async def get(self, export_id: int, actor_user_id: int) -> ExportRecord | None:
        record = await self._pool.fetchrow(
            f"""
            SELECT {_COLUMNS} FROM document_exports
            WHERE id = $1 AND actor_user_id = $2
            """,
            export_id,
            actor_user_id,
        )
        return _export(record) if record else None
