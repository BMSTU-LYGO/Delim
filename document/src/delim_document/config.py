"""Typed service configuration loaded from YAML and environment variables."""

from __future__ import annotations

import os
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import yaml


class ConfigError(ValueError):
    """Raised when service configuration is missing or invalid."""


@dataclass(frozen=True, slots=True)
class AppConfig:
    name: str
    env: str


@dataclass(frozen=True, slots=True)
class GRPCConfig:
    host: str
    port: int


@dataclass(frozen=True, slots=True)
class PostgresConfig:
    host: str
    port: int
    database: str
    sslmode: str
    min_connections: int
    max_connections: int
    user: str
    password: str


@dataclass(frozen=True, slots=True)
class StorageConfig:
    endpoint: str
    bucket: str
    use_ssl: bool
    access_key: str
    secret_key: str


@dataclass(frozen=True, slots=True)
class UploadConfig:
    max_size_mb: int

    @property
    def max_size_bytes(self) -> int:
        return self.max_size_mb * 1024 * 1024


@dataclass(frozen=True, slots=True)
class WorkerConfig:
    concurrency: int
    poll_interval_ms: int
    max_attempts: int
    retry_base_seconds: int
    stale_after_minutes: int


@dataclass(frozen=True, slots=True)
class OCRConfig:
    language: str
    confidence_threshold: float


@dataclass(frozen=True, slots=True)
class MetricsConfig:
    host: str
    port: int

    @property
    def enabled(self) -> bool:
        return self.port > 0


@dataclass(frozen=True, slots=True)
class PrivacyConfig:
    receipt_retention_days: int
    cleanup_interval_minutes: int
    cleanup_batch_size: int


@dataclass(frozen=True, slots=True)
class Config:
    app: AppConfig
    grpc: GRPCConfig
    postgres: PostgresConfig
    storage: StorageConfig
    upload: UploadConfig
    worker: WorkerConfig
    ocr: OCRConfig
    metrics: MetricsConfig
    privacy: PrivacyConfig


def _section(data: dict[str, Any], name: str) -> dict[str, Any]:
    value = data.get(name)
    if not isinstance(value, dict):
        raise ConfigError(f"missing or invalid configuration section: {name}")
    return value


def _required(section: dict[str, Any], key: str, path: str, expected: type) -> Any:
    value = section.get(key)
    if expected is str:
        if not isinstance(value, str) or not value.strip():
            raise ConfigError(f"missing or invalid configuration value: {path}")
        return value
    if expected is int:
        if isinstance(value, bool) or not isinstance(value, int) or value <= 0:
            raise ConfigError(f"missing or invalid configuration value: {path}")
        return value
    if expected is bool:
        if not isinstance(value, bool):
            raise ConfigError(f"missing or invalid configuration value: {path}")
        return value
    raise TypeError(f"unsupported configuration type for {path}")


def _secret(name: str) -> str:
    value = os.getenv(name)
    if not value:
        raise ConfigError(f"missing required environment variable: {name}")
    return value


def _required_float(section: dict[str, Any], key: str, path: str) -> float:
    value = section.get(key)
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        raise ConfigError(f"missing or invalid configuration value: {path}")
    return float(value)


def load_config(path: str | Path) -> Config:
    config_path = Path(path)
    try:
        loaded = yaml.safe_load(config_path.read_text(encoding="utf-8"))
    except OSError as exc:
        raise ConfigError(f"cannot read configuration file {config_path}: {exc}") from exc
    except yaml.YAMLError as exc:
        raise ConfigError(f"invalid YAML in configuration file {config_path}: {exc}") from exc
    if not isinstance(loaded, dict):
        raise ConfigError("configuration root must be a mapping")

    app = _section(loaded, "app")
    grpc = _section(loaded, "grpc")
    postgres = _section(loaded, "postgres")
    storage = _section(loaded, "storage")
    upload = _section(loaded, "upload")
    worker = _section(loaded, "worker")
    ocr = _section(loaded, "ocr")
    metrics = _section(loaded, "metrics")
    privacy_value = loaded.get("privacy")
    privacy = (
        privacy_value
        if isinstance(privacy_value, dict)
        else {
            "receipt_retention_days": 30,
            "cleanup_interval_minutes": 360,
            "cleanup_batch_size": 200,
        }
    )
    receipt_retention_days = _required(
        privacy, "receipt_retention_days", "privacy.receipt_retention_days", int
    )
    cleanup_interval_minutes = _required(
        privacy, "cleanup_interval_minutes", "privacy.cleanup_interval_minutes", int
    )
    cleanup_batch_size = _required(
        privacy, "cleanup_batch_size", "privacy.cleanup_batch_size", int
    )

    sslmode = _required(postgres, "sslmode", "postgres.sslmode", str)
    if sslmode not in {"disable", "allow", "prefer", "require", "verify-ca", "verify-full"}:
        raise ConfigError("invalid configuration value: postgres.sslmode")

    grpc_port = _required(grpc, "port", "grpc.port", int)
    postgres_port = _required(postgres, "port", "postgres.port", int)
    metrics_port = _required(metrics, "port", "metrics.port", int)
    if grpc_port > 65535 or postgres_port > 65535:
        raise ConfigError("port must be between 1 and 65535")
    if metrics_port > 65535:
        raise ConfigError("metrics.port must be between 0 and 65535")
    metrics_host_value = metrics.get("host", "0.0.0.0")
    if not isinstance(metrics_host_value, str) or not metrics_host_value.strip():
        raise ConfigError("missing or invalid configuration value: metrics.host")
    metrics_host = metrics_host_value.strip()
    if metrics_port == 0 and metrics_host not in {"", "0.0.0.0"}:
        raise ConfigError("metrics.host is not used when metrics.port is 0")
    min_connections = _required(
        postgres, "min_connections", "postgres.min_connections", int
    )
    max_connections = _required(
        postgres, "max_connections", "postgres.max_connections", int
    )
    if min_connections > max_connections:
        raise ConfigError("postgres.min_connections cannot exceed max_connections")
    confidence_threshold = _required_float(
        ocr, "confidence_threshold", "ocr.confidence_threshold"
    )
    if not 0.0 <= confidence_threshold <= 1.0:
        raise ConfigError("ocr.confidence_threshold must be between 0 and 1")
    worker_concurrency = _required(
        worker, "concurrency", "worker.concurrency", int
    )
    if worker_concurrency != 1:
        raise ConfigError("worker.concurrency must be 1")

    return Config(
        app=AppConfig(
            name=_required(app, "name", "app.name", str),
            env=_required(app, "env", "app.env", str),
        ),
        grpc=GRPCConfig(
            host=_required(grpc, "host", "grpc.host", str),
            port=grpc_port,
        ),
        postgres=PostgresConfig(
            host=_required(postgres, "host", "postgres.host", str),
            port=postgres_port,
            database=_required(postgres, "database", "postgres.database", str),
            sslmode=sslmode,
            min_connections=min_connections,
            max_connections=max_connections,
            user=_secret("POSTGRES_USER"),
            password=_secret("POSTGRES_PASSWORD"),
        ),
        storage=StorageConfig(
            endpoint=_required(storage, "endpoint", "storage.endpoint", str),
            bucket=_required(storage, "bucket", "storage.bucket", str),
            use_ssl=_required(storage, "use_ssl", "storage.use_ssl", bool),
            access_key=_secret("MINIO_ROOT_USER"),
            secret_key=_secret("MINIO_ROOT_PASSWORD"),
        ),
        upload=UploadConfig(
            max_size_mb=_required(upload, "max_size_mb", "upload.max_size_mb", int),
        ),
        worker=WorkerConfig(
            concurrency=worker_concurrency,
            poll_interval_ms=_required(
                worker, "poll_interval_ms", "worker.poll_interval_ms", int
            ),
            max_attempts=_required(
                worker, "max_attempts", "worker.max_attempts", int
            ),
            retry_base_seconds=_required(
                worker, "retry_base_seconds", "worker.retry_base_seconds", int
            ),
            stale_after_minutes=_required(
                worker, "stale_after_minutes", "worker.stale_after_minutes", int
            ),
        ),
        ocr=OCRConfig(
            language=_required(ocr, "language", "ocr.language", str),
            confidence_threshold=confidence_threshold,
        ),
        metrics=MetricsConfig(
            host=metrics_host,
            port=metrics_port,
        ),
        privacy=PrivacyConfig(
            receipt_retention_days=receipt_retention_days,
            cleanup_interval_minutes=cleanup_interval_minutes,
            cleanup_batch_size=cleanup_batch_size,
        ),
    )
