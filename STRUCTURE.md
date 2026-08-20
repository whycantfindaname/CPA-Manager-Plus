# CPA Manager Plus（CPAMP）结构说明

## 说明范围

本文说明这个个人 fork 的源码拓扑、接口边界、构建产物和运行时数据边界，供 Infra companion manifest 中的 `cpamp` 角色使用。当前 companion manifest 用本仓库支持 `cpamp-service` 和 `cpa-usage-monitoring`。

本文只描述 checkout 中可由源码、脚本、测试和 tracked 文件确认的事实，不把某次本地启动、Docker 状态、浏览器状态或历史部署结果当成当前 live 状态。需要判断当前是否已部署、是否运行、是否已推送，必须另外读取对应机器、容器、HTTP 健康接口和 Git 远端状态。

## Fork 与上游关系

当前 checkout 的分支关系如下：

| 项目 | 当前事实 |
| --- | --- |
| 工作分支 | `lwj_dev` |
| 分支跟踪 | 本地 `lwj_dev` 跟踪 `fork/lwj_dev` |
| 个人 fork remote | `fork` → `https://github.com/whycantfindaname/CPA-Manager-Plus.git` |
| 官方上游 remote | `origin` → `https://github.com/seakee/CPA-Manager-Plus.git` |
| 上游基线 | 以本地可用的 `origin/main` 为比较基线；同步前必须重新检查 remote ref，不假定它代表远端最新状态 |

`lwj_dev` 是个人定制分支，不等同于官方 `main`。当前分支相对本地 `origin/main` 的定制重点包括 Codex 本地会话巡检、weekly pool 估算与学习、loopback-only passwordless Manager Server，以及相应的前后端、持久化、文档和测试。具体文件集合应使用以下命令重新确认：

```bash
git diff --name-status origin/main...HEAD
git log --oneline origin/main..HEAD
```

分支、remote、commit 或部署状态的变化不应只通过修改本文“猜测”出来；先检查真实状态，再更新本文相应章节。

## Infra 角色与 CPA / CLIProxyAPI 边界

CPAMP 是 CPA / CLIProxyAPI 的管理面板和可观测性服务，不是模型代理本体。CPA（CLIProxyAPI）仍然拥有实际模型请求、Provider 路由、Provider 凭证、OAuth/Auth File、客户端 API Key、插件运行时和请求产生的 usage queue。CPAMP 通过 CPA 的 Management API、插件资源接口和 usage queue 管理或观测这些能力。

| 责任 | CPA / CLIProxyAPI | CPAMP / Manager Server |
| --- | --- | --- |
| 实际模型流量 | 处理 `/v1/*`、Codex、Claude 及 Provider 请求 | 不独立转发模型流量 |
| Provider 与凭证 | 保存并执行 Provider、Auth File、OAuth、路由和上游调用 | 面板发起配置、检查或通过管理接口代理操作 |
| 管理接口 | 提供 CPA Management API | 处理 CPAMP 自己的用量、分析、巡检接口；其他 CPA 管理路径由 `service/proxy` 使用服务端保存的 CPA Management Key 代理 |
| 用量来源 | 发布 usage queue；队列 retention 由 CPA 决定 | 采集、规范化、脱敏摘要、写入本地 SQLite 并提供分析 |
| 成本与历史 | 不由 CPAMP 的 SQLite 承担 | 保存请求历史、模型价格、聚合统计、失败摘要和账号趋势 |
| 账号巡检与自动化 | 提供可访问的 Auth File / 管理 API，实际凭证状态仍在 CPA | 保存巡检运行、结果、日志、候选处理和自身调度状态，并按配置执行受控管理操作 |

### 两种产品运行模式

1. **CPAMP 轻量面板（CPA Panel）**：CPA 直接托管同一份 `management.html`，浏览器使用 CPA Management Key 调用 CPA。该模式不启动 Manager Server，不读取 CPAMP SQLite，因此没有服务端请求历史、模型价格分析、服务端巡检等能力。
2. **CPAMP 完整模式（Manager Server）**：Go 二进制在默认 `:18317` 托管嵌入的 `management.html`，浏览器使用 CPAMP Admin Key 登录；首次 setup 保存 CPA 地址、CPA Management Key 和采集配置。Manager Server 启用 collector 后消费 CPA usage queue，并把结果保存到自己的 SQLite。

在 Manager Server 模式中，`/v0/management/*`、`/usage-service/*`、`/management.html` 和兼容的 `/models` 属于 CPAMP 管理入口；轻量面板模式下同一类 CPA 管理路径由 CPA 自己提供。实际模型请求仍应走 CPA。在线演示构建使用虚构 fixture，是预览构建，不是第三种部署模式。

## 顶层目录与根文件

### Tracked 顶层目录

| 路径 | 职责 |
| --- | --- |
| `.github/` | Issue 模板、Dependabot、PR 检查、Demo/Docs、分支晋级和 release workflow |
| `apps/` | npm workspace、React Web、VitePress 文档和 Go Manager Server 的源码 |
| `bin/` | 安装器、原生进程控制、CI 分类、release 校验和打包脚本 |
| `docs/` | 仓库级迁移、管理员密钥、release notes/posts 和实施记录；产品使用文档主要在 `apps/docs/` |
| `img/` | README 和文档引用的截图资源 |

### 重要根文件

| 路径 | 职责与边界 |
| --- | --- |
| `AGENTS.md` | 本仓库代理规则、布局、架构、安全和验证合同；修改源码前必须先读 |
| `CONTRIBUTING.md` | fork/upstream、分支/PR 流程和本地验证入口 |
| `README.md`、`README_CN.md` | 产品边界、轻量面板/完整模式、安装、开发和发布入口 |
| `package.json`、`package-lock.json` | 根 npm workspace、脚本和依赖锁定；不要用局部 manifest 绕过根锁文件 |
| `apps/web/package.json`、`apps/docs/package.json` | Web 与 VitePress workspace 的脚本和依赖 |
| `apps/manager-server/go.mod`、`go.sum` | Manager Server Go module 及依赖校验 |
| `Dockerfile.manager-server` | Node 构建单文件前端、Go 构建无 CGO 二进制、Alpine runtime 镜像 |
| `docker-compose.manager.yml` | 完整模式 Manager Server 的 Docker 服务、`:18317`、`/data` volume 和环境变量 |
| `eslint.config.js`、`.prettierrc` | 前端 lint 与格式化规则 |
| `.gitignore`、`.dockerignore`、`.gitattributes` | 运行时/依赖/构建输出忽略、Docker build context 和语言统计属性 |
| `LICENSE`、`logo.svg`、`logo-white.svg` | 许可证和品牌资源 |

未纳入上述源码边界的 `node_modules/`、`*.local`、`.DS_Store`、`dist*`、`run/`、`logs/`、`data/` 和 `bin/tmp/` 是依赖、构建或运行时内容，不是架构源文件。

## `apps/` 结构

### `apps/web/`：React/Vite 单文件管理面板

入口和 UI 分层如下：

| 路径 | 实际职责 |
| --- | --- |
| `index.html`、`src/main.tsx`、`src/App.tsx` | HTML 宿主、React 挂载、全局生命周期和路由入口 |
| `src/app/` | `AppRouter`、根 shell、生命周期和顶层 route 构造 |
| `src/router/` | hash router 的主业务路由、保护路由和重定向 |
| `src/pages/` | 页面级组合，如 Dashboard、Accounts、Config、Monitoring、Usage Analytics、Plugins、System |
| `src/features/` | 领域功能实现：`accounts`、`aiProviders`、`authFiles`、`config`、`dashboard`、`demo`、`login`、`logs`、`monitoring`、`oauth`、`plugins`、`system`、`usage-analytics` |
| `src/components/` | 跨页面布局、表格、图表、Provider、quota、通用 UI 组件 |
| `src/entities/` | 配置 section、usage service 等业务实体和解析规则 |
| `src/services/api/` | Axios/API client、CPA 管理接口、Manager Server `/usage-service` 与 `/v0/management` 调用、响应转换和错误码 |
| `src/services/storage/` | 浏览器端安全存储；不等于 Manager Server SQLite |
| `src/stores/` | Zustand auth、config、quota、usage header、theme、language 和编辑草稿状态 |
| `src/hooks/` | API、轮询、分页、媒体查询、feature availability、未保存变更等共享 hooks |
| `src/types/` | API、auth、provider、quota、config、plugin 和业务类型 |
| `src/utils/` | URL/API base、加密存储辅助、身份解析、usage、格式化、下载和校验工具 |
| `src/i18n/` | i18next 初始化及 `en`、`ru`、`zh-CN`、`zh-TW` locale |
| `src/styles/`、`src/assets/` | 全局 SCSS、主题、图标、品牌和图片资源 |
| `vite.config.ts` | alias、demo/test fixture、版本注入、single-file、内联资源和输出目录 |

前端依赖方向由测试强制：`features/` 和 `components/` 不得反向 import `pages/`；新增业务逻辑应放在对应 feature/entity/service/store，而不是把后端或页面状态塞进通用组件。

普通构建使用 `apps/web/dist/index.html`，Demo 构建使用 `apps/web/dist-demo/`。两者都是构建产物；`vite-plugin-singlefile` 配置要求 JS、CSS 和资源内联，不应新增外部 chunk、worker 或 sibling runtime 文件。

### `apps/manager-server/`：Go Manager Server

根入口是 `cmd/cpa-manager-plus/main.go`。它启动 HTTP 服务、SQLite、collector、WAL/derived maintenance、账号巡检和其他后台 worker；命令行子命令包括 `reset-admin-key`/`reset-admin-password` 和 `cleanup-derived`。

| 路径 | 实际职责 |
| --- | --- |
| `cmd/cpa-manager-plus/` | 进程入口、配置/数据库初始化、信号退出、后台 worker 组装和维护命令 |
| `internal/app/` | 组装 `app.Context`，创建 service、collector、store 和嵌入面板 |
| `internal/config/` | 配置文件、环境变量、默认值、路径解析、secret file 读取和安全校验 |
| `internal/httpapi/` | HTTP Server 与嵌入面板；`web/management.html` 由前端构建/打包同步，禁止手工修改 |
| `internal/http/router/` | `/health`、`/status`、`/setup`、`/usage-service/*`、CPAMP usage/monitoring/inspection 路由和 CPA 兼容/代理路由 |
| `internal/http/controller/<domain>/` | 按 domain 的 HTTP 解析、鉴权调用、状态码和 JSON 响应；controller 保持薄 |
| `internal/http/middleware/`、`response/` | CORS、Admin/Panel 鉴权、loopback-only 限制、恢复、request log 和统一响应 |
| `internal/model/` | setup、Manager config、usage event、quota、价格、账号动作、Codex/xAI inspection 等数据契约 |
| `internal/service/<domain>/` | 领域业务：CPA client、配置/setup、collector、usage、dashboard、monitoring、pricing、model price、proxy、quota、inspection、automation 和账号动作 |
| `internal/repository/<domain>/` | SQLite 查询和写入：settings、usage events/aggregate/rollup/pricing/monitoring、model prices、quota、inspection、dead letter、account actions 等 |
| `internal/store/` | 创建各 repository 并向 service 暴露组合后的持久化 facade；不应承载 HTTP 解析或页面逻辑 |
| `internal/collector/` | 连接 CPA usage queue、选择传输、消费并解析事件、维护 collector status 和 Auth File snapshot cache |
| `internal/httpqueue/`、`internal/resp/` | HTTP usage queue client 与 RESP client；RESP 订阅/pop 不是普通 HTTP 代理能力 |
| `internal/usage/`、`internal/usageidentity/` | usage payload 解码、事件规范化、脱敏摘要、token/cache/latency/failure 字段和身份键 |
| `internal/worker/` | collector、usage event fanout、rollup、hourly aggregate、pricing、账号动作、巡检、自动化和 migration 后台循环 |
| `internal/repository/sqlite/` | SQLite 打开、WAL、schema/migration、derived index/cleanup 和维护；`usage_events` 是权威事件输入 |
| `internal/security/` | Admin key hash、data key 读取/生成、AES-GCM 保护 Manager 配置中的 CPA Management Key |
| `internal/processlock/` | 同一个数据库路径的进程锁，避免多个 Manager Server 同时写入同一份 SQLite |
| `internal/testutil/` | HTTP、SQLite 和 CPA mock 测试辅助 |

后端调用链应保持：

```text
HTTP route -> controller -> service -> repository/store -> SQLite
                           |
                           +-> CPA Management API / Auth File / Provider checks

CPA usage queue -> collector -> usage normalization/identity -> usage_events
                -> fanout/workers -> rollups/monitoring/dashboard -> HTTP API -> Web UI
```

collector 的 `auto` 模式按代码在可用时尝试 RESP subscribe，失败后尝试 HTTP queue，最后回退到 RESP pop；显式 `subscribe`、`http`、`resp` 模式分别固定对应传输。RESP pop 会破坏性消费队列，同一个 CPA queue 不应由多个 Manager Server 同时消费。HTTP poll interval 不能超过 CPA queue retention，setup 会从 CPA Management API 读取 retention 并校验。

## `apps/docs/`：VitePress 产品文档

| 路径 | 职责 |
| --- | --- |
| `apps/docs/.vitepress/` | VitePress config、导航/sidebar、主题和 CSS |
| `apps/docs/index.md` | 中文文档首页、任务入口、运行模式说明 |
| `apps/docs/guide/`、`deployment/`、`gateway/` | 模式选择、安装部署、CPA 准备和客户端/Provider 接入 |
| `apps/docs/manual/` | Dashboard、Provider、Accounts、Monitoring、Usage Analytics、Inspection、Plugins、Config 等产品操作文档 |
| `apps/docs/operations/`、`troubleshooting/`、`reference/`、`migration/` | 运维、排障、能力矩阵、FAQ、版本和迁移 |
| `apps/docs/en/` | 与中文页面对应的英文文档集合 |
| `apps/docs/images/` | 文档站使用的图片资源 |
| `apps/docs/package.json` | VitePress dev/build/preview 入口 |

中文和英文页面集合、sidebar/nav 链接由 `tests/docsContentIntegrity.test.mjs` 校验。产品文档可以解释操作方式，但不能把历史部署观察写成当前 source 或 live 结论。

## `bin/`、`docs/`、`tests/` 二级结构

### `bin/`

| 路径 | 职责 |
| --- | --- |
| `bin/install-cpamp.sh` | 交互式或非交互式 Docker CPA+CPAMP / CPAMP-only 安装、升级/修复、secret 和 compose 配置生成；`CPAMP_DRY_RUN=1` 只输出计划 |
| `bin/native/` | Linux/macOS shell 与 Windows PowerShell 的原生二进制后台 start/stop/restart/status/logs 控制，运行目录和日志目录可由环境变量覆盖 |
| `bin/ci/` | 根据 changed files 分类应运行的前端、后端、Docker、native、release 和 workflow 检查 |
| `bin/release/` | Demo isolation、native package、release content/published asset 校验和 Telegram 通知 |
| `bin/tmp/` | 本地打包 scratch；被忽略，不是源码目录 |

安装器生成的 compose、CPA config、Auth directory、secret 和 data volume 属于部署目录/运行时，不应回写本仓库。

### `docs/`

- `migration-from-cpa-manager*.md`：旧 CPA-Manager 迁移说明。
- `reset-admin-key*.md`：重置 Manager Server Admin Key 的命令和恢复说明。
- `release.md`：release branch、release notes/posts 和发布门禁。
- `release-notes/`：版本化中英文技术 release note；`release-posts/`：版本化 Telegram HTML 文案。
- `usage-analytics-implementation-plan.zh-CN.md`：仓库级用量分析实施记录，不是运行时配置。

### `tests/`

`tests/` 是仓库级 Vitest 测试，覆盖 workflow integrity、docs content/link、frontend architecture、installer/native scripts、PR check classifier、release validation/published state 和 source integrity；后端 Go 测试仍与各 Go package 同目录。改变 `apps/manager-server` 时还要考虑 Linux/Windows SQLite、Docker 与 race 检查；改变 Web 或 docs 时按 changed-file 分类运行相应 build/test。

## 前端、后端、采集器、配置和持久化的实际边界

### 配置来源与优先级

Manager Server 读取可选的 `CPA_MANAGER_CONFIG` 指定 JSON；否则按可执行文件旁的 `config.json` 读取或在需要时创建默认配置。环境变量覆盖文件配置。常用入口包括：

| 配置类别 | 源码入口 | 代表字段 |
| --- | --- | --- |
| HTTP/面板 | `internal/config/config.go`、`httpapi` | `HTTP_ADDR`、`PANEL_PATH`、`CPA_MANAGER_PPROF_ADDR` |
| CPA 连接 | `internal/config`、`service/setup`、`service/managerconfig` | `CPA_UPSTREAM_URL`、`CPA_MANAGEMENT_KEY`/`_FILE`、setup 的 `cpaBaseUrl`/`cpaManagementKey` |
| Admin/Data key | `internal/config`、`internal/security` | `CPA_MANAGER_ADMIN_KEY`/`_FILE`、`CPA_MANAGER_DATA_KEY`/`_FILE`、`CPA_MANAGER_DATA_KEY_PATH` |
| usage collector | `internal/config`、`internal/collector` | `USAGE_COLLECTOR_MODE`、`USAGE_RESP_QUEUE`、`USAGE_RESP_POP_SIDE`、`USAGE_BATCH_SIZE`、`USAGE_POLL_INTERVAL_MS`、`USAGE_QUERY_LIMIT` |
| 数据与跨域 | `internal/config`、`internal/repository/sqlite` | `USAGE_DATA_DIR`、`USAGE_DB_PATH`、`USAGE_CORS_ORIGINS`、`USAGE_RESP_TLS_SKIP_VERIFY` |
| 账号处理/派生数据 | `internal/config`、service/worker | `USAGE_QUOTA_COOLDOWN_ENABLED`、`USAGE_ACCOUNT_ACTIONS_ENABLED`、`USAGE_ACCOUNT_ACTIONS_AUTO_DISABLE`、`USAGE_DASHBOARD_HOURLY_ROLLUP_ENABLED` |

首次 setup 会验证 CPA Management API；Manager config 可能来自环境或 SQLite，环境托管的字段不能在 UI 中覆盖。`CPA_MANAGER_DISABLE_AUTH=true` 只允许 loopback HTTP 地址和 loopback CORS，路由层还会执行 `LoopbackHostOnly`；这不是对公网开放的无密码模式。

### 持久化与数据流

默认 Docker 数据库是 `/data/usage.sqlite`；原生运行默认使用可执行文件配置目录下的 `data/usage.sqlite`，可用 `USAGE_DB_PATH` 改变。SQLite 还会产生 WAL/SHM 等伴随文件。`settings` 保存 setup、Manager config、Admin credential、bootstrap 和 automation 状态；CPA Management Key 使用 `data.key` 派生的保护器加密后再写入 settings。Admin Key 只保存 hash/salt 等凭证材料，不保存明文。

主要 SQLite 数据分组：

- `usage_events`、`dead_letter_events`：规范化请求事件和无法入库的事件；`usage_events` 是派生数据重建的权威输入。
- usage aggregate、pricing、monitoring、dashboard、account-history rollup 及 checkpoint/state：查询加速和趋势数据，可由 worker/migration 重建或追赶。
- `model_prices`、context/service tiers、`api_key_aliases`：本地价格和调用方别名。
- `codex_inspection_*`、quota snapshot/cooldown、`account_action_candidates`：巡检、配额证据、冷却和账号处理状态。
- `usage_data_migrations`、import session 元数据及 `data/usage-imports/`：迁移和分块 usage 导入生命周期。

大表扫描、backfill、索引重建、FTS/derived 清理和 `VACUUM` 不属于 HTTP controller；应放在 SQLite migration/derived maintenance 或显式 maintenance command，并保持 listener 可用、批量执行、可恢复。不得删除或重写 `usage_events` 来“修复”派生数据。

### 源码、构建产物、依赖、运行时和凭证

| 类别 | 位置/来源 | 处理规则 |
| --- | --- | --- |
| 源码 | tracked 的 `apps/`、`bin/`、`docs/`、`tests/` 和根配置 | 可审查、可修改，但要遵守本文件及 `AGENTS.md` 的 owner/架构边界 |
| Web 构建产物 | `apps/web/dist/`、`apps/web/dist-demo/` | 由 npm build 生成，不手工维护；生产单文件来自 `apps/web/dist/index.html` |
| 嵌入面板 | `apps/manager-server/internal/httpapi/web/management.html` | 打包时由 Web 单文件同步/嵌入；权威源是 `apps/web/src`，禁止直接手改该文件 |
| Native/release 产物 | `dist/native/`、`dist/release/`、`bin/tmp/` | 由 release script/CI 生成，不能当作源码提交 |
| JS 依赖 | root/app `package*.json`、`node_modules/` | 依赖版本由 package lock 和 workspace manifest 管理；`node_modules` 不属于源码 |
| Go 依赖 | `apps/manager-server/go.mod`、`go.sum` 与外部 module cache | 不把 module cache 或下载内容复制进 repo |
| 运行时数据 | `/data` 或 native `data/`、SQLite/WAL/SHM、`usage-imports/` | 只作为部署 volume/目录备份；不要提交或放进 Docker image source context |
| 运行日志/进程状态 | Docker stdout，native `run/`、`logs/`、PID 文件 | live 诊断时读取；不作为 source 或 release 事实 |
| 凭证 | secret file/env、SQLite 加密 settings、CPA 自己的 Auth File | 不提交 CPA Management Key、Admin Key、data.key、OAuth token、Auth File 或普通 API key |

## 当前个人定制的职责边界

以下是当前 `lwj_dev` 相对 `origin/main` 的代码职责，不代表官方仓库能力，也不代表某个环境已经部署：

1. **本地 Codex 会话读取**：`internal/service/codexinspection/local_session.go` 通过 `CPAMP_CODEX_EXECUTABLE` 或 PATH 中的 `codex` 启动 `codex app-server --listen stdio://`，读取 `account/read`、`account/rateLimits/read`、`account/usage/read`，由 `/v0/management/codex-inspection/local-session` 返回本地会话快照。它读取的是运行 Manager Server 的机器上的本地 Codex app-server，不等同于 CPA 中注册的 Auth File，也不把 Codex session quota 变成官方账单数据。
2. **Codex weekly pool 估算**：`internal/service/codexinspection/weekly_estimate.go`、`model`、`repository/codexinspection` 和 SQLite schema 保存同账号/同 reset 周期的 baseline，并基于 CPA 观测到的成本变化或 credits 变化给出 preliminary/reliable 等状态。`CodexWeeklyPoolEstimate.Official` 明确为非官方估算；前端 `features/monitoring/components/CodexWeeklyPoolEstimate.tsx` 还提供 Pro 无实测值时的启发式展示。该功能不能替代官方 quota、账单或价格来源。
3. **loopback-only passwordless**：`config`、`http/middleware/auth`、bootstrap/admin auth、setup 和 Web login 共同支持已配置的 loopback Manager Server 无密码访问；安全边界是 loopback host/CORS 校验，不是移除所有鉴权。
4. **监控与 UI 接口**：`apps/web/src/features/monitoring`、`services/api/usageService.ts`、多语言资源和对应测试把上述本地会话、weekly estimate、服务端巡检和账户动作展示出来；这些前端代码仍通过 Manager Server/CPA API 工作，不直接拥有后端 SQLite 或 CPA provider runtime。

修改这些定制时，保持以下接口不漂移：local session 的 `status/source/reason/snapshot` 结构、weekly estimate 的状态/来源/非官方语义、Admin Key 与 CPA Management Key 的分离、CPA Panel 与 Manager Server 两种模式的可用性判断。

## 开发与验证入口

### 开发入口

```bash
# Web
npm run dev
npm run dev:demo
npm run preview

# VitePress
npm run docs:dev
npm run docs:preview

# Manager Server（会启动本地服务，仅在需要运行时执行）
cd apps/manager-server && go run ./cmd/cpa-manager-plus

# Docker 完整模式（会构建/启动服务，仅在需要运行时执行）
docker compose -f docker-compose.manager.yml up --build
```

原生发布包先执行 `npm run build`，再由 `bin/release/package-native.sh` 编译 Linux/macOS/Windows amd64/arm64 二进制；原生包内的 `cpa-manager-plusctl`/`.ps1` 只负责进程管理，不改变业务配置语义。

### 按改动范围验证

| 改动 | 最小相关检查 |
| --- | --- |
| Web TypeScript/React | `npm run type-check`、`npm run lint`、`npm run test` |
| Web bundle/嵌入面板 | 上述检查加 `npm run build`；必要时检查 single-file/demo isolation |
| Manager Server/service/repository | `npm run manager-server:test`；并发、worker、collector 或 migration 改动加 `cd apps/manager-server && go test -race ./...` |
| docs/VitePress | `npm run docs:build`；再运行包含 `tests/docsContentIntegrity.test.mjs` 的 `npm run test` |
| installer/native/release | 对应 `tests/installerScript.test.mjs`、`tests/nativeControlScripts.test.mjs`、`tests/release*.test.mjs`，并按脚本要求执行 `bash -n`/release validator |
| Docker/compose/packaging | `docker compose -f docker-compose.manager.yml config`，必要时 Docker build 和 native package 校验 |

验证结果只说明本次命令覆盖到的源码和本地 fixture。测试通过不等于 fork 已推送、镜像已发布、服务已启动或 live queue 正常。

## 修改禁区与结构约束

- 不直接修改 `apps/manager-server/internal/httpapi/web/management.html`；修改面板必须回到 `apps/web/src`，重新 build/同步。
- 不提交 Admin Key、CPA Management Key、data.key、OAuth/Auth File、普通 API key、SQLite/WAL/SHM、usage import、日志、PID、`node_modules` 或生成包。
- 不把 CPA/CLIProxyAPI 的模型转发、Provider 路由或凭证存储重复实现到 CPAMP；CPAMP 的 proxy 只处理管理/插件/兼容路径。
- 不绕过 `HTTP route -> controller -> service -> repository/store` 分层；controller 不承载领域规则，领域逻辑放到 `internal/service/<domain>`。
- 前端 `features/`、`components/` 不 import `pages/`；共享 API、类型、store、hook 和 utility 放到既有对应目录。
- 不为清理派生数据删除或改写 `usage_events`；migration/backfill 必须批量、幂等、可恢复，且不能让启动等待随数据量增长的重建工作。
- 修改 collector、auth、setup、proxy、monitoring 或 shared config 时，必须同时考虑 CPA Panel 与 Full Docker/Manager Server 语义；不要只验证一个模式。
- `AGENTS.md`、`CONTRIBUTING.md`、workflow、release 拓扑和其他协作规则不是本结构说明的临时草稿；除非明确授权，不在业务改动中顺手重写它们。
- 不在本仓库执行未经授权的 fetch/merge/push、服务重启、数据库清理或远端发布；本地 source、保存配置、本地部署和 live 行为要分开报告。

## 结构同步规则

1. **同步前**：读取当前 `AGENTS.md`，检查 `git status --short --branch`、`git remote -v`、`git branch -vv`，再用 `git ls-files` 和 `find` 确认拓扑；保留其他 agent 或用户的 dirty changes。
2. **判断来源**：用 `git diff origin/main...HEAD` 分离官方变化和 `lwj_dev` 定制变化；不要用 commit message 或历史部署日志推断当前代码行为。
3. **更新本文**：仅当顶层目录、关键二级目录、实际接口/数据边界、构建产物或定制职责发生变化时更新；新文件应先确认已 tracked/属于 source，再加入目录表。
4. **接口变更**：如果 local session、weekly estimate、鉴权、collector、SQLite schema 或 Web feature availability 改变，先更新实现和测试，再同步本文的边界/验证入口；不要用文档掩盖未验证的接口漂移。
5. **Infra 投影**：`cpamp` 仍作为 `cpamp-service` 和 `cpa-usage-monitoring` 的 companion 引用。源码路径、启动参数、数据目录或健康接口如有变化，应在实际拥有 companion manifest 的 Infra 仓库同步更新，并在本仓库单独核验，不把 Infra 配置复制成第二份源码真相。
6. **验证后交付**：记录实际执行的命令和结果，区分“文件已修改”“Git 已保存/提交”“远端已推送”“本地服务可访问”“live collector/queue 已验证”；本文更新本身不证明后四项。

`STRUCTURE.md` 只负责长期结构和边界说明，不替代 release notes、部署 runbook、当前 live health 报告或个人环境的 secret/config 记录。
