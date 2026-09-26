"""Retention queries used by the background privacy cleanup worker."""

from __future__ import annotations

import asyncpg


class RetentionRepository:
    def __init__(self, pool: asyncpg.Pool) -> None:
        self._pool = pool

    async def due_receipts(
        self, retention_days: int, limit: int
    ) -> list[tuple[int, str]]:
        rows = await self._pool.fetch(
            """
            SELECT id, object_key
            FROM document_receipts
            WHERE original_purged_at IS NULL
              AND (
                    deleted_at IS NOT NULL
                    OR created_at < NOW() - make_interval(days => $1)
                  )
            ORDER BY id
            LIMIT $2
            """,
            retention_days,
            limit,
        )
        return [(row["id"], row["object_key"]) for row in rows]

    async def mark_receipt_purged(self, receipt_id: int) -> None:
        await self._pool.execute(
            """
            UPDATE document_receipts
            SET original_purged_at = COALESCE(original_purged_at, NOW()),
                updated_at = NOW()
            WHERE id = $1
            """,
            receipt_id,
        )

    async def due_exports(
        self, retention_days: int, limit: int
    ) -> list[tuple[int, str]]:
        rows = await self._pool.fetch(
            """
            SELECT id, object_key
            FROM document_exports
            WHERE object_purged_at IS NULL
              AND object_key IS NOT NULL
              AND created_at < NOW() - make_interval(days => $1)
            ORDER BY id
            LIMIT $2
            """,
            retention_days,
            limit,
        )
        return [(row["id"], row["object_key"]) for row in rows]

    async def mark_export_purged(self, export_id: int) -> None:
        await self._pool.execute(
            """
            UPDATE document_exports
            SET object_purged_at = COALESCE(object_purged_at, NOW())
            WHERE id = $1
            """,
            export_id,
        )
