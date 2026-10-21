# Due 适配器

使用 `../../../due` 的本地源码及其中 TCP、etcd、Redis 子模块替换。一个二进制分别以 `-role gateway`、`-role worker` 启动两个进程。网关 TCP 17001；worker 集群地址 17002；gateway 集群地址 17003。

入口采用原生 Due 包格式，route=1，route/seq 各 2 字节；业务节点使用原始字节 codec 及默认单线程路由处理，返回完整负载。Gate/Node 内部链路使用原生 DRPC over TCP。使用专用 etcd 17379、Redis 17389，不复用宿主已有服务。

在本目录执行 `go build -o ../bin/due-bench .`。Windows 交叉编译时设置 `GOOS=linux GOARCH=amd64 CGO_ENABLED=0`。源码指纹由 `../scripts/snapshot.py` 生成。
