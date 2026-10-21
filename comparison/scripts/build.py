"""Fetch pinned sources and cross-compile Go tools; build C/C++ on the target host."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tarfile
import urllib.request

root = Path(__file__).resolve().parents[2]
parser = argparse.ArgumentParser()
parser.add_argument("--fetch", action="store_true", help="download pinned upstream source archives")
args = parser.parse_args()
versions = json.loads((root / "comparison/configs/versions.json").read_text())
cache = root / ".cache/frameworks"
cache.mkdir(parents=True, exist_ok=True)
archives = {}
for name in ("skynet", "pitaya", "kbengine"):
    entry = versions[name]
    commit = entry["commit"]
    repo = entry["repository"].split("github.com/")[1]
    archive = cache / (name + ".tar.gz")
    if args.fetch:
        request = urllib.request.Request("https://api.github.com/repos/{}/tarball/{}".format(repo, commit),
                                         headers={"User-Agent": "framework-benchmark"})
        with urllib.request.urlopen(request, timeout=180) as response, archive.open("wb") as handle:
            import shutil
            shutil.copyfileobj(response, handle)
        # Official pinned upstream archive, extracted only inside this cache.
        with tarfile.open(str(archive)) as bundle:
            base = cache.resolve()
            for member in bundle.getmembers():
                destination = (cache / member.name).resolve()
                if base != destination and base not in destination.parents:
                    raise SystemExit("archive entry escapes source cache")
                if member.issym() or member.islnk():
                    link = (destination.parent / member.linkname).resolve()
                    if base != link and base not in link.parents:
                        raise SystemExit("archive link escapes source cache")
            bundle.extractall(str(cache))
    if archive.exists():
        archives[name] = {"commit": commit, "sha256": hashlib.sha256(archive.read_bytes()).hexdigest()}
env = dict(os.environ, GOOS="linux", GOARCH="amd64", CGO_ENABLED="0", GOCACHE=str(root / ".cache/go-build"))
output = root / "comparison/bin"
output.mkdir(exist_ok=True)
for directory, target, package in [("loadgen", "loadgen", "."), ("due", "due-bench", "."),
                                    ("pitaya", "pitaya-bench", "."), ("pitaya", "benchmark-etcd", "./infra")]:
    subprocess.run(["go", "build", "-o", str(output / target), package], cwd=str(root / "comparison" / directory), env=env, check=True)
manifest = {path.name: hashlib.sha256(path.read_bytes()).hexdigest() for path in output.iterdir() if path.is_file()}
(root / "comparison/configs/build-manifest.json").write_text(json.dumps({"archives": archives, "binaries": manifest}, indent=2))
print("Go tools built for Linux amd64; build Skynet and KBEngine on Linux per adapter READMEs")
