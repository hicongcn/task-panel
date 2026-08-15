task-panel v1.2.0 单文件发布版(内嵌前端,无需外部 web 目录)

使用:
  ./taskpanel-server-<os>-<arch>           # 默认 5700 端口,自动托管内嵌前端
  SERVER_PORT=8080 ./taskpanel-server-...  # 自定义端口
  WEB_DIR=/path/to/web ./taskpanel-server-...  # 使用外部前端目录(可选)

首次访问 http://localhost:5700 初始化管理员。
数据默认存于运行目录 ./data。
