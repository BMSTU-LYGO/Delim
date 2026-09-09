"""Async gRPC server and Document handlers."""

from __future__ import annotations

import logging
import time
from collections.abc import Awaitable, Callable, Iterable
from typing import Any

import grpc

from delim_document.grpc.mapper import (
    abort_for_error,
    export_format_from_proto,
    export_to_proto,
    job_to_proto,
    ocr_result_to_proto,
    receipt_to_proto,
    report_rows_from_proto,
)
from delim_document.metrics import Recorder, noop_recorder
from delim_document.service.document import DocumentService
from proto.document.v1 import document_pb2, document_pb2_grpc

REQUEST_ID_METADATA_KEY = "x-request-id"

SERVICE_NAME = "document"


def extract_request_id(metadata: Iterable[tuple[Any, Any]] | None) -> str:
    """Return the first ``x-request-id`` value from invocation metadata.

    The metadata key is matched case-insensitively. Returns an empty string
    when no usable value is found.
    """

    if not metadata:
        return ""
    for key, value in metadata:
        if not isinstance(key, str):
            continue
        if key.lower() != REQUEST_ID_METADATA_KEY:
            continue
        if isinstance(value, str):
            return value
    return ""


def _request_id_from_context(context: grpc.aio.ServicerContext) -> str:
    try:
        metadata = context.invocation_metadata()
    except Exception:  # noqa: BLE001 - never fail log extraction
        return ""
    return extract_request_id(metadata)


def _error_class(exc: BaseException) -> str:
    """Return a bounded error class identifier.

    The result never includes exception arguments so sensitive context cannot
    leak via the log record. Maps gRPC ``RpcError`` to ``grpc_<CODE>``.
    """

    name = type(exc).__name__
    if isinstance(exc, grpc.RpcError):
        try:
            code = exc.code()  # type: ignore[attr-defined]
        except Exception:  # noqa: BLE001 - code may not be available
            return name
        return f"grpc_{code}"
    return name


def _grpc_code_label(exc: BaseException | None) -> str:
    if exc is None:
        return "ok"
    if isinstance(exc, grpc.RpcError):
        try:
            return exc.code().name.lower()  # type: ignore[attr-defined]
        except Exception:  # noqa: BLE001
            return "unknown"
    return "internal"


class DocumentGRPCServicer(document_pb2_grpc.DocumentServiceServicer):
    def __init__(
        self,
        service: DocumentService,
        logger: logging.Logger,
        recorder: Recorder | None = None,
        service_name: str = SERVICE_NAME,
        ocr_health: Callable[[], str] | None = None,
    ) -> None:
        self._service = service
        self._logger = logger
        self._service_name = service_name
        self._recorder = recorder or noop_recorder()
        self._ocr_health = ocr_health

    async def _handle(
        self,
        context: grpc.aio.ServicerContext,
        operation: Callable[[], Awaitable[Any]],
        operation_name: str,
    ) -> Any:
        request_id = _request_id_from_context(context)
        started = time.monotonic()
        try:
            result = await operation()
        except Exception as exc:
            duration_ms = int((time.monotonic() - started) * 1000)
            self._logger.error(
                "gRPC request failed",
                extra={
                    "service": self._service_name,
                    "request_id": request_id,
                    "operation": operation_name,
                    "duration_ms": duration_ms,
                    "status": "error",
                    "result": "error",
                    "error_class": _error_class(exc),
                },
            )
            self._recorder.observe_grpc(
                operation_name,
                _grpc_code_label(exc),
                duration_ms / 1000.0,
                exc,
            )
            await abort_for_error(context, exc)
            raise RuntimeError("gRPC abort unexpectedly returned")
        duration_ms = int((time.monotonic() - started) * 1000)
        self._logger.info(
            "gRPC request completed",
            extra={
                "service": self._service_name,
                "request_id": request_id,
                "operation": operation_name,
                "duration_ms": duration_ms,
                "status": "success",
                "result": "success",
            },
        )
        self._recorder.observe_grpc(
            operation_name, "ok", duration_ms / 1000.0, None
        )
        return result

    async def Ping(self, request: Any, context: grpc.aio.ServicerContext) -> Any:
        del request, context
        ocr = self._ocr_health() if self._ocr_health is not None else "unknown"
        return document_pb2.PingResponse(status="ok", ocr=ocr)

    async def CreateReceipt(self, request: Any, context: grpc.aio.ServicerContext) -> Any:
        async def operation() -> Any:
            result = await self._service.create_receipt(
                actor_user_id=request.actor_user_id,
                group_id=request.group_id,
                filename=request.filename,
                content_type=request.content_type,
                content=request.content,
            )
            return document_pb2.CreateReceiptResponse(
                receipt=receipt_to_proto(result.receipt),
                job=job_to_proto(result.job),
            )

        return await self._handle(context, operation, "CreateReceipt")

    async def GetReceipt(self, request: Any, context: grpc.aio.ServicerContext) -> Any:
        async def operation() -> Any:
            receipt = await self._service.get_receipt(
                request.actor_user_id, request.receipt_id
            )
            return document_pb2.GetReceiptResponse(receipt=receipt_to_proto(receipt))

        return await self._handle(context, operation, "GetReceipt")

    async def GetDocumentJob(
        self, request: Any, context: grpc.aio.ServicerContext
    ) -> Any:
        async def operation() -> Any:
            job = await self._service.get_document_job(
                request.actor_user_id, request.job_id
            )
            return document_pb2.GetDocumentJobResponse(job=job_to_proto(job))

        return await self._handle(context, operation, "GetDocumentJob")

    async def GetOCRResult(
        self, request: Any, context: grpc.aio.ServicerContext
    ) -> Any:
        async def operation() -> Any:
            result = await self._service.get_ocr_result(
                request.actor_user_id, request.receipt_id
            )
            return ocr_result_to_proto(result)

        return await self._handle(context, operation, "GetOCRResult")

    async def RetryReceiptOCR(
        self, request: Any, context: grpc.aio.ServicerContext
    ) -> Any:
        async def operation() -> Any:
            job = await self._service.retry_receipt_ocr(
                request.actor_user_id, request.receipt_id
            )
            return document_pb2.RetryReceiptOCRResponse(job=job_to_proto(job))

        return await self._handle(context, operation, "RetryReceiptOCR")

    async def CreateExport(
        self, request: Any, context: grpc.aio.ServicerContext
    ) -> Any:
        async def operation() -> Any:
            record = await self._service.create_export(
                request.actor_user_id,
                request.group_id,
                request.group_name,
                export_format_from_proto(request.format),
                report_rows_from_proto(request.rows),
            )
            return document_pb2.CreateExportResponse(export=export_to_proto(record))

        return await self._handle(context, operation, "CreateExport")

    async def GetExport(self, request: Any, context: grpc.aio.ServicerContext) -> Any:
        async def operation() -> Any:
            record = await self._service.get_export(
                request.actor_user_id, request.export_id
            )
            return document_pb2.GetExportResponse(export=export_to_proto(record))

        return await self._handle(context, operation, "GetExport")

    async def DownloadExport(
        self, request: Any, context: grpc.aio.ServicerContext
    ) -> Any:
        request_id = _request_id_from_context(context)
        started = time.monotonic()
        operation_name = "DownloadExport"
        try:
            async for chunk in self._service.download_export(
                request.actor_user_id, request.export_id
            ):
                yield document_pb2.DownloadExportChunk(content=chunk)
        except Exception as exc:
            duration_ms = int((time.monotonic() - started) * 1000)
            self._logger.error(
                "gRPC stream failed",
                extra={
                    "service": self._service_name,
                    "request_id": request_id,
                    "operation": operation_name,
                    "duration_ms": duration_ms,
                    "status": "error",
                    "result": "error",
                    "error_class": _error_class(exc),
                },
            )
            self._recorder.observe_grpc(
                operation_name,
                _grpc_code_label(exc),
                duration_ms / 1000.0,
                exc,
            )
            await abort_for_error(context, exc)
            return
        duration_ms = int((time.monotonic() - started) * 1000)
        self._logger.info(
            "gRPC stream completed",
            extra={
                "service": self._service_name,
                "request_id": request_id,
                "operation": operation_name,
                "duration_ms": duration_ms,
                "status": "success",
                "result": "success",
            },
        )
        self._recorder.observe_grpc(
            operation_name, "ok", duration_ms / 1000.0, None
        )

    async def DeleteReceipt(self, request: Any, context: grpc.aio.ServicerContext) -> Any:
        async def operation() -> Any:
            await self._service.delete_receipt(
                request.actor_user_id, request.receipt_id
            )
            return document_pb2.DeleteReceiptResponse()

        return await self._handle(context, operation, "DeleteReceipt")

    async def DeleteReceiptOriginal(
        self, request: Any, context: grpc.aio.ServicerContext
    ) -> Any:
        async def operation() -> Any:
            already_removed = await self._service.delete_receipt_original(
                request.actor_user_id, request.receipt_id
            )
            return document_pb2.DeleteReceiptOriginalResponse(
                already_removed=already_removed
            )

        return await self._handle(context, operation, "DeleteReceiptOriginal")


def create_grpc_server(
    service: DocumentService,
    logger: logging.Logger,
    address: str,
    recorder: Recorder | None = None,
    options: list[tuple[str, int]] | None = None,
    service_name: str = SERVICE_NAME,
    ocr_health: Callable[[], str] | None = None,
) -> grpc.aio.Server:
    server = grpc.aio.server(options=options)
    document_pb2_grpc.add_DocumentServiceServicer_to_server(
        DocumentGRPCServicer(
            service, logger, recorder or noop_recorder(), service_name, ocr_health
        ),
        server,
    )
    if server.add_insecure_port(address) == 0:
        raise RuntimeError(f"cannot bind gRPC server to {address}")
    return server