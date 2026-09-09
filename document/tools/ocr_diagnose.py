"""Minimal PaddleOCR runtime diagnostic (Block 2.1).

Isolates the native OCR crash path without the rest of Delim:

    environment report -> PaddleOCR init -> single inference -> exit

The environment report is flushed *before* any native call, so the diagnostic
survives a hard SIGSEGV inside Paddle's C++ runtime (exit code 139). Run it in
the document environment (native deps installed):

    python document/tools/ocr_diagnose.py [--image PATH]

Exit codes: 0 = inference OK, 1 = Python-level error, 139/negative = native crash.
"""

from __future__ import annotations

import argparse
import platform
import sys
from pathlib import Path


def report_environment() -> None:
    print("=" * 64)
    print("PaddleOCR runtime diagnostic — environment")
    print("=" * 64)
    print(f"python          : {sys.version.split()[0]} ({sys.executable})")
    print(f"os / machine    : {platform.system()} {platform.machine()}")
    print(f"platform        : {platform.platform()}")
    for mod in ("paddle", "paddleocr", "numpy", "cv2"):
        try:
            m = __import__(mod)
            print(f"{mod:<15}: {getattr(m, '__version__', '?')}")
        except Exception as exc:  # noqa: BLE001
            print(f"{mod:<15}: import error: {exc!r}")
    try:
        import paddle

        try:
            print(f"paddle compiled: {paddle.version.commit if hasattr(paddle.version, 'commit') else '-'}")
        except Exception:
            pass
        print(f"paddle build    : {getattr(paddle.version, 'build', '-')}")
    except Exception:
        pass
    _cpu_flags()
    sys.stdout.flush()


def _cpu_flags() -> None:
    cpuinfo = Path("/proc/cpuinfo")
    if cpuinfo.exists():
        for line in cpuinfo.read_text().splitlines():
            if line.lower().startswith(("features", "flags")):
                print(f"cpu features    : {line[:300]}")
                break
    else:
        print("cpu features    : (no /proc/cpuinfo; non-Linux host?)")


def run_inference(image_path: Path) -> int:
    if not image_path.exists():
        print(f"[diag] image not found: {image_path}", file=sys.stderr)
        return 1
    import cv2

    print("-" * 64)
    print("[diag] importing PaddleOCR ...")
    sys.stdout.flush()
    from paddleocr import PaddleOCR

    # Mirror the production provider (document/src/delim_document/ocr/paddle.py)
    # so this reproduces the exact native code path that crashes.
    print("[diag] initialising PaddleOCR (lang=ru, device=cpu, mkldnn off) ...")
    sys.stdout.flush()
    ocr = PaddleOCR(
        lang="ru",
        ocr_version="PP-OCRv5",
        text_detection_model_name="PP-OCRv5_mobile_det",
        text_recognition_model_name="eslav_PP-OCRv5_mobile_rec",
        device="cpu",
        enable_mkldnn=False,
        use_doc_orientation_classify=False,
        use_doc_unwarping=False,
        use_textline_orientation=False,
        text_rec_score_thresh=0.45,
    )
    image = cv2.imread(str(image_path))
    if image is None:
        print(f"[diag] could not decode image: {image_path}", file=sys.stderr)
        return 1
    print(f"[diag] running inference on {image_path.name} ...")
    sys.stdout.flush()
    regions = 0
    for result in ocr.predict(image):
        regions += len(result.get("rec_texts", ()) or ())
    print(f"[diag] inference OK: {regions} text regions")
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(description="PaddleOCR runtime diagnostic")
    parser.add_argument(
        "--image",
        type=Path,
        default=Path(__file__).resolve().parents[1] / "testdata" / "good_ru.png",
        help="image to run inference on",
    )
    args = parser.parse_args()
    report_environment()
    try:
        return run_inference(args.image)
    except Exception as exc:  # noqa: BLE001 - any python-level failure is reported
        print(f"[diag] inference failed (python): {exc!r}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
