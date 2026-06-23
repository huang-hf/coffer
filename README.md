# Coffer

Secure secret management for AI agents and development workflows.

Coffer stores secrets in your OS native credential store (macOS Keychain, Linux GNOME Keyring, Windows Credential Manager) and injects them into commands via environment variables, file templates, or local database proxies. Secrets never live in plain-text files.

---

## Installation (关键)

Coffer 是一个 Go 项目，安装就是把编译好的二进制文件放到服务器上。以下是从零开始的完整路径。

### 前置条件

| 环境 | 依赖 |
|------|------|
| **macOS** | `security`（系统自带，无需额外安装） |
| **Linux** | `secret-tool`（`libsecret` 包提供）+ 正在运行的 keyring 守护进程 |
| **Windows** | `cmdkey`（系统自带） |

#### Linux 依赖安装

```bash
# Debian / Ubuntu
sudo apt-get update && sudo apt-get install -y libsecret-tools gnome-keyring

# RHEL / CentOS / Fedora
sudo yum install -y libsecret-tools gnome-keyring
```

Linux 服务器通常是 headless（无桌面环境），需要额外配置 D-Bus 和一个 keyring 守护进程才能使用 `secret-tool`：

```bash
# 安装后,通过 dbus-launch 启动一个 session bus
export $(dbus-launch)

# 解锁 gnome-keyring（需要设置一个密码，随便输入即可）
echo -n 'any-password' | gnome-keyring-daemon --unlock --daemonize --components=secrets

# 验证 secret-tool 可用
echo 'test' | secret-tool store --label=test service coffer name coffer.test.test
secret-tool lookup service coffer name coffer.test.test
secret-tool clear service coffer name coffer.test.test
```

> 建议把 `dbus-launch` 和 `gnome-keyring-daemon` 的启动命令加到 `~/.bashrc` 或 `~/.zshrc` 中，确保每次登录时自动可用。

### 方案 A：从开发机交叉编译并部署到服务器（推荐）

这是最常见的场景——在本地 Mac 上编译出 Linux 二进制，然后 scp 到服务器。

```bash
# 1. 在本地克隆并编译（指定 Linux 架构）
git clone <repo-url>
cd coffer

GOOS=linux GOARCH=amd64 go build -o coffer-linux ./cmd/coffer

# 2. 把二进制传到服务器
scp coffer-linux user@your-server:/tmp/coffer

# 3. 在服务器上安装到系统 PATH
ssh user@your-server
sudo mv /tmp/coffer /usr/local/bin/coffer
sudo chmod +x /usr/local/bin/coffer

# 4. 验证
coffer --version
```

> 如果你的服务器是 ARM 架构（如 AWS Graviton、树莓派），把 `GOARCH=amd64` 改为 `GOARCH=arm64`。

### 方案 B：直接在服务器上用 Go 编译

如果服务器上已经安装了 Go（版本 ≥ 1.25）：

```bash
git clone <repo-url>
cd coffer
go build -o coffer ./cmd/coffer
sudo mv coffer /usr/local/bin/
```

### 方案 C：`go install`

如果你有实际的 git 仓库地址（module 路径），可以用：

```bash
# 前提：Go 版本 ≥ 1.25
go install example.com/coffer/cmd/coffer@latest
```

安装后 `coffer` 在 `$GOPATH/bin` 下，确保该目录在 `PATH` 中。

### 验证安装

```bash
coffer --version
coffer --help
```

看到帮助信息即安装成功。

---

## 快速开始

### 1. 初始化配置

```bash
# 全局配置（推荐服务器使用）—— 存在 ~/.config/coffer/config.yaml
coffer init --global

# 或项目本地配置 —— 在当前目录生成 .coffer 文件
coffer init
```

**建议服务器场景使用 `--global`**，这样不管在哪个目录都能访问同一组密钥。

### 2. 添加密钥

```bash
# 交互式输入（推荐）
coffer secret add db-password --global --ns=production

# 或通过 stdin 非交互式输入（适合自动化脚本）
printf '%s' 'my-secret-value' | coffer secret add API_KEY --global --ns=dev
```

密钥名称保持**精确**——coffer 不会帮你转大写或替换字符。环境变量名就是密钥名本身：

```bash
coffer secret add AWS_ACCESS_KEY_ID --global --ns=aws
coffer secret add AWS_SECRET_ACCESS_KEY --global --ns=aws
```

### 3. 查看密钥状态

```bash
coffer check --global --ns=production --json
```

输出示例：
```json
{"ready": true, "ns": "production", "secrets": [
  {"name": "db-password", "configured": true}
]}
```

### 4. 运行命令（注入密钥）

```bash
# 在 child 进程的环境变量里注入密钥
coffer run --global --ns=production python app.py

# 或者用模板注入模式
coffer run --global --ns=production --inject=file python app.py
```

### 5. 完整示例

```bash
# 从头开始
coffer init --global
coffer secret add DB_PASSWORD --global --ns=prod
coffer secret add API_KEY --global --ns=prod

# 查看状态
coffer check --global --ns=prod --json

# 运行应用
coffer run --global --ns=prod python -c 'import os; print(os.environ.get("DB_PASSWORD"))'
```

---

## 命令参考

| 命令 | 说明 |
|------|------|
| `coffer init [--global]` | 初始化配置（本地或全局） |
| `coffer secret add <name>` | 添加密钥（交互式或 stdin） |
| `coffer secret update <name>` | 更新已有密钥 |
| `coffer secret list` | 列出当前命名空间的所有密钥 |
| `coffer secret get <name>` | 显示密钥值（仅交互式终端） |
| `coffer secret delete <name>` | 删除密钥 |
| `coffer check [--json]` | 检查密钥就绪状态 |
| `coffer run <command>` | 注入密钥并执行命令 |
| `coffer inject -i <tmpl> -o <out>` | 渲染 `{{coffer:name}}` 模板文件 |
| `coffer db add <name>` | 注册 PostgreSQL 数据库连接 |
| `coffer db proxy <name>` | 启动本地数据库代理 |
| `coffer migrate <env-file>` | 迁移 `.env` 到 keychain |
| `coffer status` | 显示当前配置状态 |

### 全局选项

```
--ns=<namespace>    指定命名空间（优先级：CLI > COFFER_NS > default_ns）
--global            操作全局配置（~/.config/coffer/）
--json              JSON 格式输出（适合 Agent 解析）
--inject=env|file   注入模式（env=环境变量，file=临时文件）
--config=<path>     指定配置文件路径
```

### 命名空间

通过命名空间隔离多环境密钥：

```bash
coffer secret add DB_PASSWORD --ns=staging
coffer secret add DB_PASSWORD --ns=production
coffer run --ns=production python app.py
```

优先级：`--ns` 参数 > 环境变量 `COFFER_NS` > 配置文件的 `default_ns`。

### 模板注入（`coffer inject`）

对于需要配置文件的工具（非环境变量注入），用模板方式：

```bash
# 模板文件 config.tmpl
# ---
# database:
#   password: "{{coffer:DB_PASSWORD}}"
# api_key: "{{coffer:API_KEY}}"
# ---

coffer inject -i config.tmpl -o config.yaml --global --ns=prod
```

### PostgreSQL 数据库代理

```bash
# 注册连接信息（密码存在 keychain 中）
coffer db add my-pg \
  --host db.example.com --port 5432 \
  --user admin --database app --global

# 启动本地代理（监听 127.0.0.1:<port>，自动使用 keychain 中的密码认证）
coffer db proxy my-pg --global
```

客户端连接 `127.0.0.1:<port>` 即可，无需知道数据库密码。

### 迁移 .env

```bash
coffer migrate .env --global --ns=prod --dry-run   # 预览
coffer migrate .env --global --ns=prod              # 执行
```

- 敏感值 → keychain
- `.env` → 自动生成 `{{coffer:name}}` 模板，去掉明文密钥

---

## Agent 使用说明

Coffer 专为 AI Agent 设计，详见 [`SKILL.md`](./SKILL.md)。

核心原则：
- Agent 用 `coffer check --json` 检查就绪状态，不主动读取密钥值
- 缺失密钥时，引导用户在本地终端上执行 `coffer secret add <name>`，不应让用户把密钥粘贴到对话中
- 推荐 `coffer run` 注入密钥，避免密钥暴露在进程列表中

---

## 配置参考

### 全局配置

`~/.config/coffer/config.yaml`：

```yaml
default_ns: default
inject: env
secrets:
  DB_PASSWORD: "{{coffer:DB_PASSWORD}}"
namespaces:
  production:
    secrets:
      DB_PASSWORD: "{{coffer:DB_PASSWORD}}"
      API_KEY: "{{coffer:API_KEY}}"
  staging:
    secrets:
      DB_PASSWORD: "{{coffer:DB_PASSWORD}}"
```

### 本地配置

`.coffer` 文件格式与全局相同。

### 合并规则

- 全局配置作为基础（base）
- 本地配置覆盖/追加到全局
- 实际生效的是合并后的结果
- `--global` 标志只操作全局配置

---

## 平台支持

| 平台 | 后端 | 依赖安装 |
|------|------|----------|
| macOS | Keychain (`security`) | 系统自带 |
| Linux | GNOME Keyring (`secret-tool`) | `apt install libsecret-tools gnome-keyring` |
| Windows | Credential Manager (`cmdkey`) | 系统自带 |
