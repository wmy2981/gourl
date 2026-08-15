# gourl

> 简体中文 | [English](README.md)

轻量自托管短链接服务。单个 Go 二进制（内嵌前端）+ Redis，即可运行你自己的短链接。

## 功能

- **短链接生成** — 自定义短码或自动生成，支持多级路径（`link1/link2`），位数可配置
- **链接管理** — 现代化管理后台（苹果风玻璃拟态、响应式、深色模式）
- **延迟计数** — 点击先入 Redis，每 30s 批量归并 SQLite，抗高并发尖峰
- **自动获取标题** — 创建链接时自动抓取 title/description，带 SSRF 全面防护
- **UA 屏蔽** — 管理员可定义 User-Agent 规则，命中返回 403 且不计点击
- **短链有效期** — 每条链接可设 `expires_at`（0 为永不过期），过期展示优雅双语提示页
- **REST API** — 完整 JSON API，Bearer Token 认证，便于二次开发
- **自定义站点** — 服务名称、标题、关键词、header/footer、上传图标（SVG/PNG）
- **中英文双语** — 自动检测浏览器语言，页内可手动切换
- **二维码、CSV 导出、批量导入**
- **API 文档** — `/docs/` 交互式 Swagger UI
- **结构化日志** — slog 4 级（debug/info/warning/error），文本或 JSON 输出

## 技术栈

Go（标准库 `net/http` + `log/slog`）· SQLite（[modernc](https://modernc.org/sqlite)，无 CGO）·
Redis · React 19 + Vite + Tailwind CSS 4 + shadcn 风格组件

## 快速开始（Docker）

```bash
# 1. 创建配置文件（可选，默认值可直接运行）
cp config.yaml.example config.yaml

# 2. 设置管理员密码并启动
ADMIN_PASSWORD=change-me docker compose up -d
```

访问 http://localhost:8080 会自动跳转管理后台。镜像由 GitHub Actions 构建发布，
从 [GHCR](https://github.com/wmy2981/gourl/pkgs/container/gourl) 拉取
`ghcr.io/wmy2981/gourl:latest`（正式版）或 `:dev`（预发行）。

## 配置

| 环境变量 | 默认值 | 用途 |
|---|---|---|
| `ADMIN_PASSWORD` | —（认证关闭） | 管理后台密码；留空 = 内网信任模式 |
| `SESSION_SECRET` | 不安全默认值 | 会话 Cookie 签名 —— **生产环境务必设置** |
| `REDIS_ADDR` | `localhost:6379` | 点击计数缓冲 |
| `DB_PATH` | `data/gourl.db` | SQLite 文件 |
| `PORT` | `8080` | HTTP 监听端口 |
| `CONFIG_PATH` | `config.yaml` | 业务配置（站点信息、基址等） |
| `ASSETS_DIR` | `data/assets` | 上传图标存储 |
| `TZ` | 容器默认 | 每日统计切日与过期时间按此时区解释 |
| `LOG_LEVEL` | `info` | `debug` / `info` / `warning` / `error` |
| `LOG_FORMAT` | `text` | `json` 结构化输出（日志走 stderr） |

业务配置在 `config.yaml`（见 `config.yaml.example`）：服务名称/标题/关键词/描述/
header/footer、随机短码位数、主 + 附加基址、额外保留字、自定义图标。

## API 文档

访问 http://localhost:8080/docs/ —— 覆盖全部端点的交互式 Swagger UI，
随单个二进制内置（`/docs/openapi.yaml` 为原始 OpenAPI 3.0 规范）。

## API

基础路径 `/api/v1`。管理端点接受会话 Cookie 或
`Authorization: Bearer <token>`（Token 在设置页创建）。

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/health` | **公开**：名称、版本、uptime、redis/sqlite 探活 |
| POST | `/api/v1/auth/login` | 密码登录 → 会话 Cookie |
| GET/POST | `/api/v1/links` | 列表（分页/搜索）/ 创建 |
| POST | `/api/v1/links/batch` | 批量导入（单次 ≤500 条） |
| GET/PATCH/DELETE | `/api/v1/links/{code}` | 详情 / 更新 / 删除 |
| GET | `/api/v1/links/{code}/stats` | 总数 + 每日点击 |
| GET | `/api/v1/export.csv` | 导出全部链接 |
| GET/POST/DELETE | `/api/v1/ua-blocks` | 屏蔽的 User-Agent |
| GET/POST/DELETE | `/api/v1/tokens` | API Token |
| GET/PUT | `/api/v1/config` | 站点配置（热生效，写回 YAML） |
| POST/DELETE | `/api/v1/icon` | 自定义图标上传 / 恢复默认 |
| GET | `/api/v1/dashboard` | 聚合指标 + 14 天趋势 |

跳转：`GET /{code}` → 302 到目标地址（`api`、`admin`、`expired` 等保留前缀
永远不会与短码冲突）。

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
```

约定：Conventional Commits（一个逻辑改动一个提交，自带测试）；`main` 发正式版、
`dev` 发预发行；版本号手动维护在根目录 `VERSION` 文件——CI 校验只前进，并自动
生成发行说明。

## 许可证

[MIT](LICENSE)
