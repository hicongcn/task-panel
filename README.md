# task-panel

轻量、稳定的自研定时任务管理面板。**独立实现**,架构思路参考同类项目(青龙/呆呆面板),但所有代码、命名与界面均为原创。

## 功能(v1.2)

- **管理员初始化 + 登录鉴权**:首次启动创建唯一管理员;JWT(HS256)+ bcrypt;登录失败锁定(5 次/15 分钟)。
- **定时任务**:CRUD、Cron 校验(**中文描述:每N分钟/每周几/每月几号等**)、标签分组与批量操作、命令下拉选脚本(免解释器前缀,名称自动取脚本名)、**运行/停止合一按钮**、最近日志查看、超时、重试、进程组终止(含子进程)。
- **脚本管理**:完整 Monaco 编辑器(20+ 语言语法高亮,**JS/TS/JSON 代码补全**,查找替换/多光标/代码折叠等全功能)+ 文件树(文件夹图标区分、目录优先排序、拖拽移动)、新建文件可选目录、文件类型白名单、**多层路径穿越防护 + 软链解析**。
- **脚本执行**:python3 / node / bash / go run,**命令参数化构造(禁止 shell 拼接)**;支持运行文件与调试内联代码。
- **实时日志**:SSE 流式输出;历史日志落盘 + 查询;原始文件下载(短期 HMAC 票据鉴权)。
- **环境变量**:CRUD、拖拽排序、明文显示与一键复制、更新时间、执行时注入(过滤 LD_PRELOAD 等危险变量)。
- **通知渠道**:Webhook / Telegram / Bark / 邮件;任务执行结束(成功/失败/终止)自动推送结果;支持测试发送。
- **备份与恢复**:AES-256-GCM 加密备份(数据库 + 脚本),一键创建/下载/恢复(恢复前自动备份当前状态);支持 cron 定时备份与保留份数。
- **审计日志**:登录、任务、脚本、环境变量、通知、备份、依赖操作留痕,可按用户/动作筛选。
- **依赖管理**:Python(pip 系统环境)与 Node(npm 全局)的包可视化列出/安装/卸载,命令参数化执行 + 包名白名单校验。
- **系统监控**:仪表板实时展示 CPU / 内存 / 磁盘 / 负载 / 运行时长(3 秒自动刷新)。
- **双重认证**:RFC 6238 TOTP,扫码绑定,登录强制动态码;丢失验证器可用 CLI 兜底恢复。
- **CLI 运维工具**:同一二进制子命令 —— `account-reset`(重置密码/关 2FA)、`log-clean`(清理旧日志)、`task-trigger`(触发任务)。
- **Open API**(参考青龙结构):应用管理(client_id/secret + scopes)→ `POST /open/auth/token` 换令牌 → Bearer 调用 `/open/tasks` 等开放接口。
- **安全基线**:JWT 黑名单吊销、CORS 严格白名单(拒绝 null origin)、IP 解析默认仅信任回环、**可选 IP 访问白名单**(`IP_WHITELIST`)、2FA、安全响应头、请求体上限。

## 技术栈

- 后端:Go 1.22+ / Gin / GORM / SQLite(`github.com/glebarez/sqlite` 纯 Go 驱动,基于 modernc.org/sqlite;`CGO_ENABLED=0` 静态单二进制,可交叉编译任意主流平台)
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
cd server && go test ./...      # 路径穿越 / Cron / 票据 / 校验 / 命令拆分 / 集成测试
cd web && npm run build          # 前端类型检查 + 构建
```

测试基线为三层:后端单元测试、后端集成测试(integration_test.go,真实 HTTP 引擎)、前端构建;并在 `CGO_ENABLED=0` 下全部通过。详见 [全栈测试报告](docs/TESTING.md)。

## 文档

- [架构设计](docs/ARCHITECTURE.md)
- [安全设计](docs/SECURITY.md)
- [路线图](docs/ROADMAP.md)
- [全栈测试报告](docs/TESTING.md)

## License

MIT
