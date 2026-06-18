#!/bin/bash

set -e

echo "🔧 安装 Safetool..."

if ! command -v go &> /dev/null; then
    echo "❌ 请先安装 Go: https://golang.org/dl/"
    exit 1
fi

echo "📦 构建 coffer..."
go build -o coffer ./cmd/coffer

echo "📁 安装到 ~/bin..."
mkdir -p ~/bin
cp coffer ~/bin/
chmod +x ~/bin/coffer

if [[ ":$PATH:" != *":$HOME/bin:"* ]]; then
    echo 'export PATH="$HOME/bin:$PATH"' >> ~/.zshrc
    echo "✅ 已添加 ~/bin 到 PATH"
fi

rm -f coffer

echo ""
echo "✅ 安装完成！"
echo ""
echo "请运行以下命令使配置生效："
echo "  source ~/.zshrc"
echo ""
echo "然后就可以使用 coffer 了："
echo "  coffer --help"
echo "  coffer init"
echo "  coffer secret add my-secret"
echo "  coffer run python app.py"
