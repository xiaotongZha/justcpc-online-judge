# OJ 发行版

## 下载安装

[GitHub 下载](https://github.com/xiaotongZha/justcpc-online-judge/releases/tag/v1.0.0) · [Gitee 下载](https://gitee.com/zhaxt/justcpc-online-judge/releases/tag/v1.0.0)

Ubuntu 24.04、amd64、systemd。先[安装测评引擎](https://github.com/xiaotongZha/justcpc-judge-engine/blob/master/DEPLOY.md)，两者使用同一服务器、同一普通账号；无需 Go、Node.js。

```bash
sudo apt-get update
sudo apt-get install -y curl ca-certificates
mkdir -p ~/justcpc/online-judge
cd ~/justcpc/online-judge

release_url=https://github.com/xiaotongZha/justcpc-online-judge/releases/download/v1.0.0
archive=JustCPC-OJ-1.0.0-linux-amd64.tar.gz
curl -fLO "$release_url/$archive"
curl -fLO "$release_url/$archive.sha256"
sha256sum -c "$archive.sha256" && tar -xzf "$archive" --strip-components=1

sudo ./install-service.sh
sudo journalctl -u justcpc-online-judge -g '初始管理员' --no-pager
```

首次自动生成 `config.json`，默认登录 `admin / 12345678`，登录后在账户管理改密码。升级保留已有密码。服务后台运行、开机自启。

公网访问：将 `config.json` 的 `listen` 改为 `0.0.0.0:18081`，重启 OJ，并在云防火墙及已启用的系统防火墙放行 TCP 18081；访问 `http://服务器公网IP:18081`。引擎保持本机地址。

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

构建机安装 Go 1.26.5+、Node.js 22.13+、npm，在仓库根目录执行 `./script/build-release.sh`。产物在 `release/`；指定版本用 `VERSION=1.0.0 ./script/build-release.sh`。
