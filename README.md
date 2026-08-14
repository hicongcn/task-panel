# task-panel

轻量、稳定的自研定时任务管理面板。**独立实现**,架构思路参考同类项目(青龙/呆呆面板),但所有代码、命名与界面均为原创。

## 功能(v0.2)

- **管理员初始化 + 登录鉴权**:首次启动创建唯一管理员;JWT(HS256)+ bcrypt;登录失败锁定(5 次/15 分钟)。
- **定时任务**:CRUD、Cron 校验(带中文描述)、启用/禁用、手动运行/停止、超时、重试、进程组终止(含子进程)。
- **脚本管理**:Monaco 在线编辑器(语法高亮,按扩展名自动识别语言)+ 文件树、新建/编辑/删除/重命名、文件类型白名单、**多层路径穿越防护 + 软链解析**。
- **脚本执行**:python3 / node / bash / go run,**命令参数化构造(禁止 shell 拼接)**;支持运行文件与调试内联代码。
- **实时日志**:SSE 流式输出;历史日志落盘 + 查询;原始文件下载(短期 HMAC 票据鉴权)。
- **环境变量**:CRUD、分组、值脱敏、执行时注入(过滤 LD_PRELOAD 等危险变量)。
- **通知渠道**:Webhook / Telegram / Bark / 邮件;任务执行结束(成功/失败/终止)自动推送结果;支持测试发送。
- **备份与恢复**:AES-256-GCM 加密备份(数据库 + 脚本),一键创建/下载/恢复(恢复前自动备份当前状态);支持 cron 定时备份与保留份数。
- **审计日志**:登录、任务、脚本、环境变量、通知、备份、依赖操作留痕,可按用户/动作筛选。
- **依赖管理**:Python(pip 系统环境)与 Node(npm 全局)的包可视化列出/安装/卸载,命令参数化执行 + 包名白名单校验。
- **安全基线**:JWT 黑名单吊销、CORS 严格白名单(拒绝 null origin)、IP 解析默认仅信任回环、**可选 IP 访问白名单**(`IP_WHITELIST`)、安全响应头、请求体上限。

## 技术栈

- 后端:Go 1.22+ / Gin / GORM / SQLite(纯 Go 驱动,`CGO_ENABLED=0` 单二进制)
- 前端:Vue 3 + TypeScript + Vite + Element Plus + Pinia
- 调度:robfig/cron/v3

## 快速开始

### Docker

```bash
docker compose up -d --build
# 浏览器访问 http://localhost:5700,首次进入初始化管理员
```

### 本地开发

```bash
# 后端
cd server && go mod tidy && go run .          # http://localhost:5700

# 前端(另开终端,dev server 代理 /api 到后端)
cd web && npm install && npm run dev          # http://localhost:5173
```

### 目录约定

- 配置:`server/config.yaml`(相对路径以该文件为基准)
- 数据:`server/data/taskpanel.db`
- 脚本:`server/data/scripts/`　日志:`server/data/logs/`　备份:`server/data/backups/`
- 密钥:`server/data/.jwt_secret`、`server/data/.backup_key`(均自动生成,0600)

## 配置项

| 项 | 环境变量 | 说明 |
|---|---|---|
| 端口 | `SERVER_PORT` | 默认 5700 |
| 数据目录 | `DATA_DIR` | 默认 `./data` |
| 数据库路径 | `DB_PATH` | 默认 `./data/taskpanel.db` |
| 前端目录 | `WEB_DIR` | 留空则后端不托管(走反代) |
| JWT 密钥 | `JWT_SECRET` | 留空自动生成持久化 |
| CORS 白名单 | `CORS_ORIGINS` | 逗号分隔 |
| 访问白名单 | `IP_WHITELIST` | 逗号分隔的 IP/CIDR,如 `192.168.1.0/24,10.0.0.5`;留空不启用 |
| 可信代理网段 | `TRUSTED_PROXY_CIDRS` | 默认仅回环,反代后需显式配置 |

## 测试

```bash
cd server && go test ./...      # 路径穿越 / Cron / 票据 / 校验 / 命令拆分
cd web && npm run build          # 前端类型检查 + 构建
```

## 文档

- [架构设计](docs/ARCHITECTURE.md)
- [安全设计](docs/SECURITY.md)
- [路线图](docs/ROADMAP.md)

## License

MIT
