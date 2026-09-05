"""Domain-to-protobuf mapping and public error translation."""

from __future__ import annotations

import asyncpg
import grpc
from datetime import datetime
from google.protobuf.timestamp_pb2 import Timestamp
from minio.error import S3Error

from delim_document.domain.job import DocumentJob, DocumentJobStatus, DocumentJobType
from delim_document.domain.receipt import Receipt, ReceiptStatus
from delim_document.service.document import (
    ConflictError,
    DependencyUnavailableError,
    ForbiddenError,
    InvalidInputError,
    NotFoundError,
    OCRResultView,
)
from proto.document.v1 import document_pb2


_RECEIPT_STATUSES = {
    ReceiptStatus.UPLOADED: document_pb2.RECEIPT_STATUS_UPLOADED,
    ReceiptStatus.QUEUED: document_pb2.RECEIPT_STATUS_QUEUED,
    ReceiptStatus.PROCESSING: document_pb2.RECEIPT_STATUS_PROCESSING,
    ReceiptStatus.READY: document_pb2.RECEIPT_STATUS_READY,
    ReceiptStatus.FAILED: document_pb2.RECEIPT_STATUS_FAILED,
    ReceiptStatus.DELETED: document_pb2.RECEIPT_STATUS_DELETED,
}
_JOB_TYPES = {DocumentJobType.OCR: document_pb2.DOCUMENT_JOB_TYPE_OCR}
_JOB_STATUSES = {
    DocumentJobStatus.PENDING: document_pb2.DOCUMENT_JOB_STATUS_PENDING,
    DocumentJobStatus.PROCESSING: document_pb2.DOCUMENT_JOB_STATUS_PROCESSING,
    DocumentJobStatus.COMPLETED: document_pb2.DOCUMENT_JOB_STATUS_COMPLETED,
    DocumentJobStatus.FAILED: document_pb2.DOCUMENT_JOB_STATUS_FAILED,
}


def _timestamp(value: datetime) -> Timestamp:
    timestamp = Timestamp()
    timestamp.FromDatetime(value)
    return timestamp


def receipt_to_proto(receipt: Receipt) -> document_pb2.Receipt:
    return document_pb2.Receipt(
        id=receipt.id,
        actor_user_id=receipt.actor_user_id,
        group_id=receipt.group_id,
        filename=receipt.filename,
        content_type=receipt.content_type,
        size_bytes=receipt.size_bytes,
        status=_RECEIPT_STATUSES[receipt.status],
        created_at=_timestamp(receipt.created_at),
    )


def job_to_proto(job: DocumentJob) -> document_pb2.DocumentJob:
    message = document_pb2.DocumentJob(
        id=job.id,
        receipt_id=job.receipt_id,
        type=_JOB_TYPES[job.type],
        status=_JOB_STATUSES[job.status],
        error_code=job.error_code or "",
        created_at=_timestamp(job.created_at),
    )
    if job.started_at is not None:
        message.started_at.CopyFrom(_timestamp(job.started_at))
    if job.finished_at is not None:
        message.finished_at.CopyFrom(_timestamp(job.finished_at))
    return message


def ocr_result_to_proto(view: OCRResultView) -> document_pb2.GetOCRResultResponse:
    message = document_pb2.GetOCRResultResponse(status=_RECEIPT_STATUSES[view.status])
    result = view.result
    if result is None:
        return message
    if result.merchant is not None:
        message.merchant = result.merchant
    if result.date is not None:
        message.date.CopyFrom(_timestamp(result.date))
    if result.total_minor is not None:
        message.total_minor = result.total_minor
    if result.currency is not None:
        message.currency = result.currency
    message.confidence = result.confidence
    message.qr_found = result.qr_raw is not None
    for item in result.items:
        item_message = message.items.add(
            name=item.name,
            amount_minor=item.amount_minor,
            confidence=item.confidence,
        )
        if item.quantity is not None:
            item_message.quantity = format(item.quantity, "f")
        if item.unit_price_minor is not None:
            item_message.unit_price_minor = item.unit_price_minor
    return message


async def abort_for_error(
    context: grpc.aio.ServicerContext, error: Exception
) -> None:
    if isinstance(error, InvalidInputError):
        await context.abort(grpc.StatusCode.INVALID_ARGUMENT, str(error))
    if isinstance(error, NotFoundError):
        await context.abort(grpc.StatusCode.NOT_FOUND, str(error))
    if isinstance(error, ForbiddenError):
        await context.abort(grpc.StatusCode.PERMISSION_DENIED, str(error))
    if isinstance(error, ConflictError):
        await context.abort(grpc.StatusCode.FAILED_PRECONDITION, str(error))
    if isinstance(
        error,
        (DependencyUnavailableError, asyncpg.PostgresError, S3Error, OSError),
    ):
        await context.abort(grpc.StatusCode.UNAVAILABLE, "dependency unavailable")
    await context.abort(grpc.StatusCode.INTERNAL, "internal service error")
