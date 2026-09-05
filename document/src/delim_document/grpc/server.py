"""Async gRPC server and Document handlers."""

from __future__ import annotations

import logging
from collections.abc import Awaitable, Callable
from typing import Any

import grpc

from delim_document.grpc.mapper import abort_for_error, job_to_proto, receipt_to_proto
from delim_document.service.document import DocumentService
from proto.document.v1 import document_pb2, document_pb2_grpc


class DocumentGRPCServicer(document_pb2_grpc.DocumentServiceServicer):
    def __init__(self, service: DocumentService, logger: logging.Logger) -> None:
        self._service = service
        self._logger = logger

    async def _handle(
        self,
        context: grpc.aio.ServicerContext,
        operation: Callable[[], Awaitable[Any]],
    ) -> Any:
        try:
            return await operation()
        except Exception as exc:
            self._logger.exception("gRPC request failed")
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

        return await self._handle(context, operation)

    async def GetReceipt(self, request: Any, context: grpc.aio.ServicerContext) -> Any:
        async def operation() -> Any:
            receipt = await self._service.get_receipt(
                request.actor_user_id, request.receipt_id
            )
            return document_pb2.GetReceiptResponse(receipt=receipt_to_proto(receipt))

        return await self._handle(context, operation)

    async def GetDocumentJob(
        self, request: Any, context: grpc.aio.ServicerContext
    ) -> Any:
        async def operation() -> Any:
            job = await self._service.get_document_job(
                request.actor_user_id, request.job_id
            )
            return document_pb2.GetDocumentJobResponse(job=job_to_proto(job))

        return await self._handle(context, operation)

    async def DeleteReceipt(self, request: Any, context: grpc.aio.ServicerContext) -> Any:
        async def operation() -> Any:
            await self._service.delete_receipt(
                request.actor_user_id, request.receipt_id
            )
            return document_pb2.DeleteReceiptResponse()

        return await self._handle(context, operation)


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
