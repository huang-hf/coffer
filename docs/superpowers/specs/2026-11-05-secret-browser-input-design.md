# 设计：通过浏览器页面设置密钥（agent 聊天场景）

日期：2026-11-05
状态：待用户确认

## 1. 背景与动机

用户主要在和 AI agent 聊天时遇到“需要设置密钥”。agent 运行在用户自己的 Mac 上。
当前 skill 规则（`internal/skill/SKILL.md` Agent Rules 第 3 条）要求 agent 让用户自己去执行
`coffer secret add <name>`。实际体验是：复制命令 → 打开另一个 terminal → 粘贴执行 → 输入值 → 回到聊天告诉 agent。
步骤多、打断对话。

目标：agent 需要密钥时，直接在浏览器弹出一个本机页面，用户在页面里输入并提交，agent 自动继续。
页面同时展示当前 namespace 已有的密钥（只显示名字/状态/时间，不显示值），并允许顺手更新。

频率：大部分时候一次 1 个密钥，少数时候 2 个。

## 2. 目标

- G1：agent 需要密钥时，一条命令即可让浏览器自动弹出设置页，并阻塞等待用户提交。
- G2：用户提交后，命令退出并输出结果（不含值），agent 自动继续，用户无需回到终端说话。
- G3：一次可设置 1~N 个 agent 指定的密钥（主要场景 1 个，偶尔 2 个），同一页面完成。
- G4：页面展示命名空间、scope（global/local）、每个密钥名，以及 agent 提供的用途说明。
- G5：页面列出当前 namespace 下所有已有密钥：名字、状态、更新时间；**不展示值**。
- G6：列表中每一项可“更新”（状态异常的显示“补设”），点击后加入上方待提交区域，与 agent 指定的密钥一起保存。
- G7：密钥值只在本机浏览器 → coffer 进程之间单向传输，直接经由现有 `secret.Store.Set` 落盘；
  页面与服务端任何接口都不返回密钥值，agent 全程接触不到明文。
- G8：skill 更新，让 agent 在缺密钥时优先走这个流程。

## 3. 范围与非目标

- 目标环境：**Claude Code**，运行在用户本机 macOS。
- **Codex 本期不专门处理**（用户决定）：不为 Codex 的 `workspace-write` 沙箱（写 `~/.coffer`/全局配置、本机监听、`open`）
  或其工具超时做适配。skill 同时会装到 Codex，不需要为 Codex 写额外说明；如果在 Codex 中失败，只要报错清晰即可。
- 不展示任何密钥值（即使打码/点击显示也不做），不提供删除，不跨 namespace 浏览/操作。
- 不支持远程/SSH 场景。
- 不改变现有 `secret add` 的终端交互/stdin 行为。
- 不引入前端构建工具链或外部依赖。

## 4. 反目标（即使“能用”也算失败）

- 密钥值出现在：命令的 stdout/stderr、日志、URL、进程参数、任何 HTTP 响应、agent 上下文、任何非 store 的文件里。
- 页面依赖任何外部网络资源（CDN 字体/脚本等）或第三方托管（如 claude.ai artifact）。
- 其他网页能跨站向该服务写入或读取内容。
- 命令在用户没有操作时无限期挂住。
- 超时/取消后 agent 自作主张重试，或改为让用户把值贴到聊天里。
- 页面做出与实际不符的安全承诺（例如实际未加密却写“已加密”）。

## 5. 用户体验（已与用户确认）

交接方式：**自动弹出 + agent 阻塞等待**（用户在草图中选定）。

1. agent 在聊天里说明需要哪个密钥、哪个命名空间，然后执行命令；浏览器自动打开设置页；agent 等待。
2. 设置页上半部分 —— 待提交区域：
   - 命名空间 + scope（global / local）
   - agent 指定的每个密钥一个输入区：密钥名、用途说明（agent 传入，可选）、密码输入框（可切换显示/隐藏）
   - 已存在的密钥明确提示“将覆盖现有值”
3. 设置页下半部分 —— 当前 namespace 已有密钥列表：
   - 列：名字、状态（✅ 已设置 / ⚠️ 配置中登记但存储缺值）、更新时间
   - 每行有“更新”按钮（⚠️ 行显示“补设”），点击后该密钥加入上方待提交区域（带“将覆盖现有值”提示，补设除外），可移除
4. 页面只有一个“保存”和一个“取消”：
   - **一次保存就结束**（用户确认）：保存会一次提交上方所有已填写的输入；agent 指定的密钥必须全部填写才能保存。
     保存后命令退出、页面显示结果，不能继续编辑。
   - 成功：页面显示“✅ 已保存 …，可以关闭此页”；命令输出保存了哪些名字；agent 继续。
5. 超时（默认 5 分钟）或点“取消”：页面显示对应状态；命令以非零退出码退出并说明原因；
   agent 告诉用户没设成并询问下一步，**不自动重试**。

## 6. 已确定的实现方向

- 功能做在 coffer 二进制内（Go），沿用现有代码结构；不新增第三方依赖（标准库 `net/http`、`embed`）。
- 在 coffer 进程内临时启动 HTTP 服务，页面 HTML 用 `embed` 内嵌进二进制。
- 只监听 `127.0.0.1`，随机端口。
- 用 macOS `open <url>` 打开默认浏览器；其他平台只打印 URL。
- 写入走现有逻辑：`secret.NewStore()` → `store.Set(ns, name, value)`，并与 `secret add` 一样更新 config
  （`cfg.SetSecretForNamespace` + `config.Save`），global/local、`--ns` 解析与现有 `secret add` 一致。
  建议把 `runSecretAdd` 中“校验 → 加载 config → Set → 更新 config”抽成共用函数，避免两套逻辑漂移。
- 列表数据来源：当前 scope config 中该 namespace 的密钥名（与 `secret list` 一致）∪ store 中该 namespace 已有的名字；
  对每个名字判断 store 中是否有值，得出状态。
- 更新时间：file store 取对应文件（`.json.age` 或 `.json`）的修改时间；keychain 等后端取不到时显示“—”。
  页面上同时可以把这个时间用于“将覆盖现有值”提示（例如“将覆盖 2026-06-23 设置的值”）。

## 7. 安全要求

- 每次启动生成高熵一次性 token（如 32 字节随机，hex），放在打开的 URL 中；所有请求必须携带并校验，不匹配返回 403。
- 提交接口只接受 POST，校验 `Origin`/`Host` 为本服务自身（防 DNS rebinding 与跨站提交）。
- 响应头：`Content-Security-Policy`（禁止外部资源）、`Cache-Control: no-store`、`Referrer-Policy: no-referrer`、
  `X-Frame-Options: DENY`。
- **任何接口都不返回密钥值**；列表接口只返回名字、状态、时间。不记录请求体；错误信息不含值。
- 可写入的名字限定为：agent 本次指定的名字 ∪ 当前 namespace 列表中已有的名字。其他名字一律拒绝。
- 密钥名沿用 `isValidSecretName` 校验；空值拒绝。
- 一次成功保存后服务关闭；超时自动关闭。
- 威胁模型说明（作者判断）：URL（含 token）会打印到 stderr，因此 agent 能拿到 token、理论上能自行提交写入。
  这不构成新能力 —— agent 本就可以 `echo x | coffer secret add`。防护重点是“读不到值”，而接口不返回值保证了这一点。

## 8. 命令接口（草案，builder 可微调命名）

```bash
coffer secret add <NAME> [<NAME2> ...] --ui [--desc "<NAME>=<用途说明>"]... [--timeout=5m] -g -n <ns>
```

- `--ui`：走浏览器流程。不带时保持现有终端行为（只接受 1 个名字）；多名字仅在 `--ui` 下允许。
- `--desc`：可选，为某个密钥提供用途说明。
- 启动后先向 stderr 打印 URL（以防浏览器没弹出，用户可手动打开），再阻塞等待。
- 退出码：0 = 全部保存成功；非 0 = 取消 / 超时 / 有失败（stderr 写明原因，如 `cancelled by user`、`timed out after 5m`）。
- stdout 输出每个名字的结果（保存成功 / 失败及原因）与 namespace，**不输出值**。支持 `--json` 结构化输出。

## 9. 失败与边界处理（作者判断）

- 多个密钥逐个写入，**不回滚**已成功的。页面标出哪些成功、哪些失败。
  若有失败，服务不关闭、token 继续有效，页面允许在超时前只重试失败项；全部成功后才关闭并以 0 退出。
  超时时仍有失败项 → 非 0 退出，stdout 列出各名字结果。
- store 写入成功但 config 保存失败：与现有 `secret add` 一致视为保存成功并给出警告（页面与 stderr 都显示警告）。
- “将覆盖现有值”的判断依据是 **store 中该 namespace 下是否已有该名字的值**（因为被覆盖的是 store 里的值；
  store 不区分 global/local，`~/.coffer/<ns>/` 是共享的），而不只是当前 scope 的 config。
- 页面安全说明文案按实际后端显示：file store 且存在 age key → “加密写入 ~/.coffer”；
  file store 无 key → “以仅本人可读的文件写入 ~/.coffer（未加密）”；keychain 后端 → “写入 macOS 钥匙串”。

## 10. Skill 更新

- 修改 `internal/skill/SKILL.md` Agent Rules 第 3 条：缺密钥（或用户要更新密钥）时，agent 执行
  `coffer secret add <NAME> --ui ...`，执行前用一句话告诉用户“浏览器会弹出设置页”。
- 写明 agent 调用要点：
  - 命令会阻塞直到用户提交（最长 `--timeout`）。Claude Code 的 Bash 工具默认超时 2 分钟，
    调用时需把工具超时设得大于 `--timeout`（例如 `--timeout=5m` 配合工具超时 360000ms）。
  - 超时/取消时告知用户并询问，不自动重试，绝不让用户把值贴到聊天里。
  - 写好 `--desc` 用途说明。
  - 用户顺手更新的其他密钥会出现在命令输出中，agent 应如实告知用户。
- 同时更新 README 的命令说明。
- **注意**：仓库根目录 `SKILL.md` 比 `internal/skill/SKILL.md` 旧，而 `make build` 会执行 `sync-skill`
  把根目录版本覆盖到 `internal/`。实施时以 `internal/skill/SKILL.md` 为准，并把根目录 `SKILL.md` 同步成相同内容。

## 11. 实现注意事项

- `.gitignore` 里有 `*.html`：内嵌页面若用 `.html` 后缀会被 git 忽略。需要为该文件加例外（如 `!internal/**/*.html`）或换后缀。
- 页面纯 HTML + 少量内联 JS/CSS，适配浅色/深色模式；不依赖外部资源。
- 测试：至少覆盖 token 校验、Origin 校验、非允许名字被拒、任何响应不含值、成功写入 store 与 config、
  部分失败后重试、取消与超时路径（可通过注入假 store / 可配置超时 / 不真正调用 `open` 来测）。

## 12. 留给 builder 决定

- 具体 HTTP 路由、页面视觉细节、JS 写法。
- `--desc` 的确切语法（只要能为每个密钥给一段说明即可）。
- 共用函数的具体拆分方式与文件组织。
- `--json` 输出的字段结构。
