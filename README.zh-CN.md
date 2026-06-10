# Slow SQL Observer

Slow SQL Observer 是一个基于 Go 的 MySQL 慢 SQL 分析工具。它围绕单个被观测 source 采集 slow log，对重复 SQL 做 fingerprint 归一化，聚合性能指标，并提供 API 与轻量 Web UI。

英文说明见 `README.md`。

## V3 运行模型

当前版本仍然坚持单 source 边界，但在 V3 中加入了正式的采集层：

- 一个被观测的 MySQL source
- 一个专门存分析结果的 analysis MySQL schema
- 一个 collector 进程
- 一个 API / Web 进程
- 一种 source 日志模式：
  - `local_file`：直接解析本地可读的 slow log
  - `ssh_pull`：通过 SSH 从远端 Linux / OpenSSH 主机拉取 slow log 到本地 spool 文件，再解析 spool

分析库和被观测库是两件独立的事情：

- `SSO_ANALYSIS_DB_DSN` 用来保存分析结果
- `SSO_SOURCE_DB_DSN` 是可选项，只用于 source 元数据探测和连通性校验，不参与主采集链路

## 配置项

### 通用配置

- `SSO_SERVER_ADDR`
- `SSO_WEB_DIR`
- `SSO_SOURCE_INSTANCE_NAME`
- `SSO_SOURCE_LOG_MODE`
- `SSO_SOURCE_DB_DSN`（可选）
- `SSO_SOURCE_TIMEZONE`
- `SSO_SOURCE_DESCRIPTION`
- `SSO_ANALYSIS_DB_DSN`
- `SSO_ANALYSIS_DB_SCHEMA`
- `SSO_COLLECTOR_POLL_INTERVAL`
- `SSO_RAW_RECORD_RETENTION_DAYS`
- `SSO_LOG_LEVEL`

### `local_file` 模式

- `SSO_SOURCE_SLOW_LOG_PATH`

### `ssh_pull` 模式

- `SSO_SOURCE_REMOTE_HOST`
- `SSO_SOURCE_REMOTE_PORT`
- `SSO_SOURCE_REMOTE_USER`
- `SSO_SOURCE_REMOTE_SLOW_LOG_PATH`
- `SSO_SOURCE_SSH_PRIVATE_KEY_PATH`
- `SSO_SOURCE_SSH_KNOWN_HOSTS_PATH`
- `SSO_SOURCE_LOCAL_SPOOL_DIR`
- `SSO_SOURCE_INITIAL_POSITION`
- `SSO_SOURCE_LOCAL_SPOOL_MAX_BYTES`

### V1 兼容变量

下面这些 V1 变量名仍然保留一个过渡周期：

- `SSO_INSTANCE_NAME`
- `SSO_SLOW_LOG_PATH`
- `SSO_DB_DSN`
- `SSO_DB_SCHEMA`

如果同时设置了新旧变量名，优先使用新变量名，并输出 deprecated warning。

## 快速启动

1. 复制环境变量模板：

   ```powershell
   Copy-Item .env.example .env
   ```

2. 把 `SSO_ANALYSIS_DB_DSN` 配成一个具备建库建表权限的 MySQL 账号。

3. 选择一种 source 模式：

   `local_file`
   设置 `SSO_SOURCE_LOG_MODE=local_file`，并把 `SSO_SOURCE_SLOW_LOG_PATH` 指向一个当前机器可读的 slow log 文件。

   `ssh_pull`
   设置 `SSO_SOURCE_LOG_MODE=ssh_pull`，并补齐远端主机、SSH 用户、远端 slow log、私钥、known_hosts、本地 spool 目录等配置。

4. 如果你希望系统额外探测 source DB 的 host/version，可以再配置 `SSO_SOURCE_DB_DSN`。

5. 启动 API 服务：

   ```powershell
   go run ./cmd/server
   ```

6. 在另一个终端启动 collector：

   ```powershell
   go run ./cmd/collector
   ```

7. 打开 [http://localhost:8080](http://localhost:8080)。

## Source 前置条件

### 通用要求

- 已开启 MySQL slow query log
- `log_output=FILE`
- slow log 路径配置正确

### `local_file` 模式要求

- collector 所在机器能直接读取 slow log 文件

### `ssh_pull` 模式要求

- 远端主机是 Linux，并且具备 OpenSSH shell 环境
- 配置的 SSH 用户有权限读取 MySQL slow log
- `SSO_SOURCE_SSH_KNOWN_HOSTS_PATH` 中已经写入远端 host key
- `SSO_SOURCE_SSH_PRIVATE_KEY_PATH` 指向用于认证的私钥文件
- 本地 collector 机器对 `SSO_SOURCE_LOCAL_SPOOL_DIR` 有写权限

V3 只支持基于私钥文件 + known_hosts 校验的 SSH 方式。不支持密码登录、不支持纯 agent-only 登录，也不支持远端 Windows 主机。

## 采集与 spool 行为

`ssh_pull` 模式的链路是：

`远端 slow log -> SSH 拉取 -> 本地 spool 文件 -> parser -> fingerprint -> analysis storage`

需要特别注意的运行规则：

- `SSO_SOURCE_INITIAL_POSITION=end` 是默认值，适合首次接入线上环境时只追踪新写入日志
- `SSO_SOURCE_INITIAL_POSITION=start` 表示从当前远端文件头开始回放
- 远端采集 checkpoint 与 parser checkpoint 是两套独立状态
- 当本地 spool 达到 `SSO_SOURCE_LOCAL_SPOOL_MAX_BYTES` 时，本轮采集会进入 blocked 状态，不再继续拉取新字节
- 当 parser 已经完全消费掉 spool 中的全部内容时，系统会把 spool 截断清空，并把 parser checkpoint 重置为 `0`
- V3 只跟踪当前远端慢日志文件，不会回补已经轮转归档的旧日志

## 原始记录保留策略

`SSO_RAW_RECORD_RETENTION_DAYS` 用来控制 `slow_query_records` 的保留天数：

- `0` 或负数表示关闭自动清理
- 正数表示删除早于该天数的原始慢 SQL 记录
- fingerprint 聚合统计会继续保留

清理动作在 collector 轮询中顺手执行。清理失败会让 parser status 进入 degraded，但不会回滚已经成功提交的采集结果。

## API

当前提供的接口：

- `GET /api/source`
- `GET /api/acquisition/status`
- `GET /api/collector/status`
- `GET /api/dashboard/overview`
- `GET /api/slow-sql/fingerprints`
- `GET /api/slow-sql/fingerprints/:id`
- `GET /api/slow-sql/fingerprints/:id/records`

现在的 Web UI 会分别展示 source 元数据、acquisition 运行状态、parser 运行状态、远端上下文与本地 spool 状态。

## OpenSpec 流程

- V1 已归档 change：`openspec/changes/archive/2026-06-09-build-v1-slow-log-pipeline/`
- V2 已归档 change：`openspec/changes/archive/2026-06-09-add-source-aware-v2/`
- 当前进行中的 V3 change：`openspec/changes/add-remote-slow-log-acquisition/`
