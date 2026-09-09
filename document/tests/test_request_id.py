from __future__ import annotations

import asyncio
import json
import logging
import unittest
from typing import Any

from delim_document.grpc.server import (
    REQUEST_ID_METADATA_KEY,
    SERVICE_NAME,
    DocumentGRPCServicer,
    extract_request_id,
)
from delim_document.logging import StructuredFormatter, configure_logging


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


def _extra(record: logging.LogRecord, name: str, default: Any = None) -> Any:
    return getattr(record, name, default)


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

    def test_handle_emits_normalized_completion_record(self) -> None:
        async def scenario() -> None:
            logger, records = self._make_logger()
            context = _StubContext([("x-request-id", "from-client")])
            servicer = DocumentGRPCServicer(_StubService(), logger)  # type: ignore[arg-type]

            async def operation() -> str:
                return "ok"

            result = await servicer._handle(context, operation, "GetReceipt")
            self.assertEqual(result, "ok")
            self.assertIsNone(context.aborted)

            completed = [
                record
                for record in records
                if record.getMessage() == "gRPC request completed"
            ]
            self.assertEqual(len(completed), 1)
            record = completed[0]
            self.assertEqual(_extra(record, "service"), SERVICE_NAME)
            self.assertEqual(_extra(record, "request_id"), "from-client")
            self.assertEqual(_extra(record, "operation"), "GetReceipt")
            self.assertEqual(_extra(record, "status"), "success")
            self.assertEqual(_extra(record, "result"), "success")
            duration = _extra(record, "duration_ms")
            self.assertIsInstance(duration, int)
            self.assertGreaterEqual(duration, 0)
            self.assertFalse(hasattr(record, "error_class"))

        asyncio.run(scenario())

    def test_handle_emits_normalized_error_record_with_bounded_class(self) -> None:
        async def scenario() -> None:
            logger, records = self._make_logger()
            context = _StubContext([("x-request-id", "trace-error")])
            servicer = DocumentGRPCServicer(_StubService(), logger)  # type: ignore[arg-type]

            secret = "MAX_BOT_TOKEN=must-not-leak"

            async def operation() -> None:
                raise ValueError(secret)

            with self.assertRaises(RuntimeError):
                await servicer._handle(context, operation, "GetReceipt")

            failed = [
                record
                for record in records
                if record.getMessage() == "gRPC request failed"
            ]
            self.assertEqual(len(failed), 1)
            record = failed[0]
            self.assertEqual(_extra(record, "service"), SERVICE_NAME)
            self.assertEqual(_extra(record, "request_id"), "trace-error")
            self.assertEqual(_extra(record, "operation"), "GetReceipt")
            self.assertEqual(_extra(record, "status"), "error")
            self.assertEqual(_extra(record, "result"), "error")
            self.assertEqual(_extra(record, "error_class"), "ValueError")
            duration = _extra(record, "duration_ms")
            self.assertIsInstance(duration, int)
            self.assertGreaterEqual(duration, 0)

            serialized = logging.Formatter().format(record)
            self.assertNotIn(secret, serialized)

        asyncio.run(scenario())

    def test_handle_uses_empty_request_id_when_metadata_missing(self) -> None:
        async def scenario() -> None:
            logger, records = self._make_logger()
            context = _StubContext(None)
            servicer = DocumentGRPCServicer(_StubService(), logger)  # type: ignore[arg-type]

            async def operation() -> str:
                return "ok"

            await servicer._handle(context, operation, "GetReceipt")
            completed = [
                record
                for record in records
                if record.getMessage() == "gRPC request completed"
            ]
            self.assertEqual(len(completed), 1)
            self.assertEqual(_extra(completed[0], "request_id"), "")

        asyncio.run(scenario())

    def test_download_export_emits_normalized_stream_completion(self) -> None:
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
        completed = [
            record
            for record in records
            if record.getMessage() == "gRPC stream completed"
        ]
        self.assertEqual(len(completed), 1)
        record = completed[0]
        self.assertEqual(_extra(record, "service"), SERVICE_NAME)
        self.assertEqual(_extra(record, "request_id"), "stream-trace")
        self.assertEqual(_extra(record, "operation"), "DownloadExport")
        self.assertEqual(_extra(record, "status"), "success")
        self.assertEqual(_extra(record, "result"), "success")
        self.assertIsInstance(_extra(record, "duration_ms"), int)

    def test_download_export_emits_normalized_stream_error(self) -> None:
        async def scenario() -> None:
            logger, records = self._make_logger()
            context = _StubContext([("x-request-id", "stream-err")])

            class _FailingService:
                async def download_export(
                    self, actor_user_id: int, export_id: str
                ) -> Any:
                    raise RuntimeError("stream-secret")
                    yield b""  # pragma: no cover - makes it an async generator

            servicer = DocumentGRPCServicer(_FailingService(), logger)  # type: ignore[arg-type]
            request = type("R", (), {"actor_user_id": 1, "export_id": "exp-1"})()
            async for _ in servicer.DownloadExport(request, context):
                pass

            failed = [
                record
                for record in records
                if record.getMessage() == "gRPC stream failed"
            ]
            self.assertEqual(len(failed), 1)
            record = failed[0]
            self.assertEqual(_extra(record, "service"), SERVICE_NAME)
            self.assertEqual(_extra(record, "request_id"), "stream-err")
            self.assertEqual(_extra(record, "operation"), "DownloadExport")
            self.assertEqual(_extra(record, "status"), "error")
            self.assertEqual(_extra(record, "result"), "error")
            self.assertEqual(_extra(record, "error_class"), "RuntimeError")
            self.assertIsInstance(_extra(record, "duration_ms"), int)

        asyncio.run(scenario())

    def test_completion_record_omits_sensitive_fields(self) -> None:
        async def scenario() -> None:
            logger, records = self._make_logger()
            context = _StubContext([("x-request-id", "from-client")])
            servicer = DocumentGRPCServicer(_StubService(), logger)  # type: ignore[arg-type]

            async def operation() -> str:
                return "ok"

            await servicer._handle(context, operation, "GetReceipt")
            completed = [
                record
                for record in records
                if record.getMessage() == "gRPC request completed"
            ]
            self.assertEqual(len(completed), 1)
            payload = json.loads(StructuredFormatter().format(completed[0]))
            serialized = json.dumps(payload)
            for forbidden in (
                "authorization",
                "init_data",
                "initdata",
                "max_bot_token",
                "session_token",
                "invite_token",
                "webhook_secret",
                "raw_text",
                "ocr_text",
            ):
                self.assertNotIn(forbidden, serialized.lower())

        asyncio.run(scenario())


class StructuredFormatterTest(unittest.TestCase):
    def _record(self) -> logging.LogRecord:
        record = logging.LogRecord(
            name="document",
            level=logging.INFO,
            pathname=__file__,
            lineno=0,
            msg="hello",
            args=(),
            exc_info=None,
        )
        record.service = "document"
        record.request_id = "abc"
        record.operation = "GetReceipt"
        record.duration_ms = 12
        record.status = "success"
        record.result = "success"
        return record

    def test_emits_normalized_json_fields(self) -> None:
        formatter = StructuredFormatter()
        line = formatter.format(self._record())
        payload = json.loads(line)
        self.assertEqual(payload["service"], "document")
        self.assertEqual(payload["request_id"], "abc")
        self.assertEqual(payload["operation"], "GetReceipt")
        self.assertEqual(payload["duration_ms"], 12)
        self.assertEqual(payload["status"], "success")
        self.assertEqual(payload["result"], "success")
        self.assertEqual(payload["message"], "hello")
        self.assertNotIn("error_class", payload)

    def test_uses_allowlist_and_drops_unknown_extras(self) -> None:
        record = self._record()
        record.authorization = "Bearer super-secret-token"
        record.init_data = "raw-init-data-must-not-leak"
        record.max_bot_token = "MAX_BOT_TOKEN=abc"
        record.raw_text = "ocr text that must never reach logs"
        record.ocr_text = "another ocr leak"
        record.session_token = "session-token-secret"
        record.invite_token = "invite-token-secret"
        record.webhook_secret = "webhook-secret"
        record.exception_text = "stack trace details"
        payload = json.loads(StructuredFormatter().format(record))
        serialized = json.dumps(payload)
        for forbidden in (
            "authorization",
            "init_data",
            "initdata",
            "max_bot_token",
            "raw_text",
            "ocr_text",
            "session_token",
            "invite_token",
            "webhook_secret",
            "exception_text",
            "super-secret-token",
            "raw-init-data-must-not-leak",
            "MAX_BOT_TOKEN=abc",
            "ocr text that must never reach logs",
            "session-token-secret",
            "invite-token-secret",
            "webhook-secret",
            "stack trace details",
        ):
            self.assertNotIn(forbidden, serialized)

    def test_emits_safe_known_optional_fields(self) -> None:
        record = logging.LogRecord(
            name="document",
            level=logging.INFO,
            pathname=__file__,
            lineno=0,
            msg="boot",
            args=(),
            exc_info=None,
        )
        record.environment = "test"
        record.address = "127.0.0.1:50051"
        record.count = 3
        payload = json.loads(StructuredFormatter().format(record))
        self.assertEqual(payload["environment"], "test")
        self.assertEqual(payload["address"], "127.0.0.1:50051")
        self.assertEqual(payload["count"], 3)

    def test_does_not_emit_exception_text_or_stack(self) -> None:
        try:
            raise ValueError("must-not-leak-exception-message")
        except ValueError:
            import sys

            record = logging.LogRecord(
                name="document",
                level=logging.ERROR,
                pathname=__file__,
                lineno=0,
                msg="failed",
                args=(),
                exc_info=sys.exc_info(),
            )
            record.error_class = "ValueError"
        payload = json.loads(StructuredFormatter().format(record))
        serialized = json.dumps(payload)
        self.assertNotIn("must-not-leak-exception-message", serialized)
        self.assertNotIn("Traceback", serialized)
        self.assertEqual(payload["error_class"], "ValueError")

    def test_configure_logging_sets_service_attribute(self) -> None:
        configure_logging("document", "test")
        record = logging.LogRecord(
            name="document",
            level=logging.INFO,
            pathname=__file__,
            lineno=0,
            msg="hello",
            args=(),
            exc_info=None,
        )
        record.service = "document"
        payload = json.loads(StructuredFormatter().format(record))
        self.assertEqual(payload["service"], "document")


if __name__ == "__main__":
    unittest.main()