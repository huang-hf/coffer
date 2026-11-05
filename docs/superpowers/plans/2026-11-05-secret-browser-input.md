# 浏览器设置密钥（secret add --ui）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增 `coffer secret add <NAME>... --ui`：在本机临时起 HTTP 服务并自动打开浏览器设置页，用户在页面里填写/更新密钥，一次保存后命令退出，agent 全程拿不到值。

**Architecture:** 新包 `internal/secretui` 负责一次性本机 HTTP 服务（token + Host/Origin 校验、内嵌页面、`/api/state`、`/api/save`、`/api/cancel`、超时），通过注入的 `Save` 回调写入，不直接依赖 store/config。`internal/cli` 负责参数解析、构造列表数据与存储说明文案、打开浏览器、输出结果。`internal/secret` 新增后端类型与修改时间查询。

**Tech Stack:** Go 1.25 标准库（`net/http`、`embed`、`html/template`、`crypto/rand`、`crypto/subtle`），无新增第三方依赖；页面为原生 HTML/CSS/JS。

**Spec:** `docs/superpowers/specs/2026-11-05-secret-browser-input-design.md`

## Global Constraints

- 不新增第三方依赖；页面不得引用任何外部资源（CDN/字体/脚本）。
- 密钥值不得出现在：stdout/stderr、日志、URL、进程参数、任何 HTTP 响应体。
- 服务只监听 `127.0.0.1`，随机端口；URL 形如 `http://127.0.0.1:<port>/?t=<token>`，token 为 32 字节 `crypto/rand` 的 hex。
- 默认超时 `5m`；超时/取消以非零退出码结束。
- 一次全部保存成功即结束；部分失败时服务继续、token 继续有效，可只重试失败项。
- 已有密钥更新时不改动 config 中已有的条目值（只对 config 中不存在的名字新增 `""` 条目，与 `secret add` 一致）。
- 不提供删除；只操作当前 namespace。
- 不处理 Codex 沙箱/超时。
- 仓库 `.gitignore` 含 `*.html`，内嵌页面需加例外 `!internal/secretui/assets/*.html`。
- `make build` 会把根目录 `SKILL.md` 复制覆盖 `internal/skill/SKILL.md`：以 `internal/skill/SKILL.md` 为准，改完后两份保持一致。

## Review Focus

- 用户粘贴的值带结尾换行（`\n`/`\r\n`）→ 只去掉结尾换行后保存，其他空白原样保留（Task 3 测试 `TestSaveTrimsTrailingNewlineOnly`）。
- 页面刷新/重复打开 → `/api/state` 可重复获取，`pending` 反映已保存进度，不导致结束（Task 3 测试 `TestStateIsRepeatableAndReflectsProgress`）。
- 命令里重复写同一个名字（`add A A --ui`）→ 去重，只出现一次（Task 5 测试 `TestParseSecretAddArgsDedupes`）。
- 超大请求体 → 413，不写入（Task 3 测试 `TestSaveRejectsOversizedBody`）。
- 浏览器打不开（`open` 失败）→ stderr 提示并继续等待，URL 已打印可手动打开（Task 3 测试 `TestOpenBrowserFailureKeepsWaiting`）。

---

### Task 0: 提交设计与计划文档

- [ ] **Step 1: Commit**

```bash
git add docs/superpowers/specs/2026-11-05-secret-browser-input-design.md docs/superpowers/plans/2026-11-05-secret-browser-input.md
git commit -m "docs: design and plan for browser-based secret input"
```

---

### Task 1: secret 包 — 后端类型与修改时间

**Files:**
- Create: `internal/secret/info.go`
- Modify: `internal/secret/file.go`（为 `*fileStore` 实现两个方法）
- Modify: `internal/secret/keychain.go`（darwin，为 `*KeychainStore` 实现 `backendKind`）
- Test: `internal/secret/info_test.go`

**Interfaces:**
- Produces:
  ```go
  type BackendKind string
  const (
      BackendFileEncrypted BackendKind = "file-encrypted"
      BackendFilePlain     BackendKind = "file-plain"
      BackendOSKeychain    BackendKind = "os-keychain" // macOS Keychain
      BackendOther         BackendKind = "other"
  )
  func Backend(s Store) BackendKind                                  // 未实现可选接口的 store → BackendOther
  func ModTime(s Store, namespace, name string) (time.Time, bool)    // 取不到 → (zero, false)
  ```
  实现方式：包内未导出可选接口 `backendKind() BackendKind` 与 `modTime(ns, name string) (time.Time, bool)`；`fileStore.modTime` 先 stat `.json.age` 再 stat `.json`。Linux/Windows store 不实现（落到 `BackendOther` / false）。

- [ ] **Step 1: Write the failing tests**

```go
func TestBackendAndModTimeForFileStore(t *testing.T) {
    t.Setenv("HOME", t.TempDir())
    t.Setenv("COFFER_USE_KEYCHAIN", "")
    s, err := NewStore()
    if err != nil { t.Fatal(err) }
    if got := Backend(s); got != BackendFilePlain { t.Fatalf("Backend = %q, want file-plain", got) }
    if _, ok := ModTime(s, "ns1", "A"); ok { t.Fatal("ModTime ok for missing secret") }
    before := time.Now().Add(-time.Second)
    if err := s.Set("ns1", "A", []byte("v")); err != nil { t.Fatal(err) }
    mt, ok := ModTime(s, "ns1", "A")
    if !ok || mt.Before(before) { t.Fatalf("ModTime = %v, %v", mt, ok) }
}

func TestBackendEncryptedWhenAgeKeyExists(t *testing.T) {
    home := t.TempDir()
    t.Setenv("HOME", home)
    t.Setenv("COFFER_USE_KEYCHAIN", "")
    dir, _ := StoreDir()
    if err := EnsureAgeKey(dir); err != nil { t.Fatal(err) }
    s, err := NewStore()
    if err != nil { t.Fatal(err) }
    if got := Backend(s); got != BackendFileEncrypted { t.Fatalf("Backend = %q", got) }
    _ = s.Set("ns1", "A", []byte("v"))
    if _, ok := ModTime(s, "ns1", "A"); !ok { t.Fatal("ModTime not found for encrypted secret") }
}

type fakeStore struct{ Store }
func TestBackendOtherForUnknownStore(t *testing.T) {
    if Backend(fakeStore{}) != BackendOther { t.Fatal("want other") }
    if _, ok := ModTime(fakeStore{}, "n", "x"); ok { t.Fatal("want false") }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/secret/ -run 'TestBackend' -v`
Expected: FAIL（`undefined: Backend`）

- [ ] **Step 3: Implement `info.go` 与各 store 的可选方法**（见 Interfaces）

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/secret/ -v && go build ./...`
Expected: PASS，编译通过

- [ ] **Step 5: Commit**

```bash
git add internal/secret/
git commit -m "feat(secret): report backend kind and secret mod time"
```

---

### Task 2: cli — 抽出共用保存函数

**Files:**
- Modify: `internal/cli/secret.go`（`runSecretAdd` 改用新函数，行为不变）
- Test: `internal/cli/secret_save_test.go`

**Interfaces:**
- Produces（`internal/cli/secret.go`）：
  ```go
  // 写 store；若 name 不在该 ns 的 config 中则新增 "" 条目并保存 config。
  // config 保存失败不算错误，返回非空 warning。store 写入失败返回 err。
  func saveSecretValue(store secret.Store, cfg *config.Config, cfgPath, ns, name string, value []byte) (warning string, err error)
  ```

- [ ] **Step 1: Write the failing tests**

```go
type memStore struct{ m map[string][]byte; failSet bool }
// Set/Get/Delete/List 基于 m，key = ns+"/"+name；failSet 时 Set 返回 error

func TestSaveSecretValueAddsConfigEntryForNewName(t *testing.T) {
    dir := t.TempDir(); cfgPath := filepath.Join(dir, ".coffer")
    cfg := &config.Config{Secrets: map[string]string{}, Namespaces: map[string]*config.NamespaceConfig{}}
    st := &memStore{m: map[string][]byte{}}
    warn, err := saveSecretValue(st, cfg, cfgPath, "dev", "A", []byte("v"))
    // err == nil, warn == "", st.m["dev/A"] == "v"
    // config.Load(cfgPath) 后 GetSecretsForNamespace("dev") 含 "A"
}

func TestSaveSecretValueKeepsExistingConfigValue(t *testing.T) {
    // cfg.Namespaces["dev"].Secrets["A"] = "{{coffer:A}}"；保存后 reload，值仍为 "{{coffer:A}}"
}

func TestSaveSecretValueStoreFailure(t *testing.T) {
    // failSet=true → err != nil，cfgPath 文件不存在（未写 config）
}

func TestSaveSecretValueConfigFailureIsWarning(t *testing.T) {
    // cfgPath 指向不存在的目录下的文件 → err == nil，warn != ""，st 中已有值
}
```

- [ ] **Step 2: Run to verify fail** — `go test ./internal/cli/ -run TestSaveSecretValue -v`，Expected: FAIL（undefined）

- [ ] **Step 3: Implement `saveSecretValue`，并让 `runSecretAdd` 调用它**（警告输出保持原文案 `Warning: secret saved to keychain but failed to update config: ...` 改为 `Warning: secret saved but failed to update config: ...`）

- [ ] **Step 4: Run** — `go test ./internal/cli/ -v`，Expected: PASS（含原有测试）

- [ ] **Step 5: Commit**

```bash
git add internal/cli/
git commit -m "refactor(cli): extract saveSecretValue shared by add and ui"
```

---

### Task 3: secretui 包 — 一次性本机服务

**Files:**
- Create: `internal/secretui/server.go`
- Create: `internal/secretui/assets/index.html`（本任务只需最小骨架：引用 `app.css?t={{.Token}}`、`app.js?t={{.Token}}`，一个 `<div id="app">`；完整 UI 在 Task 4）
- Create: `internal/secretui/assets/app.js`、`internal/secretui/assets/app.css`（本任务可为空文件）
- Modify: `.gitignore`（追加 `!internal/secretui/assets/*.html`）
- Test: `internal/secretui/server_test.go`

**Interfaces:**
- Produces:
  ```go
  type RequiredSecret struct{ Name, Description string }

  type EntryStatus string
  const ( EntrySet EntryStatus = "set"; EntryMissing EntryStatus = "missing" ) // missing = config 有、store 缺

  type Entry struct {
      Name      string      `json:"name"`
      Status    EntryStatus `json:"status"`
      UpdatedAt *time.Time  `json:"updated_at,omitempty"`
  }

  type Request struct {
      Namespace   string
      Scope       string // "global" | "local"
      StorageNote string // 原样显示在页面
      Required    []RequiredSecret
      Existing    []Entry // 当前 ns 全部已知名字，按名字排序
      Save        func(name string, value []byte) (warning string, err error)
      Timeout     time.Duration
      OpenBrowser func(url string) error // nil = 不打开
      OnReady     func(url string)       // 服务就绪后、打开浏览器前调用
      Log         io.Writer              // 打开浏览器失败等提示；nil = io.Discard
  }

  type Result struct {
      Name    string `json:"name"`
      Saved   bool   `json:"saved"`
      Warning string `json:"warning,omitempty"`
      Error   string `json:"error,omitempty"`
  }

  type Status string
  const ( StatusSaved Status = "saved"; StatusCancelled Status = "cancelled"; StatusTimeout Status = "timeout" )

  type Outcome struct {
      Status  Status
      Results []Result // 每个提交过的名字的最终结果，按名字排序
  }

  // 仅启动失败（监听失败、token 生成失败）返回 error。ctx 取消 → StatusCancelled。
  func Run(ctx context.Context, req Request) (Outcome, error)
  ```

- HTTP 行为（全部在 `server.go`）：
  - 所有请求：`Host` 必须等于 `127.0.0.1:<port>`，查询参数 `t` 必须与 token 相等（`subtle.ConstantTimeCompare`），否则 403。
  - 所有响应头：`Content-Security-Policy: default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self' data:; base-uri 'none'; form-action 'none'; frame-ancestors 'none'`，`Cache-Control: no-store`，`Referrer-Policy: no-referrer`，`X-Frame-Options: DENY`，`X-Content-Type-Options: nosniff`。
  - `GET /` → 用 `html/template` 渲染 `index.html`（注入 `.Token`）；`GET /app.js`、`GET /app.css` → 内嵌文件。
  - `GET /api/state` → `{"namespace","scope","storage_note","required":[{"name","description","exists","updated_at"}],"existing":[Entry],"pending":[name...]}`；`exists/updated_at` 由 `Existing` 中同名项推出；`pending` = 尚未保存成功的 required 名字。
  - `POST /api/save`、`POST /api/cancel`：额外要求 `Origin == "http://127.0.0.1:<port>"`，否则 403；非 POST → 405。
  - `/api/save` 请求体 `{"secrets":[{"name","value"}]}`，`http.MaxBytesReader` 限 64 KiB（超出 413）。先整体校验，任一不通过返回 400 且不写入：名字 ∈ (Required ∪ Existing)、无重复、值去掉结尾 `\r\n`/`\n` 后非空、`pending` 中的名字全部在本次提交里。通过后逐个调用 `Save`（值为去掉结尾换行后的字节），响应 `{"results":[Result],"done":bool}`；`done` = 本次全部成功且 `pending` 为空 → Run 以 `StatusSaved` 结束。
  - `/api/cancel` → 响应 `{"done":true}`，Run 以 `StatusCancelled` 结束。
  - 结束时 `http.Server.Shutdown`（2 秒宽限）以保证最后一个响应送达；超时 → `StatusTimeout`。
  - `OpenBrowser` 返回错误：向 `Log` 写一行 `Could not open browser: <err>` 后继续等待。

- [ ] **Step 1: Write the failing tests**

测试辅助：`startRun(t, req) (base string, token string, done <-chan Outcome)` —— 在 goroutine 里 `Run`，经 `OnReady` 拿到 URL，`t.Cleanup` 里用 cancel 结束；`post(t, base, path, token, origin, body)`。`Save` 使用记录调用的 fake（可按名字配置失败一次）。

```go
func TestRejectsWrongToken(t *testing.T)            // GET /api/state?t=bad → 403
func TestRejectsWrongHost(t *testing.T)             // 请求带 Host: localhost:<port> → 403
func TestSaveRejectsMissingOrForeignOrigin(t *testing.T) // 无 Origin / Origin http://evil.example → 403，Save 未被调用
func TestSecurityHeadersPresent(t *testing.T)       // GET / 的响应含上述 CSP、no-store、DENY
func TestStateNeverContainsValues(t *testing.T)     // 保存 "s3cr3t-value" 部分失败后再 GET /api/state，响应体不含该字符串；/api/save 响应也不含
func TestStateIsRepeatableAndReflectsProgress(t *testing.T) // Required A,B；B 第一次 Save 失败 → state 两次都 200，pending == ["B"]，Run 未结束
func TestSaveRejectsNameNotAllowed(t *testing.T)    // 提交 Z（不在 Required/Existing）→ 400，Save 未被调用
func TestSaveRequiresAllPending(t *testing.T)       // Required A,B；只提交 A → 400，Save 未被调用
func TestSaveTrimsTrailingNewlineOnly(t *testing.T) // 值 " ab c \r\n" → Save 收到 " ab c "
func TestSaveRejectsOversizedBody(t *testing.T)     // 70 KiB 请求体 → 413，Save 未被调用
func TestSaveAllSuccessFinishesSaved(t *testing.T)  // Required A + Existing X 一起提交 → done:true，Outcome{StatusSaved, Results: A,X saved}
func TestPartialFailureAllowsRetry(t *testing.T)    // B 首次失败 → done:false；只提交 B 重试成功 → StatusSaved，Results 中 A、B 均 Saved
func TestWarningIsReported(t *testing.T)            // Save 返回 warning "cfg" → Result.Warning == "cfg"
func TestCancel(t *testing.T)                       // POST /api/cancel → Outcome.Status == StatusCancelled
func TestTimeout(t *testing.T)                      // Timeout 200ms，不操作 → StatusTimeout，部分失败的 Results 仍保留
func TestOpenBrowserFailureKeepsWaiting(t *testing.T) // OpenBrowser 返回 error → Log 含 "Could not open browser"，之后 save 仍可完成
```

- [ ] **Step 2: Run to verify fail** — `go test ./internal/secretui/ -v`，Expected: FAIL（package/undefined）

- [ ] **Step 3: Implement `server.go` 与骨架资源，更新 `.gitignore`**

- [ ] **Step 4: Run** — `go test ./internal/secretui/ -v -race`，Expected: PASS；`git status` 能看到 `internal/secretui/assets/index.html`（未被忽略）

- [ ] **Step 5: Commit**

```bash
git add .gitignore internal/secretui/
git commit -m "feat(secretui): one-shot localhost server for browser secret input"
```

---

### Task 4: secretui 页面

**Files:**
- Modify: `internal/secretui/assets/index.html`、`app.js`、`app.css`

**Interfaces:**
- Consumes: Task 3 的 `/api/state`、`/api/save`、`/api/cancel`（token 从 `location.search` 的 `t` 取，fetch 时附加 `?t=`）。

页面要求（中文文案；不使用 inline script/style，满足 CSP）：
- 头部：命名空间 + scope；`storage_note` 原样显示。
- 上半“待提交区域”：每个 required 密钥一张卡片 —— 名字、description、密码输入框 + 显示/隐藏切换；`exists` 为真时提示“将覆盖 YYYY-MM-DD HH:mm 设置的值”（无时间则“将覆盖现有值”）。
- 下半列表：`existing` 每行 名字 / 状态（`set` → “✅ 已设置”，`missing` → “⚠️ 配置中登记但存储缺值”）/ 更新时间（无则 “—”）/ 按钮（`set` → “更新”，`missing` → “补设”）。点击后该名字加入上方区域（带覆盖提示，补设除外），可“移除”；已在上方的名字按钮禁用。
- 一个“保存”、一个“取消”。保存时 required 未填写则前端提示并阻止提交。
- 保存响应：`done` → 整页替换为“✅ 已保存：A、B，可以关闭此页”（含 warning 时一并显示）；否则在对应卡片标出失败原因，成功项变为已保存只读，用户可只重试失败项。
- 取消 → 整页替换为“已取消，可以关闭此页”。
- 任一请求网络失败 → 提示“coffer 命令已结束或已超时，请回到聊天”。
- 适配 `prefers-color-scheme: dark`；窄屏可用。

- [ ] **Step 1: 实现页面**

- [ ] **Step 2: 手动验证**（用一个临时 `main` 或 Task 5 完成后的命令；在 Task 6 统一做端到端验证也可）
Run: `go test ./internal/secretui/ -v && go vet ./...`
Expected: PASS；浏览器 DevTools Console 无 CSP 违规报错。

- [ ] **Step 3: Commit**

```bash
git add internal/secretui/assets/
git commit -m "feat(secretui): settings page with existing secrets list"
```

---

### Task 5: CLI 接入 `secret add --ui`

**Files:**
- Create: `internal/cli/secret_ui.go`
- Modify: `internal/cli/secret.go`（`runSecretAdd` 先调用 `parseSecretAddArgs`；`UI` 为真走 `runSecretAddUI`；usage 文案加入 `--ui`）
- Modify: `internal/cli/cli.go`（`printUsage` 中 `secret add` 行补充 `[--ui]`）
- Test: `internal/cli/secret_ui_test.go`

**Interfaces:**
- Consumes: `saveSecretValue`（Task 2）、`secret.Backend` / `secret.ModTime`（Task 1）、`secretui.Run` 及其类型（Task 3）。
- Produces（`internal/cli/secret_ui.go`）：
  ```go
  type secretAddArgs struct {
      Names   []string          // 去重、保持顺序
      UI      bool
      Descs   map[string]string
      Timeout time.Duration     // 默认 5 * time.Minute
  }
  func parseSecretAddArgs(args []string) (secretAddArgs, error)
  func buildEntries(cfgNames, storeNames []string, modTime func(name string) (time.Time, bool)) []secretui.Entry
  func storageNote(kind secret.BackendKind) string
  func runSecretAddUI(a secretAddArgs, stdout, stderr io.Writer, opts *Options) int
  ```

参数规则：
- `--ui`；`--desc=NAME=text` 与 `--desc NAME=text` 均可、可多次；`--timeout=5m` / `--timeout 5m`（`time.ParseDuration`，≤0 报错）。
- 名字逐个过 `isValidSecretName`；不带 `--ui` 时名字数必须为 1（报错 `multiple names require --ui`）；`--desc` 的 NAME 必须在 Names 中；`--desc`/`--timeout` 不带 `--ui` 时报错。

`buildEntries`：名字集合 = cfgNames ∪ storeNames，排序；在 storeNames 中 → `EntrySet`（`UpdatedAt` 来自 `modTime`，取不到为 nil），否则 → `EntryMissing`。

`storageNote` 文案（逐字）：
- `BackendFileEncrypted` → `🔒 只在本机传输，加密写入 ~/.coffer，agent 看不到内容`
- `BackendFilePlain` → `🔒 只在本机传输，以仅本人可读的文件写入 ~/.coffer（未加密），agent 看不到内容`
- `BackendOSKeychain` → `🔒 只在本机传输，写入 macOS 钥匙串，agent 看不到内容`
- `BackendOther` → `🔒 只在本机传输，写入系统凭据存储，agent 看不到内容`

`runSecretAddUI` 流程：按 `configPath(opts)` 加载 config（错误文案与 `runSecretAdd` 相同）→ `ns` → `secret.NewStore()` → `store.List(ns)`（失败则当作空并在 stderr 警告）→ 组装 `secretui.Request`（`Save` 闭包调用 `saveSecretValue`；`OpenBrowser` 在 darwin 用 `exec.Command("open", url).Run()`，其他平台 nil；`Log` = stderr；ctx 用 `signal.NotifyContext(ctx, os.Interrupt)`）。
- `OnReady` 向 stderr 输出：
  ```
  Opening browser to set secrets. If it does not open, visit:
    <url>
  Waiting up to <timeout>...
  ```
- 文本输出（stdout）：每个 Result 一行 `✓ Secret 'X' saved to <scope> namespace '<ns>'`；失败 `✗ Secret 'X' failed: <error>`。warning 写到 stderr：`Warning: X: <warning>`（`--json` 模式下 warning 只在 JSON 的 Result 中）。
- 结束状态（stderr）：cancelled → `Error: cancelled by user`；timeout → `Error: timed out after <timeout>`（`time.Duration.String()`，如 `5m0s`）。
- `--json`（`opts.JSON`）：stdout 输出 `{"status","namespace","scope","results":[Result]}`，不再输出文本行。
- 退出码：`StatusSaved` → 0，其余 → 1。

- [ ] **Step 1: Write the failing tests**

```go
func TestParseSecretAddArgsDefaults(t *testing.T)   // ["A","--ui"] → Names [A], UI, Timeout 5m
func TestParseSecretAddArgsDedupes(t *testing.T)    // ["A","B","A","--ui"] → [A B]
func TestParseSecretAddArgsDesc(t *testing.T)       // "--desc=A=用于 x=y" 与 "--desc","B=z" → Descs{A:"用于 x=y",B:"z"}
func TestParseSecretAddArgsErrors(t *testing.T)     // 表驱动：["A","B"]、["A","--desc=A=x"]、["A","--ui","--desc=C=x"]、["A","--ui","--timeout=0s"]、["bad name","--ui"] 均返回 error
func TestBuildEntries(t *testing.T)                 // cfg [B,C]，store [A,B]，A 有时间 → [A set 有时间, B set 无时间, C missing]
func TestStorageNoteCopy(t *testing.T)              // 4 种 kind 文案逐字比对；BackendFilePlain 文案含“未加密”
```

- [ ] **Step 2: Run to verify fail** — `go test ./internal/cli/ -run 'TestParseSecretAddArgs|TestBuildEntries|TestStorageNote' -v`，Expected: FAIL

- [ ] **Step 3: Implement `secret_ui.go` 与 `runSecretAdd` 分支、usage 文案**

- [ ] **Step 4: Run** — `go test ./... && go vet ./...`，Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/
git commit -m "feat(cli): secret add --ui opens browser page to set secrets"
```

---

### Task 6: Skill、README 与端到端验证

**Files:**
- Modify: `internal/skill/SKILL.md`
- Modify: `SKILL.md`（与 `internal/skill/SKILL.md` 完全一致）
- Modify: `README.md`（命令表与一节“通过浏览器设置密钥”）

Skill 必须包含（Agent Rules 第 3 条改写 + 新小节）：
- 缺密钥或用户要更新密钥时：先用一句话告诉用户“浏览器会弹出设置页”，再执行
  `coffer secret add <NAME> [<NAME2>] --ui --desc "<NAME>=<用途>" -g -n <ns>`。
- 该命令阻塞直到用户提交，最长 `--timeout`（默认 5m）。在 Claude Code 中调用 Bash 时把 `timeout` 设为 `360000`（大于 5 分钟）。
- 退出码 0 才算成功；把输出中每个名字的结果如实告诉用户（包括用户顺手更新的其他密钥）。
- 超时/取消：告诉用户没有设置成功并询问下一步；不要自动重试；绝不让用户把值贴到聊天里。
- 不要读取或打印命令输出之外的任何密钥值；不要用 `secret get`。

- [ ] **Step 1: 修改两份 SKILL.md 与 README**

- [ ] **Step 2: 验证两份一致** — `diff SKILL.md internal/skill/SKILL.md`，Expected: 无输出

- [ ] **Step 3: 构建** — `go build -o /tmp/coffer-ui-test ./cmd/coffer`，Expected: 成功（不要用 `make build`/`make install`，避免改动用户已安装的 coffer）

- [ ] **Step 4: 端到端手动验证（需要用户配合在浏览器操作）**
用临时 HOME 隔离，避免碰真实 `~/.coffer`：
```bash
export HOME=$(mktemp -d) && /tmp/coffer-ui-test init --global
/tmp/coffer-ui-test secret add OLD_ONE -g -n e2e <<< "old"
/tmp/coffer-ui-test secret add NEW_ONE --ui --desc "NEW_ONE=端到端测试" -g -n e2e
```
Expected：浏览器自动打开；页面显示 namespace `e2e (global)`、加密存储文案、NEW_ONE 卡片、列表中 OLD_ONE ✅ 带时间；点 OLD_ONE“更新”后两者一起保存 → 页面显示已保存；终端输出两行 `✓`、退出码 0；终端与页面响应中无值。再分别验证“取消”（退出码 1，`Error: cancelled by user`）和 `--timeout=20s` 不操作（`Error: timed out after 20s`）。验证后删除临时 HOME 目录与 `/tmp/coffer-ui-test`。

- [ ] **Step 5: Commit**

```bash
git add SKILL.md internal/skill/SKILL.md README.md
git commit -m "docs: teach agents to use secret add --ui"
```
