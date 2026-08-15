# 全栈测试报告

本文档记录 task-panel 的全栈测试基线、覆盖范围与历史修复,保证每次合入前 `go test ./...` + `npm run build` + API 冒烟全部通过。

## 测试基线(三层)

| 层 | 命令 | 覆盖 |
|---|---|---|
| 后端单元测试 | `cd server && go test ./...` | pkg(cron/路径穿越/票据/TOTP/校验/备份)+ service(命令拆分/执行器/通知/依赖) |
| 后端集成测试 | `go test .`(integration_test.go) | 真实 HTTP 引擎全链路:认证/任务/脚本/日志/审计/通知/备份/OpenAPI/2FA/迁移 |
| 前端构建 | `cd web && npm run build` | vue-tsc 类型检查 + Vite 生产构建(含 Monaco 全量语法) |

> 验证默认 **`CGO_ENABLED=0`**(与 Dockerfile 一致)下全量测试通过 —— 纯 Go SQLite 驱动的硬性要求。

## 后端单元测试

- `pkg/cronutil`:Cron 表达式校验 + 中文描述(每N分钟/每周几/每月几号)
- `pkg/pathutil`:路径归一化、绝对路径/`..`/盘符拒绝、软链解析、base 自身软链
- `pkg/dlticket`:HMAC 下载票据签名/防篡改/过期
- `pkg/totp`:RFC 6238 官方测试向量
- `pkg/validator`:用户名/密码规则
- `pkg/backup`:AES-256-GCM 备份加密解密往返
- `service`:命令参数化拆分(tokenize 引号处理)、执行器超时/停止、通知模板渲染、依赖包名白名单、pip/npm 输出容错解析(extractJSONValue)

## 后端集成测试(integration_test.go)

覆盖场景(HTTP 级断言,非 mock):

- **认证**:check-init → 初始化管理员 → 重复初始化拒绝 → 登录 → 错误密码 401 → 登出后 token 吊销(黑名单)
- **登录防护**:5 次失败锁定 15 分钟(阶梯延长)、每 IP 限流 5 次/分钟
- **任务**:CRUD、非法 cron 拒绝、脚本不存在拒绝、命令穿越防护、手动运行 → last_run_status=success、长任务停止 → aborted、标签创建/过滤/统计、批量启用/禁用/运行/删除
- **脚本**:读写/重命名/目录/移动、`../` 与绝对路径穿越拒绝、文件树
- **日志**:历史列表/详情/原文下载(票据)、SSE live-ticket 鉴权(无票据/坏票据拒绝)
- **审计**:登录/脚本/任务操作留痕、按动作过滤
- **通知**:渠道 CRUD/toggle/非法类型、任务失败通知、成功通知、自定义模板渲染、系统事件告警(全部走本地 webhook 真实 HTTP)
- **备份**:创建 → 修改数据 → 上传恢复 → 校验回滚(脚本树回到备份时状态)
- **IP 白名单**:CIDR/单 IP 放行与拒绝(httptest RemoteAddr + 可信代理逻辑)
- **依赖管理**:注入字符/空包名拒绝,pip3/npm 存在时列表 200
- **系统监控**:stats 字段非空、未登录 401
- **OpenAPI**:应用创建/保留字/非法 scope 拒绝、列表含 scopes 数组、错误 secret 401、正确换 token、scope 越权 403、密钥重置后旧 secret 失效
- **2FA/TOTP**:状态 → setup → 错误码启用拒绝 → 正确码启用 → 登录强制动态码 → 错误密码解除拒绝 → 解除后恢复
- **迁移**:任务/脚本/环境变量 JSON 导出 → 清空 → 导入恢复(含明文环境变量)
- **排序/面板配置**:环境变量拖拽排序、面板标题/图标/日志清理天数、推送模板

## 前端构建

- `vue-tsc` 类型检查零错误;Vite 生产构建通过(约 12s)。
- 唯一警告:Monaco 编辑器导致 `Scripts` chunk 约 3.9MB(>500KB 阈值),属预期,后续可用动态导入优化。

## 真实服务 API 冒烟(独立实例 + curl)

启动真实二进制(独立数据目录/端口)后的 HTTP 冒烟矩阵:

| 类别 | 断言 |
|---|---|
| 公开接口 | /version /health /system/panel /robots.txt /(前端首页)/assets 静态资源 200 |
| 安全响应头 | X-Content-Type-Options: nosniff、X-Frame-Options: DENY |
| CORS | 白名单 origin preflight 204;非法 origin 403;**null origin 403** |
| 鉴权 | 初始化 201 → 重复初始化 400 → 登录 → 无 token 401 → 登出吊销 401 |
| 任务端到端 | 建脚本 → 建任务 → 运行 → last_run_status=success → 日志落盘含输出 → 清理 |
| 登录限流 | 高频登录触发 429(每 IP 5 次/分钟) |
| OpenAPI | 换 token 200 / 错误 secret 401 / 无 scope 接口 403 |
| 备份 | 创建 + 下载 200 |
| 系统监控 | mem_total/cpu_percent/hostname/uptime 非空 |
| 404/SPA | 未知 API 404;未知路径回退前端 index.html |

## 平台兼容性

`CGO_ENABLED=0` 交叉编译验证(全部成功且静态链接):

| 平台 | 产物 |
|---|---|
| linux/amd64、linux/arm64、linux/386 | ELF 静态 |
| darwin/amd64 | Mach-O |
| windows/amd64、windows/arm64 | PE32+ |
| freebsd/amd64 | ELF 静态 |

`CGO_ENABLED=0` 二进制实测启动 + 全链路 API 正常(健康检查/初始化/登录/任务/标签过滤/运行),Docker 镜像即按此构建。

## 历史修复记录

### 2026-08-15:SQLite 驱动切换为纯 Go(glebarez/modernc)

- **问题**:原 `gorm.io/driver/sqlite`(mattn/go-sqlite3)必须 CGO;`CGO_ENABLED=0` 构建能"通过"但运行时是 stub,启动即失败(`go-sqlite3 requires cgo to work`),Docker 镜像不可用。
- **修复**:换用 `github.com/glebarez/sqlite`(基于 modernc.org/sqlite 纯 Go),`CGO_ENABLED=0` 真正可运行,交叉编译零 C 工具链依赖。
- **连带修复**:`model.StringList.Value()` 原返回 `[]byte`,纯 Go 驱动按 BLOB 绑定存储,导致 `tags LIKE` 文本匹配全部失效(集成测试 TestTaskTags 暴露);改为返回 `string`,存储类别恢复 TEXT。

### 2026-08-15:pip/npm 列表输出容错解析

- **问题**:macOS/Linux 上 pip 把 cache 等 WARNING 写到 stderr,`runCmd` 合并输出后整体按 JSON 解析失败,`GET /deps/python` 报错(集成测试 TestDeps 暴露)。
- **修复**:`service` 新增 `extractJSONValue`,从合并输出提取首个完整 JSON 值(容忍前置警告),pip/npm 列表共用。
