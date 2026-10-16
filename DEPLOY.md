# OJ 发行版

## 构建

构建机安装 Go 1.26.5+、Node.js 22.12+、npm。在仓库根目录执行：

```bash
./script/build-release.sh
```

产物在 `release/`；指定版本：`VERSION=0.2.1 ./script/build-release.sh`。

## 部署

与测评引擎部署在同一台 Linux 服务器，使用同一普通账号。解压发行包后进入目录：

```bash
sudo ./install-service.sh
sudo journalctl -u justcpc-online-judge -g '初始管理员' --no-pager
```

首次自动生成 `config.json`，日志显示 `admin` 初始密码。访问 `http://服务器IP:18081`，登录后改密码。服务后台运行、开机自启。

临时前台运行可用 `./start.sh`（不要与服务同时启动）；`Ctrl+C` 停止。

配置改 `config.json`：`judge_url` 为引擎地址，`judge_workers: 0` 自动选择并发。部署机无需 Go、Node.js。

升级：停服务，备份并保留 `data/`、`config.json`，更新程序后重启：

```bash
sudo systemctl stop justcpc-online-judge
# 将新包文件复制到原部署目录，保留 data/、config.json
sudo systemctl start justcpc-online-judge
```
