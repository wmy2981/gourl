# gourl

> 简体中文 | [English](README.md)

<p align="left">
  <img src="assets/favicon.svg" width="72" height="72" alt="gourl" />
</p>

轻量自托管短链接服务。单个 Go 二进制（内嵌前端）+ Redis，即可运行你自己的短链接。

## 功能

- **短链接生成** — 自定义短码或自动生成，支持多级路径（`link1/link2`）与简体中文，位数可配置
- **链接管理** — 现代化管理后台（苹果风玻璃拟态、响应式、深色模式，移动端支持右缘左滑手势打开侧栏）
- **延迟计数** — 点击先入 Redis，每 30s 批量归并 SQLite，抗高并发尖峰；短码查询走内存 TTL 缓存
- **历史保留、软删除** — 短链删除后，点击总数与趋势图统计依然保留；删除只是标记行（绝不真正删除），释放的短码可复用，新链接从零开始计数
- **自动获取标题** — 创建链接后后台异步抓取 title/description（不阻塞请求），支持任意可达主机，内外网全覆盖
- **稳定 id 与编辑快照** — 每条链接带稳定自增 id；每次修改都会把旧记录追加到备份表（`b-1, b-2, …`）；db-export 脚本以 JSON 数组导出短链表（含软删除）、token 表、计数表与备份表
- **任意协议目标** — 目标链接支持 `tcp://`、`openapp://`、`mailto:` 等（非 http/https 不抓标题）；表单内 http/https 快捷按钮可补全或替换协议前缀
- **自链防护** — 目标指向本实例短链时，创建、编辑与导入一律拒绝
- **UA / IP 屏蔽** — User-Agent 规则与 IP 规则（精确 IP、CIDR、`192.168.*.*` 通配），命中返回 403 并指明命中规则，不计点击；IP 拦截覆盖所有路由
- **批量操作** — 批量创建（每行一条严格语法）、跨页多选批量删除、一键清空过期、过期状态筛选与红色标识
- **实时日志页** — SSE 实时流 + 级别/关键字/时间筛选 + `.log` 导出；历史来自 LOG_DIR 文件
- **短链有效期** — 每条链接可设 `expires_at`（0 为永不过期），过期短码与不存在一样返回 404
- **REST API** — 完整 JSON API，Bearer Token 认证，便于二次开发
- **Setup 初始化流程** — 首次启动无密码时管理 API 保持锁定（403 `setup_required`，Bearer Token 仍可用），第一个访问者在 `/admin/setup` 设置密码；bcrypt 哈希保存在 `config.yaml`，不再依赖环境变量
- **限流防护** — 登录按 IP 锁定（默认错 10 次锁 300 秒）与短链访问每秒上限（默认 100 次/秒），均可配置
- **自定义站点** — 服务名称、标题、关键词、描述、上传图标（SVG/PNG，一键恢复默认）
- **多基址** — 附加基址并排展示，链接行可选择展示或复制的基址
- **中英文双语** — 自动检测浏览器语言，页内可手动切换
- **二维码、CSV/JSON 导出、批量导入** — 粘贴 JSON 或加载 `.csv`/`.json` 文件；导入遇重复 code 可选报错/跳过/更新，返回各类数量与短码明细；解析宽松（字段名不区分大小写、多种日期格式、数字/字符串互转）；`click_count` 一律不导入
- **额外保留字** — 除内置系统前缀外可自定义，支持中文与多级保留字（多级条目保留整棵子树）
- **API 文档** — `/docs/` 交互式 Swagger UI
- **结构化日志** — slog 4 级（debug/info/warning/error），文本或 JSON 输出，镜像到 `./data/log` 轮转文件；每个 API 请求记录状态、耗时与响应体摘要（错误/警告按 HTTP 状态分级）

## 技术栈

Go（标准库 `net/http` + `log/slog`）· SQLite（[modernc](https://modernc.org/sqlite)，无 CGO）·
Redis · React 19 + Vite + Tailwind CSS 4 + shadcn 风格组件

## 快速开始（Docker）

单容器部署——镜像内置 Redis 实例，一次 `compose up` 即完成整套部署。

```bash
# 1. 配置目录（可选，默认值可直接运行）：
#    将 config.yaml.example 复制为 config/config.yaml 并按需调整。

# 2. 在 docker-compose.yml 同级创建 .env：
echo "SESSION_SECRET=$(openssl rand -hex 32)" > .env

# 3. 启动
docker compose up -d
```

打开 Web 界面（服务默认监听 8080 端口，可用 `PORT` 覆盖）—— 未配置密码时
会进入一次性 Setup 页面，由第一个访问者设置管理员密码（bcrypt 哈希保存在
`config.yaml`），之后进入管理后台。
数据（SQLite、上传图标、内置 Redis 的 rdb、轮转日志）持久化在 `./data`，
配置在 `./config`（设置页会写回其中）。

首次部署无需额外操作：入口脚本仅以 root 运行片刻，把新建的 `./data` 与
`./config` 挂载目录 chown 给非特权 gourl 用户，随后用 su-exec 降权运行
Redis 与 gourl。容器内的 Redis "vm.overcommit_memory" 警告无害可忽略。

镜像由 GitHub Actions 构建发布到 [GHCR](https://github.com/wmy2981/gourl/pkgs/container/gourl)：
`ghcr.io/wmy2981/gourl:latest`（正式版，main 分支）或 `:dev`（预发行）。
部署预发行版：`GOURL_IMAGE=ghcr.io/wmy2981/gourl:dev docker compose up -d`。

预发行镜像在版本号中内嵌提交哈希（如 `0.1.0 (abc1234)`），可见于
`/api/v1/health` 与管理后台页脚，便于识别正在运行的构建；main 构建保持纯版本号。

## 配置

| 环境变量 | 默认值 | 用途 |
|---|---|---|
| `ADMIN_PASSWORD` | — | 旧版环境变量密码。若已设置且 `config.yaml` 无 `password_hash`，启动时一次性迁移（哈希写回配置文件）后即被忽略。优先使用 Setup 流程。 |
| `SESSION_SECRET` | 自动生成 | 会话 Cookie 签名。未设置时启动自动生成强随机密钥（仅内存，重启后会话失效）；**生产环境务必设置**，使会话跨重启保持 |
| `REDIS_ADDR` | `localhost:6379` | 点击计数缓冲 |
| `DB_PATH` | `./data/gourl.db` | SQLite 文件 |
| `PORT` | `8080` | HTTP 监听端口 |
| `CONFIG_PATH` | `./config/config.yaml` | 业务配置（站点信息、基址等） |
| `ASSETS_DIR` | `./data/assets` | 上传图标存储 |
| `TZ` | 容器默认 | 每日统计切日与过期时间按此时区解释 |
| `LOG_FORMAT` | `text` | `json` 结构化输出（日志走 stderr）。日志**等级**在 `config.yaml` 的 `log_level` 中配置（设置页），不再使用环境变量 |
| `LOG_DIR` | `./data/log` | 日志镜像写入该目录的轮转文件（容器内解析为 `/app/data/log`；10 MB × 5 份 × 30 天，gzip） |

业务配置在 `config.yaml`（见 `config.yaml.example`）：服务名称/标题/关键词/描述、
随机短码位数、主 + 附加基址、额外保留字、UA 屏蔽规则、IP 屏蔽规则、登录限流、
短链访问限流、管理员 `password_hash`（由 Setup 流程写入）、自定义图标。

## API 文档

在运行中的实例打开 `/docs/` —— 覆盖全部端点的交互式 Swagger UI，
随单个二进制内置（`/docs/openapi.yaml` 为原始 OpenAPI 3.0 规范）。
API 基础路径 `/api/v1`；管理端点接受会话 Cookie 或
`Authorization: Bearer <token>`（Token 在设置页创建）。

## 开发

```bash
# 后端测试（miniredis，无需真实 Redis）
go test ./...

# 前端（vitest + 类型检查）
cd frontend && npm ci && npm run test && npm run typecheck

# 端到端（自动启动测试服务：内存 SQLite + miniredis）
cd frontend && npm run e2e

# 构建完整二进制（先构建前端并内嵌）
powershell -File scripts/build-frontend.ps1 && go build ./cmd/gourl
# POSIX 环境：./scripts/build-frontend.sh && go build ./cmd/gourl
```

## 许可证

[MIT](LICENSE)
