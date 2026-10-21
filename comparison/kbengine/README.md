# KBEngine 适配器及适用范围

这里测量 `TCP socket 扩展 → BaseApp → CellApp → BaseApp → socket`，不是原生玩家登录、Proxy 通信协议。扩展采用官方模板 Poller 所展示的 registerReadFileDescriptor/registerWriteFileDescriptor；非阻塞读写处理 TCP 拆包、合包、部分写入。唯一连接 token 避免 fd 复用导致异步响应送错连接。

Echo 的 base 和 cell 是引擎原生实体，跨进程使用声明在 Echo.def 中的 EntityCall，传输 token(UINT64)、payload(BLOB)。只有 cell 创建成功的 onGetCell 回调才监听 TCP 17001。业务期间没有数据库操作。

固定上游源码 `kbe/src` 中执行 `make -j4`，依赖 GCC/G++、make、MySQL 开发头文件及 mysql_config。当前 CentOS 7 使用 `mariadb-devel-5.5.68-1.el7`；引擎内部依赖及 Python 3.7 由上游构建流程编译。

```bash
python3 comparison/kbengine/prepare.py src/kbengine
python3 comparison/scripts/run.py --frameworks kbengine --smoke --out results/kb-smoke
```

prepare.py 从固定源码的官方 python_assets 模板创建 `comparison/runtime/kbengine-assets`，再覆盖本目录 overrides；拒绝覆盖已有资产。独立 MySQL 5.7 容器只映射 loopback:17406，配置中的口令只用于这个一次性测试库，并非机器登录凭据。

控制器启动 machine、logger、interfaces、dbmgr、baseappmgr、cellappmgr、baseapp、cellapp、loginapp；服务 CPU/RSS 图只统计 BaseApp 与 CellApp，管理组件及数据库成本另行披露，不算成零成本。
