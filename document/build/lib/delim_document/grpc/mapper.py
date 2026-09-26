"""Domain-to-protobuf mapping and public error translation."""

from __future__ import annotations

import asyncpg
import grpc
from datetime import datetime, timezone
from collections.abc import Iterable
from typing import Any
from google.protobuf.timestamp_pb2 import Timestamp
from minio.error import S3Error

from delim_document.domain.job import DocumentJob, DocumentJobStatus, DocumentJobType
from delim_document.domain.receipt import Receipt, ReceiptStatus
from delim_document.export.models import (
    ExportFormat,
    ExportRecord,
    ExportStatus,
    ReportRow,
)
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
_EXPORT_FORMATS = {
    ExportFormat.CSV: document_pb2.EXPORT_FORMAT_CSV,
    ExportFormat.PDF: document_pb2.EXPORT_FORMAT_PDF,
    ExportFormat.XLSX: document_pb2.EXPORT_FORMAT_XLSX,
}
_EXPORT_STATUSES = {
    ExportStatus.PENDING: document_pb2.EXPORT_STATUS_PENDING,
    ExportStatus.PROCESSING: document_pb2.EXPORT_STATUS_PROCESSING,
    ExportStatus.READY: document_pb2.EXPORT_STATUS_READY,
    ExportStatus.FAILED: document_pb2.EXPORT_STATUS_FAILED,
}
_PROTO_EXPORT_FORMATS = {value: key for key, value in _EXPORT_FORMATS.items()}


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
        original_purged=receipt.original_purged_at is not None,
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


def export_format_from_proto(value: int) -> ExportFormat:
    export_format = _PROTO_EXPORT_FORMATS.get(value)
    if export_format is None:
        raise InvalidInputError("unsupported export format")
    return export_format


def report_rows_from_proto(rows: Iterable[Any]) -> tuple[ReportRow, ...]:
    result: list[ReportRow] = []
    for row in rows:
        if not row.HasField("date"):
            raise InvalidInputError("export row date is required")
        result.append(
            ReportRow(
                date=row.date.ToDatetime(tzinfo=timezone.utc),
                description=row.description,
                payer=row.payer,
                amount_minor=row.amount_minor,
                currency=row.currency,
                note=row.note,
            )
        )
    return tuple(result)


def export_to_proto(record: ExportRecord) -> document_pb2.Export:
    message = document_pb2.Export(
        id=record.id,
        actor_user_id=record.actor_user_id,
        group_id=record.group_id,
        format=_EXPORT_FORMATS[record.format],
        status=_EXPORT_STATUSES[record.status],
        filename=record.filename,
        error_code=record.error_code or "",
        created_at=_timestamp(record.created_at),
    )
    if record.finished_at is not None:
        message.finished_at.CopyFrom(_timestamp(record.finished_at))
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
