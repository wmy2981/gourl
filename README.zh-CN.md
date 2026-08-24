# gourl

<p align="left">
  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white" alt="Go 1.26" />
  <img src="https://img.shields.io/badge/SQLite-003B57?logo=sqlite&logoColor=white" alt="SQLite" />
  <img src="https://img.shields.io/badge/Redis-DC382D?logo=redis&logoColor=white" alt="Redis" />
  <img src="https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=black" alt="React 19" />
  <img src="https://img.shields.io/badge/Vite-646CFF?logo=vite&logoColor=white" alt="Vite" />
  <img src="https://img.shields.io/badge/Tailwind_CSS-4-06B6D4?logo=tailwindcss&logoColor=white" alt="Tailwind CSS 4" />
  <img src="https://img.shields.io/badge/Docker-single_container-2496ED?logo=docker&logoColor=white" alt="Docker" />
</p>

> 简体中文 | [English](README.md)

<p align="left">
  <img src="assets/favicon.svg" width="72" height="72" alt="gourl" />
</p>

轻量自托管短链接服务。使用Docker快速开始，即可运行你自己的短链接服务。

## 功能

- **短链接生成** — 自定义短码或自动生成，支持多级路径与简体中文，可设有效期，支持显示二维码
- **管理后台** — 响应式高斯模糊界面，浅色/深色/跟随系统三种主题，中英双语；REST API + Swagger 文档
- **点击统计** — 先入 Redis 缓冲、每 30s 批量归并 SQLite；软删除后历史统计依然保留
- **自动获取标题** — 后台异步抓取 title/description（不阻塞请求）
- **安全防护** — Setup 需输入服务器日志中的一次性校验码；bcrypt 密码、可配置会话有效期、登录按 IP 限流、UA/IP 屏蔽、自链防护
- **批量与导入导出** — 批量创建/删除、一键清空过期、CSV/JSON 导入、完整短链项 JSON 导出
- **编辑快照** — 每次修改后数据库自动追加不可变备份到数据库
- **容器内 CLI** — gourl实例管理命令

## 快速开始（Docker）

单容器部署——镜像内置 Redis 实例，一次 `compose up` 即完成整套部署。

```bash
# 1. 可选：将 config.example.yaml 复制为 config/config.yaml 并按需调整。
# 2. 在 docker-compose.yml 同级创建 .env：
echo "SESSION_SECRET=$(openssl rand -hex 32)" > .env

# 3. 启动
docker compose up -d
```

打开 8080 端口的 Web 界面——未配置密码时会进入 Setup 页面（输入服务器日志中的
校验码，再设置管理员密码）。数据持久化在 `./data`，配置在 `./config`。

镜像由 GitHub Actions 构建发布到 [GHCR](https://github.com/wmy2981/gourl/pkgs/container/gourl)：
`ghcr.io/wmy2981/gourl:latest`（正式版）或 `:dev`（预发行）。
容器内执行管理命令：`docker compose exec app gourl <command>`。

## 配置

| 环境变量 | 默认值 | 用途 |
|---|---|---|
| `SESSION_SECRET` | 自动生成 | 会话签名——生产环境务必设置，使会话跨重启保持 |
| `PORT` | `8080` | HTTP 监听端口 |
| `DB_PATH` | `./data/gourl.db` | SQLite 文件 |
| `REDIS_ADDR` | `localhost:6379` | 点击计数缓冲（未设置时使用内置 Redis） |
| `CONFIG_PATH` | `./config/config.yaml` | 业务配置文件 |
| `LOG_DIR` | `./data/log` | 日志镜像轮转文件目录 |

业务配置（站点信息、基址、保留字、限流、日志等级等）在 `config.yaml` 中，
示例见 [Example](config.example.yaml)

## API 文档

在运行中的实例打开 `/docs/` —— 覆盖全部端点的交互式 Swagger UI。
或查看Github Pages托管的[Swagger UI 文档](https://wmy2981.github.io/gourl/docs/)

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
./scripts/build-frontend.sh && go build ./cmd/gourl
```

## 许可证

[MIT](LICENSE)
