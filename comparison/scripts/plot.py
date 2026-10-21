#!/usr/bin/env python3
"""Plot successful runs only; show median and full repeat range, never pool P99s."""
import argparse
from collections import defaultdict
import csv
import json
from pathlib import Path
import statistics


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("input", nargs="+")
    parser.add_argument("--out-dir", required=True)
    args = parser.parse_args()
    rows = [json.loads(line) for source in args.input for line in Path(source).read_text().splitlines() if line.strip()]
    if not rows or any(r["failed"] or r["success"] != r["requests"] for r in rows):
        raise SystemExit("No data, or failed/incomplete requests: refusing comparison charts")
    groups = defaultdict(list)
    for row in rows:
        groups[(row["framework"], row["payload_bytes"], row["concurrency"])].append(row)
    metrics = [("qps", "Throughput", "requests / second", 1),
               ("p99_us", "P99 latency", "milliseconds", .001),
               ("p999_us", "P99.9 latency", "milliseconds", .001),
               ("server_cpu_pct", "Gateway + worker CPU", "% (100% = one logical CPU)", 1),
               ("server_peak_rss_mib", "Gateway + worker peak RSS", "MiB (sum of processes)", 1),
               ("loadgen_cpu_pct", "Load generator CPU", "% (100% = one logical CPU)", 1)]
    output = Path(args.out_dir)
    output.mkdir(parents=True, exist_ok=True)
    import matplotlib
    matplotlib.use("Agg")
    import matplotlib.pyplot as plt
    plt.rcParams.update({"font.size": 10, "axes.spines.top": False, "axes.spines.right": False})
    frameworks = sorted({r["framework"] for r in rows})
    names = {"due": "Due", "skynet": "Skynet", "pitaya": "Pitaya", "kbengine": "KBEngine*"}
    colors = {"due": "#16835d", "skynet": "#d67d16", "pitaya": "#2879b9", "kbengine": "#8961ae"}
    sizes = sorted({r["payload_bytes"] for r in rows})
    summary = []
    for (framework, size, concurrency), values in sorted(groups.items()):
        record = dict(framework=framework, payload_bytes=size, concurrency=concurrency, repeats=len(values))
        for metric, _, _, _ in metrics:
            numbers = [v[metric] for v in values]
            record.update({metric + "_median": statistics.median(numbers), metric + "_min": min(numbers), metric + "_max": max(numbers)})
        summary.append(record)
    with (output / "summary.csv").open("w", newline="") as handle:
        writer = csv.DictWriter(handle, fieldnames=list(summary[0]))
        writer.writeheader()
        writer.writerows(summary)
    for metric, title, unit, scale in metrics:
        fig, axes = plt.subplots(1, len(sizes), figsize=(5 * len(sizes), 4.6), squeeze=False)
        for ax, size in zip(axes[0], sizes):
            for framework in frameworks:
                values = [r for r in summary if r["framework"] == framework and r["payload_bytes"] == size]
                values.sort(key=lambda r: r["concurrency"])
                x = [r["concurrency"] for r in values]
                y = [r[metric + "_median"] * scale for r in values]
                errors = [[(r[metric + "_median"] - r[metric + "_min"]) * scale for r in values],
                          [(r[metric + "_max"] - r[metric + "_median"]) * scale for r in values]]
                ax.errorbar(x, y, yerr=errors, marker="o", capsize=4,
                            label=names.get(framework, framework), color=colors.get(framework))
            ax.set_title("{} byte payload".format(size))
            ax.set_xlabel("Persistent TCP connections")
            ax.set_ylabel(unit)
            ticks = sorted({r["concurrency"] for r in rows})
            ax.set_xscale("log", base=2)
            ax.set_xticks(ticks, list(map(str, ticks)))
            ax.grid(alpha=.2)
            ax.set_ylim(bottom=0)
        fig.suptitle(title + " — single host / cross-process echo")
        handles, labels = axes[0][0].get_legend_handles_labels()
        fig.legend(handles, labels, loc="upper center", bbox_to_anchor=(.5, .91), ncol=4, frameon=False)
        fig.text(.5, .035, "Median across repeats; whiskers = min–max. Per-run latency percentiles are not pooled.", ha="center", fontsize=9)
        fig.text(.5, .005, "* KBEngine: poller TCP extension + native EntityCall; excludes native player login / Proxy client protocol.", ha="center", fontsize=9)
        fig.tight_layout(rect=(0, .075, 1, .94))
        for suffix in ("png", "svg"):
            fig.savefig(output / (metric + "." + suffix), dpi=160)
        plt.close(fig)


if __name__ == "__main__":
    main()
