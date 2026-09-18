#!/usr/bin/env python3
"""Create a source-only, reproducible hackathon archive and SHA-256 checksum."""

from __future__ import annotations

import gzip
import hashlib
import io
import subprocess
import tarfile
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
OUTPUT = ROOT / "docs/hackathon/delim-source.tar.gz"
EXTRA = {
    "DATA-API.yaml",
    "docs/hackathon/submission.md",
    "internal/core/usecase/group_activity_test.go",
    "internal/gateway/delivery/http/groups_budget_test.go",
    "migrations/core/003_group_activities.sql",
    "scripts/package-hackathon.py",
}
EXCLUDE = {"core", "gateway", "smoke"}


def main() -> None:
    tracked = subprocess.check_output(["git", "ls-files", "-z"], cwd=ROOT).decode().split("\0")
    deleted = set(subprocess.check_output(["git", "ls-files", "--deleted", "-z"], cwd=ROOT).decode().split("\0"))
    names = sorted(({name for name in tracked if name} - deleted) | EXTRA)
    OUTPUT.parent.mkdir(parents=True, exist_ok=True)
    with OUTPUT.open("wb") as raw, gzip.GzipFile(fileobj=raw, mode="wb", mtime=0, filename="") as zipped:
        with tarfile.open(fileobj=zipped, mode="w") as archive:
            for name in names:
                if name in EXCLUDE:
                    continue
                path = ROOT / name
                if not path.is_file():
                    raise FileNotFoundError(path)
                data = path.read_bytes()
                info = tarfile.TarInfo(name)
                info.size = len(data)
                info.mode = 0o755 if path.stat().st_mode & 0o111 else 0o644
                info.mtime = info.uid = info.gid = 0
                archive.addfile(info, io.BytesIO(data))
    digest = hashlib.sha256(OUTPUT.read_bytes()).hexdigest()
    (OUTPUT.parent / "delim-source.sha256").write_text(f"{digest}  {OUTPUT.name}\n")
    print(f"{OUTPUT.relative_to(ROOT)}: sha256 {digest}")


if __name__ == "__main__":
    main()
