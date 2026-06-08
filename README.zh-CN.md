# Slow SQL Observer

一个基于 Go 的 MySQL 慢查询日志分析系统，用于采集、归一化、聚合并可视化慢 SQL 事件。

英文版说明文档见 `README.md`。

## 当前已实现内容

当前 V1 代码已经包含：

- 一个 `collector` 命令，用于增量读取单个慢查询日志文件
- 基于 MySQL `# Time:` 边界的事件分块能力
- 将慢日志解析为结构化慢 SQL 记录
- 保守型 SQL 指纹归一化与稳定哈希生成
- 使用 MySQL 持久化原始记录、指纹、聚合统计和 checkpoint
- 提供 overview、fingerprint 列表、fingerprint 详情、样本记录等 HTTP API
- 一个轻量级静态 Web UI，用于展示总览、排行和详情页面

## 使用 Docker 快速开始

这是推荐给首次使用者的方式，因为它能提供一个更稳定、可预期的 MySQL 运行环境，手动准备工作也最少。

1. 启动 MySQL：

   ```powershell
   docker compose up -d
   ```

2. 复制环境变量模板：

   ```powershell
   Copy-Item .env.example .env
   ```

3. 如果你需要修改数据库连接、schema、慢日志路径或监听端口，可以先编辑 `.env`。

4. 启动 API 服务：

   ```powershell
   go run ./cmd/server
   ```

5. 在另一个终端中启动 collector：

   ```powershell
   go run ./cmd/collector
   ```

6. 打开 [http://localhost:8080](http://localhost:8080)。

默认已经配置好了示例慢日志文件：`scripts/sample-slow.log`。

## 使用已有的本地 MySQL

如果你本机已经有可用的 MySQL，或者你准备连接一个可访问的 MySQL 实例，也可以完全跳过 Docker。

1. 复制环境变量模板：

   ```powershell
   Copy-Item .env.example .env
   ```

2. 修改 `.env` 中的数据库相关配置：

   - 将 `SSO_DB_DSN` 改成你自己的 MySQL DSN，要求该账号有建库建表权限
   - 如果你不想使用默认库名，可以调整 `SSO_DB_SCHEMA`
   - 如果你要分析自己的慢日志文件，可以修改 `SSO_SLOW_LOG_PATH`

   你不需要手动创建 analysis schema 或数据表。只要当前配置的 MySQL 账号权限足够，程序启动时会自动完成这些初始化。

3. 启动 API 服务：

   ```powershell
   go run ./cmd/server
   ```

4. 在另一个终端中启动 collector：

   ```powershell
   go run ./cmd/collector
   ```

5. 打开 [http://localhost:8080](http://localhost:8080)。

程序启动时会自动读取 `.env`。如果你在当前 shell、CI 或部署环境里显式设置了环境变量，那么显式设置的值会优先于 `.env`。

## 核心流程

```text
MySQL slow query log
  -> collector
  -> parser
  -> fingerprint
  -> storage
  -> API
  -> web UI
```

## API 接口

- `GET /api/dashboard/overview`
- `GET /api/slow-sql/fingerprints`
- `GET /api/slow-sql/fingerprints/:id`
- `GET /api/slow-sql/fingerprints/:id/records`

## OpenSpec 变更

当前 V1 实现计划对应的 OpenSpec change 位于：

- `openspec/changes/build-v1-slow-log-pipeline/`
