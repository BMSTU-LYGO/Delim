"""PostgreSQL connection management."""

from __future__ import annotations

import asyncpg

from delim_document.config import PostgresConfig


async def create_pool(config: PostgresConfig) -> asyncpg.Pool:
    return await asyncpg.create_pool(
        host=config.host,
        port=config.port,
        database=config.database,
        user=config.user,
        password=config.password,
        ssl=config.sslmode,
        min_size=config.min_connections,
        max_size=config.max_connections,
    )
