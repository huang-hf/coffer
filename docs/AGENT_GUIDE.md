# Agent 使用指南

## 概述

Safetool 是一个安全的密钥管理工具，专为 AI Agent 工作流设计。它允许 Agent 在不接触明文密钥的情况下，安全地将密钥注入到命令中。

## 核心概念

### 1. 密钥存储
- 密钥存储在操作系统原生的密钥链中（macOS Keychain、Linux GNOME Keyring、Windows Credential Manager）
- 密钥不会存储在文件系统中

### 2. 命名空间
- 命名空间用于隔离不同环境的密钥（如 staging、production）
- 支持多环境配置

### 3. 注入模式
- **env 模式**：将密钥作为环境变量注入（默认）
- **file 模式**：将密钥写入临时文件

## Agent 工作流

### 步骤 1：检查项目状态

```bash
# 检查所有密钥是否就绪
coffer check

# JSON 格式输出（推荐 Agent 使用）
coffer check --json
```

JSON 响应示例：
```json
{
  "ready": false,
  "ns": "production",
  "secrets": [
    {"name": "db-pwd", "configured": true, "fix": ""},
    {"name": "api-key", "configured": false, "fix": "coffer secret add api-key --ns=production"}
  ]
}
```

### 步骤 2：添加缺失的密钥

如果 `check` 返回 `ready: false`，需要添加缺失的密钥：

```bash
# 添加密钥（会提示输入值）
coffer secret add api-key --ns=production
```

**注意**：Agent 无法直接设置密钥值，需要用户交互输入。

### 步骤 3：运行命令

```bash
# 运行命令，密钥会自动注入
coffer run python app.py

# 使用文件注入模式
coffer run --inject=file python app.py

# 指定命名空间
coffer run --ns=staging python app.py
```

## 完整示例

### 场景：Agent 需要运行一个 Python 应用

1. **检查状态**
```bash
$ coffer check --json
{
  "ready": false,
  "ns": "default",
  "secrets": [
    {"name": "db-pwd", "configured": false, "fix": "coffer secret add db-pwd"}
  ]
}
```

2. **提示用户添加密钥**
```
我发现项目缺少以下密钥：
- db-pwd

请运行以下命令添加密钥：
coffer secret add db-pwd
```

3. **用户添加密钥后**
```bash
$ coffer secret add db-pwd
Enter value for db-pwd: [用户输入]
✓ Secret 'db-pwd' saved to namespace 'default'
```

4. **再次检查状态**
```bash
$ coffer check --json
{
  "ready": true,
  "ns": "default",
  "secrets": [
    {"name": "db-pwd", "configured": true}
  ]
}
```

5. **运行应用**
```bash
$ coffer run python app.py
```

## 命令参考

### `coffer check [--ns=<namespace>] [--json]`

检查密钥状态。

- `--ns=<namespace>`: 指定命名空间
- `--json`: JSON 格式输出

### `coffer secret add <name> [--ns=<namespace>]`

添加密钥。

- `<name>`: 密钥名称（只能包含字母、数字、连字符、下划线）
- `--ns=<namespace>`: 指定命名空间

### `coffer secret list [--ns=<namespace>]`

列出密钥。

### `coffer secret delete <name> [--ns=<namespace>]`

删除密钥。

### `coffer run [--ns=<namespace>] [--inject=<mode>] <command> [args...]`

运行命令并注入密钥。

- `--ns=<namespace>`: 指定命名空间
- `--inject=<mode>`: 注入模式（env 或 file）

### `coffer status`

显示当前状态。

## 安全注意事项

1. **Agent 模式限制**
   - `coffer secret get` 在 `--json` 模式下被禁止
   - 防止 Agent 获取明文密钥

2. **调用方验证**
   - 环境变量标记：`COFFER_CALLER=1`
   - 进程树验证

3. **密钥隔离**
   - 不同命名空间的密钥完全隔离
   - 密钥不会存储在文件系统中

## 故障排除

### 错误："not initialized"
运行 `coffer init` 初始化项目。

### 错误："secret not found"
检查密钥是否存在：`coffer secret list`

### 错误："secret get is not allowed in JSON mode"
这是预期行为，Agent 无法获取明文密钥。

### 错误："invalid secret name"
密钥名称只能包含字母、数字、连字符、下划线。

## 最佳实践

1. **始终使用 `--json` 模式**
   - Agent 应该使用 `coffer check --json` 而不是 `coffer check`

2. **检查后再操作**
   - 在运行命令前，先用 `coffer check` 确认密钥状态

3. **使用命名空间**
   - 为不同环境使用不同的命名空间

4. **不要尝试获取明文密钥**
   - Agent 不应该使用 `coffer secret get`
   - 密钥应该通过 `coffer run` 注入

## 示例交互

```
Agent: 我需要运行 Python 应用，先检查密钥状态
$ coffer check --json
{
  "ready": false,
  "ns": "default",
  "secrets": [
    {"name": "db-pwd", "configured": false, "fix": "coffer secret add db-pwd"}
  ]
}

Agent: 项目缺少 db-pwd 密钥，请运行以下命令：
coffer secret add db-pwd

User: [运行命令并输入密钥值]

Agent: 密钥已添加，现在可以运行应用了
$ coffer run python app.py
```
