"""OCR regression check over the fixtures in ``document/testdata``.

By default this exercises the real deterministic pipeline
(decode -> preprocess -> QR read -> fiscal parse -> receipt parse -> confidence
selection) against small synthetic fixtures using a manifest-driven stub OCR
provider, so it does not require the Paddle model or heavy inference.

With ``--live`` the real PaddleOCR provider is used instead and only
structural invariants are asserted (pipeline completes, confidence in range,
service lines never become items), because exact text is not stable across
Paddle versions.

Run inside the document environment:

    python document/tools/ocr_check.py            # deterministic pipeline
    python document/tools/ocr_check.py --live     # real Paddle inference
"""

from __future__ import annotations

import argparse
import asyncio
import json
from pathlib import Path
import sys

import numpy as np

from delim_document.image.decoder import decode_image
from delim_document.image.preprocess import preprocess_receipt
from delim_document.ocr.confidence import build_ocr_result
from delim_document.ocr.provider import OCRLine
from delim_document.qr.fiscal import parse_fiscal_qr
from delim_document.qr.reader import ReceiptQRReader

TESTDATA = Path(__file__).resolve().parents[1] / "testdata"


def _to_lines(stub_lines: list[dict], confidence_default: float) -> tuple[OCRLine, ...]:
    out: list[OCRLine] = []
    for entry in stub_lines:
        confidence = float(entry.get("confidence", confidence_default))
        y = float(entry["y"])
        bbox = ((40, y), (560, y), (560, y + 24), (40, y + 24))
        out.append(OCRLine(text=str(entry["text"]), confidence=confidence, bbox=bbox))
    return tuple(out)


class _ManifestProvider:
    def __init__(self, lines: tuple[OCRLine, ...]) -> None:
        self._lines = lines

    async def recognize(self, image: np.ndarray) -> tuple[OCRLine, ...]:
        del image
        return self._lines


class _Pipeline:
    def __init__(self) -> None:
        self._reader = ReceiptQRReader()

    async def run(self, image_bytes: bytes, lines: tuple[OCRLine, ...]):
        original = decode_image(image_bytes)
        processed = preprocess_receipt(original)
        qr = self._reader.read(original, processed.enhanced, processed.grayscale)
        fiscal = parse_fiscal_qr(qr.raw_payload) if qr.raw_payload else None
        result = build_ocr_result(lines, fiscal, qr.raw_payload)
        return qr, result


class _LivePipeline(_Pipeline):
    async def run(self, image_bytes: bytes, lines: tuple[OCRLine, ...]):
        from delim_document.ocr.paddle import PaddleOCRProvider

        provider = PaddleOCRProvider("ru", 0.45, 1)
        original = decode_image(image_bytes)
        processed = preprocess_receipt(original)
        qr = self._reader.read(original, processed.enhanced, processed.grayscale)
        fiscal = parse_fiscal_qr(qr.raw_payload) if qr.raw_payload else None
        recognized = await provider.recognize(processed.enhanced)
        return qr, build_ocr_result(recognized, fiscal, qr.raw_payload)


def _check_result(name: str, result, fixture: dict, manifest: dict, *, live: bool) -> list[str]:
    problems: list[str] = []
    forbidden = [value.lower() for value in manifest["forbidden_item_substrings"]]
    for item in result.items:
        lowered = item.name.lower()
        if any(token in lowered for token in forbidden):
            problems.append(f"{name}: service line leaked into item {item.name!r}")
        if not 0.0 <= item.confidence <= 1.0:
            problems.append(f"{name}: item confidence out of range")
    if not 0.0 <= result.confidence <= 1.0:
        problems.append(f"{name}: confidence out of range {result.confidence}")
    if live:
        # Exact text/total is not stable across Paddle versions; only require the
        # pipeline produced a parseable result above.
        return problems
    if result.total_minor != fixture["expected_total_minor"]:
        problems.append(
            f"{name}: total {result.total_minor} != expected {fixture['expected_total_minor']}"
        )
    if result.date is None:
        problems.append(f"{name}: expected a date from the fixture lines")
    if not result.items:
        problems.append(f"{name}: expected at least one parsed item")
    if bool(result.total_mismatch) != fixture["expect_mismatch"]:
        problems.append(
            f"{name}: total_mismatch={result.total_mismatch} != expected {fixture['expect_mismatch']}"
        )
    return problems


def main() -> int:
    parser = argparse.ArgumentParser(description="OCR regression check")
    parser.add_argument("--live", action="store_true", help="use real PaddleOCR inference")
    args = parser.parse_args()

    manifest = json.loads((TESTDATA / "manifest.json").read_text(encoding="utf-8"))
    pipeline = _LivePipeline() if args.live else _Pipeline()

    async def run() -> int:
        problems: list[str] = []
        for name, fixture in manifest["fixtures"].items():
            path = TESTDATA / name
            if not path.exists():
                problems.append(f"{name}: fixture file missing")
                continue
            image_bytes = path.read_bytes()
            lines = _to_lines(fixture["stub_lines"], manifest["stub_line_confidence"])
            try:
                qr, result = await pipeline.run(image_bytes, lines)
            except Exception as exc:  # noqa: BLE001 - report any pipeline crash
                problems.append(f"{name}: pipeline crashed: {exc!r}")
                continue
            if bool(fixture["expect_qr"]) != qr.decoded:
                problems.append(
                    f"{name}: QR decoded={qr.decoded}, expected={fixture['expect_qr']}"
                )
            problems.extend(_check_result(name, result, fixture, manifest, live=args.live))
            status = "ok" if not any(problem.startswith(name) for problem in problems) else "FAIL"
            print(f"  {name}: qr={qr.decoded} total={result.total_minor} "
                  f"items={len(result.items)} conf={result.confidence:.2f} [{status}]")
        if args.live:
            print("document-ocr-check: LIVE mode (Paddle) — structural invariants only")
        else:
            print("document-ocr-check: deterministic stub pipeline")
        if problems:
            print("document-ocr-check: FAILED")
            for problem in problems:
                print(f"  - {problem}")
            return 1
        print("document-ocr-check: ok")
        return 0

    return asyncio.run(run())


if __name__ == "__main__":
    sys.exit(main())
