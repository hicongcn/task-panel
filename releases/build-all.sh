#!/bin/bash
set -e
export GOCACHE=/Users/licong/Documents/Deepseek/task-panel/.gocache
cd /Users/licong/Documents/Deepseek/task-panel/server
OUT=/Users/licong/Documents/Deepseek/task-panel/releases

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
