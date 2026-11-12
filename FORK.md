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
| `backend/internal/service/affiliate_service.go` | `AccrueInviteRebateForOrder` 里算出 `rebateRatePercent` 后加一段 `mokTieredRebateRatePercent` 调用；`GetAffiliateDetail` 的 `EffectiveRebateRatePercent:` 一行改为调用 `mokEffectiveRebateRatePercent`；`AdminGetUserOverview` 里加一行 `mokTieredDisplayRatePercent` | 阶梯返利的计算和比例展示；消耗返利模式下 `mokTieredRebateRatePercent` 返回 0，充值不再返利。见第 11 节 |
| `backend/internal/repository/affiliate_repo.go` | 常量 `affiliateCodeLength` 从 12 改为 4 | 系统生成的邀请码长度，见第 11 节 |
| `frontend/src/views/admin/SettingsView.vue` | 在「专属用户管理」注释上方加 `<MokAffiliateSettings />` | 系统设置里的「返利方式」（含消耗返利明细）和「阶梯返利」两张卡片 |
| `frontend/src/views/user/AffiliateView.vue` | 在统计卡片下方加 `<MokAffiliateUserPanel />` | 用户邀请返利页的阶梯进度卡片和消耗返利卡片；消耗返利模式下改写页面上讲充值返利的两句话 |

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
- fork 的后台任务在 `NewModule` 里启动；`mok.ProvideHTTPServer` 用 `srv.RegisterOnShutdown(m.Stop)` 让它们在 HTTP server 优雅退出时停止，同样不用碰上游的 `Application`。新的后台任务在 `Module.Stop()` 里加上停止调用即可。

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
├── affiliate/    # 邀请返利的扩展（返利方式、阶梯返利）
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

**往上游页面里插组件**：在 `installMok` 里用 `app.component('MokXxx', defineAsyncComponent(...))` 注册成全局异步组件，再到 `src/mok/components.d.ts` 的 `GlobalComponents` 里声明类型。这样上游模板里只要加一行 `<MokXxx />`，不用在上游文件的 `<script>` 里加 import；import 区是上游经常改的地方，容易冲突。上游的单元测试不认识这个组件，Vue 会打印一条 resolve 警告，测试照常通过。

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

**改写上游页面上的文案**：不改上游语言包，把新句子写在 fork 里，运行时用 `mergeLocaleMessage` 盖到上游的 key 上，需要时再换回原句。现有的例子是 `src/mok/i18n/affiliateUsageWording.ts` 和 `setAffiliateUsageWording()`。`i18n.spec.ts` 会检查被改写的 key 在上游语言包里还存在、占位符（如 `{rate}`）和上游一致；上游改了这些 key 的名字或占位符时，这个测试会失败。

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

fork 的 Docker 镜像由 fork 自己的 `.github/workflows/mok-release.yml` 构建：

- 先跑 fork 的前后端测试，通过后构建 amd64 + arm64 双架构镜像，推到 `ghcr.io/mokself/sub2api`。
- 上游的 `release.yml` 不用它。原因是它由 `v*` tag 触发，发版成功后还会往默认分支推一个修改 `backend/cmd/server/VERSION` 的提交，以后每次同步上游都会在这个文件上冲突。

### 发一个版本

在要发布的提交上打 tag 并推送。tag 格式是 `mok-v<上游版本>.<fork 序号>`：

```bash
git tag mok-v0.2.20.1
```

```bash
git push origin mok-v0.2.20.1
```

- 推送后会得到两个镜像标签：`0.2.20-mok.1` 和 `latest`。后台显示的版本号是 `0.2.20-mok.1`。
- 同一个上游版本上再发版就递增序号，比如 `mok-v0.2.20.2`；同步到上游 0.2.21 之后序号从 1 重新开始，即 `mok-v0.2.21.1`。
- 不要打 `v` 开头的 tag，那会触发上游的 `release.yml`。
- 想重新构建已有的 tag：进入 Actions → mok release image → Run workflow，在 "Use workflow from" 里选中那个 tag。

版本号由工作流通过构建参数 `VERSION` 传给 Dockerfile，再用 ldflags 注入程序，所以不需要修改 `VERSION` 文件。

### 第一次使用前（在 GitHub 网页上操作）

1. **启用 Actions。** GitHub 默认不运行 fork 里的工作流。打开仓库的 Actions 页，点 "I understand my workflows, go ahead and enable them"。
2. **停用两个上游工作流（建议）。** 启用后，上游的 `release.yml` 和 `cla.yml` 也会一起启用。在 Actions 页选中它们，点右上角 "…" → Disable workflow，不要删文件。上游的 CI 和 Security Scan 可以留着，每次推送都会帮你跑一遍测试。
3. **确认镜像能匿名拉取。** 2026-11-08 第一次构建（`mok-v0.2.20.1`）之后验证过：仓库是公开的，`ghcr.io/mokself/sub2api` 不登录就能拉取。如果以后把仓库或镜像包改成私有，就要在服务器上用一个有 `read:packages` 权限的 token 先执行 `docker login ghcr.io`。镜像包的可见性在 GitHub 个人主页 → Packages → sub2api → Package settings 里调整。

### 服务器上的部署

在部署目录的 `docker-compose.override.yml` 里，把镜像换成 fork 的：

```yaml
services:
  sub2api:
    image: ghcr.io/mokself/sub2api:latest
```

以后的更新和用上游镜像时一样：

```bash
docker compose pull
```

```bash
docker compose up -d
```

- 想固定版本、方便回滚，就把 `latest` 换成具体的标签，比如 `0.2.20-mok.1`。
- 后台的「在线更新」和「回滚」会从上游 Wei-Shaw/sub2api 下载程序，把 fork 的功能覆盖掉。fork 部署不要用这两个功能，更新一律走「推 tag → `docker compose pull`」这条路。

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

- **不开 Docker 也能跑 fork 的集成测试。** 把 `MOK_TEST_POSTGRES_DSN` 设成一个本地 PostgreSQL 的连接串（`postgres://` URL 格式，用户要有建库权限），测试会在上面为每个用例建一个临时库、跑完全部迁移、结束后删掉：

  ```bash
  MOK_TEST_POSTGRES_DSN='postgres://postgres:postgres@127.0.0.1:5432/postgres?sslmode=disable' go test -tags=integration ./internal/mok/
  ```

- **上游自带会失败的单元测试。** 下面两个 `internal/service` 里的测试用纯上游代码跑也会失败，和 fork 无关。全量跑单元测试看到它们失败时不用排查，同步上游后再看是否已修复：
  - `TestOllamaProbeCallback_StaleLongDoesNotOverrideNewShort`：2026-11-09（`6b4b37db4`）和 2026-11-11（`530c5e3a4`）都会失败；
  - `TestCommandCodeGatewayPassesThroughCatalogProtocols`：2026-11-11（`530c5e3a4`）起，全量跑时失败，单独跑能通过，应该是和其他测试共用了状态。
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

### 阶梯返利（2026-11-09）

邀请返利原来是固定比例，现在邀请人的返利比例会随「累计推广额」提高。在后台「系统设置 → 功能开关 → 邀请返利」里，全局返利比例下方的「阶梯返利」卡片里配置，例如「累计推广额达到 $50,000 → 12%，达到 $100,000 → 15%」。

- **累计推广额**：邀请人本人加上名下所有直接被邀请人的累计充值。包括：
  - 已完成、未退款的在线支付订单：余额充值按到账金额、不含充值赠送，订阅按订单金额；有退款申请但还没退的仍然计入，已退款和部分退款的不计入；
  - 已使用的余额兑换码（不含支付订单自动生成的 `PAY-` 兑换码，避免重复计算）。

  管理员在后台直接加的余额不计入累计额，但这笔充值本身照常按当前档位返利。被邀请人的被邀请人不算在内。累计额每次从订单表和兑换码表实时汇总，不另外存表，所以上线前的历史充值和之后的退款都会自动反映出来。
- **计算规则**：
  - 判断档位用的累计额包含本笔充值；达到某一档后，这一笔整笔按该档比例返利。
  - 低于第一档时按「全局返利比例」。
  - 设了专属比例的邀请人继续用专属比例，不走阶梯。
  - 单人返利上限、返利有效期、冻结期照常生效。
  - 不配置任何档位时，行为和上游完全一样。
- **展示**：
  - 用户的邀请返利页：顶部「我的返利比例」和「使用说明」都显示当前档位的比例；下方新增阶梯进度卡片，显示累计推广额、离下一档还差多少和完整阶梯。设了专属比例的用户不显示这张卡片。
  - 管理端「邀请返利 → 记录」里的用户概览，也显示当前档位的比例。
- **存储与接口**：
  - 阶梯存在 `settings` 表的 `mok_affiliate_rebate_tiers` 键里（JSON 数组），不需要迁移。
  - 管理端：`GET` / `PUT /api/v1/mok/admin/affiliate-rebate-tiers`，最多 20 档，比例 0–100，门槛大于 0 且不能重复；`PUT` 会记入操作审计。
  - 用户端：`GET /api/v1/mok/affiliate/rebate-tiers`。
- **出错时**：查询累计额失败时，这笔返利会返回错误：在线支付订单会走上游原有的重试流程，不会按全局比例少返；只用于展示的地方查询失败时退回显示全局比例。
- **代码位置**：
  - 后端：计算在 `backend/internal/service/affiliate_service_mok.go`，累计额 SQL 在 `backend/internal/repository/affiliate_repo_mok.go`，接口在 `backend/internal/mok/affiliate_tiers_handler.go`。
  - 前端：组件在 `frontend/src/mok/affiliate/`，接口封装在 `frontend/src/mok/api/affiliateTiers.ts`。

### 消耗返利（2026-11-09；2026-11-12 改为每小时结算并增加明细）

返利方式可以在「充值返利」和「消耗返利」之间切换，在后台「系统设置 → 功能开关 → 邀请返利」的「返利方式」卡片里操作，切换前会弹窗确认。默认是充值返利，和上游一样。

- **充值返利**：被邀请人充值时按比例返利，即上游原有方式，叠加上面的阶梯返利。
- **消耗返利**：
  - 充值不再返利；每小时结算一次，按每个被邀请人上一个小时的**余额消耗**给邀请人返利。
  - 余额消耗 = `usage_logs` 里计费方式为余额（`billing_type = 0`）的 `actual_cost` 之和；订阅套餐内的用量不算。
  - 比例和充值返利一样按「专属比例 → 阶梯 → 全局比例」确定，阶梯仍按累计充值（推广额）定档，按结算时的档位计算。
  - 单人返利上限、返利有效期（从被邀请人绑定邀请关系时算起）、冻结期照常生效；返利额度记进上游的 `aff_quota` 等字段，和充值返利共用提现、转余额流程。
  - 某个小时结算时邀请返利功能是关闭的，这个小时会记为「已跳过」，不返利，之后再打开也不补。
- **结算时间**：
  - 结算时段是服务器时区（`TZ`）的整点小时。每个整点过 5 分钟后结算上一个小时；后台任务每分钟检查一次，启动 1 分钟后第一次检查。
  - 切换返利方式的那个小时不满一小时：从切换时刻到下一个整点，或从整点到切换时刻。
  - 服务停机期间漏掉的小时会在恢复后补结算；每次检查最多处理 744 个小时（31 天），更多的积压在后面几次检查里接着补。
  - 多实例部署时用 Redis 锁（`mok:affiliate_usage_rebate`）保证同一时间只有一个实例在结算。
- **切换的边界**：
  - 切到消耗返利时记下切换时刻（`mok_affiliate_usage_rebate_since`），只结算这之后的消耗。
  - 切回充值返利时记下结束时刻（`mok_affiliate_usage_rebate_until`），结束之前的消耗照常在下一次结算时结算完，之后的不再结算。
  - 再次切到消耗返利时，起点重新记为新的切换时刻，结束时刻清空。切回充值返利后，如果没等最后一个时段结算完（最长约 1 小时 5 分钟）就又切到消耗返利，那段还没结算的消耗不会再结算。
- **不会重复返利**：每个「被邀请人 + 时段」只结算一次。结算记录和返利额度在同一个事务里写入，中途失败会整体回滚，下次重试。
- **明细和展示**：
  - 管理端「返利方式」卡片：当前方式、起止时刻、已结算到的时刻，最近 14 天按天汇总（被邀请人数、消耗额、返利额）。点「查看明细」打开全部结算记录：时段、邀请人、被邀请人、消耗、比例、返利，可以按邮箱或用户名搜索。
  - 用户的邀请返利页：多一张「消耗返利」卡片，显示最近一笔，并能打开自己的明细；被邀请人只显示打码后的邮箱。消耗返利模式下这张卡片一直显示；切回充值返利后，有过记录的用户仍然能看到。
  - 用户的邀请返利页上，「我的返利比例」下的说明和「使用说明」第 2 条改为讲消耗返利；阶梯进度卡片里加一句说明。
  - 上游管理端的「返利记录」里，每条消耗返利也各占一行（订单相关的列显示 `-`）。
- **存储与接口**：
  - 方式和起止时刻存在 `settings` 表的 `mok_affiliate_rebate_mode`（`recharge` / `usage`）、`mok_affiliate_usage_rebate_since`、`mok_affiliate_usage_rebate_until` 三个键里。
  - 结算明细在 `mok_affiliate_usage_rebate_items` 表（每个被邀请人每个时段一行），时段汇总在 `mok_affiliate_usage_rebate_periods` 表。迁移 `mok_003_affiliate_usage_rebate_periods.sql` 建这两张表，并把 `mok_002` 时期按天结算的记录搬过来，一天作为一个时段。
  - `mok_002` 的两张表：`mok_affiliate_usage_rebate_days` 在 `mok_003` 里删除；`mok_affiliate_usage_rebates` 保留，里面只留「升级时某一天结算到一半」的记录，正常情况下是空的。按小时结算时会跳过这些被邀请人的那一天，避免再返一次。
  - 管理端：
    - `GET` / `PUT /api/v1/mok/admin/affiliate-rebate-mode`，`PUT` 的请求体是 `{"mode": "usage"}`，会记入操作审计；
    - `GET /api/v1/mok/admin/affiliate-usage-rebates`，参数 `page`、`page_size`（最大 100）、`search`。
  - 用户端：
    - `GET /api/v1/mok/affiliate/usage-rebates`，参数 `page`、`page_size`，只返回自己作为邀请人的记录；
    - `GET /api/v1/mok/affiliate/rebate-tiers` 多返回一个 `rebate_mode` 字段。
- **升级注意**：
  - 从按天结算的版本（`0.2.20-mok.2`、`0.2.21-mok.1`）升级时，`mok_003` 在启动时自动执行。按小时结算从最后一个已结算的日子结束处接着算，升级当天已经过去的小时会在启动后一次补齐。
  - 升级后不要回滚到按天结算的旧镜像：旧版本找不到被删掉的 `mok_affiliate_usage_rebate_days` 表，消耗返利会停止结算并在日志里报错。这样设计是为了回滚后不会把已经按小时结算过的日子再按天返一次。
- **代码位置**：
  - 后端：结算逻辑在 `backend/internal/service/affiliate_usage_rebate_mok.go`，SQL 在 `backend/internal/repository/affiliate_usage_rebate_repo_mok.go`，接口在 `backend/internal/mok/affiliate_mode_handler.go`。
  - 前端：`frontend/src/mok/affiliate/` 下的 `AffiliateRebateModeSettings.vue`（管理端卡片）、`AffiliateUsageRebateRecords.vue`（两端共用的明细弹窗）、`AffiliateUserPanel.vue`（用户端卡片）；接口封装在 `frontend/src/mok/api/affiliateMode.ts` 和 `affiliateUsageRebates.ts`；页面文案改写在 `frontend/src/mok/i18n/affiliateUsageWording.ts`。

### 4 位邀请码（2026-11-12）

系统生成的邀请码（邀请返利页上的那串码）从 12 位改成 4 位，方便口头告诉别人或手动输入。

- **生成规则**：字符集不变，仍是 `ABCDEFGHJKLMNPQRSTUVWXYZ23456789`（没有容易看错的 I、O、0、1），4 位一共约 105 万个码。新注册用户和管理员「重置」出来的邀请码都是 4 位。管理员手动设置的专属邀请码不受影响，仍然可以是 4–32 位。
- **已有用户**：迁移 `mok_004_affiliate_codes_4_chars.sql` 在升级后第一次启动时执行，把系统生成的旧码全部换成随机的 4 位码，互不重复。下面两种不动：
  - 管理员手动设置过的专属邀请码；
  - 本来就是 4 位的码。
- **影响**：
  - 换码之后，之前发出去的旧邀请码和旧邀请链接全部失效，需要用户重新分享新的。
  - 邀请关系（谁邀请了谁）按用户 ID 记录，不受影响，已有的返利照常。
  - 访客浏览器里记住的旧邀请码，注册时绑定不上邀请人，注册本身不受影响。
- **容量**：码用得越多，生成新码时撞上已有码的机会越大，系统会重试最多 12 次。用户数在十几万以内基本不会重试失败；到几十万以上应该把长度调大。迁移时如果用户数超过可用的码数，多出来的用户保留旧码。
- **同步上游时**：`affiliate_repo.go` 里那一行如果冲突，保留 4。`backend/internal/repository/affiliate_code_mok_test.go` 会在生成的码不是 4 位时失败，也会在上游把绑定校验的最小长度调到 4 以上时失败。
- **代码位置**：上游文件 `backend/internal/repository/affiliate_repo.go` 的一行常量；迁移 `backend/migrations/mok_004_affiliate_codes_4_chars.sql`；测试 `backend/internal/repository/affiliate_code_mok_test.go` 和 `backend/internal/mok/affiliate_codes_integration_test.go`。
