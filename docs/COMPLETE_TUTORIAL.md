# 从零开始使用 Coffer

## 前提条件

你需要先安装 Go 语言环境。

### 检查是否已安装 Go

```bash
go version
```

如果显示版本号（如 `go version go1.21.0 darwin/arm64`），说明已安装。

如果没有安装，请先安装 Go：

```bash
# macOS
brew install go

# 或者下载安装包
# https://golang.org/dl/
```

## 第一步：获取 Coffer 代码

```bash
# 克隆代码（或者你已经有代码）
git clone <repo-url>
cd coffer
```

## 第二步：构建 Coffer

```bash
# 构建可执行文件
go build -o coffer ./cmd/coffer
```

构建成功后，当前目录会出现一个 `coffer` 可执行文件。

## 第三步：安装到系统

```bash
# 创建用户本地目录（如果不存在）
mkdir -p ~/bin

# 复制可执行文件
cp coffer ~/bin/

# 添加执行权限
chmod +x ~/bin/coffer

# 添加到 PATH（永久生效）
echo 'export PATH="$HOME/bin:$PATH"' >> ~/.zshrc

# 使配置立即生效
source ~/.zshrc
```

## 第四步：验证安装

```bash
# 检查版本
coffer --version

# 查看帮助
coffer --help
```

如果显示版本号和帮助信息，说明安装成功。

## 第五步：在你的项目中使用

### 5.1 初始化项目

```bash
# 进入你的项目目录
cd /path/to/your/project

# 初始化 coffer
coffer init
```

这会在当前目录创建 `.coffer` 配置文件。

### 5.2 添加密钥

```bash
# 添加数据库密码（会提示输入）
coffer secret add db-password

# 添加 API 密钥
coffer secret add api-key

# 添加到特定环境
coffer secret add db-password --ns=staging
coffer secret add db-password --ns=production
```

### 5.3 查看密钥列表

```bash
coffer secret list
```

### 5.4 运行命令

```bash
# 运行 Python 应用（密钥会自动注入为环境变量）
coffer run python app.py

# 运行 Node.js 应用
coffer run node server.js

# 运行带参数的命令
coffer run python app.py --port 8080
```

## 完整示例

假设你有一个 Python 应用 `app.py`：

```python
import os

# 从环境变量获取密钥
db_password = os.environ.get('DB_PASSWORD')
api_key = os.environ.get('API_KEY')

print(f"Database password: {db_password}")
print(f"API key: {api_key}")
```

### 操作步骤：

```bash
# 1. 进入项目目录
cd /path/to/myapp

# 2. 初始化
coffer init

# 3. 添加密钥
coffer secret add db-password
# 输入: mysecretpassword

coffer secret add api-key
# 输入: myapikey123

# 4. 运行应用
coffer run python app.py
```

输出：
```
Database password: mysecretpassword
API key: myapikey123
```

## 使用命名空间

命名空间用于隔离不同环境的密钥。

```bash
# 开发环境
coffer secret add db-password --ns=development
# 输入: devpassword

# 生产环境
coffer secret add db-password --ns=production
# 输入: prodpassword

# 运行时指定环境
coffer run --ns=development python app.py
# 输出: devpassword

coffer run --ns=production python app.py
# 输出: prodpassword
```

## 检查密钥状态

```bash
# 检查所有密钥是否就绪
coffer check

# JSON 格式（适合脚本）
coffer check --json
```

输出示例：
```json
{
  "ready": true,
  "ns": "default",
  "secrets": [
    {"name": "db-password", "configured": true},
    {"name": "api-key", "configured": true}
  ]
}
```

## 常见问题

### Q: 命令找不到？

```bash
# 检查 PATH
echo $PATH

# 确保 ~/bin 在 PATH 中
export PATH="$HOME/bin:$PATH"

# 或者重新启动终端
```

### Q: 权限错误？

```bash
# 确保有执行权限
chmod +x ~/bin/coffer
```

### Q: 密钥添加后看不到？

```bash
# 查看当前命名空间的密钥
coffer secret list

# 查看特定命名空间
coffer secret list --ns=production
```

### Q: 运行命令时密钥没有注入？

```bash
# 检查密钥状态
coffer check --json

# 确保使用正确的命名空间
coffer run --ns=production python app.py
```

## 快速命令参考

```bash
# 初始化
coffer init

# 密钥管理
coffer secret add <name> [--ns=<namespace>]
coffer secret list [--ns=<namespace>]
coffer secret delete <name> [--ns=<namespace>]

# 运行命令
coffer run [--ns=<namespace>] <command> [args...]

# 检查状态
coffer check [--json]

# 帮助
coffer --help
```

## 下一步

1. 在你的项目中运行 `coffer init`
2. 添加你需要的密钥
3. 使用 `coffer run` 运行你的应用

有问题随时问我！
