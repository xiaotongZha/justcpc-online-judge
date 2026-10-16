# OJ 发行版

## 下载安装

Ubuntu 24.04、amd64、systemd。先[安装测评引擎](https://github.com/xiaotongZha/justcpc-judge-engine/blob/master/DEPLOY.md)，两者使用同一服务器、同一普通账号；无需 Go、Node.js。

```bash
sudo apt-get update
sudo apt-get install -y curl ca-certificates
mkdir -p ~/justcpc/online-judge
cd ~/justcpc/online-judge

release_url=https://github.com/xiaotongZha/justcpc-online-judge/releases/download/v0.2.0
archive=JustCPC-OJ-0.2.0-linux-amd64.tar.gz
curl -fLO "$release_url/$archive"
curl -fLO "$release_url/$archive.sha256"
sha256sum -c "$archive.sha256" && tar -xzf "$archive" --strip-components=1

sudo ./install-service.sh
sudo journalctl -u justcpc-online-judge -g '初始管理员' --no-pager
```

首次自动生成 `config.json`，日志显示 `admin` 初始密码。访问 `http://服务器IP:18081`，登录后改密码。服务后台运行、开机自启。

临时前台运行可用 `./start.sh`（不要与服务同时启动）；`Ctrl+C` 停止。

配置改 `config.json`：`judge_url` 为引擎地址，`judge_workers: 0` 自动选择并发；修改后执行 `sudo systemctl restart justcpc-online-judge`。

## 升级

停止服务，备份并保留 `data/`、`config.json`；在原目录重复下载、校验、解压命令（换成新版本），然后启动：

```bash
sudo systemctl stop justcpc-online-judge
# 备份 data/、config.json，下载并解压新版本
sudo systemctl start justcpc-online-judge
```

## 从源码构建

构建机安装 Go 1.26.5+、Node.js 22.12+、npm，在仓库根目录执行 `./script/build-release.sh`。产物在 `release/`；指定版本用 `VERSION=0.2.1 ./script/build-release.sh`。
