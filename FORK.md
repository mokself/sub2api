# FORK.md：二创约定

本仓库 fork 自 [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api)，在它的基础上做二次开发，并持续合并上游更新。

**硬约束：尽量少改上游文件，让每次同步上游时不冲突或少冲突。**

新增的文件永远不会和上游冲突，冲突只出在双方都改过的文件上。所以二创代码全部放在自己的命名空间里；必须改上游文件时，只留一行接入点。

本文件、`backend/internal/mok/`、`frontend/src/mok/` 等都是 fork 独有的，上游没有，可以随意修改。

## 1. 命名空间：`mok`

| 位置 | 约定 |
|---|---|
| 后端代码 | `backend/internal/mok/` 及其子包 |
| 后端同包扩展 | 在上游包里新建 `<原文件名>_mok.go`，测试文件为 `<原文件名>_mok_test.go` |
| 前端代码 | `frontend/src/mok/`，测试放在 `__tests__/` 里，和上游一致 |
| 数据库迁移 | `backend/migrations/mok_NNN_<描述>.sql`，并发索引照旧用 `_notx.sql` 后缀 |
| 数据库表 | 表名以 `mok_` 开头 |
| HTTP 接口 | `/api/v1/mok/...` |
| 配置 | 配置文件中的 `mok:` 段，对应环境变量 `MOK_*` |
| 前端文案 | key 统一放在 `mok.` 下 |
| CI | `.github/workflows/mok-*.yml` |

不要用 `custom` 作前缀，上游已经有 `custom_admin_keys`、`custom_menu_items` 等同名概念。

## 2. 接入点规则

必须修改上游文件时：

1. 只加调用，不搬动、不重排、不格式化上游原有的代码，逻辑写在 fork 自己的文件里。
2. 每个接入行都打标记：Go/TS/JS 用 `// [mok]`，Vue 模板和 HTML 用 `<!-- [mok] -->`，YAML、Shell、Dockerfile 用 `# [mok]`。接入点是多行代码块时，在第一行打标记。
3. 能写成一行就写成一行，例如 `price = mokAdjustPrice(ctx, price) // [mok]`。
4. 尽量加在上游不常改动的位置，比如函数末尾、单独一行，不要夹在上游经常增删的列表中间。
5. 新增或删除接入点时，同步更新下面的接入点清单。

列出当前所有接入点：

```bash
git grep -n "\[mok\]"
```

### 接入点清单

| 上游文件 | 接入内容 | 用途 |
|---|---|---|
| `backend/cmd/server/wire.go` | 导入 `internal/mok`；`wire.Build` 里的 `server.ProviderSet,` 改为 `server.ProvideRouter, mok.ServerProviderSet,` | 把 fork 后端模块接入依赖注入，并注册它的路由，见 4.1 |
| `backend/cmd/server/wire_gen.go` | 生成文件，内容随 `wire.go` 变化 | 不要手改；按 4.1 的命令重新生成 |
| `frontend/src/main.ts` | 第一行导入 `installMok`；在 `await initI18n()` 之后调用 `await installMok(app)` | fork 前端入口 |
| `frontend/src/views/admin/AccountsView.vue` | 导入 `@/mok/accounts`；在 `allColumns` 的 `return c` 前调用 `mokAddAccountColumns(c, t)`；加 `#cell-mok_claude_plan` 插槽 | 账号管理的「Claude 套餐」列，见第 11 节 |

## 3. 先看能不能零代码

- 管理后台的系统设置里可以直接改：站点名称、Logo、副标题、首页内容、文档链接、联系方式、自定义菜单（以 iframe 嵌入外部页面），以及一些界面元素的隐藏开关。
- 内嵌前端会优先返回运行目录下 `data/public/` 里的同路径文件（见 `backend/internal/web/embed_on.go`），可以用来替换前端构建产物里已有的静态文件。
- 上游的插件系统（`.s2plugin`，见 `docs/PLUGIN_DEVELOPMENT.md`）目前只开放了 `openai.oauth.outbound_transport.v1` 这一种能力，只适合定制 OpenAI OAuth 账号的出站传输。

## 4. 后端

### 4.1 路由与依赖注入

新模块放在 `backend/internal/mok/`，导出一个 wire `ProviderSet`。根对象 `mok.Module` 的 provider 接收 `*gin.Engine` 和所需的上游依赖，在构造时注册自己的路由。这样 `router.go`、`http.go`、`handler.go` 都不用改。实际写法见 `backend/internal/mok/module.go`：

- 管理端接口挂在 `/api/v1/mok/admin` 下；
- 中间件链和上游 `/api/v1/admin` 一致（管理员鉴权、审计、合规确认）；
- 另外拒绝受限范围的管理密钥，因为 fork 接口没有实现上游按账号做的范围校验。

`mok.Module` 是这样被构建出来的，`wire.go` 里只改了一行：把 `server.ProviderSet,` 换成 `server.ProvideRouter, mok.ServerProviderSet,`。

- `mok.ServerProviderSet` 里的 `mok.ProvideHTTPServer` 包装了上游的 `server.ProvideHTTPServer`，并以 `*mok.Module` 为参数。所以 wire 会先构建 fork 模块（注册路由），再创建 HTTP server。
- 这样做不需要往上游的 `Application` 结构体和 `wire.Struct(...)` 那一行加字段。那两处每次上游给 `Application` 加字段都会改，而 `server.ProviderSet` 自 2025-12 起就没变过。
- 如果上游往 `server.ProviderSet` 里新增了 provider，要把它补到 `wire.go` 那一行里。重新生成 `wire_gen.go` 时，wire 会直接报出缺少的 provider，不会悄悄出错。
- 代价是 fork 模块拿不到上游 `Application` 的生命周期（目前只有 `Cleanup`）。以后 fork 功能需要后台任务和优雅退出时，再考虑给 `Application` 加字段。

新功能要接入时，往 `mok.ProviderSet` 里加 provider，再把 handler 挂到 `NewModule` 上就行，不用再改 `wire.go`。

用上游过去 180 天的真实提交做过合并模拟：`wire.go`（18 次提交）、`main.ts`（2 次）、`AccountsView.vue`（44 次）上的接入点都能干净合并。

改了依赖注入之后，在 `backend` 目录下重新生成 `wire_gen.go`：

```bash
go run github.com/google/wire/cmd/wire@v0.7.0 gen ./cmd/server
```

不要用 `go generate ./cmd/server`：上游的 `go.sum` 缺少 wire 命令行工具的依赖，`main.go` 里的那条 generate 指令会直接失败。而 `wire_gen.go` 里那条带 `-mod=mod` 的指令会改写 `go.mod` 和 `go.sum`。上面这条命令在模块之外运行 wire，不改动任何上游文件。

接口必须挂在 `/api/` 下面。内嵌前端的中间件（`internal/web/embed_on.go` 中的 `shouldBypassEmbeddedFrontend`）只放行 `/api/`、`/v1/` 等少数前缀，其他路径一律返回前端的 `index.html`。

依赖注入里可以直接拿到的常用依赖有：`*ent.Client`、`*sql.DB`、`*redis.Client`、`*config.Config`、各个上游 service，以及 `middleware` 包里的鉴权中间件。

### 4.2 配置

不要往 `internal/config/config.go` 的 `Config` 结构体里加字段，它是上游改得最频繁的文件之一。上游用全局 viper 加载配置，`viper.Unmarshal` 会忽略不认识的段，所以 fork 在配置文件里加一个 `mok:` 段不影响上游。

读取方式：先 `viper.SetDefault("mok.xxx", 默认值)`，再用 `viper.GetString("mok.xxx")` 这类方法读取，配置文件和环境变量（`mok.xxx` 对应 `MOK_XXX`）都会生效。读配置的 provider 要以 `*config.Config` 为参数，保证在上游加载完配置之后才执行。不要用 `viper.Sub("mok")`，配置文件里没有 `mok:` 段时它会返回 nil。

### 4.3 数据库

- 迁移执行器（`internal/repository/migrations_runner.go`）把 `backend/migrations/*.sql` 按文件名排序，执行所有还没记录在 `schema_migrations` 里的文件，不检查编号格式。
- fork 的迁移命名为 `mok_001_<描述>.sql`、`mok_002_<描述>.sql`。字母排在数字后面，所以在新建的库上，它们总是在全部上游迁移之后执行。
- 迁移一旦执行过就不能再改（有 SHA256 校验）。需要改动就新增一个迁移。
- 优先建 `mok_` 前缀的扩展表，用上游表的 ID 关联，不要 `ALTER` 上游的表。
- 不要往 `backend/ent/schema` 加实体。`ent/client.go`、`ent/mutation.go` 这类汇总型生成文件，会和上游每次 schema 变更冲突。fork 的表有两种做法：
  - 用原生 SQL，`*sql.DB` 可以直接注入；
  - 在 `backend/internal/mok/ent/` 单独生成一套 ent。这套 ent 和上游实体之间建不了 edge，用普通的 ID 字段关联。

### 4.4 修改上游行为

计费、网关转发这类逻辑都在上游最热的文件里，见第 8 节。改的时候只在上游函数里加一行调用，逻辑写进同一个包下的新文件 `xxx_mok.go`。同包可以直接访问未导出的成员。

### 4.5 依赖

尽量不往 `backend/go.mod` 加新依赖。必须加时，冲突以上游版本为准，再用 `go get` 补回 fork 的依赖，然后执行 `go mod tidy`。

## 5. 前端

### 5.1 入口

fork 代码放在 `frontend/src/mok/`：

```text
frontend/src/mok/
├── index.ts      # installMok(app)：fork 前端入口
├── i18n/         # fork 文案 zh.ts / en.ts，合并逻辑，mt() / mokKey()
├── api/          # fork 接口封装
├── accounts/     # 账号管理页的扩展（Claude 套餐列）
└── __tests__/    # fork 自己的测试
```

`frontend/src/main.ts` 有 2 处接入。import 放在文件第一行，因为上游加 import 都加在 import 块的末尾，放第一行可以避开：

```ts
import { installMok } from './mok' // [mok] fork features, see FORK.md

// bootstrap() 中，紧跟在 await initI18n() 之后、app.use(router) 之前：
  await installMok(app) // [mok] fork features, see FORK.md
```

调用位置有两个要求：

- 必须在 `initI18n()` 之后，原因见 5.3。
- 必须在 `app.use(router)` 之前。路由器在 `app.use(router)` 时开始首次导航，fork 的路由如果注册晚了，直接打开 fork 页面的地址会匹配不到。

以后新增的 fork 前端功能都在 `installMok` 里接入，不用再改 `main.ts`。

### 5.2 路由

在 `installMok` 里用 `router.addRoute()` 注册，不改 `router/index.ts`。路由守卫照常生效：`meta.requiresAuth` 默认为 true，管理员页面另加 `meta.requiresAdmin: true`。页面组件自己在外层套 `<AppLayout>`，和上游页面一样。

`addRoute` 遇到同名路由时会先删掉旧的再添加，所以不改上游文件也能整体替换一个上游页面。代价是这个页面以后拿不到上游的更新，只在改动很大时才这样做。

### 5.3 文案（i18n）

不要往 `src/i18n/locales/` 里加 key。fork 的文案放在 `src/mok/i18n/zh.ts` 和 `en.ts`，全部挂在顶层 `mok` 下，两种语言的 key 要一一对应。

语言包是懒加载的：`loadLocaleMessages()` 用 `setLocaleMessage` 整体覆盖该语言的全部文案。所以 fork 的文案必须等语言包加载完再合并。`installMokI18n()` 已经处理好了：

1. 先对当前语言合并一次；
2. 再监听 locale，切换语言后重新合并。`setLocale()` 是先加载语言包、再切换 locale，所以监听触发时，新的语言包已经加载好了。

引用文案时有个坑：上游的 `src/i18n/__tests__/localeKeyCompleteness.spec.ts` 会扫描 `src/` 下所有 `.ts` / `.vue` 文件（连注释也扫），找出 `t('…')`、`$t('…')`、`i18n.t('…')`，以及 `titleKey` / `descriptionKey` / `keypath` 后面跟的字面量。凡是上游语言包里没有的 key 都会报错，而 `pnpm build` 会先跑这个测试。所以 fork 代码遵守这几条：

- 组件里用 `useMokI18n()` 返回的 `mt('claudePlan.column')`，它会读 `mok.claudePlan.column`；
- 需要完整 key 的地方（比如上游组件里的 `t`、路由 meta 的 `titleKey`）用 `mokKey('claudePlan.column')`，不要直接写 `'mok.xxx'` 字面量；
- 注释里也别写出上面那几种调用形式。

fork 自己的 key 由 `src/mok/__tests__/i18n.spec.ts` 检查：zh 和 en 的 key 一一对应、没有空值，所有 `mt('…')` / `mokKey('…')` 引用的 key 都存在。

### 5.4 侧边栏菜单

菜单写死在 `src/components/layout/AppSidebar.vue` 里，加菜单项需要接入：

- 用户端在 `finalizeNav()` 里，
- 管理端在 `adminNavItems` 的 `applyFeatureFlags(baseItems)` 处，

把菜单列表交给 fork 的函数，由它插入条目。

### 5.5 依赖

尽量不加 npm 依赖。必须加时：

1. `pnpm-lock.yaml` 冲突时，取上游的版本；
2. 手工合并 `package.json`；
3. 重新运行 `pnpm install`，生成新的锁文件。

CI 用 `--frozen-lockfile` 安装，锁文件必须提交。

## 6. 不要直接改的文件

| 文件 | 原因 | 替代做法 |
|---|---|---|
| `.gitignore` | 上游经常改 | 只在本机忽略的写进 `.git/info/exclude`；要提交被忽略的文件，用 `git add -f` |
| `backend/go.mod` 的 `module` 路径 | 会牵动所有 import | 保持 `github.com/Wei-Shaw/sub2api` |
| `backend/cmd/server/VERSION` | 上游每次发版都由机器人修改 | fork 的版本号通过 ldflags 注入，见第 9 节 |
| `README*.md`、`DEV_GUIDE.md` | 上游经常改 | fork 的说明写在本文件里 |
| 生成文件（`wire_gen.go`、`backend/ent/**`） | 重新生成时会被覆盖 | 改源头，再重新生成 |
| `internal/config/config.go`、`src/i18n/locales/**`、`src/types/index.ts` | 上游热点 | 见 4.2、5.3 |

上游 `.gitignore` 里有不少不带路径的规则，在任何层级都会生效，例如 `scripts`、`tests`、`build/`、`dist/`、`release/`、`tmp/`、`output/`。fork 的目录不要用这些名字。此外，`CLAUDE.md`、`.claude/`、`AGENTS.md`、根目录的 `docs/*` 和 `/plugins/` 也都被忽略了。提交前可以这样确认某个路径有没有被忽略：

```bash
git check-ignore -v <路径>
```

## 7. 同步上游

新克隆的仓库先配置一次：

```bash
git remote add upstream https://github.com/Wei-Shaw/sub2api.git
```

```bash
git config remote.upstream.fetch '+refs/heads/main:refs/remotes/upstream/main'
```

```bash
git config remote.upstream.tagOpt --no-tags
```

```bash
git config rerere.enabled true
```

- 只拉 `main`，不拉上游的 tag，免得把上游的 tag 推到 fork 上触发 `release.yml`。
- rerere 会记住冲突的解决方式，同一处冲突以后会自动按记下的方式解决。

日常同步用 merge，不要用 rebase，免得改写已经推送过的历史：

```bash
git fetch upstream
```

```bash
git merge upstream/main
```

### 冲突处理

| 文件 | 处理方式 |
|---|---|
| `backend/cmd/server/wire_gen.go` | 用 `git checkout --theirs` 取上游版本；修好 `wire.go` 的接入点后，在 `backend` 下运行 `go run github.com/google/wire/cmd/wire@v0.7.0 gen ./cmd/server` |
| `backend/ent/**` | fork 不改这里，正常不会冲突。万一冲突，取上游版本后运行 `go generate ./ent` |
| 带 `[mok]` 标记的接入行 | 取上游版本，再对照接入点清单把接入行加回去 |
| `go.mod`、`go.sum` | 取上游版本，用 `go get` 补回 fork 的依赖，再运行 `go mod tidy` |
| `pnpm-lock.yaml` | 取上游版本，手工合并 `package.json` 后运行 `pnpm install` |
| `backend/cmd/server/VERSION` | 一律取上游版本 |

merge 时 `--theirs` 指上游，`--ours` 指 fork。rebase 时两者正好相反，这也是不用 rebase 的原因之一。

### 同步后检查

1. 运行 `git grep -n "\[mok\]"`，对照接入点清单，确认接入点都还在。
2. 后端（在 `backend` 目录下）：

   ```bash
   go build ./...
   ```

   ```bash
   go test -tags=unit ./...
   ```

3. 前端（在 `frontend` 目录下）：

   ```bash
   pnpm install
   ```

   ```bash
   pnpm run typecheck
   ```

   ```bash
   pnpm run build
   ```

4. fork 的测试：

   ```bash
   go test -tags=unit ./internal/mok/
   ```

   ```bash
   npx vitest run src/mok
   ```

   第一条在 `backend` 下运行，第二条在 `frontend` 下运行。

Windows 上没有 make。重新生成 ent 代码在 `backend` 下运行 `go generate ./ent`；重新生成 wire 用 4.1 里的命令。

## 8. 上游热点文件

截至 2026-11-07，上游最近 90 天里改动最频繁的文件如下（括号内是提交次数），fork 尽量不碰：

- 后端：
  - `backend/internal/service/openai_gateway_forward.go`（62）
  - `backend/internal/handler/openai_gateway_handler.go`（59）
  - `backend/internal/service/openai_gateway_passthrough.go`（55）
  - `backend/internal/service/billing_service.go`（44）
  - `backend/cmd/server/wire_gen.go`（39）
  - `backend/internal/config/config.go`（39）
- 前端：
  - `frontend/src/i18n/locales/{zh,en}/admin/accounts.ts`（各 58）
  - `frontend/src/components/account/EditAccountModal.vue`（48）
  - `frontend/src/components/account/CreateAccountModal.vue`（41）
  - `frontend/src/types/index.ts`（40）

改动少、适合放接入点的文件：

- `frontend/src/main.ts`（0）
- `frontend/src/i18n/index.ts`（0）
- `backend/cmd/server/main.go`（1）
- `backend/internal/server/http.go`（1）
- `backend/cmd/server/wire.go`（7）

重新统计：

```bash
git log upstream/main --since="90 days ago" --name-only --pretty=format: | grep -v '^$' | sort | uniq -c | sort -rn | head -30
```

## 9. 发版

- 上游的 `release.yml` 由 `v*` tag 触发，镜像发到 `ghcr.io/<仓库所有者>/sub2api`，在 fork 上也能用。但它发版成功后会往默认分支推一个修改 `backend/cmd/server/VERSION` 的提交，之后每次同步都会在这个文件上冲突。
- 建议在 GitHub 网页上停用上游的 `release.yml` 和 `cla.yml`：Actions → 选中 workflow → Disable workflow。不要删除这两个文件。
- fork 的发版流程另写在 `.github/workflows/mok-release.yml` 里。用和上游区分开的 tag 前缀，例如 `mok-v*`。
- 版本号用 `-ldflags "-X main.Version=<版本号>"` 注入。`backend/cmd/server/main.go` 里，ldflags 注入的版本号优先于内嵌的 `VERSION` 文件，所以不需要修改 `VERSION`。

## 10. 本地开发注意事项

- **前端依赖用 pnpm 9 安装。** 上游 CI 用的是 pnpm 9；更新版本的 pnpm 对 `pnpm.overrides` 的校验不同，`--frozen-lockfile` 会报错。本机 pnpm 版本更高时，在 `frontend` 下运行：

  ```bash
  npx -y pnpm@9 install --frozen-lockfile
  ```

  不要为了让安装通过去改 `pnpm-lock.yaml`。
- **`pnpm build` 会清空 `backend/internal/web/dist/`，包括其中被 git 跟踪的 `.keep`。** 只想检查能否构建时，把产物输出到别处：

  ```bash
  npx vite build --outDir <临时目录> --emptyOutDir
  ```

- **Windows 的 Docker Desktop 上跑集成测试**，testcontainers 的 Ryuk 清理容器可能起不来（报错 `reaper: ... unexpected container status "removing"`）。本地运行时先设置环境变量：

  ```bash
  TESTCONTAINERS_RYUK_DISABLED=true go test -tags=integration ./internal/mok/
  ```

- **fork 的测试在不在 CI 里：**
  - 后端测试打了 `unit` / `integration` 构建标签，会跟着上游 CI 的 `make test-unit` / `make test-integration` 一起跑；
  - 前端的 CI 只跑根目录 `Makefile` 里列出的关键测试，fork 的前端测试不在其中，需要本地运行 `npx vitest run src/mok`。lint 和类型检查在 CI 里覆盖了 fork 代码。

## 11. 已实现的二创功能

### Claude 套餐列（2026-11-08）

在账号管理列表里新增「Claude 套餐」列，位于「平台/类型」之后，显示 Claude OAuth 账号的套餐档位。数据来自 `GET https://api.anthropic.com/api/oauth/profile` 返回的 `organization_type`、`rate_limit_tier`、`seat_tier`、`subscription_status`。

- **显示规则**：
  - 套餐名称按 `rate_limit_tier` 里的倍数决定：`max_20x` 显示为 Ultra，`max_5x` 显示为 Max，其他倍数显示为 `Max Nx`。没有倍数时按 `organization_type` 显示 Pro / Max / Team / Enterprise / Free。
  - 这套对应关系集中在 `frontend/src/mok/accounts/claudePlan.ts` 的 `MAX_MULTIPLIER_BADGES` 和 `ORGANIZATION_TYPE_BADGES` 里，要改名改这两处。
  - 悬停可以看到四个原始字段和更新时间。`subscription_status` 有值且不是 `active` / `trialing` 时，会额外用琥珀色显示状态。
- **哪些账号会获取**：只有平台为 anthropic、类型为 oauth 的账号；其他账号显示 `-`。授权里缺少 `user:profile` 权限的账号同样显示 `-`，悬停提示需要重新授权。
- **获取与缓存**：
  - 后端：
    - `backend/internal/service/claude_reset_credits_mok.go` 复用上游 `ClaudeResetCreditService` 的账号检查、令牌、代理和请求头去调用 profile 接口；
    - 结果存在 `mok_claude_plan_tiers` 表（迁移 `mok_001_claude_plan_tiers.sql`）。
  - 缓存策略：
    - 成功结果缓存 12 小时；
    - 失败后 10 分钟内不自动重试，并保留上次成功的套餐；
    - 全局最多同时发起 8 个 profile 请求。
  - 前端：
    - 列表加载后，把当前页的 Claude OAuth 账号按每批 20 个合并请求 `GET /api/v1/mok/admin/claude-plan-tiers?ids=…`；
    - 单元格上的刷新按钮会带 `refresh=true` 强制重新获取。
- **代码位置**：后端在 `backend/internal/mok/`，前端在 `frontend/src/mok/accounts/` 和 `frontend/src/mok/api/claudePlan.ts`。
