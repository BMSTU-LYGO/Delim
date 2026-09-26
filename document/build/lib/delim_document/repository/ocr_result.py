"""Persistence for structured OCR results and items."""

from __future__ import annotations

import json
from typing import Any

import asyncpg

from delim_document.ocr.provider import OCRItem, OCRLine, OCRResult


def _raw_lines(value: str | list[dict[str, Any]]) -> tuple[OCRLine, ...]:
    records = json.loads(value) if isinstance(value, str) else value
    return tuple(
        OCRLine(
            text=record["text"],
            confidence=float(record["confidence"]),
            bbox=tuple(tuple(float(axis) for axis in point) for point in record["bbox"]),
        )
        for record in records
    )


class OCRResultRepository:
    def __init__(self, pool: asyncpg.Pool) -> None:
        self._pool = pool

    async def replace_result(
        self, receipt_id: int, actor_user_id: int, result: OCRResult
    ) -> bool:
        raw_lines = json.dumps(
            [
                {
                    "text": line.text,
                    "confidence": line.confidence,
                    "bbox": line.bbox,
                }
                for line in result.raw_lines
            ],
            ensure_ascii=False,
        )
        async with self._pool.acquire() as connection:
            async with connection.transaction():
                owned = await connection.fetchval(
                    """
                    SELECT id
                    FROM document_receipts
                    WHERE id = $1 AND actor_user_id = $2 AND deleted_at IS NULL
                    FOR SHARE
                    """,
                    receipt_id,
                    actor_user_id,
                )
                if owned is None:
                    return False
                result_id = await connection.fetchval(
                    """
                    INSERT INTO document_ocr_results (
                        receipt_id, qr_raw, merchant, receipt_date, total_minor,
                        currency, confidence, raw_text, raw_lines
                    )
                    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb)
                    ON CONFLICT (receipt_id) DO UPDATE SET
                        qr_raw = EXCLUDED.qr_raw,
                        merchant = EXCLUDED.merchant,
                        receipt_date = EXCLUDED.receipt_date,
                        total_minor = EXCLUDED.total_minor,
                        currency = EXCLUDED.currency,
                        confidence = EXCLUDED.confidence,
                        raw_text = EXCLUDED.raw_text,
                        raw_lines = EXCLUDED.raw_lines,
                        updated_at = NOW()
                    RETURNING id
                    """,
                    receipt_id,
                    result.qr_raw,
                    result.merchant,
                    result.date,
                    result.total_minor,
                    result.currency,
                    result.confidence,
                    result.raw_text,
                    raw_lines,
                )
                await connection.execute(
                    "DELETE FROM document_ocr_items WHERE result_id = $1", result_id
                )
                if result.items:
                    await connection.executemany(
                        """
                        INSERT INTO document_ocr_items (
                            result_id, position, name, quantity,
                            unit_price_minor, amount_minor, confidence
                        )
                        VALUES ($1, $2, $3, $4, $5, $6, $7)
                        """,
                        [
                            (
                                result_id,
                                position,
                                item.name,
                                item.quantity,
                                item.unit_price_minor,
                                item.amount_minor,
                                item.confidence,
                            )
                            for position, item in enumerate(result.items)
                        ],
                    )
        return True

    async def get_by_receipt(
        self, receipt_id: int, actor_user_id: int
    ) -> OCRResult | None:
        record = await self._pool.fetchrow(
            """
            SELECT o.id, o.qr_raw, o.merchant, o.receipt_date, o.total_minor,
                   o.currency, o.confidence, o.raw_text, o.raw_lines
            FROM document_ocr_results AS o
            JOIN document_receipts AS r ON r.id = o.receipt_id
            WHERE o.receipt_id = $1 AND r.actor_user_id = $2
              AND r.deleted_at IS NULL
            """,
            receipt_id,
            actor_user_id,
        )
        if record is None:
            return None
        item_records = await self._pool.fetch(
            """
            SELECT name, quantity, unit_price_minor, amount_minor, confidence
            FROM document_ocr_items
            WHERE result_id = $1
            ORDER BY position
            """,
            record["id"],
        )
        items = tuple(
            OCRItem(
                name=item["name"],
                quantity=item["quantity"],
                unit_price_minor=item["unit_price_minor"],
                amount_minor=item["amount_minor"],
                confidence=float(item["confidence"]),
            )
            for item in item_records
        )
        return OCRResult(
            merchant=record["merchant"],
            merchant_confidence=0.0,
            date=record["receipt_date"],
            date_confidence=0.0,
            total_minor=record["total_minor"],
            total_confidence=0.0,
            currency=record["currency"],
            items=items,
            confidence=float(record["confidence"]),
            raw_text=record["raw_text"],
            raw_lines=_raw_lines(record["raw_lines"]),
            qr_raw=record["qr_raw"],
            total_mismatch=False,
        )
