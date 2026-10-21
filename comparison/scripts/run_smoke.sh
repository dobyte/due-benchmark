#!/usr/bin/env bash
set -euo pipefail

framework=${1:?framework name required}
addr=${BENCH_ADDR:-127.0.0.1:17001}
out=${BENCH_OUT:-results/${framework}-smoke.jsonl}
mkdir -p "$(dirname "$out")"
out="$(cd "$(dirname "$out")" && pwd)/$(basename "$out")"
case "$framework" in
  due|pitaya) protocol="$framework" ;;
  skynet|kbengine) protocol=length32 ;;
  *) echo "unknown framework: $framework" >&2; exit 1 ;;
esac
cd "$(dirname "$0")/../loadgen"
go run . -framework "$framework" -protocol "$protocol" -addr "$addr" -concurrency 4 -warmup 100 -requests 1000 -bytes 64 -out "$out"
