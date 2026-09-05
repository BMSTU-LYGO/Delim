"""Application lifecycle and dependency assembly."""

from __future__ import annotations

import asyncio
import logging

from delim_document.config import Config
from delim_document.grpc.server import create_grpc_server
from delim_document.repository.job import JobRepository
from delim_document.repository.database import create_pool
from delim_document.repository.receipt import ReceiptRepository
from delim_document.service.document import DocumentService
from delim_document.storage.minio import MinioStorage


class App:
    def __init__(self, config: Config, logger: logging.Logger) -> None:
        self._config = config
        self._logger = logger

    async def run(self, stop_event: asyncio.Event) -> None:
        pool = await create_pool(self._config.postgres)
        storage = MinioStorage(self._config.storage)
        try:
            await storage.verify_bucket()
            service = DocumentService(
                ReceiptRepository(pool),
                JobRepository(pool),
                storage,
                self._config.upload,
            )
            address = f"{self._config.grpc.host}:{self._config.grpc.port}"
            protobuf_overhead = 1024 * 1024
            server = create_grpc_server(
                service,
                self._logger,
                address,
                options=[
                    (
                        "grpc.max_receive_message_length",
                        self._config.upload.max_size_bytes + protobuf_overhead,
                    )
                ],
            )
            await server.start()
            self._logger.info("service started", extra={"address": address})
            await stop_event.wait()
            await server.stop(grace=5)
        finally:
            await pool.close()
            self._logger.info("service stopped")
