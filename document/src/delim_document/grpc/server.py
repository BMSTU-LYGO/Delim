"""Async gRPC server and Document handlers."""

from __future__ import annotations

import logging
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
from delim_document.service.document import DocumentService
from proto.document.v1 import document_pb2, document_pb2_grpc

REQUEST_ID_METADATA_KEY = "x-request-id"


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


class DocumentGRPCServicer(document_pb2_grpc.DocumentServiceServicer):
    def __init__(self, service: DocumentService, logger: logging.Logger) -> None:
        self._service = service
        self._logger = logger

    async def _handle(
        self,
        context: grpc.aio.ServicerContext,
        operation: Callable[[], Awaitable[Any]],
        operation_name: str,
    ) -> Any:
        request_id = _request_id_from_context(context)
        self._logger.info(
            "gRPC request started",
            extra={"request_id": request_id, "operation": operation_name},
        )
        try:
            return await operation()
        except Exception as exc:
            self._logger.exception(
                "gRPC request failed",
                extra={"request_id": request_id, "operation": operation_name},
            )
            await abort_for_error(context, exc)
            raise RuntimeError("gRPC abort unexpectedly returned")

    async def Ping(self, request: Any, context: grpc.aio.ServicerContext) -> Any:
        del request, context
        return document_pb2.PingResponse(status="ok")

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
        self._logger.info(
            "gRPC streaming request started",
            extra={"request_id": request_id, "operation": "DownloadExport"},
        )
        try:
            async for chunk in self._service.download_export(
                request.actor_user_id, request.export_id
            ):
                yield document_pb2.DownloadExportChunk(content=chunk)
            self._logger.info(
                "gRPC streaming request completed",
                extra={"request_id": request_id, "operation": "DownloadExport"},
            )
        except Exception as exc:
            self._logger.exception(
                "gRPC streaming request failed",
                extra={"request_id": request_id, "operation": "DownloadExport"},
            )
            await abort_for_error(context, exc)

    async def DeleteReceipt(self, request: Any, context: grpc.aio.ServicerContext) -> Any:
        async def operation() -> Any:
            await self._service.delete_receipt(
                request.actor_user_id, request.receipt_id
            )
            return document_pb2.DeleteReceiptResponse()

        return await self._handle(context, operation, "DeleteReceipt")


def create_grpc_server(
    service: DocumentService,
    logger: logging.Logger,
    address: str,
    options: list[tuple[str, int]] | None = None,
) -> grpc.aio.Server:
    server = grpc.aio.server(options=options)
    document_pb2_grpc.add_DocumentServiceServicer_to_server(
        DocumentGRPCServicer(service, logger), server
    )
    if server.add_insecure_port(address) == 0:
        raise RuntimeError(f"cannot bind gRPC server to {address}")
    return server
