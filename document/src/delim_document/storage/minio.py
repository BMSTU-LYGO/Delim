"""Private MinIO object storage adapter."""

from __future__ import annotations

import asyncio
import io
from collections.abc import AsyncIterator

from minio import Minio
from minio.error import S3Error

from delim_document.config import StorageConfig
from delim_document.metrics import Recorder, noop_recorder


class MinioStorage:
    def __init__(self, config: StorageConfig, recorder: Recorder | None = None) -> None:
        self._bucket = config.bucket
        self._client = Minio(
            config.endpoint,
            access_key=config.access_key,
            secret_key=config.secret_key,
            secure=config.use_ssl,
        )
        self._recorder = recorder or noop_recorder()

    async def verify_bucket(self) -> None:
        try:
            exists = await asyncio.to_thread(self._client.bucket_exists, self._bucket)
        except Exception as exc:
            self._recorder.observe_minio_error("stat", exc)
            raise
        if not exists:
            raise RuntimeError(f"object storage bucket does not exist: {self._bucket}")

    async def put_receipt(
        self, object_key: str, content: bytes, content_type: str
    ) -> None:
        try:
            await asyncio.to_thread(
                self._client.put_object,
                self._bucket,
                object_key,
                io.BytesIO(content),
                len(content),
                content_type=content_type,
            )
        except Exception as exc:
            self._recorder.observe_minio_error("put", exc)
            raise

    async def get_receipt(self, object_key: str) -> bytes:
        def read() -> bytes:
            response = self._client.get_object(self._bucket, object_key)
            try:
                return response.read()
            finally:
                response.close()
                response.release_conn()

        try:
            return await asyncio.to_thread(read)
        except Exception as exc:
            self._recorder.observe_minio_error("get", exc)
            raise

    async def delete_receipt(self, object_key: str) -> None:
        try:
            await asyncio.to_thread(
                self._client.remove_object,
                self._bucket,
                object_key,
            )
        except Exception as exc:
            self._recorder.observe_minio_error("delete", exc)
            raise

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
            self._recorder.observe_minio_error("stat", exc)
            raise
        except Exception as exc:
            self._recorder.observe_minio_error("stat", exc)
            raise
        return True

    async def put_export(
        self, object_key: str, content: bytes, content_type: str
    ) -> None:
        try:
            await asyncio.to_thread(
                self._client.put_object,
                self._bucket,
                object_key,
                io.BytesIO(content),
                len(content),
                content_type=content_type,
            )
        except Exception as exc:
            self._recorder.observe_minio_error("put", exc)
            raise

    async def delete_export(self, object_key: str) -> None:
        try:
            await asyncio.to_thread(self._client.remove_object, self._bucket, object_key)
        except Exception as exc:
            self._recorder.observe_minio_error("delete", exc)
            raise

    async def stream_export(
        self, object_key: str, chunk_size: int = 64 * 1024
    ) -> AsyncIterator[bytes]:
        try:
            response = await asyncio.to_thread(
                self._client.get_object, self._bucket, object_key
            )
        except Exception as exc:
            self._recorder.observe_minio_error("stream", exc)
            raise
        try:
            while chunk := await asyncio.to_thread(response.read, chunk_size):
                yield chunk
        finally:
            response.close()
            response.release_conn()
