"""Tests for the retention cleanup worker."""

from __future__ import annotations

import asyncio
import logging
import unittest

from delim_document.worker.retention import RetentionWorker


class FakeRepository:
    def __init__(self) -> None:
        self.receipts: list[tuple[int, str]] = []
        self.exports: list[tuple[int, str]] = []
        self.purged_receipts: list[int] = []
        self.purged_exports: list[int] = []

    async def due_receipts(self, retention_days: int, limit: int) -> list[tuple[int, str]]:
        return self.receipts[:limit]

    async def mark_receipt_purged(self, receipt_id: int) -> None:
        self.purged_receipts.append(receipt_id)

    async def due_exports(self, retention_days: int, limit: int) -> list[tuple[int, str]]:
        return self.exports[:limit]

    async def mark_export_purged(self, export_id: int) -> None:
        self.purged_exports.append(export_id)


class FakeStorage:
    def __init__(self) -> None:
        self.deleted_receipts: list[str] = []
        self.deleted_exports: list[str] = []

    async def delete_receipt(self, object_key: str) -> None:
        self.deleted_receipts.append(object_key)

    async def delete_export(self, object_key: str) -> None:
        self.deleted_exports.append(object_key)


def _worker(repo: FakeRepository, storage: FakeStorage) -> RetentionWorker:
    return RetentionWorker(
        repo,
        storage,
        logging.getLogger("test-retention"),
        retention_days=30,
        interval_seconds=3600,
        batch_size=10,
    )


class RetentionWorkerTest(unittest.TestCase):
    def test_run_once_purgs_due_receipts_and_exports(self) -> None:
        repo = FakeRepository()
        repo.receipts = [(1, "receipts/a"), (2, "receipts/b")]
        repo.exports = [(5, "exports/c")]
        storage = FakeStorage()

        purged = asyncio.run(_worker(repo, storage).run_once())

        self.assertEqual(purged, 3)
        self.assertEqual(storage.deleted_receipts, ["receipts/a", "receipts/b"])
        self.assertEqual(repo.purged_receipts, [1, 2])
        self.assertEqual(storage.deleted_exports, ["exports/c"])
        self.assertEqual(repo.purged_exports, [5])

    def test_run_once_noop_when_nothing_due(self) -> None:
        purged = asyncio.run(
            _worker(FakeRepository(), FakeStorage()).run_once()
        )
        self.assertEqual(purged, 0)

    def test_storage_failure_leaves_row_unmarked(self) -> None:
        class FailingStorage(FakeStorage):
            async def delete_receipt(self, object_key: str) -> None:
                raise RuntimeError("minio unavailable")

        repo = FakeRepository()
        repo.receipts = [(1, "receipts/a")]
        storage = FailingStorage()

        with self.assertRaises(RuntimeError):
            asyncio.run(_worker(repo, storage).run_once())
        self.assertEqual(repo.purged_receipts, [])

    def test_run_loop_survives_failures_and_stops(self) -> None:
        class ExplodingRepository(FakeRepository):
            def __init__(self) -> None:
                super().__init__()
                self.calls = 0

            async def due_receipts(self, retention_days: int, limit: int):
                self.calls += 1
                if self.calls == 1:
                    raise RuntimeError("transient")
                return self.receipts[:limit]

        repo = ExplodingRepository()
        repo.receipts = [(1, "receipts/a")]
        storage = FakeStorage()
        worker = RetentionWorker(
            repo,
            storage,
            logging.getLogger("test-retention"),
            retention_days=30,
            interval_seconds=0.05,
            batch_size=10,
        )
        stop = asyncio.Event()

        async def scenario() -> None:
            task = asyncio.create_task(worker.run(stop))
            while repo.purged_receipts != [1]:
                await asyncio.sleep(0.01)
            stop.set()
            await asyncio.wait_for(task, timeout=5)

        asyncio.run(scenario())


if __name__ == "__main__":
    unittest.main()
