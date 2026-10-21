# Pitaya 适配器

`main.go` 基于固定版本的官方 cluster_grpc 示例：gateway 为 TCP frontend，worker 为 backend，路由 `worker.echo.echo`，使用原生 gRPC 集群传输。禁用消息压缩和连接限流；handler 返回原始字节。

显式替换框架的独立 logger，设为 Error。只调用全局 logrus.SetLevel 不会改变 Pitaya 的默认 Debug logger；早期验证日志受此影响，不能纳入最终性能结果。

go.mod 将 Pitaya 替换为仓库 `.cache/frameworks/topfreegames-pitaya-f3d0af5`；固定完整 commit 见 `../configs/versions.json`，require 中 v3.0.0 不代表实际测量代码版本。

```bash
go build -o ../bin/pitaya-bench .
go build -o ../bin/benchmark-etcd ./infra
```

Windows 交叉编译设置 `GOOS=linux GOARCH=amd64 CGO_ENABLED=0`。专用嵌入式 etcd 提供 17379/17380；不启动 NATS。入口使用原生握手及消息格式，由共同压测器的 `pitaya` codec 实现。
