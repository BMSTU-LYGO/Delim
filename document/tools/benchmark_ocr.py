"""Measure two sequential OCR runs for one receipt JPEG; do not use in CI.

Usage (inside the document image):
    python tools/benchmark_ocr.py /path/to/receipt.jpg
"""

from __future__ import annotations

import argparse
import asyncio
from pathlib import Path
import time

from delim_document.image.decoder import decode_image
from delim_document.image.preprocess import preprocess_receipt
from delim_document.ocr.confidence import build_ocr_result
from delim_document.ocr.subprocess_provider import SubprocessOCRProvider
from delim_document.qr.fiscal import parse_fiscal_qr
from delim_document.qr.reader import ReceiptQRReader


async def run_once(
    provider: SubprocessOCRProvider, image_bytes: bytes, *, dump_result: bool = False
) -> None:
    total_started = time.monotonic()
    started = time.monotonic()
    original = await asyncio.to_thread(decode_image, image_bytes)
    processed = await asyncio.to_thread(preprocess_receipt, original)
    qr = await asyncio.to_thread(
        ReceiptQRReader().read,
        processed.normal,
        processed.enhanced,
    )
    preprocess_ms = round((time.monotonic() - started) * 1000)

    started = time.monotonic()
    lines = await provider.recognize(processed.enhanced)
    ocr_ms = round((time.monotonic() - started) * 1000)

    started = time.monotonic()
    result = await asyncio.to_thread(
        build_ocr_result,
        lines,
        parse_fiscal_qr(qr.raw_payload) if qr.raw_payload else None,
        qr.raw_payload,
    )
    parse_ms = round((time.monotonic() - started) * 1000)
    total_ms = round((time.monotonic() - total_started) * 1000)
    print(
        f"preprocess_ms={preprocess_ms} ocr_ms={ocr_ms} "
        f"parse_ms={parse_ms} total_ms={total_ms}"
    )
    if dump_result:
        print(f"raw_line_count={len(lines)}")
        for line in lines:
            print(f"confidence={line.confidence:.3f} text={line.text!r}")
        print(
            f"parsed merchant={result.merchant!r} date={result.date!r} "
            f"total_minor={result.total_minor!r} items={len(result.items)}"
        )


async def main(path: Path, runs: int, dump_result: bool) -> None:
    image_bytes = path.read_bytes()
    provider = SubprocessOCRProvider("ru", 0.45)
    try:
        warmup_started = time.monotonic()
        await provider.warmup()
        print(f"warmup_ms={round((time.monotonic() - warmup_started) * 1000)}")
        for index in range(runs):
            print(f"run_{index + 1}")
            await run_once(provider, image_bytes, dump_result=dump_result)
    finally:
        provider.close()


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("jpeg", type=Path)
    parser.add_argument("--runs", type=int, default=2)
    parser.add_argument("--dump-result", action="store_true")
    args = parser.parse_args()
    asyncio.run(main(args.jpeg, max(1, args.runs), args.dump_result))
