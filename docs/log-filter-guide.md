# Hướng Dẫn Sử Dụng Hệ Thống Filter Log

> **Xem thêm:** [Tổng hợp nguồn log trong project](log-sources-review.md) — liệt kê toàn bộ file dùng log và loại log (có bị filter hay không).

## Tổng Quan

**Toàn bộ log** trong ứng dụng đều đi qua **logrus** và **log filter**, theo **format chung** (text hoặc json từ `LOG_FORMAT`):

- **logrus** (AppLogger, GetLogger, GetJobLoggerByName, .Info, .Debug, .Warn, .Error): đi trực tiếp qua filter và format chung.
- **Standard log** (`log.Printf`, `log.Println`): được chuyển qua **StdLogBridge** → logrus với `logger_name=stdlog`, nên cũng đi qua filter và format chung.
- **Format chung**: timestamp, level, message, caller (nếu bật), các field (logger_name, job_name, …). Text: `2006-01-02 15:04:05.000`; JSON: `timestamp`, `level`, `message`, `caller`.

Hệ thống filter cho phép bật/tắt log theo:
- **Agent ID**: Filter log theo agent cụ thể
- **Job Name / Logger Name**: Filter log theo job hoặc logger (ví dụ: `workflow-commands-job`, `stdlog`, `app`)
- **Log Level**: Filter log theo mức độ (debug, info, warn, error, fatal)
- **Log Method**: Filter log theo phương thức (console, file)

## Cấu Hình

### 1. Tạo File Config

Tạo file `config/log-filter.json` (hoặc copy từ `config/log-filter.json.example`):

```json
{
  "enabled": true,
  "default_action": "allow",
  "agents": {
    "*": true
  },
  "jobs": {
    "*": true,
    "sync-incremental-conversations-job": false
  },
  "log_levels": {
    "*": true,
    "debug": false,
    "info": true,
    "warn": true,
    "error": true,
    "fatal": true
  },
  "log_methods": {
    "*": true,
    "console": true,
    "file": true
  },
  "rules": []
}
```

### 2. Cấu Trúc Config

#### enabled (boolean)
- `true`: Bật hệ thống filter
- `false`: Tắt hệ thống filter (tất cả log đều được ghi)

#### default_action (string)
- `"allow"`: Cho phép log mặc định (nếu không match rule nào)
- `"deny"`: Chặn log mặc định (nếu không match rule nào)

#### agents (map[string]bool)
- Key: Agent ID hoặc `"*"` (tất cả agents)
- Value: `true` = cho phép log, `false` = chặn log

#### jobs (map[string]bool)
- Key: Job name hoặc `"*"` (tất cả jobs)
- Value: `true` = cho phép log, `false` = chặn log

#### log_levels (map[string]bool)
- Key: Log level (`"debug"`, `"info"`, `"warn"`, `"error"`, `"fatal"`) hoặc `"*"` (tất cả)
- Value: `true` = cho phép log, `false` = chặn log

#### log_methods (map[string]bool)
- Key: `"console"`, `"file"`, hoặc `"*"` (tất cả)
- Value: `true` = cho phép log, `false` = chặn log

#### rules (array)
- Mảng các rule phức tạp (kết hợp nhiều điều kiện)
- Mỗi rule có các trường:
  - `name`: Tên rule (để dễ quản lý)
  - `enabled`: Bật/tắt rule này
  - `agent`: Agent ID hoặc `"*"` (tất cả)
  - `job`: Job name hoặc `"*"` (tất cả)
  - `log_level`: Log level hoặc `"*"` (tất cả)
  - `log_method`: `"console"`, `"file"`, hoặc `"*"` (tất cả)
  - `action`: `"allow"` hoặc `"deny"`
  - `priority`: Độ ưu tiên (số càng cao càng ưu tiên, mặc định: 0)

## Ví Dụ Sử Dụng

### Ví Dụ 1: Chặn tất cả debug log

```json
{
  "enabled": true,
  "default_action": "allow",
  "log_levels": {
    "*": true,
    "debug": false
  }
}
```

### Ví Dụ 2: Chặn log của một job cụ thể

```json
{
  "enabled": true,
  "default_action": "allow",
  "jobs": {
    "*": true,
    "sync-incremental-conversations-job": false
  }
}
```

### Ví Dụ 3: Chỉ log error và fatal của một agent

```json
{
  "enabled": true,
  "default_action": "deny",
  "agents": {
    "693ed5a948235615e21a5522": true
  },
  "log_levels": {
    "error": true,
    "fatal": true
  },
  "rules": [
    {
      "name": "Chỉ log error và fatal của agent cụ thể",
      "enabled": true,
      "agent": "693ed5a948235615e21a5522",
      "job": "*",
      "log_level": "error",
      "log_method": "*",
      "action": "allow",
      "priority": 20
    },
    {
      "name": "Chỉ log error và fatal của agent cụ thể (fatal)",
      "enabled": true,
      "agent": "693ed5a948235615e21a5522",
      "job": "*",
      "log_level": "fatal",
      "log_method": "*",
      "action": "allow",
      "priority": 20
    }
  ]
}
```

### Ví Dụ 4: Chặn console log nhưng vẫn log ra file

```json
{
  "enabled": true,
  "default_action": "allow",
  "log_methods": {
    "*": true,
    "console": false,
    "file": true
  }
}
```

### Ví Dụ 5: Sử dụng rules phức tạp

```json
{
  "enabled": true,
  "default_action": "allow",
  "rules": [
    {
      "name": "Chặn debug log của job cụ thể",
      "enabled": true,
      "agent": "*",
      "job": "sync-incremental-conversations-job",
      "log_level": "debug",
      "log_method": "*",
      "action": "deny",
      "priority": 10
    },
    {
      "name": "Chỉ log error của agent cụ thể",
      "enabled": true,
      "agent": "693ed5a948235615e21a5522",
      "job": "*",
      "log_level": "error",
      "log_method": "*",
      "action": "allow",
      "priority": 20
    }
  ]
}
```

## Thứ Tự Ưu Tiên

1. **Rules** (theo priority, cao -> thấp): Nếu rule match, sẽ áp dụng action của rule
2. **Filters đơn giản** (agents, jobs, log_levels, log_methods): Nếu không có rule nào match
3. **Default action**: Nếu không match rule nào và không bị chặn bởi filter đơn giản

## Đường Dẫn Config

Hệ thống sẽ tìm file config theo thứ tự ưu tiên:

1. Biến môi trường `LOG_FILTER_CONFIG_PATH`
2. `./config/log-filter.json`
3. `{rootDir}/config/log-filter.json`

Nếu file không tồn tại, hệ thống sẽ tạo file mặc định (tất cả log đều được ghi).

## Reload Config

Để reload config mà không cần restart ứng dụng, sử dụng:

```go
import "agent_pancake/utility/logger"

err := logger.ReloadLogFilterConfig()
if err != nil {
    // Xử lý lỗi
}
```

## Lưu Ý

1. **Wildcard `"*"`**: Có thể sử dụng `"*"` để match tất cả agents, jobs, log levels, hoặc log methods
2. **Priority**: Rules có priority cao hơn sẽ được kiểm tra trước
3. **Performance**: Filter được thực hiện trong hook, không ảnh hưởng nhiều đến performance
4. **Job Name**: Job name phải khớp chính xác với tên job trong code (ví dụ: `"sync-incremental-conversations-job"`)
5. **Agent ID**: Agent ID phải khớp chính xác với `AGENT_ID` trong config

## Debug

Để debug filter config, kiểm tra log entry có field `__filtered__`:

```go
if filtered, ok := entry.Data["__filtered__"].(bool); ok && filtered {
    // Log đã bị filter
}
```

## Ví Dụ Job Names

Các job names phổ biến trong hệ thống:

- `sync-incremental-conversations-job`
- `sync-backfill-conversations-job`
- `sync-verify-conversations-job`
- `sync-full-recovery-conversations-job`
- `sync-incremental-posts-job`
- `sync-backfill-posts-job`
- `sync-incremental-customers-job`
- `sync-backfill-customers-job`
- `sync-incremental-pancake-pos-customers-job`
- `sync-backfill-pancake-pos-customers-job`
- `sync-incremental-pancake-pos-orders-job`
- `sync-backfill-pancake-pos-orders-job`
- `sync-pancake-pos-products-job`
- `sync-pancake-pos-shops-warehouses-job`
- `workflow-commands-job`
- `check-in-job`
