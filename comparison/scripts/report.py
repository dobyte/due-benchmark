"""Validate a complete suite and write a report alongside the generated charts."""
import argparse
from collections import defaultdict
from datetime import datetime, timedelta
import itertools
import json
from pathlib import Path
import statistics


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("results")
    args = parser.parse_args()
    root = Path(args.results)
    metadata = json.loads((root / "metadata.json").read_text())
    rows = [json.loads(line) for line in (root / "results.jsonl").read_text().splitlines() if line.strip()]
    expected = set(itertools.product(["skynet", "pitaya", "kbengine", "due"], [64, 1024, 4096], [32, 128, 512], range(metadata["repeats"])))
    actual = [(r["framework"], r["payload_bytes"], r["concurrency"], r["repeat"]) for r in rows]
    if metadata["smoke"] or len(actual) != len(set(actual)) or set(actual) != expected:
        raise SystemExit("incomplete/duplicate suite: refusing final report")
    if any(r["failed"] or r["success"] != metadata["requests"] or r["requests"] != metadata["requests"] for r in rows):
        raise SystemExit("failed/inconsistent suite")
    groups = defaultdict(list)
    for row in rows:
        groups[(row["framework"], row["payload_bytes"], row["concurrency"])].append(row)
    table = ["| 框架 | QPS 中位数（min–max） | P99 ms | P99.9 ms | 服务 CPU % | 服务 RSS MiB |",
             "|---|---:|---:|---:|---:|---:|"]
    for framework in ["skynet", "pitaya", "kbengine", "due"]:
        values = groups[(framework, 1024, 128)]
        med = lambda key: statistics.median([r[key] for r in values])
        table.append("| {} | {:,.0f} ({:,.0f}–{:,.0f}) | {:.3f} | {:.3f} | {:.1f} | {:.1f} |".format(
            framework, med("qps"), min(r["qps"] for r in values), max(r["qps"] for r in values),
            med("p99_us") / 1000, med("p999_us") / 1000, med("server_cpu_pct"), med("server_peak_rss_mib")))
    unstable = [key for key, values in groups.items()
                if (max(r["qps"] for r in values) - min(r["qps"] for r in values)) / statistics.median(r["qps"] for r in values) > .2]
    short = sum(r["elapsed_ms"] < 1000 for r in rows)
    starts = [datetime.fromisoformat(r["started_at"].replace("Z", "+00:00")) for r in rows]
    ends = [start + timedelta(milliseconds=r["elapsed_ms"]) for start, r in zip(starts, rows)]
    text = """# Skynet / Pitaya / KBEngine / Due 单机分布式 TCP 测评

## 数据完整性

测量窗口：{begin} 至 {end}（UTC）。完成 {points} 个点，共 {success:,} 次正式请求，失败或跳过请求为 0；另有每点 10,000 次预热未计入延迟与 QPS。

环境：i5-10400F（6 核 12 线程）、32GB RAM、CentOS 7、Linux 3.10。网关与业务进程各可使用一个物理核的两个兄弟线程；压测器使用另外两个物理核。Go 1.27.1、GCC/G++ 4.8.5。主机已有服务保持运行，CPU 亲和性不等于全机独占。

每种框架覆盖 64/1024/4096 字节 × 32/128/512 个持久连接，三轮重复。每连接只有一个未完成请求。图中点为三轮中位数，误差线为 min–max；P99/P99.9 为三轮各自分位数的中位数，并非合并分位数。

## 同一条件示例：1024 字节、128 连接

{table}

此表仅展示一个相同负载条件，不以不同条件下的最高 QPS 构造排名。全部 36 组统计见 [summary.csv](charts/summary.csv)，108 个原始点见 [results.jsonl](results.jsonl)，运行配置见 [metadata.json](metadata.json)。

## 图表

![吞吐量](charts/qps.png)

![P99 延迟](charts/p99_us.png)

![P99.9 延迟](charts/p999_us.png)

![服务 CPU](charts/server_cpu_pct.png)

![服务 RSS](charts/server_peak_rss_mib.png)

![压测器 CPU](charts/loadgen_cpu_pct.png)

每张图均另存 SVG，可用于后续排版。

## 结论适用范围与限制

- 结论只适用于本次明确的适配方案、固定源码、CPU 配额、单机 loopback 及闭环 echo 负载；不代表跨机器部署、真实游戏逻辑或框架所有模式的总体排名。
- Skynet 使用 skynet.socket 自定义分帧及原生 cluster.call；Pitaya 使用原生 TCP 协议和 gRPC；Due 使用原生 Gate 协议和 DRPC；KBEngine 使用官方 poller 的 TCP 扩展和原生 BaseApp/CellApp EntityCall。KBEngine 没有测量原生玩家登录及 Proxy 客户端协议。
- 服务 CPU/RSS 只统计网关与业务进程，不包含 Redis、etcd、MySQL、KBEngine 管理服务；不能据此声称整套集群的资源成本。RSS 采样间隔 20ms，共享内存页可能重复计入。
- 每轮每个框架启动一组新服务进程，该轮的九个负载点复用这组进程。RSS 会受前序负载、内存分配器和 GC 保留影响，不是每点冷启动内存。
- CPU 从测量边界附近读取进程时钟 tick，存在调度和量化误差；{short} 个点持续时间不足 1 秒，这些点的 CPU 指标尤其应谨慎解释。保留压测器 CPU 图，便于审视发压端瓶颈。
- 压测器总 CPU 未用满配额不能排除其事件循环等局部瓶颈；本报告给出当前配置下的链路表现，不声称测得各框架的理论吞吐上限。
- {unstable} 组测试的 QPS 三轮 min–max 跨度超过中位数的 20%；误差线完整保留，三轮样本不足以给出统计显著性或置信区间。
- Skynet 使用 glibc malloc（归档未带 jemalloc 子模块），GCC 4.8 兼容构建参数见适配器 build.sh；KBEngine 使用其上游默认构建及内嵌 Python，不能将差异全归因于网络层。
- 早期试运行曾发现 Pitaya 独立 logger 默认 Debug；修复为 Error 后重新进行完整测量。本报告只读取最终目录，未混入这些试运行数据。

完整方法、构建步骤见 [comparison/README.md](../../README.md)。[源码版本](versions.json)、[Due 源码指纹](due-manifest.json)、[原始运行日志](raw-logs.tar.gz)、主机信息及 [实际测试二进制 SHA256](binaries-sha256.txt) 随结果保存。
""".format(begin=min(starts).isoformat(), end=max(ends).isoformat(), points=len(rows),
           success=sum(r["success"] for r in rows), table="\n".join(table), short=short, unstable=len(unstable))
    (root / "REPORT.md").write_text(text, encoding="utf-8")
    print("Validated {} points, {} successful requests".format(len(rows), sum(r["success"] for r in rows)))


if __name__ == "__main__":
    main()
