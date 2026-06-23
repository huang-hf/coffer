# Coffer 问题记录 — 来自 gradio-inference-platform 接入(2026-06-23, macOS darwin)

场景:把 inference 服务 `.env`(19 个密钥)+ AWS `[power]` profile 凭证(2 个)
导入 coffer **global / ns=`inference-dev`**,用 `coffer run --inject=env` 注入本地启动。
过程中发现以下问题,优先级从高到低。

---

## 1. `secret add` 管道喂值缺结尾换行时报 `EOF`(功能性 bug)

**现象**

```bash
printf '%s' "$val" | coffer secret add NAME --global --ns=inference-dev
# => Error reading password: EOF   （值未保存）
printf '%s\n' "$val" | coffer secret add NAME ...   # 加 \n 才成功
```

**根因** — `internal/cli/secret.go` `readPassword()`（约 340–357 行）

非 TTY 分支用 `reader.ReadString('\n')`，输入无结尾 `\n` 时返回 `(data, io.EOF)`，
调用方把任何 `err != nil` 当致命错误，丢弃已读到的 `data`。

**建议**

非 TTY 分支在 `err == io.EOF && len(value) > 0` 时接受已读字节（TrimRight 后），
不要求结尾换行。这样批量导入（从 .env 逐行管道喂值）更稳。

---

## 2. `secret list` 在 macOS 返回重复 + 混入无关系统钥匙串条目（严重，影响可信度）

**现象**

对 21 个真实 secret，`coffer secret list --global --ns=inference-dev` 输出 42 行：
- 多数 secret 出现两次；
- 混入 `Siri Global - AnalyticsIdentifiers.checkpoint`、裸 `AWS` 等**非 coffer 的系统钥匙串项**。

**根因（推测）** — `internal/secret/store_darwin.go` 的 `List(ns)`

对 Keychain 查询过宽 / 解析 `security` 输出有误：按空格切分把带空格/下划线的条目名截断
（`AWS_ACCESS_KEY_ID` 出现裸 `AWS`），并把不属于该 ns/service 的项也带出。

**影响**

无法用 `secret list` 核对导入结果或计数 —— 这是 agent 工作流的主要校验手段。
（本次改用读 `~/.config/coffer/config.yaml` 占位名才可靠核对。）

**建议**

List 严格按 coffer 自己的 service 前缀 + namespace 过滤；按条目名整体取值，不要空格分词。

---

## 3. `migrate` 硬编码本地 `.coffer` + 强制 file 注入 + 自带 `--namespace=`（设计/一致性）

**现象**

- 无法把 `.env` migrate 进 **global** 配置；
- migrate 后被强制改成 `inject: file`；
- 用 `--namespace=` 而非全局统一的 `--ns=`，也不认 `--global`。

**根因** — `internal/cli/migrate.go`

写死 `config.Load(".coffer")` / `config.Save(cfg, ".coffer")`（161/209 行），
并 `cfg.Inject = "file"`（206 行）；flag 解析（74–86 行）只认
`--template/--namespace/--dry-run/--force`，与其余命令的全局 flag 不一致。

**建议**

让 migrate 复用全局 `--global`/`--ns`/`--inject`，允许导入 global 且可选 env 注入；
否则在 help/文档里明确它只支持「本地 + file」。

---

## 5. `coffer run` (env 模式) 强制大写 secret 名,且不走 EnvInjector（一致性 bug）

**现象**

global/ns 下注册了 21 个 secret(含小写名如 `db_password`),`coffer run` 注入后
小写名按原名查不到 —— 实际被注入成 `DB_PASSWORD`。即 12 个小写命名的 secret
全被转成大写,9 个本就大写的不受影响。

**根因** — `internal/cli/run.go`

```go
envName := secretNameToEnvName(secretName)        // 注入时改名
env = append(env, envName+"="+string(value))

func secretNameToEnvName(name string) string {
    return strings.ToUpper(strings.ReplaceAll(name, "-", "_"))   // 强制大写
}
```

两个问题:
1. **改名**:与 `internal/inject/env.go` 的 `EnvInjector.Inject`(`os.Setenv(name, value)` 原样)
   矛盾 —— 文档/源码看 env 注入应保留原名,实际 `run` 自己内联拼 env 并大写,**没用 EnvInjector**。
2. **不一致**:`run` 的 env 路径完全绕过 `Injector` 接口,只有 file 路径用了 `RenderConfigFile`。

**影响**

- 依赖**精确小写**环境变量名的程序会读不到(本例靠 pydantic `case_sensitive=False` 才幸免)。
- 用户按自己设的 secret 名(小写)去核对/引用会对不上。

**建议**

env 模式直接用 `EnvInjector`(原样 `Setenv`),不要 `ToUpper`;
或把改名行为做成可选(flag/config),并在文档里写明。至少应与 `inject/env.go` 统一。

---

## 4. 无 per-command help（易用性）

**现象**

`coffer run --help`、`coffer secret --help` 不工作 —— `--help` 被当成位置参数
（`exec: "--help": ... not found` / `Unknown secret command: --help`）。只有顶层 `coffer --help`。

**建议**

各子命令支持 `-h/--help`，或在顶层 help 列出子命令的 flag。

---

## 备注:本次实际可用的导入姿势（绕过上述坑）

```bash
coffer init --global                      # 已存在则跳过
# 逐 key 从 .env 管道喂值（注意 printf 带 \n —— 见问题 1）
printf '%s\n' "$val" | coffer secret add <KEY> --global --ns=inference-dev
# 核对改读 config.yaml 占位名，不用 secret list —— 见问题 2
coffer run --global --ns=inference-dev <command>   # 默认 env 注入
```
