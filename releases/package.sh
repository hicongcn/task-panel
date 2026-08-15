#!/bin/bash
set -e
cd /Users/licong/Documents/Deepseek/task-panel/releases
PKG=/tmp/tp-release-pkg
rm -rf "$PKG" && mkdir -p "$PKG"

cat > README-release.txt <<'EOF'
task-panel v1.2.0 单文件发布版(内嵌前端,无需外部 web 目录)

使用:
  ./taskpanel-server-<os>-<arch>           # 默认 5700 端口,自动托管内嵌前端
  SERVER_PORT=8080 ./taskpanel-server-...  # 自定义端口
  WEB_DIR=/path/to/web ./taskpanel-server-...  # 使用外部前端目录(可选)

首次访问 http://localhost:5700 初始化管理员。
数据默认存于运行目录 ./data。
EOF

for f in taskpanel-server-*; do
  [ "$f" = "taskpanel-server-*" ] && continue
  base="${f%.exe}"
  if [[ "$f" == *.exe ]]; then
    mkdir -p "$PKG/tp-win"
    cp "$f" "$PKG/tp-win/"
    cp README-release.txt "$PKG/tp-win/"
    (cd "$PKG" && zip -q -r "$base.zip" tp-win && rm -rf tp-win)
    echo "packed: $base.zip"
  else
    mkdir -p "$PKG/tp-unix"
    cp "$f" "$PKG/tp-unix/"
    cp README-release.txt "$PKG/tp-unix/"
    (cd "$PKG" && tar czf "$base.tar.gz" tp-unix && rm -rf tp-unix)
    echo "packed: $base.tar.gz"
  fi
done
ls -la "$PKG"
