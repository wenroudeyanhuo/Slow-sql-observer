# Slow SQL Observer API Reference

面向前端联调的 V2 接口文档。

Base URL:

```text
http://localhost:8080
```

所有接口当前均为 `GET`，返回 `application/json`。

## 通用约定

### 成功响应

- HTTP 状态码：`200`
- Content-Type：`application/json`

### 失败响应

- 常见状态码：`400`、`404`、`500`
- 响应格式：

```json
{
  "error": "error message"
}
```

### 时间字段

接口中的时间字段均为服务端序列化后的时间字符串，前端按 ISO 时间处理即可。

---

## 1. 获取当前 Source 信息

### 请求

```http
GET /api/source
```

### Query 参数

无

### 响应字段

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | `number` | source 主键 ID |
| `key` | `string` | source 唯一标识 hash |
| `instanceName` | `string` | source 名称 |
| `slowLogPath` | `string` | 当前 slow log 文件路径 |
| `description` | `string \| null` | source 描述 |
| `databaseDsnConfigured` | `boolean` | 是否配置了 `SSO_SOURCE_DB_DSN` |
| `databaseHost` | `string \| null` | source DB host，probe 成功时可能返回 |
| `databaseVersion` | `string \| null` | source DB version，probe 成功时可能返回 |
| `createdAt` | `string` | source 创建时间 |
| `updatedAt` | `string` | source 更新时间 |

### 响应示例

```json
{
  "id": 1,
  "key": "5f3fe37f4f6a4c0d0d905f9d8c8d83d9f3a0b111",
  "instanceName": "local-mysql",
  "slowLogPath": "./scripts/sample-slow.log",
  "description": "Local sample source",
  "databaseDsnConfigured": false,
  "databaseHost": null,
  "databaseVersion": null,
  "createdAt": "2026-06-08T14:00:00Z",
  "updatedAt": "2026-06-08T14:00:00Z"
}
```

---

## 2. 获取 Collector 状态

### 请求

```http
GET /api/collector/status
```

### Query 参数

无

### 响应字段

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `sourceId` | `number` | 当前状态所属 source ID |
| `collectorState` | `string` | collector 状态：`idle` / `healthy` / `degraded` / `error` |
| `sourceAccessState` | `string` | source 可访问状态：`unknown` / `accessible` / `inaccessible` |
| `lastSuccessfulIngestAt` | `string \| null` | 最近一次成功采集时间 |
| `lastCheckpointOffset` | `number \| null` | 最近 checkpoint offset |
| `lastFileIdentity` | `string \| null` | 最近一次文件 identity |
| `lastErrorAt` | `string \| null` | 最近错误时间 |
| `lastErrorMessage` | `string \| null` | 最近错误消息 |
| `updatedAt` | `string` | 状态更新时间 |

### 响应示例

```json
{
  "sourceId": 1,
  "collectorState": "healthy",
  "sourceAccessState": "accessible",
  "lastSuccessfulIngestAt": "2026-06-08T14:05:00Z",
  "lastCheckpointOffset": 728,
  "lastFileIdentity": "6687472:392078",
  "lastErrorAt": null,
  "lastErrorMessage": null,
  "updatedAt": "2026-06-08T14:05:00Z"
}
```

---

## 3. 获取 Dashboard Overview

### 请求

```http
GET /api/dashboard/overview
```

### Query 参数

无

### 响应字段

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `totalRecords` | `number` | raw slow query record 总数 |
| `totalFingerprints` | `number` | fingerprint 总数 |
| `totalQueryTimeSec` | `number` | query time 总和，单位秒 |
| `avgQueryTimeSec` | `number` | query time 平均值，单位秒 |
| `maxQueryTimeSec` | `number` | query time 最大值，单位秒 |
| `lastIngestedAt` | `string \| null` | 最近采集时间 |
| `topFingerprints` | `array` | top fingerprint 列表，字段结构见下方 |

### `topFingerprints[]` 字段

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | `number` | fingerprint ID |
| `sourceId` | `number` | 所属 source ID |
| `fingerprintHash` | `string` | fingerprint hash |
| `normalizedSql` | `string` | 归一化后的 SQL |
| `sqlType` | `string` | SQL 类型 |
| `mainTableName` | `string \| null` | 主表名 |
| `firstSeenAt` | `string` | 首次出现时间 |
| `lastSeenAt` | `string` | 最近出现时间 |
| `createdAt` | `string` | 创建时间 |
| `updatedAt` | `string` | 更新时间 |
| `fingerprintId` | `number` | 对应统计中的 fingerprint ID |
| `totalCount` | `number` | 出现次数 |
| `totalQueryTimeSec` | `number` | 总耗时 |
| `avgQueryTimeSec` | `number` | 平均耗时 |
| `maxQueryTimeSec` | `number` | 最大耗时 |
| `totalRowsExamined` | `number` | 总扫描行数 |
| `avgRowsExamined` | `number` | 平均扫描行数 |
| `maxRowsExamined` | `number` | 最大扫描行数 |
| `totalRowsSent` | `number` | 总返回行数 |
| `avgRowsSent` | `number` | 平均返回行数 |
| `maxRowsSent` | `number` | 最大返回行数 |

---

## 4. 获取 Fingerprint 列表

### 请求

```http
GET /api/slow-sql/fingerprints
```

### Query 参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- |
| `page` | `number` | 否 | `1` | 页码 |
| `pageSize` | `number` | 否 | `20` | 每页条数，服务端最大 `100` |
| `sortBy` | `string` | 否 | `totalQueryTimeSec` | 排序字段 |
| `sortOrder` | `string` | 否 | `desc` | 排序方向：`asc` / `desc` |
| `keyword` | `string` | 否 | `""` | 按 `normalizedSql` 模糊搜索 |
| `dbName` | `string` | 否 | `""` | 按数据库名过滤 |
| `sqlType` | `string` | 否 | `""` | 按 SQL 类型过滤：`SELECT` / `INSERT` / `UPDATE` / `DELETE` |

### `sortBy` 可选值

- `totalQueryTimeSec`
- `avgQueryTimeSec`
- `maxQueryTimeSec`
- `totalCount`
- `lastSeenAt`
- `avgRowsExamined`

### 响应字段

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `items` | `array` | fingerprint 列表 |
| `total` | `number` | 总条数 |
| `page` | `number` | 当前页 |
| `pageSize` | `number` | 每页条数 |

`items[]` 的字段结构与 `overview.topFingerprints[]` 一致。

### 请求示例

```http
GET /api/slow-sql/fingerprints?page=1&pageSize=20&sortBy=totalQueryTimeSec&sortOrder=desc&keyword=orders
```

---

## 5. 获取单个 Fingerprint 详情

### 请求

```http
GET /api/slow-sql/fingerprints/:id
```

### Path 参数

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `id` | `number` | 是 | fingerprint ID |

### 响应字段

响应对象字段与 `overview.topFingerprints[]` 单项结构一致。

### 响应示例

```json
{
  "id": 1,
  "sourceId": 1,
  "fingerprintHash": "7f1d0f7f8b16d3f02f3e7c1f1c5f0abc0b3d1111",
  "normalizedSql": "SELECT * FROM orders WHERE id = ?",
  "sqlType": "SELECT",
  "mainTableName": "orders",
  "firstSeenAt": "2026-06-08T14:00:00Z",
  "lastSeenAt": "2026-06-08T14:05:00Z",
  "createdAt": "2026-06-08T14:00:00Z",
  "updatedAt": "2026-06-08T14:05:00Z",
  "fingerprintId": 1,
  "totalCount": 2,
  "totalQueryTimeSec": 3.445678,
  "avgQueryTimeSec": 1.722839,
  "maxQueryTimeSec": 2.345678,
  "totalRowsExamined": 160,
  "avgRowsExamined": 80,
  "maxRowsExamined": 100,
  "totalRowsSent": 2,
  "avgRowsSent": 1,
  "maxRowsSent": 1,
  "updatedAt": "2026-06-08T14:05:00Z"
}
```

### 失败场景

当 `id` 不存在时：

- HTTP 状态码：`404`
- 响应：

```json
{
  "error": "sql: no rows in result set"
}
```

---

## 6. 获取 Fingerprint 对应的原始记录列表

### 请求

```http
GET /api/slow-sql/fingerprints/:id/records
```

### Path 参数

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `id` | `number` | 是 | fingerprint ID |

### Query 参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- |
| `page` | `number` | 否 | `1` | 页码 |
| `pageSize` | `number` | 否 | `20` | 每页条数，服务端最大 `100` |
| `sortBy` | `string` | 否 | `occurredAt` | 排序字段 |
| `sortOrder` | `string` | 否 | `desc` | 排序方向：`asc` / `desc` |

### `sortBy` 可选值

- `occurredAt`
- `queryTimeSec`

### 响应字段

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `items` | `array` | 原始记录列表 |
| `total` | `number` | 总条数 |
| `page` | `number` | 当前页 |
| `pageSize` | `number` | 每页条数 |

### `items[]` 字段

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | `number` | 记录 ID |
| `sourceId` | `number` | 所属 source ID |
| `sourceInstanceName` | `string` | source 名称 |
| `sourceLogFilePath` | `string` | slow log 路径 |
| `sourceFileIdentity` | `string` | 日志文件 identity |
| `sourceOffsetStart` | `number` | 原始 block 起始 offset |
| `sourceOffsetEnd` | `number` | 原始 block 结束 offset |
| `occurredAt` | `string` | SQL 发生时间 |
| `dbName` | `string \| null` | 数据库名 |
| `userName` | `string \| null` | 用户名 |
| `clientHost` | `string \| null` | 客户端 host |
| `rawBlock` | `string` | 原始 slow log block |
| `rawSql` | `string` | 原始 SQL |
| `normalizedSql` | `string` | 归一化 SQL |
| `fingerprintId` | `number` | fingerprint ID |
| `fingerprintHash` | `string` | fingerprint hash |
| `queryTimeSec` | `number` | 查询耗时（秒） |
| `lockTimeSec` | `number \| null` | 锁耗时（秒） |
| `rowsSent` | `number \| null` | 返回行数 |
| `rowsExamined` | `number \| null` | 扫描行数 |
| `createdAt` | `string` | 入库时间 |

### 请求示例

```http
GET /api/slow-sql/fingerprints/1/records?page=1&pageSize=20&sortBy=occurredAt&sortOrder=desc
```

---

## 7. 前端联调建议

### 推荐页面加载顺序

1. 先请求 `/api/source`
2. 再请求 `/api/collector/status`
3. 然后按页面请求业务数据：
   - overview 页：`/api/dashboard/overview`
   - 列表页：`/api/slow-sql/fingerprints`
   - 详情页：`/api/slow-sql/fingerprints/:id`
   - 样本记录：`/api/slow-sql/fingerprints/:id/records`

### 状态判断建议

- 正常空数据：
  - `collectorState = healthy`
  - 但 overview / list 没有数据
- 降级状态：
  - `collectorState = degraded`
  - 可继续展示已有数据，同时提示 `lastErrorMessage`
- 错误状态：
  - `collectorState = error`
  - 或 `sourceAccessState = inaccessible`
  - 前端应优先提示 source / collector 异常
