# 脚本

- `run.py`：Linux 统一生命周期、CPU 绑定、测量采样及 JSONL；默认 100k 请求、三轮，可传 --smoke。
- `plot.py`：校验失败数，按报文大小分图，以并发为横轴生成 PNG/SVG，导出三轮统计 summary.csv。
- `run_smoke.sh`：针对已经启动的单个框架进行响应校验，不负责部署。
- `snapshot.py`：记录本地 Due commit、工作区状态和 Go 源码 SHA256。
- `report.py`：核对四框架完整矩阵、重复数及成功数，生成最终中文报告，拒绝不完整数据。
- `build.py`：下载固定上游源码，交叉编译 Linux Go 二进制并记录 SHA256。
- `remote.py`：Paramiko SSH/SFTP，交互读取密码，验证已知主机密钥。
- `remote_check.sh`：采集硬件及工具信息；源码归档的版本应读取 configs/versions.json，不依赖 .git 存在。

构建及部署目录约定见 comparison/README.md。
