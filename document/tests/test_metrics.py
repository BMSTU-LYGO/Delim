"""Tests for the Document service metrics module."""

from __future__ import annotations

import unittest
from dataclasses import dataclass

from delim_document.metrics import (
    EXPORT_FORMATS,
    MINIO_OPERATIONS,
    OCR_STATES,
    Recorder,
    build_recorder,
    noop_recorder,
)
from prometheus_client import CollectorRegistry


@dataclass(frozen=True, slots=True)
class _Config:
    host: str = "127.0.0.1"
    port: int = 0

    @property
    def enabled(self) -> bool:
        return self.port > 0


def _new_recorder() -> Recorder:
    # Build with an isolated registry so the no-op helpers below stay
    # separated from any global state introduced by other suites.
    return Recorder(_Config())


def _samples(
    recorder: Recorder, metric_name: str
) -> dict[tuple[str, ...], float]:
    return _samples_with_registry(recorder, metric_name, recorder.registry)


def _samples_with_registry(
    recorder: Recorder, metric_name: str, registry: CollectorRegistry
) -> dict[tuple[str, ...], float]:
    for metric in registry.collect():
        if metric_name not in {metric.name, metric.name + "_total"}:
            continue
        result: dict[tuple[str, ...], float] = {}
        for sample in metric.samples:
            if sample.name.endswith("_created"):
                continue
            result[tuple(sample.labels.values())] = sample.value
        return result
    return {}


class RecorderObservationTest(unittest.TestCase):
    def test_observe_grpc_success_increments_counter_with_zero_code(self) -> None:
        recorder = _new_recorder()
        recorder.observe_grpc("/doc.v1.DocumentService/Ping", "ok", 0.01, None)
        self.assertEqual(
            _samples(recorder, "delim_grpc_requests_total"),
            {("/doc.v1.DocumentService/Ping", "success", "ok"): 1.0},
        )

    def test_observe_grpc_error_normalises_code(self) -> None:
        recorder = _new_recorder()
        recorder.observe_grpc(
            "/doc.v1.DocumentService/Ping",
            "not_found",
            0.02,
            RuntimeError("missing"),
        )
        self.assertEqual(
            _samples(recorder, "delim_grpc_requests_total"),
            {("/doc.v1.DocumentService/Ping", "error", "not_found"): 1.0},
        )

    def test_observe_ocr_job_rejects_unknown_states(self) -> None:
        recorder = _new_recorder()
        recorder.observe_ocr_job("completed")
        recorder.observe_ocr_job("unknown-state")
        recorder.observe_ocr_job("failed")
        # Unknown states are normalised to "failed" so the label set stays
        # bounded (mirrors the Go recorder behaviour).
        self.assertEqual(
            _samples(recorder, "delim_ocr_jobs_total"),
            {("completed",): 1.0, ("failed",): 2.0},
        )

    def test_observe_export_job_normalises_format_and_result(self) -> None:
        recorder = _new_recorder()
        recorder.observe_export_job("PDF", None)
        recorder.observe_export_job("xlsx", RuntimeError("nope"))
        recorder.observe_export_job("docx", None)
        self.assertEqual(
            _samples(recorder, "delim_export_jobs_total"),
            {
                ("pdf", "success"): 1.0,
                ("xlsx", "error"): 1.0,
                ("unspecified", "success"): 1.0,
            },
        )

    def test_observe_minio_error_normalises_operation(self) -> None:
        recorder = _new_recorder()
        recorder.observe_minio_error("Put", None)
        recorder.observe_minio_error("rm", RuntimeError("nope"))
        recorder.observe_minio_error("delete", RuntimeError("nope"))
        self.assertEqual(
            _samples(recorder, "delim_minio_errors_total"),
            {
                ("put", "success"): 1.0,
                ("other", "error"): 1.0,
                ("delete", "error"): 1.0,
            },
        )

    def test_observe_grpc_client_error_skips_successes(self) -> None:
        recorder = _new_recorder()
        recorder.observe_grpc_client_error("core", "Ping", None)
        recorder.observe_grpc_client_error("core", "Ping", RuntimeError("boom"))
        self.assertEqual(
            _samples(recorder, "delim_grpc_client_errors_total"),
            {("core", "Ping", "error"): 1.0},
        )

    def test_observe_ocr_duration_records_bounded_result(self) -> None:
        recorder = _new_recorder()
        recorder.observe_ocr_duration("completed", 1.5)
        recorder.observe_ocr_duration("bogus", 0.1)
        # Both observations are recorded; the "bogus" value is normalised to
        # "failed" so the histogram label set stays bounded.
        for metric in recorder.registry.collect():
            if metric.name != "delim_ocr_duration_seconds":
                continue
            labels = [
                tuple(sample.labels.values())[0]
                for sample in metric.samples
                if sample.name.endswith("_count")
            ]
            self.assertEqual(set(labels), {"completed", "failed"})
            break
        else:
            self.fail("ocr duration histogram not found")

    def test_endpoint_disabled_when_port_zero(self) -> None:
        recorder = _new_recorder()
        recorder.start_endpoint()
        self.assertFalse(recorder.enabled)

    def test_recorder_does_not_use_global_registry(self) -> None:
        recorder_a = _new_recorder()
        recorder_b = _new_recorder()
        recorder_a.observe_ocr_job("completed")
        recorder_b.observe_ocr_job("completed")
        self.assertEqual(
            _samples_with_registry(
                recorder_a, "delim_ocr_jobs_total", recorder_a.registry
            ),
            {("completed",): 1.0},
        )
        self.assertEqual(
            _samples_with_registry(
                recorder_b, "delim_ocr_jobs_total", recorder_b.registry
            ),
            {("completed",): 1.0},
        )

    def test_label_allowlists_are_frozen(self) -> None:
        self.assertIsInstance(OCR_STATES, frozenset)
        self.assertIsInstance(EXPORT_FORMATS, frozenset)
        self.assertIsInstance(MINIO_OPERATIONS, frozenset)

    def test_noop_recorder_returns_disabled_instance(self) -> None:
        recorder = noop_recorder()
        self.assertFalse(recorder.enabled)
        recorder.observe_ocr_job("completed")


class FactorySmokeTest(unittest.TestCase):
    def test_build_recorder_uses_provided_config(self) -> None:
        recorder = build_recorder(_Config())
        self.assertFalse(recorder.enabled)


if __name__ == "__main__":
    unittest.main()