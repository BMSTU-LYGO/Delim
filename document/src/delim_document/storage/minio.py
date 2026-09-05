"""Private MinIO object storage adapter."""

from __future__ import annotations

import asyncio
import io

from minio import Minio
from minio.error import S3Error

from delim_document.config import StorageConfig


class MinioStorage:
    def __init__(self, config: StorageConfig) -> None:
        self._bucket = config.bucket
        self._client = Minio(
            config.endpoint,
            access_key=config.access_key,
            secret_key=config.secret_key,
            secure=config.use_ssl,
        )

    async def verify_bucket(self) -> None:
        exists = await asyncio.to_thread(self._client.bucket_exists, self._bucket)
        if not exists:
            raise RuntimeError(f"object storage bucket does not exist: {self._bucket}")

    async def put_receipt(
        self, object_key: str, content: bytes, content_type: str
    ) -> None:
        await asyncio.to_thread(
            self._client.put_object,
            self._bucket,
            object_key,
            io.BytesIO(content),
            len(content),
            content_type=content_type,
        )

    async def get_receipt(self, object_key: str) -> bytes:
        def read() -> bytes:
            response = self._client.get_object(self._bucket, object_key)
            try:
                return response.read()
            finally:
                response.close()
                response.release_conn()

        return await asyncio.to_thread(read)

    async def delete_receipt(self, object_key: str) -> None:
        await asyncio.to_thread(
            self._client.remove_object,
            self._bucket,
            object_key,
        )

    async def exists(self, object_key: str) -> bool:
        try:
            await asyncio.to_thread(
                self._client.stat_object,
                self._bucket,
                object_key,
            )
        except S3Error as exc:
            if exc.code in {"NoSuchKey", "NoSuchObject", "NotFound"}:
                return False
            raise
        return True
