"""Receipt repository."""

from __future__ import annotations

from typing import Any

import asyncpg

from delim_document.domain.receipt import Receipt, ReceiptStatus


def _receipt(record: asyncpg.Record) -> Receipt:
    values: dict[str, Any] = dict(record)
    values["status"] = ReceiptStatus(values["status"])
    return Receipt(**values)


class ReceiptRepository:
    def __init__(self, pool: asyncpg.Pool) -> None:
        self._pool = pool

    async def create(
        self,
        *,
        actor_user_id: int,
        group_id: int,
        filename: str,
        content_type: str,
        size_bytes: int,
        object_key: str,
    ) -> Receipt:
        record = await self._pool.fetchrow(
            """
            INSERT INTO document_receipts (
                actor_user_id, group_id, filename, content_type, size_bytes,
                object_key, status
            )
            VALUES ($1, $2, $3, $4, $5, $6, 'uploaded')
            RETURNING id, actor_user_id, group_id, filename, content_type,
                      size_bytes, object_key, status, created_at, updated_at,
                      deleted_at, original_purged_at
            """,
            actor_user_id,
            group_id,
            filename,
            content_type,
            size_bytes,
            object_key,
        )
        assert record is not None
        return _receipt(record)

    async def get(self, receipt_id: int, actor_user_id: int) -> Receipt | None:
        record = await self._pool.fetchrow(
            """
            SELECT id, actor_user_id, group_id, filename, content_type,
                   size_bytes, object_key, status, created_at, updated_at,
                   deleted_at, original_purged_at
            FROM document_receipts
            WHERE id = $1 AND actor_user_id = $2 AND deleted_at IS NULL
            """,
            receipt_id,
            actor_user_id,
        )
        return _receipt(record) if record else None

    async def get_for_processing(self, receipt_id: int) -> Receipt | None:
        record = await self._pool.fetchrow(
            """
            SELECT id, actor_user_id, group_id, filename, content_type,
                   size_bytes, object_key, status, created_at, updated_at,
                   deleted_at, original_purged_at
            FROM document_receipts
            WHERE id = $1 AND deleted_at IS NULL
            """,
            receipt_id,
        )
        return _receipt(record) if record else None

    async def mark_status(
        self, receipt_id: int, status: ReceiptStatus
    ) -> Receipt | None:
        record = await self._pool.fetchrow(
            """
            UPDATE document_receipts
            SET status = $2, updated_at = NOW()
            WHERE id = $1 AND deleted_at IS NULL
            RETURNING id, actor_user_id, group_id, filename, content_type,
                      size_bytes, object_key, status, created_at, updated_at,
                      deleted_at, original_purged_at
            """,
            receipt_id,
            status.value,
        )
        return _receipt(record) if record else None

    async def mark_original_purged(self, receipt_id: int) -> None:
        await self._pool.execute(
            """
            UPDATE document_receipts
            SET original_purged_at = COALESCE(original_purged_at, NOW()),
                updated_at = NOW()
            WHERE id = $1
            """,
            receipt_id,
        )

    async def soft_delete(
        self, receipt_id: int, actor_user_id: int
    ) -> Receipt | None:
        record = await self._pool.fetchrow(
            """
            UPDATE document_receipts
            SET status = 'deleted',
                deleted_at = COALESCE(deleted_at, NOW()),
                updated_at = NOW()
            WHERE id = $1 AND actor_user_id = $2
            RETURNING id, actor_user_id, group_id, filename, content_type,
                      size_bytes, object_key, status, created_at, updated_at,
                      deleted_at, original_purged_at
            """,
            receipt_id,
            actor_user_id,
        )
        return _receipt(record) if record else None
