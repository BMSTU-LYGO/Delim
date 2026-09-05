"""Application lifecycle and dependency assembly."""

from __future__ import annotations

import asyncio
import logging

from delim_document.config import Config
from delim_document.repository.database import create_pool


class App:
    def __init__(self, config: Config, logger: logging.Logger) -> None:
        self._config = config
        self._logger = logger

    async def run(self, stop_event: asyncio.Event) -> None:
        pool = await create_pool(self._config.postgres)
        self._logger.info(
            "service started",
            extra={"address": f"{self._config.grpc.host}:{self._config.grpc.port}"},
        )
        try:
            await stop_event.wait()
        finally:
            await pool.close()
            self._logger.info("service stopped")
