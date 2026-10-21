#!/usr/bin/env bash
set -euo pipefail
printf 'host: '; hostname
uname -srmo
lscpu
free -h
printf '\nTools:\n'
for tool in go gcc g++ make cmake python3 lua luajit docker; do
  command -v "$tool" || true
done
printf '\nProcesses:\n'
ps -eo pid,pcpu,pmem,args --sort=-pcpu | head -n 20
