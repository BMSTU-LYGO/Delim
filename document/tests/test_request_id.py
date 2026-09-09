from __future__ import annotations

import asyncio
import logging
import unittest
from typing import Any

from delim_document.grpc.server import (
    REQUEST_ID_METADATA_KEY,
    DocumentGRPCServicer,
    extract_request_id,
)


def _record_tuples(records: list[logging.LogRecord]) -> list[tuple[str, dict[str, Any]]]:
    return [(record.getMessage(), getattr(record, "request_id", "")) for record in records]


class ExtractRequestIDTest(unittest.TestCase):
    def test_returns_value_when_metadata_has_request_id(self) -> None:
        metadata = (("x-request-id", "abc-123"),)
        self.assertEqual(extract_request_id(metadata), "abc-123")

    def test_is_case_insensitive_for_metadata_key(self) -> None:
        metadata = (("X-Request-ID", "upper"),)
        self.assertEqual(extract_request_id(metadata), "upper")

    def test_returns_empty_string_when_metadata_missing(self) -> None:
        self.assertEqual(extract_request_id(None), "")
        self.assertEqual(extract_request_id(()), "")
        self.assertEqual(extract_request_id((("other", "value"),)), "")

    def test_returns_empty_string_for_non_string_value(self) -> None:
        metadata = ((REQUEST_ID_METADATA_KEY, b"bytes"),)
        self.assertEqual(extract_request_id(metadata), "")

    def test_ignores_non_string_keys(self) -> None:
        metadata = ((b"x-request-id", "raw-bytes"), ("x-request-id", "real"))
        self.assertEqual(extract_request_id(metadata), "real")


class _StubService:
    async def create_receipt(self, **_kwargs: Any) -> Any:
        raise AssertionError("stub should not be invoked in this test")

    async def get_receipt(self, *_args: Any, **_kwargs: Any) -> Any:
        raise AssertionError("stub should not be invoked in this test")

    async def download_export(
        self, *_args: Any, **_kwargs: Any
    ) -> Any:
        raise AssertionError("stub should not be invoked in this test")


class _StubContext:
    def __init__(self, metadata: list[tuple[str, str]] | None) -> None:
        self._metadata = metadata or []
        self.aborted: tuple[Any, str] | None = None

    def invocation_metadata(self) -> list[tuple[str, str]]:
        return self._metadata

    async def abort(self, code: Any, details: str) -> None:
        self.aborted = (code, details)


class DocumentGRPCLoggingTest(unittest.TestCase):
    def _make_logger(self) -> tuple[logging.Logger, list[logging.LogRecord]]:
        records: list[logging.LogRecord] = []

        class _Capture(logging.Handler):
            def emit(self, record: logging.LogRecord) -> None:
                records.append(record)

        logger = logging.getLogger(f"delim_document.test.{id(self)}")
        logger.setLevel(logging.DEBUG)
        logger.propagate = False
        logger.handlers = [_Capture()]
        return logger, records

    def test_handle_logs_request_id_from_metadata(self) -> None:
        async def scenario() -> None:
            logger, records = self._make_logger()
            context = _StubContext([("x-request-id", "from-client")])
            servicer = DocumentGRPCServicer(_StubService(), logger)  # type: ignore[arg-type]

            async def operation() -> str:
                return "ok"

            result = await servicer._handle(context, operation, "GetReceipt")
            self.assertEqual(result, "ok")
            self.assertIsNone(context.aborted)

            captured = _record_tuples(records)
            self.assertEqual(captured[0], ("gRPC request started", "from-client"))
            self.assertEqual(
                getattr(records[0], "operation"),
                "GetReceipt",
            )

        asyncio.run(scenario())

    def test_handle_logs_empty_request_id_when_metadata_missing(self) -> None:
        async def scenario() -> None:
            logger, records = self._make_logger()
            context = _StubContext(None)
            servicer = DocumentGRPCServicer(_StubService(), logger)  # type: ignore[arg-type]

            async def operation() -> str:
                return "ok"

            await servicer._handle(context, operation, "GetReceipt")
            captured = _record_tuples(records)
            self.assertEqual(captured[0], ("gRPC request started", ""))

        asyncio.run(scenario())

    def test_handle_logs_error_with_request_id_when_operation_fails(self) -> None:
        async def scenario() -> None:
            logger, records = self._make_logger()
            context = _StubContext([("x-request-id", "trace-error")])
            servicer = DocumentGRPCServicer(_StubService(), logger)  # type: ignore[arg-type]

            async def operation() -> None:
                raise RuntimeError("boom")

            with self.assertRaises(RuntimeError):
                await servicer._handle(context, operation, "GetReceipt")

            error_records = [
                record for record in records if record.getMessage() == "gRPC request failed"
            ]
            self.assertEqual(len(error_records), 1)
            self.assertEqual(getattr(error_records[0], "request_id", None), "trace-error")
            self.assertEqual(getattr(error_records[0], "operation", None), "GetReceipt")

        asyncio.run(scenario())

    def test_download_export_stream_logs_request_id(self) -> None:
        async def scenario() -> list[logging.LogRecord]:
            logger, records = self._make_logger()
            context = _StubContext([("x-request-id", "stream-trace")])

            class _DownloadService:
                async def download_export(
                    self, actor_user_id: int, export_id: str
                ) -> Any:
                    yield b"chunk-1"
                    yield b"chunk-2"

            servicer = DocumentGRPCServicer(_DownloadService(), logger)  # type: ignore[arg-type]
            request = type("R", (), {"actor_user_id": 1, "export_id": "exp-1"})()
            chunks: list[Any] = []
            async for chunk in servicer.DownloadExport(request, context):
                chunks.append(chunk)
            self.assertEqual(len(chunks), 2)
            return records

        records = asyncio.run(scenario())
        started = [
            record
            for record in records
            if record.getMessage() == "gRPC streaming request started"
        ]
        completed = [
            record
            for record in records
            if record.getMessage() == "gRPC streaming request completed"
        ]
        self.assertEqual(len(started), 1)
        self.assertEqual(getattr(started[0], "request_id", None), "stream-trace")
        self.assertEqual(getattr(started[0], "operation", None), "DownloadExport")
        self.assertEqual(len(completed), 1)
        self.assertEqual(getattr(completed[0], "request_id", None), "stream-trace")


if __name__ == "__main__":
    unittest.main()