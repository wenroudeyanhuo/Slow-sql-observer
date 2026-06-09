# Slow SQL Observer

这是一个基于 Go 的 MySQL 慢查询日志分析系统，用来读取单个 slow log source、对重复 SQL 做 fingerprint、聚合性能指标，并提供 API 和 Web UI。

英文版说明见 `README.md`。

## 当前 V2 运行模型

当前版本采用 source-aware V2 模型：

- 一个被观测的 MySQL source
- 一个 slow log 文件作为主采集通道
- 一个 Slow SQL Observer 自己使用的 analysis MySQL schema
- 一个 collector 进程
- 一个 API / Web UI 进程

collector 必须运行在能够读取 slow log 文件的环境中。推荐与 MySQL 同机部署，或者至少能挂载访问 slow log 路径。

## 配置项

推荐使用的 V2 环境变量：

- `SSO_SOURCE_INSTANCE_NAME`
- `SSO_SOURCE_SLOW_LOG_PATH`
- `SSO_SOURCE_DB_DSN`（可选，用于 source 连通性校验和元数据补充）
- `SSO_SOURCE_TIMEZONE`
- `SSO_SOURCE_DESCRIPTION`
- `SSO_ANALYSIS_DB_DSN`
- `SSO_ANALYSIS_DB_SCHEMA`
- `SSO_SERVER_ADDR`
- `SSO_WEB_DIR`
- `SSO_COLLECTOR_POLL_INTERVAL`
- `SSO_RAW_RECORD_RETENTION_DAYS`
- `SSO_LOG_LEVEL`

V1 旧变量名在一个兼容周期内仍然可用：

- `SSO_INSTANCE_NAME`
- `SSO_SLOW_LOG_PATH`
- `SSO_DB_DSN`
- `SSO_DB_SCHEMA`

如果使用旧变量名，程序会输出 deprecated warning；如果新旧同时存在，以 V2 新变量名为准。

## 快速启动

1. 复制环境变量模板：

   ```powershell
   Copy-Item .env.example .env
   ```

2. 将 `SSO_ANALYSIS_DB_DSN` 配置为一个有建库建表权限的 MySQL 账号。

3. 将 `SSO_SOURCE_SLOW_LOG_PATH` 配置为一个当前进程可读取的 MySQL slow log 文件路径。

4. 如果你希望系统补充 source DB 元数据并做连通性校验，可以额外设置 `SSO_SOURCE_DB_DSN`。

5. 启动 API 服务：

   ```powershell
   go run ./cmd/server
   ```

6. 在另一个终端中启动 collector：

   ```powershell
   go run ./cmd/collector
   ```

7. 打开 [http://localhost:8080](http://localhost:8080)。

## Source 侧前提

被观测的 MySQL source 需要满足：

- 已开启 slow query log
- `log_output=FILE`
- 配置的 slow log 路径正确
- collector 进程对该日志文件有读取权限

`SSO_SOURCE_DB_DSN` 是可选项。它只用于 source DB 探测和元数据补充，不是主要采集通道。

## Raw record retention

`SSO_RAW_RECORD_RETENTION_DAYS` 用来控制 `slow_query_records` 的清理：

- `0` 或负数表示禁用自动清理
- 正数表示删除超过保留天数的原始 slow query records
- `fingerprints` 和聚合统计默认继续保留

清理动作由 collector 在轮询过程中顺手执行。清理失败会让 collector status 进入 degraded，但不会回滚已经成功提交的采集结果。

## API 接口

- `GET /api/source`
- `GET /api/collector/status`
- `GET /api/dashboard/overview`
- `GET /api/slow-sql/fingerprints`
- `GET /api/slow-sql/fingerprints/:id`
- `GET /api/slow-sql/fingerprints/:id/records`

## OpenSpec 变更

- V1 基线：`openspec/changes/build-v1-slow-log-pipeline/`
- 当前 V2 change：`openspec/changes/add-source-aware-v2/`
