"""Application lifecycle and dependency assembly."""

from __future__ import annotations

import asyncio
import logging

from delim_document.config import Config
from delim_document.grpc.server import create_grpc_server
from delim_document.metrics import Recorder, build_recorder
from delim_document.repository.database import create_pool
from delim_document.repository.export import ExportRepository
from delim_document.repository.job import JobRepository
from delim_document.repository.ocr_result import OCRResultRepository
from delim_document.repository.receipt import ReceiptRepository
from delim_document.repository.retention import RetentionRepository
from delim_document.service.document import DocumentService
from delim_document.storage.minio import MinioStorage
from delim_document.worker.ocr import OCRWorker
from delim_document.worker.retention import RetentionWorker


class App:
    def __init__(self, config: Config, logger: logging.Logger) -> None:
        self._config = config
        self._logger = logger

    async def run(self, stop_event: asyncio.Event) -> None:
        recorder: Recorder = build_recorder(self._config.metrics)
        # Surface a clean configuration error if the metrics port cannot be
        # bound; ``start_http_server`` raises ``OSError`` when the listener
        # fails. We do this eagerly so the service never accepts work while
        # being unable to expose its internal metrics endpoint.
        try:
            recorder.start_endpoint()
        except OSError as exc:
            raise RuntimeError(
                f"cannot bind metrics endpoint on "
                f"{self._config.metrics.host}:{self._config.metrics.port}: {exc}"
            ) from exc
        pool = await create_pool(self._config.postgres)
        storage = MinioStorage(self._config.storage, recorder=recorder)
        try:
            await storage.verify_bucket()
            receipts = ReceiptRepository(pool)
            jobs = JobRepository(pool)
            results = OCRResultRepository(pool)
            exports = ExportRepository(pool)
            service = DocumentService(
                receipts,
                jobs,
                results,
                exports,
                storage,
                self._config.upload,
                recorder=recorder,
            )
            from delim_document.ocr.paddle import PaddleOCRProvider

            provider = await asyncio.to_thread(
                PaddleOCRProvider,
                self._config.ocr.language,
                self._config.ocr.confidence_threshold,
                self._config.worker.concurrency,
            )
            worker = OCRWorker(
                jobs,
                receipts,
                results,
                storage,
                provider,
                self._config.worker.max_attempts,
                self._config.worker.retry_base_seconds,
                recorder,
            )
            recovered = await jobs.recover_stale(
                self._config.worker.stale_after_minutes
            )
            if recovered:
                self._logger.info("recovered stale OCR jobs", extra={"count": recovered})
            address = f"{self._config.grpc.host}:{self._config.grpc.port}"
            protobuf_overhead = 1024 * 1024
            server = create_grpc_server(
                service,
                self._logger,
                address,
                recorder=recorder,
                options=[
                    (
                        "grpc.max_receive_message_length",
                        self._config.upload.max_size_bytes + protobuf_overhead,
                    )
                ],
            )
            await server.start()
            worker_stop = asyncio.Event()
            worker_task = asyncio.create_task(
                worker.run(worker_stop, self._config.worker.poll_interval_ms),
                name="ocr-worker",
            )
            stop_task = asyncio.create_task(stop_event.wait(), name="service-stop")
            retention = RetentionWorker(
                RetentionRepository(pool),
                storage,
                self._logger,
                self._config.privacy.receipt_retention_days,
                self._config.privacy.cleanup_interval_minutes * 60,
                self._config.privacy.cleanup_batch_size,
            )
            retention_stop = asyncio.Event()
            retention_task = asyncio.create_task(
                retention.run(retention_stop), name="retention-worker"
            )
            self._logger.info("service started", extra={"address": address})
            try:
                done, _ = await asyncio.wait(
                    {stop_task, worker_task, retention_task},
                    return_when=asyncio.FIRST_COMPLETED,
                )
                if worker_task in done:
                    await worker_task
                if retention_task in done:
                    await retention_task
            finally:
                worker_stop.set()
                retention_stop.set()
                await asyncio.gather(
                    worker_task, retention_task, return_exceptions=True
                )
                stop_task.cancel()
                await server.stop(grace=5)
        finally:
            await pool.close()
            self._logger.info("service stopped")
