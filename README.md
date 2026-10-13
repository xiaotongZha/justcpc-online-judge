# JustCPC 赛后评测平台（Online Judge）

面向**命题、比赛管理、赛后统一评测**的工作台：创建题目、组织比赛、导入机房考试导出的**提交包**、批量评测、生成成绩单与结果包。

- 后端 Go（gin + SQLite），前端 Vue 3 + Vite + Element Plus；单文件数据库，无需外部服务。
- 与另外两个组件配合：
  - **机房考试端（exam-server）** 导出提交包 → 本平台导入评测；
  - 本平台把判题任务发给 **判题引擎（judge-engine）**，由它执行编译/运行/比对。

## 环境要求

- Go 1.26+
- Node.js（仅构建前端时需要）
- 一个在运行中的 **judge-engine**（默认 `http://127.0.0.1:18080`），本平台通过它实际判题。

## 构建与运行

```bash
# 1) 构建前端（产物 web/dist，由后端静态托管）
cd web && npm install && npm run build && cd ..

# 2) 启动后端
go run ./src
```

首次启动需要 `config.json` 且其中 `session_secret`（≥32 字节）；参考 `config.example.json`。

## 配置（config.json）

| 字段 | 说明 |
|---|---|
| `data` | 数据目录，默认 `./data` |
| `listen` | 监听地址，默认 `127.0.0.1:18081` |
| `judge_url` | judge-engine 地址，默认 `http://127.0.0.1:18080` |
| `web_dist` | 前端构建产物目录，默认 `./web/dist` |
| `session_secret` | 会话签名密钥，**≥32 字节** |
| `initial_admin_password` | 首次启动创建管理员 `admin` 的初始密码 |

配置文件路径可用环境变量 `JUSTCPC_CONFIG` 指定；`session_secret`、`initial_admin_password` 可用 `JUSTCPC_SESSION_SECRET`、`JUSTCPC_INITIAL_ADMIN_PASSWORD` 补充。

## 角色与功能

- **管理员（admin）**：账户管理、创建题目、上传测试数据、创建比赛、导入考试数据包、筛选批量评测、导出成绩单/结果包；judge 队列查看与重试。
- **普通用户（user）**：查看题目并提交自测（提交进入评测队列）。

## 主要接口

- 账户：`POST /api/login`、`GET /api/me`、`/api/accounts`（管理员）
- 题目：`GET/POST /api/problems`、`GET/PUT/DELETE /api/problems/:id`、`POST /api/problems/:id/testdata`
- 提交：`GET /api/submissions`（分页筛选）、`GET /api/submissions/:id`
- 比赛：`GET/POST /api/contests`、`GET/DELETE /api/contests/:id`
  - `POST /api/contests/:id/import-submissions`（导入考试提交包）
  - `GET /api/contests/:id/evaluation`（实时评测成绩表）
  - `POST /api/contests/:id/enqueue`（把选中提交加入评测队列）
  - `GET /api/contests/:id/scoreboard.csv`、`GET /api/contests/:id/result-package`
- judge 队列：`GET /api/admin/judge-queue`、`POST /api/admin/judge-queue/retry`

## 数据与备份

运行时数据全部在 `<config.data>/` 下：

```
data/
├── justcpc.sqlite (+ -wal / -shm)
└── store/
    ├── contest-statements/   # 比赛题面 PDF
    ├── contest-rosters/      # 比赛名单
    ├── tests/<题号>/N.in|out # 题目测试数据
    ├── submissions/<id>/     # 选手提交源码
    └── outputs/<submission>/ # WA 的逐点输出
```

- **备份/迁移 = 打包整个 `data/`**（SQLite 是 WAL 模式，备份时最好先停服务，或确保 `-wal`/`-shm` 一起复制）；恢复即解压覆盖。
- `config.json` 不在 `data/` 内（含 `session_secret`），换机器完整恢复时需一并带走。

