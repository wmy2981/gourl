# gourl 设计文档

> 状态：**v2，待审阅**（2026-08-15；v2 并入首轮审阅的 9 条改动 + 4 轮补充对齐）
> 审阅通过后按本文档实施；实施细节与文档冲突时，以本文档为基准并更新文档。

## 1. 项目概述

轻量自托管短链接服务。公开面仅做跳转（无公开创建入口），链接由管理员在后台创建与管理。提供完整 REST API 支持二次开发。

## 2. 已确认决策总览

| 维度 | 决策 |
|---|---|
| 后端 | Go（最新稳定版）+ 标准库 `net/http`（1.22+ 路由增强），零框架依赖 |
| 数据库 | SQLite（`modernc.org/sqlite` 纯 Go 驱动，无 CGO，支持多平台交叉编译）。**只存储短链接及相关记录**（links / daily_clicks / ua_blocks / api_tokens） |
| 配置 | **YAML 文件**（config.yaml）保存业务配置：站点信息、服务名称、短码长度、保留字、base_url、图标；**环境变量**保存运行时与机密：端口、SQLite 路径、Redis 地址、ADMIN_PASSWORD、SESSION_SECRET |
| 时区 | **容器 TZ 环境变量**（docker-compose 设置，如 `Asia/Shanghai`），决定每日统计切日边界、expires_at 解释与时间展示 |
| 缓存/计数 | Redis（`go-redis/v9`），点击计数先落 Redis，30s 批量归并 SQLite |
| 前端 | React 19 + TypeScript + Vite + Tailwind CSS 4 + shadcn/ui + react-i18next + TanStack Query + recharts + qrcode.react；**响应式布局**（移动端可用） |
| 产品形态 | 仅管理后台；`/` 重定向 `/admin`；公开面只有跳转与过期提示页 |
| 认证 | 单管理员密码（`ADMIN_PASSWORD`）+ HttpOnly 会话 Cookie；API 用 Bearer Token |
| 短码 | 创建时自定义任意短码（**可多级**，如 `link1/link2`，层级 ≤5，允许 `[a-zA-Z0-9/-_]`），或自动生成随机单级 N 位 base62（N 可配置，默认 6） |
| 保留字 | 内置保留列表（api、admin、expired、health、assets、favicon、export 等）+ **YAML 可追加自定义保留字**；校验大小写不敏感；命中返回 400；保留前缀优先于短链路由 |
| 跳转 | 302 临时重定向（改目标即时生效） |
| 统计 | 每链接点击总数 + 每日分布；Redis 计数 30s 归并（可接受崩溃丢失 ≤30s 数据） |
| UA 屏蔽 | 大小写不敏感子串匹配；命中返回 403 且不计入点击 |
| 过期 | 渲染双语过期提示页（200），包含站点 header/footer |
| 标题抓取 | SSRF 全面防护：禁私网/环回/链路本地/云元数据地址、跟随重定向逐跳校验、超时 + 响应大小限制 |
| 短链基址 | **主 base_url + 多个副 baseurl**（extra_base_urls），未配置时按请求 Host 推断；列表/复制/二维码同时展示所有地址生成的完整短链 |
| 后台附加功能 | 二维码展示（链接旁二维码图标，点击弹窗展示二维码，多地址可切换）、CSV 导出、批量导入 |
| 自定义图标 | 配置页上传 SVG/PNG（≤1MB，白名单类型），存挂载 volume 的 `assets/` 目录、YAML 记录文件名；立即用于 favicon、后台 logo、过期页；未上传用内置默认图标 |
| 站点信息 | **单语言**字段（title / keywords / description / header / footer / 服务名称），页面文案才走双语 |
| health | 公开 `GET /api/v1/health`：服务名称、版本、启动时间、uptime + Redis/SQLite 探活（任一不可用返回 503） |
| i18n | 页面文案中文/英文，自动检测浏览器语言 + 页内手动切换（记忆选择） |
| 前端页脚 | 项目名称 + GitHub 跳转链接 + 版本号（版本号构建时从 VERSION 注入） |
| 设计语言 | 苹果风玻璃拟态 + 响应式；**不默认蓝紫渐变**——按 frontend-design skill 两 pass 流程产出 token 系统（从短链接"路由/信号"主题提取方向），配置页可换服务名称与图标 |
| 图标 | 内置默认：圆角方块 + 双环链接符号（配色实施时定）；可上传自定义 |
| 分支模型 | main（正式版）+ dev（预发行）双分支，feature 分支 PR 合入 |
| 版本机制 | 根目录 `VERSION` 文件为单一事实来源，手动维护；发行脚本做前进性校验 |
| CI/CD | 复刻 `wmy2981/connection-checker` 三 workflow 方案（ci / release / build） |
| 镜像 | `ghcr.io/wmy2981/gourl`，多平台 `linux/amd64, linux/arm64`，非 root + HEALTHCHECK（打 `/api/v1/health`） |
| 仓库 | public；**README 双语（en/zh-CN）互相跳转** |
| 测试 | Go 单测（miniredis + httptest）+ vitest + tsc + Playwright E2E；每个改动一个提交 + 测试 |

## 3. 架构

### 3.1 仓库结构（monorepo）

```
gourl/
├── VERSION                     # 版本号单一事实来源（手动维护）
├── config.yaml.example         # 业务配置示例（复制为 config.yaml 使用）
├── go.mod / go.sum
├── cmd/gourl/main.go           # 入口：加载配置、启动 HTTP 服务
├── internal/
│   ├── config/                 # YAML 加载 + 校验 + 热更新（原子写回，文件锁）
│   ├── store/                  # SQLite：schema、links、daily_clicks、ua_blocks、api_tokens
│   ├── counter/                # Redis 计数 + 30s 归并任务
│   ├── fetcher/                # 标题/描述抓取 + SSRF 防护
│   ├── shortcode/              # 随机短码生成（base62）+ 保留字校验
│   ├── api/                    # REST 路由、handler、认证中间件
│   ├── admin/                  # 登录会话、管理后台 API
│   ├── assets/                 # 图标上传：存储、类型/大小校验
│   └── version/                # 构建时注入版本号
├── frontend/                   # Vite React SPA（管理后台）
│   ├── src/                    # pages: login / dashboard / links / settings
│   └── e2e/                    # Playwright 测试
├── web/                        # 前端构建产物 embed 入口（dist 复制于此）
├── assets/                     # 内置默认图标（SVG）
├── .github/
│   ├── workflows/ci.yml        # push/PR → 全量检查
│   ├── workflows/release.yml   # push main/dev → 版本校验 + tag + Release
│   ├── workflows/build.yml     # dev 直推 / main 等 Release 成功后构建推 GHCR
│   └── scripts/release_check.py# 版本前进性校验 + 自动生成 Release notes
├── Dockerfile                  # 多阶段：node 构建前端 → go build → alpine runtime
└── docker-compose.yml          # app + redis；挂载 config.yaml、data 卷（SQLite）、assets 卷（图标）
```

前端构建产物 `frontend/dist` 复制到 `web/` 并以 `embed` 打进单个二进制，单镜像部署。

### 3.2 配置分层

**config.yaml（业务，挂载 volume，API 可写回热生效）**

```yaml
site:
  name: gourl                       # 服务名称（health/页脚/后台标题/过期页）
  title: gourl - Short Links        # 站点标题（meta）
  keywords: short link, url shortener
  description: Lightweight self-hosted URL shortener
  header: ""                        # 过期页 header HTML（可空）
  footer: ""                        # 过期页 footer HTML（可空）
short_code_length: 6                # 随机短码位数（新建时生效）
base_url: ""                        # 主基址，空则按 Host 推断（如 https://s.example.com）
extra_base_urls: []                 # 副基址列表，如 [https://s2.example.com]
reserved_codes: []                  # 追加保留字（内置列表之外）
icon: ""                            # 自定义图标文件名（assets/ 下，空=内置默认）
```

**环境变量（运行时与机密，docker-compose 注入）**

`PORT`（默认 8080）、`DB_PATH`（SQLite 文件路径）、`REDIS_ADDR`、`ADMIN_PASSWORD`（空=禁用登录，内网信任模式）、`SESSION_SECRET`、`CONFIG_PATH`（默认 config.yaml）、`TZ`（容器时区，决定每日统计切日边界、expires_at 解释与时间展示；docker-compose 默认 `Asia/Shanghai`）。

### 3.3 数据流

```
访客 GET /{code...}（多级 path 整体匹配短码）
  → 保留前缀 /api /admin /expired /health /assets 等优先于短链路由
  → UA 屏蔽检查（命中 → 403，不计数）
  → 链接存在/过期检查（不存在 → 404 页；过期 → 渲染过期页）
  → Redis INCRBY counter:{code}、counter:{code}:{YYYY-MM-DD}
  → 302 → 目标 URL

归并任务（每 30s）
  → 对每个活跃 key GETDEL 取走计数值
  → SQLite 事务内 upsert links.click_count / daily_clicks
  → 写库失败则将数值 INCRBY 回补 Redis（下轮重试）

配置热更新
  → 后台配置页保存 → PUT /api/v1/config 校验 → 原子写回 config.yaml（文件锁）
  → 内存配置对象热替换（站点信息/图标/保留字/短码长度即刻生效，无需重启）
```

## 4. 数据模型（SQLite，仅短链接及相关记录）

| 表 | 字段 |
|---|---|
| `links` | `code` TEXT PK（可含 `/`）、`url` TEXT NOT NULL、`title` TEXT、`description` TEXT、`expires_at` INTEGER（unix 秒，0=永不）、`click_count` INTEGER DEFAULT 0、`created_at`、`updated_at` |
| `daily_clicks` | `code` TEXT、`date` TEXT（YYYY-MM-DD）、`count` INTEGER，联合主键 (code, date) |
| `ua_blocks` | `id` INTEGER PK、`pattern` TEXT UNIQUE、`created_at`；匹配规则：大小写不敏感子串匹配 |
| `api_tokens` | `id` INTEGER PK、`token` TEXT UNIQUE（明文存储，满足"查看 Token"；备注：后续可换 sha256 哈希 + 仅创建时展示一次）、`note` TEXT、`created_at` |

管理密码不落库。建表用启动时自动迁移（embed schema.sql + schema_version 表）。

## 5. Redis 数据结构

- `counter:{code}` → 总点击数（`INCRBY`）
- `counter:{code}:{YYYY-MM-DD}` → 当日点击数

归并：`GETDEL` 原子取走 → SQLite upsert → 失败 `INCRBY` 回补。间隔固定 30s。

## 6. API 设计

统一前缀 `/api/v1`，JSON。认证二选一：管理后台会话 Cookie（HttpOnly、SameSite=Lax）或 `Authorization: Bearer <token>`；Token 与管理会话同权限。

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/health` | **公开**：服务名称、版本、启动时间、uptime、redis/sqlite 状态；依赖异常返回 503 |
| POST | `/api/v1/auth/login` | 密码登录，签发会话 |
| POST | `/api/v1/auth/logout` | 登出 |
| GET | `/api/v1/links` | 列表（分页 + 关键字搜索 code/url/title + 排序） |
| POST | `/api/v1/links` | 创建（`url` 必填；`code` 可选，可多级，命中保留字/重复 → 400/409；`expires_at` 可选，0=永不；自动抓取 title/description；响应含 `urls` 数组 = 主+副 baseurl 生成的完整短链列表） |
| POST | `/api/v1/links/batch` | 批量导入（JSON 数组，上限 500 条/次，逐条校验，返回逐条结果） |
| GET | `/api/v1/links/{code}` | 详情（响应含完整短链列表） |
| PATCH | `/api/v1/links/{code}` | 修改 url/title/description/expires_at/code（改 code 重新校验保留字/重复；改 url 可重新抓取标题） |
| DELETE | `/api/v1/links/{code}` | 删除（级联删除 daily_clicks） |
| GET | `/api/v1/links/{code}/stats` | 统计：总数 + 每日数组 |
| GET | `/api/v1/export.csv` | CSV 导出全部链接（code, url, title, expires_at, clicks, created_at） |
| GET/POST | `/api/v1/ua-blocks` | UA 屏蔽列表 / 新增 |
| DELETE | `/api/v1/ua-blocks/{id}` | 删除屏蔽规则 |
| GET/POST | `/api/v1/tokens` | Token 列表（仅前缀+note）/ 生成新 Token（响应展示一次完整值） |
| DELETE | `/api/v1/tokens/{id}` | 吊销 Token |
| GET/PUT | `/api/v1/config` | 站点信息/服务名称/短码长度/保留字/base_url/图标配置（PUT 原子写回 YAML 热生效） |
| POST | `/api/v1/icon` | 上传图标（SVG/PNG ≤1MB）；DELETE `/api/v1/icon` 恢复默认 |
| GET | `/api/v1/dashboard` | 仪表盘：链接总数、总点击数、今日点击、最近 14 天每日总点击 |

公开端点：`GET /{code...}`（302 跳转）、`GET /expired`（服务端渲染）、`GET /assets/{file}`（自定义图标静态服务）。保留前缀（api/admin/expired/health/assets/favicon/export + 自定义）优先于短链匹配。

错误统一格式：`{ "error": { "code": "...", "message": "..." } }`，message 按 `Accept-Language` 双语。

## 7. 前端页面（管理后台 SPA，响应式）

| 路由 | 页面 |
|---|---|
| `/admin/login` | 登录（玻璃拟态卡片） |
| `/admin` | 仪表盘：指标卡片 + 最近 14 天点击趋势图（recharts） |
| `/admin/links` | 链接列表：搜索、分页、复制短链（主+副地址全部可复制）、二维码图标（点击弹窗，多地址切换展示二维码）、编辑、删除、批量导入、CSV 导出 |
| `/admin/settings` | 站点信息（名称/title/keywords/description/header/footer）、服务名称、短码长度、base_url + 副 baseurl 列表、保留字、自定义图标上传/恢复默认、UA 屏蔽列表、API Token 管理 |

通用元素：页脚含项目名称 "gourl"、GitHub 跳转链接（`https://github.com/wmy2981/gourl`）、版本号（构建时从 VERSION 注入）。

设计语言（M7 实施时执行 frontend-design skill 两 pass）：
- 苹果风玻璃拟态（backdrop-blur + 半透明卡片）、圆角、克制、深浅色模式
- **配色不使用默认蓝紫渐变**——先产出 token 系统（4–6 个具名色值 + 2+ 字族 + 布局概念 + 一个签名元素），从短链接"路由/信号/跳转"主题提取方向，再评审唯一性后实现
- 响应式：移动端列表卡片化、抽屉式筛选、二维码弹窗全屏适配

i18n：react-i18next；自动检测 + 页内切换（localStorage 记忆）。过期页/404 页服务端渲染，按 `Accept-Language` + `?lang=` 双语。

## 8. 站点信息应用范围

服务名称：health 响应、后台标题、页脚、过期页、`<title>`。title/keywords/description：管理后台与过期页 `<head>` meta。header/footer：过期页 HTML。自定义图标：favicon（带版本参数防缓存）、后台 logo、过期页。

## 9. 标题抓取

- 单次 GET，跟随重定向（上限 5 跳），每跳逐级 SSRF 校验
- 防护：拒绝私有段/环回/链路本地/组播/保留段与云元数据地址（169.254.169.254 等）；DNS 解析后校验实际 IP；超时 5s；响应体上限 1MB；仅接受 text/html
- 提取 `<title>` 与 `<meta name="description">`，HTML 解码 + 空白折叠，截断至合理长度

## 10. 版本与发行机制（复刻 connection-checker）

- **版本事实来源**：根目录 `VERSION` 文件（如 `0.1.0`），手动维护；`-ldflags -X` 注入二进制，health 与启动日志输出；前端构建时同步注入页脚版本号
- **release.yml**（push main/dev）：`release_check.py` 读 VERSION → main 仅接受 `x.y.z` 正式版（无变化/倒退报错退出）；dev 仅接受 `x.y.z.alpha.n`/`x.y.z.beta.n` 预发行（dev 出现正式版号则跳过）；排序键 `(major, minor, patch, rank[alpha=0,beta=1,正式=2], n)`；发行动作 `git tag v{VERSION}` + `gh release create`（--prerelease），Release notes 按 Conventional Commits 分组（上一 tag → HEAD，含短哈希链接）
- **build.yml**：dev push 直接构建；main 经 `workflow_run` 监听 Release 成功后构建。Tag：正式版 `vX.Y.Z + latest + sha7`；dev 预发行 `vX.Y.Z + dev + sha7`；dev 正式版号只 `dev + sha7`。多平台（setup-qemu + setup-buildx），`GITHUB_TOKEN`（packages: write）
- **ci.yml**（push/PR main/dev）：go vet + go test ./...（miniredis 计数归并、httptest API）+ 前端 npm ci → tsc + vitest + Playwright E2E（CI 内置 `redis-server` + 临时 SQLite + 测试端口服务）
- 三个 workflow 均带 concurrency 组 + `cancel-in-progress: true`

起始版本 `0.1.0`。开发流程：feature 分支 → PR → dev（VERSION bump `x.y.z.beta.n`）→ main（正式版）。

## 11. Docker

- **Dockerfile** 多阶段：`node:22-alpine` 构建前端 → 复制 dist 到 web/ → `golang:1.24-alpine` 构建（CGO_ENABLED=0）→ `alpine` 运行时，非 root（uid 10001），HEALTHCHECK 打 `/api/v1/health`
- **docker-compose.yml**：`app`（映射 8080，挂载：config.yaml、data 卷（SQLite）、assets 卷（图标）；设置 `TZ: Asia/Shanghai`）+ `redis`（AOF 持久化）；`ADMIN_PASSWORD`/`SESSION_SECRET` 等经环境变量注入
- 本机不构建、不部署 docker，镜像全部由 GitHub Actions 工作流构建并推 GHCR

## 12. 测试策略

- **Go 单测**：短码生成唯一性、多级短码/保留字校验、链接 CRUD、跳转/过期/UA 屏蔽/404、计数归并（miniredis，模拟写库失败回补）、SSRF 校验器（内网段/元数据/重定向）、认证（会话 + Token）、批量导入校验、CSV 导出、health 探活（依赖故障 → 503）、YAML 配置加载/写回/热更新、图标上传（类型/大小校验）
- **前端**：vitest 组件测试（表单校验、列表渲染、i18n 切换、设置保存）+ `tsc --noEmit`
- **E2E（Playwright）**：登录 → 创建链接（自定义/多级/随机短码）→ 复制 → 访问跳转 302 → 计数归并后可见 → UA 屏蔽 403 → 过期提示页 → 批量导入 → Token 调 API → 保留字被拒 → 配置页保存热生效（含图标上传）→ health 返回版本
- **铁律**：每个改动一个 conventional commit，先写/后补测试，改动未过测试不得提交

## 13. 实施顺序（里程碑）

| 里程碑 | 内容 | 验收 |
|---|---|---|
| M0 | 仓库初始化：git、VERSION、.gitignore、LICENSE、go.mod、目录骨架、DESIGN.md | `go build` 通过 |
| M1 | 配置模块（YAML 加载/校验/热更新/原子写回）+ SQLite schema + links CRUD + 短码生成与保留字校验（含多级） | go test 通过 |
| M2 | 跳转（302/过期/UA 屏蔽/404/保留前缀路由/Redis 计数） | go test 通过 |
| M3 | 计数 30s 归并任务（miniredis 验证） | go test 通过 |
| M4 | 标题抓取 + SSRF 防护 | go test 通过 |
| M5 | 认证（登录会话 + API Token）+ ua-blocks/tokens/config/icon/dashboard API + health 探活 | go test 通过 |
| M6 | 批量导入 + CSV 导出 + base_url 多地址组装 | go test 通过 |
| M7 | 前端骨架（Vite+Tailwind+shadcn+i18n+版本注入）+ 设计 token 系统（frontend-design 两 pass）+ 登录/列表/仪表盘/设置页（含图标上传、多地址二维码弹窗、页脚、响应式） | vitest + tsc 通过 |
| M8 | Playwright E2E | E2E 通过 |
| M9 | Dockerfile + docker-compose + embed 静态资源 + HEALTHCHECK + 内置默认图标 | 推送 dev 由 GitHub Actions build.yml 构建镜像验证（本地不构建） |
| M10 | CI/CD workflows + release_check.py + 双语 README（互跳） | 推送 dev 触发完整流水线 |

## 14. 工程约定

- Conventional Commits（英文，`type(scope): lowercase description`），每个逻辑改动一个提交
- 每次改动伴随对应测试，全部通过方可提交
- 代码风格：Go 标准 `gofmt` + `go vet`；前端 ESLint + Prettier
- 后端错误消息、前端 UI 文案均中英双语（站点信息为单语）
