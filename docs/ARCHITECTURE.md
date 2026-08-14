# 架构设计

## 分层

```
HTTP (handler)  →  业务 (service)  →  数据 (model / database)
              ↑
        pkg: cron / pathutil / dlticket / response / validator
              ↑
        middleware: JWT / CORS / 限流 / IP 解析 / 安全头
```

- **handler 层薄**:只做参数绑定、调用 service、统一响应。
- **service 层厚**:调度器、执行器、日志广播、环境变量注入等核心逻辑都在这里,可独立测试。
- **pkg 自研工具**:与业务无关的可复用能力,每个包单一职责。

## 调度器 (`service/scheduler.go`)

- 基于 `robfig/cron/v3`,进程内单实例。
- 启动时 `LoadEnabled()` 从数据库恢复所有启用任务并注册。
- 任务的 enable/disable/create/update/delete 联动 `Add/Remove`。
- `RunNow(id)` 手动触发;MVP 采用"同一任务串行执行,已在运行则拒绝"的简单并发模型。

## 执行器 (`service/executor.go`)

- 每个任务独立 goroutine;`exec.Command` **参数化构造**(`interpreter + scriptPath + args`),绝不走 shell,杜绝命令注入。
- `setpgid` 建进程组,停止/超时按组 kill(含子进程)。
- 超时与手动停止共用一个 `context.CancelFunc`:超时用 `time.AfterFunc`,停止由 `ManualStop` 直接调用。
- 输出经行扫描器实时分发到日志广播器并落盘;任务结束后结算状态(idle + last_run_status)。

## 命令解析 (`service/command_plan.go`)

- 支持两种写法:`<解释器> <脚本> [args]` 或 `<脚本> [args]`(按扩展名推断解释器)。
- `tokenize` 支持单/双引号包裹,引号未闭合报错。
- 脚本路径必须经 `pathutil.SafeJoin` 校验(在脚本目录内 + 存在)。

## 实时日志 (`service/log_broker.go`)

- 每个执行实例(taskLogID)对应一个 broadcaster:环形历史缓冲(2000 行)+ 订阅者 channel。
- SSE 订阅先回放历史,再实时转发;消费不过来则丢弃,避免阻塞执行器。
- 结束发送 `\x00DONE` 哨兵,前端据此关闭流。

## 鉴权

- JWT HS256,密钥自动生成持久化(`data/.jwt_secret`,0600)。
- 登出/改密按 jti 进黑名单。
- SSE 与原始下载:浏览器无法带 Authorization 头,SSE 走 `?token=` 校验,下载走短期 HMAC 票据(`pkg/dlticket`)。

## 配置

两层:启动配置(`config.yaml` + 环境变量)与运行期配置表 `setting`(键值对)。v0.2 起运行期配置用于通知/定时备份等动态开关;写操作经 `SettingService`,定时备份设置变更会同步重注册 cron。

## 通知 (`service/notify.go`)

- 渠道配置存 `notify_channel` 表,`config` 为 JSON 文本,类型决定字段(webhook/telegram/bark/email)。
- 发送为 best-effort:任务执行结束(成功/失败/终止)后由 `NotifyTaskResult` 异步推送至所有启用渠道;失败仅记日志,不影响任务结算。
- email 密码在 API 响应中脱敏,更新时掩码视为"不修改"。

## 备份 (`service/backup_service.go` + `pkg/backup`)

- 备份文件 = tar.gz(数据库 + 脚本目录)经 AES-256-GCM 加密,密钥由 `config.BackupKey` 自动生成并持久化(`data/.backup_key`,0600)。
- 恢复流程:解密解包到临时目录 → 自动备份当前状态(安全网)→ 停调度器/执行器 → 替换文件 → 重开数据库 → 重载调度器。
- 定时备份:运行期设置(启用/cron/保留份数),独立 cron 实例,超量自动清理旧备份。

## 审计 (`service/audit_service.go`)

- `audit_logs` 表记录登录/任务/脚本/环境变量/通知/备份等关键操作,含用户名、动作、对象、详情、IP。
- handler 层 `recordAudit` 统一埋点;查询支持按用户/动作过滤 + 分页。

## 部署形态

- Docker:多阶段(前端构建 → 后端编译 → alpine 运行时),Go 二进制直接托管前端 dist,无需 nginx。
- 镜像以非 root 用户运行。
