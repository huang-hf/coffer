# Coffer 快速开始

## 1. 安装 Go（如果没有）

```bash
# macOS
brew install go
```

## 2. 构建 Coffer

```bash
cd /path/to/coffer
go build -o coffer ./cmd/coffer
```

## 3. 安装到系统

```bash
mkdir -p ~/bin
cp coffer ~/bin/
chmod +x ~/bin/coffer
echo 'export PATH="$HOME/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

## 4. 验证安装

```bash
coffer --version
```

## 5. 在你的项目中使用

```bash
# 初始化
cd /path/to/your/project
coffer init

# 添加密钥
coffer secret add db-password

# 运行命令
coffer run python app.py
```

## 完成！

现在你可以：
- `coffer init` - 初始化项目
- `coffer secret add <name>` - 添加密钥
- `coffer run <command>` - 运行带密钥的命令
- `coffer check` - 检查密钥状态
