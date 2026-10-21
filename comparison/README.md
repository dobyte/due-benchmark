# 分布式 TCP 框架横向测评

测试链路为 `Go 压测器 → 网关进程 → 业务进程 → 网关 → 压测器`。所有进程部署在同一台 Linux 机器上，使用 loopback TCP；这是单机跨进程测试，不能据此推断跨机器网络表现。

| 框架 | TCP 入口 | 跨进程业务调用 |
|---|---|---|
| Skynet | skynet.socket，4 字节大端包长 + 原始字节 | 原生 cluster.call，独立 worker 进程 |
| Pitaya | 原生 TCP Acceptor，原生握手及 request/response 协议 | 原生集群路由及 gRPC，独立 backend |
| Due | 原生 TCP Gate，原生包头、路由号及序号 | 原生 DRPC over TCP，独立 Node |
| KBEngine | 官方文件描述符轮询扩展，4 字节大端包长 + 原始字节 | BaseApp → CellApp → BaseApp 原生 EntityCall |

KBEngine 的场景**不包含原生玩家登录、Proxy 客户端协议**。其 socket 由引擎网络事件循环驱动，不是旁路 Python 服务器；业务转发必须穿过原生 EntityCall。Skynet 同样使用自定义 TCP 分帧。四者比较的是这些明确适配方案的完整链路，不代表四个框架所有模式的性能排名。

## 测量方法

- 应用负载为不解释内容的 64、1024、4096 字节。框架协议包头不计入应用负载长度，压测器校验响应序号和全部原始字节。
- 并发为 32、128、512 个持久 TCP 连接；每连接最多一个未完成请求，属于闭环模型。握手、建连及 10,000 次预热不计入正式计时。
- 每点 100,000 次请求，三轮重复；固定种子打乱每轮框架及组合顺序。任何失败或未完成请求使该点无效、整轮停止；绘图拒绝失败数据。
- QPS = 成功请求数 / 测量耗时；延迟为客户端单次往返时间，包含排队。分位数按 nearest rank 计算。每轮单独计算 P99/P99.9，图上取各轮结果中位数及 min–max，不合并成总体分位数。
- 这是闭环饱和吞吐测试，不测固定到达率，不作 coordinated-omission 修正；不能用尾延迟宣称开放流量过载时仍满足 SLO。
- CPU 为测量阶段两端读取 `/proc/PID/stat` 的进程 CPU 时间差；100% 表示一个逻辑核。RSS 为网关、业务进程之和，每 20ms 采样，可能错过短暂峰值，共享页可能重复计入。
- 图表中的服务 CPU、内存不含注册中心、数据库、管理进程等基础设施；各框架依赖成本不相同，不能将该图解释为整套集群总资源消耗。另记录压测器 CPU 和 etcd CPU。
- 每轮为每个框架启动新服务进程，同一轮九个点复用这些进程；RSS 包含前序负载、分配器及 GC 保留内存的影响。

## 机器与 CPU 配额

当前脚本针对 i5-10400F：6 个物理核、12 个逻辑核，兄弟线程为 `(0,6)` 至 `(5,11)`。

| 用途 | 逻辑 CPU | 配置 |
|---|---|---|
| 网关 | 0,6 | Go GOMAXPROCS=2；Skynet thread=2 |
| 业务 | 1,7 | Go GOMAXPROCS=2；Skynet thread=2 |
| 压测器 | 2,3,8,9 | GOMAXPROCS=4 |
| 新建基础设施及 KBEngine 管理组件 | 4,10 | taskset / Docker cpuset |

CPU 绑定限制本测试进程的运行范围，并未修改宿主机隔离配置。机器上已有 MySQL、Redis、Nacos 等服务继续运行，它们也可能争用 CPU。运行前保存进程列表和硬件信息，以便审视环境干扰。

## 构建与运行

源码版本见 `configs/versions.json`；Due 本地源码及工作区状态见 `configs/due-manifest.json`。外部源码压缩包解压到仓库 `.cache/frameworks/`，不提交下载产物。构建目录和命令见各适配器 README。

可执行 `python comparison/scripts/build.py --fetch` 下载固定源码并交叉编译四个 Go 二进制；已有源码时省略 `--fetch`。Due 源码须位于本仓库旁的 `../due`。Skynet、KBEngine 在目标 Linux 上按各 README 构建。Python、Go 及 Go 依赖需要预先可用。

部署目录约定：

```text
/opt/framework-benchmark/
  comparison/         # 本目录及 bin/ 内的 Linux 二进制
  src/skynet/         # 已编译的固定版本源码
  src/kbengine/       # 已编译的固定版本源码
  results/            # 每次运行独立目录，含日志和 JSONL
```

预先准备 KBEngine 运行资产，然后执行统一控制器：

```bash
python3 comparison/kbengine/prepare.py src/kbengine
python3 comparison/scripts/run.py --smoke --frameworks skynet pitaya kbengine due --out results/smoke
python3 comparison/scripts/run.py --frameworks skynet pitaya kbengine due --out results/formal
python3 comparison/scripts/plot.py results/formal/results.jsonl --out-dir results/formal/charts
```

`--out` 必须是尚不存在的目录，避免混入旧结果。控制器占用前检查测试端口，启动独立 etcd、Redis，以及 KBEngine 独立 MySQL 容器；只清理自己创建的进程和容器。SIGINT/SIGTERM 触发清理。不会停止已有服务。需要 Python ≥3.6、taskset、Docker 和本地缓存的 `redis:6.2.18`、`mysql:5.7` 镜像。绘图另需 matplotlib。

`scripts/remote.py` 使用交互输入 SSH 密码及指定 known_hosts 文件，代码不保存 SSH 凭据。
