"""Fingerprint the exact local Due source used for the adapter build."""
import hashlib
import json
from pathlib import Path
import subprocess
import sys

source = Path(sys.argv[1]).resolve()
destination = Path(sys.argv[2]).resolve()
files = {}
for path in source.rglob("*"):
    relative = path.relative_to(source)
    if any(part in {".git", ".cache", "benchmark"} for part in relative.parts):
        continue
    if path.is_file() and (path.suffix == ".go" or path.name in {"go.mod", "go.sum"}):
        files[relative.as_posix()] = hashlib.sha256(path.read_bytes()).hexdigest()
manifest = {
    "source": str(source),
    "commit": subprocess.check_output(["git", "-C", str(source), "rev-parse", "HEAD"], universal_newlines=True).strip(),
    "status": subprocess.check_output(["git", "-C", str(source), "status", "--short"], universal_newlines=True),
    "files_sha256": files,
}
destination.write_text(json.dumps(manifest, indent=2), encoding="utf-8")
print("{} source files fingerprinted".format(len(files)))
