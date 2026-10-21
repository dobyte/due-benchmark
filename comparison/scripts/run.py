#!/usr/bin/env python3
"""Linux single-host runner. Stops only processes/containers created by this run."""
import argparse
import itertools
import json
import os
from pathlib import Path
import random
import signal
import socket
import subprocess
import threading
import time
import uuid

ROOT = Path(__file__).resolve().parents[1]
CPUS = {"gateway": "0,6", "worker": "1,7", "loadgen": "2,3,8,9", "infra": "4,10"}
LOADGENS = []


def port_open(port):
    with socket.socket() as sock:
        sock.settimeout(.2)
        return sock.connect_ex(("127.0.0.1", port)) == 0


def wait_port(port, processes):
    deadline = time.monotonic() + 35
    while time.monotonic() < deadline:
        if any(p.poll() is not None for p in processes):
            raise RuntimeError("server exited; inspect run logs")
        if port_open(port):
            return
        time.sleep(.2)
    raise RuntimeError("port {} not ready".format(port))


def stats(pids):
    ticks = rss = 0
    for pid in pids:
        # comm can contain spaces/parentheses; stat fields start after final ')'.
        fields = Path("/proc/{}/stat".format(pid)).read_text().rsplit(")", 1)[1].split()
        ticks += int(fields[11]) + int(fields[12])
        rss += int(fields[21]) * os.sysconf("SC_PAGE_SIZE")
    return ticks / os.sysconf("SC_CLK_TCK"), rss / (1024 ** 2)


class Group:
    def __init__(self, output):
        self.output = output
        self.processes = []
        self.handles = []

    def start(self, name, role, args, cwd=None, extra_env=None):
        handle = (self.output / (name + ".log")).open("w")
        self.handles.append(handle)
        env = dict(os.environ, GOMAXPROCS="2")
        env.update(extra_env or {})
        process = subprocess.Popen(["taskset", "-c", CPUS[role]] + list(map(str, args)),
                                   cwd=str(cwd or ROOT), env=env, stdout=handle,
                                   stderr=subprocess.STDOUT, start_new_session=True)
        self.processes.append(process)
        return process

    def close(self):
        for p in reversed(self.processes):
            if p.poll() is None:
                os.killpg(p.pid, signal.SIGTERM)
        for p in reversed(self.processes):
            try:
                p.wait(timeout=8)
            except subprocess.TimeoutExpired:
                os.killpg(p.pid, signal.SIGKILL)
                p.wait()
        for h in self.handles:
            h.close()


def measure(framework, protocol, size, concurrency, requests, warmup, processes, infra, output):
    command = ["taskset", "-c", CPUS["loadgen"], str(ROOT / "bin/loadgen"),
               "-framework", framework, "-protocol", protocol, "-bytes", str(size),
               "-concurrency", str(concurrency), "-requests", str(requests),
               "-warmup", str(warmup), "-timeout", "3s"]
    p = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                         universal_newlines=True, env=dict(os.environ, GOMAXPROCS="4"))
    LOADGENS.append(p)
    all_pids = [x.pid for x in processes]
    begin = end = None
    peak = [0.0]
    stop = threading.Event()

    def sample():
        while not stop.wait(.02):
            try:
                peak[0] = max(peak[0], stats(all_pids)[1])
            except FileNotFoundError:
                return

    sampler = threading.Thread(target=sample, daemon=True)
    for line in p.stderr:
        if line.strip() == "BENCH_MEASURE_BEGIN":
            begin = (time.monotonic(), stats(all_pids), stats([p.pid]), stats([infra.pid]))
            peak[0] = begin[1][1]
            sampler.start()
        elif line.strip() == "BENCH_MEASURE_END":
            end = (time.monotonic(), stats(all_pids), stats([p.pid]), stats([infra.pid]))
            stop.set()
        else:
            print(line.rstrip(), flush=True)
    data = p.stdout.read()
    code = p.wait()
    LOADGENS.remove(p)
    stop.set()
    if sampler.ident:
        sampler.join()
    if code or not begin or not end:
        raise RuntimeError("{} measurement failed ({}): {}".format(framework, code, data))
    row = json.loads(data)
    if row["failed"] or row["success"] != requests:
        raise RuntimeError("invalid benchmark point")
    seconds = end[0] - begin[0]
    row.update(server_cpu_pct=100 * (end[1][0] - begin[1][0]) / seconds,
               loadgen_cpu_pct=100 * (end[2][0] - begin[2][0]) / seconds,
               etcd_cpu_pct=100 * (end[3][0] - begin[3][0]) / seconds,
               server_peak_rss_mib=peak[0], server_pids=all_pids)
    return row


def main():
    def interrupt(signum, frame):
        raise KeyboardInterrupt()
    signal.signal(signal.SIGTERM, interrupt)
    signal.signal(signal.SIGINT, interrupt)
    parser = argparse.ArgumentParser()
    parser.add_argument("--frameworks", nargs="+", default=["skynet", "pitaya", "kbengine", "due"])
    parser.add_argument("--out", required=True)
    parser.add_argument("--smoke", action="store_true")
    parser.add_argument("--requests", type=int, default=100000)
    parser.add_argument("--repeats", type=int, default=3)
    args = parser.parse_args()
    if args.requests < 512 or args.repeats < 1 or len(set(args.frameworks)) != len(args.frameworks):
        parser.error("requests >= 512, repeats > 0, unique frameworks required")
    if set(args.frameworks) - {"skynet", "pitaya", "due", "kbengine"}:
        parser.error("unsupported adapter")
    output = Path(args.out).resolve()
    output.mkdir(parents=True, exist_ok=False)
    for port in (17001, 17002, 17003, 17379, 17380, 17389):
        if port_open(port):
            raise RuntimeError("port {} already occupied".format(port))
    metadata = {"cpu_affinity": CPUS, "requests": args.requests, "repeats": args.repeats,
                "smoke": args.smoke, "seed": 20261021, "cpu_percent_basis": "100% = one logical CPU",
                "cpu_window": "loadgen stderr begin/end markers; /proc clock ticks",
                "rss": "sum gateway+worker RSS, sampled every 20ms; shared pages counted per process",
                "infrastructure": "dedicated etcd, Redis and KBEngine MySQL; server metrics exclude infrastructure",
                "host_services_not_stopped": True,
                "frameworks": args.frameworks}
    (output / "metadata.json").write_text(json.dumps(metadata, indent=2))
    infra = Group(output)
    container = "framework-benchmark-redis-" + uuid.uuid4().hex[:10]
    redis_created = False
    mysql = container + "-mysql"
    mysql_created = False
    try:
        subprocess.run(["docker", "run", "-d", "--name", container, "--cpuset-cpus", CPUS["infra"],
                        "-p", "127.0.0.1:17389:6379", "redis:6.2.18", "redis-server",
                        "--save", "", "--appendonly", "no"], check=True)
        redis_created = True
        if "kbengine" in args.frameworks:
            if port_open(17406):
                raise RuntimeError("MySQL benchmark port occupied")
            subprocess.run(["docker", "run", "-d", "--name", mysql, "--cpuset-cpus", CPUS["infra"],
                            "-p", "127.0.0.1:17406:3306", "-e", "MYSQL_ROOT_PASSWORD=benchmark-local-only",
                            "-e", "MYSQL_DATABASE=framework_benchmark", "mysql:5.7"], check=True)
            mysql_created = True
            deadline = time.monotonic() + 90
            while subprocess.run(["docker", "exec", mysql, "mysql", "-uroot", "-pbenchmark-local-only",
                                  "-e", "SELECT 1"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode:
                if time.monotonic() > deadline:
                    raise RuntimeError("benchmark MySQL not ready")
                time.sleep(1)
        etcd = infra.start("etcd", "infra", [ROOT / "bin/benchmark-etcd", "-data", output / "etcd-data"])
        wait_port(17379, [etcd])
        for repeat in range(1 if args.smoke else args.repeats):
            order = args.frameworks[:]
            random.Random(20261021 + repeat).shuffle(order)
            for framework in order:
                servers = Group(output)
                try:
                    name = "{}-{}".format(framework, repeat)
                    if framework == "kbengine":
                        cwd = ROOT.parent / "src/kbengine"
                        assets = ROOT / "runtime/kbengine-assets"
                        env = {"KBE_ROOT": str(cwd), "KBE_RES_PATH": "{}:{}:{}:{}".format(
                            cwd / "kbe/res", assets, assets / "res", assets / "scripts"),
                            "KBE_BIN_PATH": str(cwd / "kbe/bin/server")}
                        roles = ["machine", "logger", "interfaces", "dbmgr", "baseappmgr", "cellappmgr", "baseapp", "cellapp", "loginapp"]
                        actors = {}
                        for index, component in enumerate(roles):
                            role = "gateway" if component == "baseapp" else "worker" if component == "cellapp" else "infra"
                            actors[component] = servers.start(name + "-" + component, role,
                                [cwd / "kbe/bin/server" / component, "--cid={}".format(900000000000000000 + index),
                                 "--gus={}".format(index + 1)], assets, env)
                            time.sleep(.5)
                        gateway, worker = actors["baseapp"], actors["cellapp"]
                        wait_port(17001, list(actors.values()))
                        protocol = "length32"
                    elif framework == "skynet":
                        cwd = ROOT.parent / "src/skynet"
                        worker = servers.start(name + "-worker", "worker", [cwd / "skynet", ROOT / "skynet/worker.conf"], cwd)
                        wait_port(17002, [worker])
                        gateway = servers.start(name + "-gateway", "gateway", [cwd / "skynet", ROOT / "skynet/gateway.conf"], cwd)
                        protocol = "length32"
                    elif framework == "pitaya":
                        worker = servers.start(name + "-worker", "worker", [ROOT / "bin/pitaya-bench"])
                        wait_port(17002, [worker])
                        gateway = servers.start(name + "-gateway", "gateway", [ROOT / "bin/pitaya-bench", "-frontend", "-rpc-port", "17003"])
                        protocol = "pitaya"
                    else:
                        worker = servers.start(name + "-worker", "worker", [ROOT / "bin/due-bench", "-role", "worker"])
                        wait_port(17002, [worker])
                        gateway = servers.start(name + "-gateway", "gateway", [ROOT / "bin/due-bench", "-role", "gateway"])
                        protocol = "due"
                    wait_port(17001, [worker, gateway])
                    time.sleep(4)  # service discovery and routing registration
                    points = [(64, 4)] if args.smoke else list(itertools.product([64, 1024, 4096], [32, 128, 512]))
                    random.Random(20261021 + repeat).shuffle(points)
                    for size, concurrency in points:
                        row = measure(framework, protocol, size, concurrency,
                                      1000 if args.smoke else args.requests,
                                      100 if args.smoke else 10000, [gateway, worker], etcd, output)
                        row["repeat"] = repeat
                        with (output / "results.jsonl").open("a") as handle:
                            handle.write(json.dumps(row) + "\n")
                finally:
                    servers.close()
    finally:
        for p in LOADGENS:
            if p.poll() is None:
                p.terminate()
                p.wait(timeout=5)
        infra.close()
        if redis_created:
            subprocess.run(["docker", "rm", "-f", container], check=True)
        if mysql_created:
            subprocess.run(["docker", "rm", "-fv", mysql], check=True)


if __name__ == "__main__":
    main()
