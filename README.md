# JustCPC 赛后评测平台（Online Judge）

面向**命题、比赛管理、赛后统一评测**的工作台：创建题目、组织比赛、导入机房考试导出的**提交包**、批量评测、生成成绩单与结果包。

**发行版构建、部署：[DEPLOY.md](DEPLOY.md)。** 下文为源码开发与接口说明。

- 后端 Go（gin + SQLite），前端 Vue 3 + Vite + Element Plus；单文件数据库，无需外部服务。
- 支持 C++17、Java 和 Python 3。非零退出的测试点不计分，其余通过点照常计分。
- 与另外两个组件配合：
  - **机房考试端（exam-server）** 导出提交包 → 本平台导入评测；
  - 本平台把判题任务发给 **判题引擎（judge-engine）**，由它执行编译/运行/比对。

新建比赛时，题面 PDF 和选手名单分别上传、右侧预览。PDF 请提供考生可阅读及留档的正式版本；名单支持 CSV/XLSX，下载对应“格式模板”填写，名单校验通过后即可创建。

## 环境要求

- Go 1.26.5+
- Node.js（仅构建前端时需要）
- 一个在运行中的 **judge-engine**（默认 `http://127.0.0.1:18080`），本平台通过它实际判题。
- OJ 与引擎部署在同机，测试数据保留在 OJ 的数据目录内；评测请求提供题目元数据、资源限制和测试文件绝对路径，引擎账户需能读取这些路径并写入输出目录。

## 构建与运行

```bash
# 1) 构建前端（产物 web/dist，由后端静态托管）
cd web && npm install && npm run build && cd ..

# 2) 启动后端
go run ./src
```

首次启动自动生成 `config.json`，创建管理员 `admin`，默认密码 `12345678`；登录后在账户管理修改密码。已有配置和密码不会覆盖。`admin` 的账号名和管理员身份固定。自备配置参考 `config.example.json`。

## 配置（config.json）

| 字段 | 说明 |
|---|---|
| `data` | 数据目录，默认 `./data` |
| `listen` | 监听地址，默认 `127.0.0.1:18081` |
| `judge_url` | judge-engine 地址，默认 `http://127.0.0.1:18080` |
| `judge_workers` | OJ 同时派发评测的 worker 数；默认 `0` 自动读取引擎容量，正整数手动设置，负数报错 |
| `web_dist` | 前端构建产物目录，默认 `./web/dist` |
| `session_secret` | 会话签名密钥，**≥32 字节** |

配置文件路径可用环境变量 `JUSTCPC_CONFIG` 指定；`session_secret` 可用 `JUSTCPC_SESSION_SECRET` 补充。旧配置中的 `initial_admin_password` 已不再生效，可删除。

`judge_workers` 与引擎的 `workers` 分别控制派发并发和实际执行并发。两边省略或设为 `0` 时，引擎根据 CPU 亲和性、cgroup CPU 配额及可用内存保守选择固定容量，OJ 在启动时通过 `GET /api/capacity` 读取这个值，用户无需分别选择两份并发数。

引擎尚未启动或容量查询暂时失败时，OJ 网页仍可访问，提交保留在队列，每 5 秒重试容量查询，尚未派发的任务不会因此变成失败。旧版引擎不支持容量接口（HTTP 404）时回退到 1 个 worker，并打印日志。

手动设置时，推荐只把引擎 `workers` 改为正整数，例如 `2`，OJ 保持 `judge_workers: 0` 自动跟随。也可以单独指定 OJ `judge_workers`，用于限制派发数量；引擎的执行容量仍是最终上限。额外任务排队，排队时间不计入选手用时。已有配置里的正整数继续按手动模式执行，改为 `0` 才启用自动。

容量在启动时确定，运行期间保持固定。修改配置或引擎资源配额后重启引擎及 OJ；启动日志会显示实际采用的容量。自动选择用于减少过度并发，计时容差和边界裁决仍需另行确定。

## 角色与功能

- **管理员（admin）**：账户管理、创建题目、上传测试数据、创建比赛、导入考试数据包、筛选批量评测、导出成绩单/结果包；judge 队列查看与重试。
- **普通用户（user）**：查看题目并提交自测（提交进入评测队列）。

创建比赛的 CSV/XLSX 名单必须包含 `student_id,name,password`，三列均非空，不再提供默认学生密码。页面提供模板、格式示例及名单预览；密码不在预览中展示，替换名单后重新校验，无需额外勾选确认。学号和密码请按文本填写，保留前导零。

数据交接：**OJ 导出考试包 → 考场 Exam 导入并考试 → 导出提交包 → OJ 对应比赛预览并导入测评**。两种 ZIP 不可混用，文件名包含比赛名称。提交包名单仍为 `student_id,name`，不含密码；缺少平台 ID 的历史包需明确确认目标比赛和全部题号映射。

管理员可在比赛详情页删除比赛，确认后会删除本场考生、提交和评测记录，题库中的原题仍然保留。

## 主要接口

- 账户：`POST /api/login`、`GET /api/me`、`/api/accounts`（管理员）
- 详情链接包含记录 ID，可直接访问、刷新及使用浏览器前进后退。
- 题目：`GET/POST /api/problems`、`GET/PUT/DELETE /api/problems/:id`、`POST /api/problems/:id/testdata`
- 提交：`GET /api/submissions`（分页筛选）、`GET /api/submissions/:id`
- 比赛：`GET/POST /api/contests`、`GET/DELETE /api/contests/:id`
  - `GET /api/problems/:id/testdata`（题目所有者下载已保存的测试数据 ZIP）
  - `POST /api/contest-roster-preview`（管理员上传 `roster` 文件，校验并预览名单，不创建比赛）
  - `POST /api/contests/:id/preview-submissions`（上传 `file`，只校验并预览，不写入）
  - `POST /api/contests/:id/import-submissions`（导入考试提交包；历史包缺 ID 时附 `mapping` JSON，明确 `contest_id` 和 `problems` 题号→OJ 题目 ID）
  - `GET /api/contests/:id/evaluation`（实时评测成绩表）
  - `POST /api/contests/:id/enqueue`（把选中提交加入评测队列）
  - `GET /api/contests/:id/scoreboard.csv`、`GET /api/contests/:id/result-package`
- judge 队列：`GET /api/admin/judge-queue`、`POST /api/admin/judge-queue/retry`

## 计分与任务恢复

- 普通自测按题目的默认分值计分；比赛提交按该比赛设置的题目分值计分。同一道题在不同比赛中可以使用不同分值，部分分按通过测试点比例计算。
- 正在评测的任务持有 15 分钟租约，评测期间每 5 分钟续约。后端在启动时及运行期间每 30 秒检查过期任务，重新入队，避免重启后一直停留在 `running`。`failed` 任务仍由管理员手动重试。
- 引擎满载返回 HTTP 429 时，任务退回队列；对应 worker 等待至少 500 ms 后再尝试，避免连续立即重试。

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

上传的测试数据 ZIP 必须包含从 1 开始连续编号的 `1.in / 1.out、2.in / 2.out…`，每个测试点必须一一配对；缺失、跳号、重号和非规范文件名都会拒绝。无输入题也必须显式上传对应的零字节 `1.in、2.in…`，不能省略；预期无输出时也必须显式上传对应的空 `.out` 文件。平台不会自动补全缺失文件。ZIP 内条目的存放顺序不作要求，评测按编号顺序执行。

