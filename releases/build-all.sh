#!/bin/bash
# 构建全平台发布二进制(内嵌前端,单文件分发)。
# 用法:bash releases/build-all.sh [--rebuild-web]  (加参数则先重建前端)
set -e
export GOCACHE=/Users/licong/Documents/Deepseek/task-panel/.gocache
cd /Users/licong/Documents/Deepseek/task-panel
OUT=$PWD/releases

# 1) 前端产物缺失或要求重建时,先构建前端
if [ ! -f web/dist/index.html ] || [ "$1" = "--rebuild-web" ]; then
  echo "== building frontend =="
  (cd web && npm run build)
fi

# 2) 同步前端产物到 embed 目录(go:embed 无法引用模块外路径)
echo "== syncing web/dist -> server/webembed/dist =="
rm -rf server/webembed/dist
mkdir -p server/webembed/dist
cp -r web/dist/* server/webembed/dist/

cd server
build_one() {
  local os=$1 arch=$2
  local name="taskpanel-server-$os-$arch"
  [ "$os" = "windows" ] && name="$name.exe"
  echo "== building $os/$arch =="
  CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath -ldflags="-s -w" -o "$OUT/$name" .
}

build_one linux amd64
build_one linux arm64
build_one linux 386
build_one linux arm
build_one darwin amd64
build_one darwin arm64
build_one windows amd64
build_one windows arm64
build_one freebsd amd64
build_one freebsd arm64

echo "=== ALL DONE ==="
ls -la "$OUT"
