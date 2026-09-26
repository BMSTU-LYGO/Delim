"""Background retention cleanup.

Purges receipt originals from object storage once the configured privacy
retention has expired (or immediately for user-deleted receipts) and removes
expired export artifacts. Database metadata, OCR results, and the
structured financial history in Core are never deleted here.
"""

from __future__ import annotations

import asyncio
import logging
from typing import Protocol

class RetentionStore(Protocol):
    async def delete_receipt(self, object_key: str) -> None: ...

    async def delete_export(self, object_key: str) -> None: ...


class RetentionRepositoryLike(Protocol):
    async def due_receipts(self, retention_days: int, limit: int) -> list[tuple[int, str]]: ...

    async def mark_receipt_purged(self, receipt_id: int) -> None: ...

    async def due_exports(self, retention_days: int, limit: int) -> list[tuple[int, str]]: ...

    async def mark_export_purged(self, export_id: int) -> None: ...


class RetentionWorker:
    def __init__(
        self,
        repository: RetentionRepositoryLike,
        storage: RetentionStore,
        logger: logging.Logger,
        retention_days: int,
        interval_seconds: float,
        batch_size: int,
    ) -> None:
        self._repository = repository
        self._storage = storage
        self._logger = logger
        self._retention_days = retention_days
        self._interval = interval_seconds
        self._batch_size = batch_size

    async def run(self, stop_event: asyncio.Event) -> None:
        while not stop_event.is_set():
            try:
                await self.run_once()
            except Exception:
                # Cleanup is best-effort: log and retry on the next tick.
                self._logger.exception("retention cleanup pass failed")
            try:
                await asyncio.wait_for(stop_event.wait(), timeout=self._interval)
            except TimeoutError:
                pass

    async def run_once(self) -> int:
        purged = 0
        for receipt_id, object_key in await self._repository.due_receipts(
            self._retention_days, self._batch_size
        ):
            await self._storage.delete_receipt(object_key)
            await self._repository.mark_receipt_purged(receipt_id)
            purged += 1
        for export_id, object_key in await self._repository.due_exports(
            self._retention_days, self._batch_size
        ):
            await self._storage.delete_export(object_key)
            await self._repository.mark_export_purged(export_id)
            purged += 1
        if purged:
            self._logger.info(
                "retention cleanup", extra={"purged_objects": purged}
            )
        return purged
