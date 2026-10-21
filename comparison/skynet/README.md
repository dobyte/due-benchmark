# Skynet 适配器

两个 OS 进程分别启动 gateway.lua、worker.lua；gateway 使用 Skynet socket，worker 注册集群名 `echo`，经 `cluster.call("worker", "@echo", payload)` 跨原生 TCP 集群传输转发。两进程均配置 thread=2、harbor=0。

GCC 4.8 的实际构建命令保存在 `build.sh`。调用 `sh comparison/skynet/build.sh /opt/framework-benchmark/src/skynet`。使用上游兼容原子操作分支，线程局部存储映射到 `__thread`，并补充 x86 pause 内建函数宏。源码归档不含 jemalloc 子模块，此次通过官方 NOUSE_JEMALLOC 开关使用 glibc malloc。报告不能将这些构建条件省略。

由统一 `scripts/run.py` 在 Skynet 源码目录启动，配置内的 Lua 服务相对路径依赖约定部署目录。
