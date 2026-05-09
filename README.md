# Heap Alarm

监控指定进程的内存占用，当超过设定阈值时通过邮件发送告警通知。支持 Windows 服务模式（后台持续运行）和控制台模式（前台调试）。

## 功能特性

- 按进程名匹配并监控内存占用（RSS/Working Set）
- 支持多个同名进程独立监控
- 邮件告警（SMTP，支持 SMTPS 端口 465 和 STARTTLS 端口 587）
- 告警冷却机制，避免重复通知
- 可注册为 Windows 服务，后台持续运行
- 进程恢复正常后自动重置冷却状态

## 快速开始

### 1. 下载或编译

```bash
# 克隆仓库
git clone https://github.com/814742394/heap_alarm.git
cd heap_alarm

# 编译（Windows 目标）
GOOS=windows GOARCH=amd64 go build -o heap_alarm.exe .
```

### 2. 配置文件

在可执行文件同目录下创建 `config.toml`：

```toml
# 要监控的进程名称
process_name = "java.exe"

# 内存告警阈值，单位：MB
memory_threshold_mb = 2000

# 检查间隔，单位：秒
check_interval_seconds = 60

# 告警冷却时间，单位：秒
alert_cooldown_seconds = 3600

# 日志文件路径（留空则只输出到控制台）
log_file = "heap_alarm.log"

# 运行模式：console（控制台）或 service（Windows 服务）
service_mode = "console"

[smtp]
  host = "smtp.qq.com"
  port = 465
  username = "your-email@qq.com"
  password = "your-auth-code"
  from = "your-email@qq.com"
  to = ["admin@example.com"]
  use_tls = true
```

**配置项说明：**

| 字段 | 说明 |
|---|---|
| `process_name` | 要监控的进程名（不区分大小写，`.exe` 后缀可选） |
| `memory_threshold_mb` | 内存告警阈值（MB） |
| `check_interval_seconds` | 每次检查的间隔（秒） |
| `alert_cooldown_seconds` | 告警冷却时间，首次告警后在此时间内不重复发送 |
| `log_file` | 日志文件路径，为空则只输出到控制台 |
| `service_mode` | `console`（控制台）或 `service`（Windows 服务） |
| `smtp.host` | SMTP 服务器地址 |
| `smtp.port` | SMTP 端口：465 为 SMTPS（隐式 TLS），587 为 STARTTLS |
| `smtp.username` | SMTP 用户名 |
| `smtp.password` | SMTP 密码（QQ 邮箱使用授权码） |
| `smtp.from` | 发件人邮箱地址 |
| `smtp.to` | 收件人邮箱地址列表（支持多个） |
| `smtp.use_tls` | 是否使用 TLS，端口 465 为 true，587 为 false |

### 3. 运行

**控制台模式（前台运行）：**

```bash
# 默认加载当前目录下的 config.toml
heap_alarm.exe

# 指定配置文件
heap_alarm.exe D:\config\myapp.toml
```

**Windows 服务模式（后台运行）：**

```bash
# 安装服务
heap_alarm.exe install

# 启动服务
heap_alarm.exe start

# 查看状态
heap_alarm.exe status

# 重启服务
heap_alarm.exe restart

# 停止服务
heap_alarm.exe stop

# 卸载服务
heap_alarm.exe remove
```

> 所有服务管理命令需以 **管理员身份** 运行。

## 输出示例

```
2024/01/15 10:00:00 main.go:53: out_heap_alarm_go starting...
2024/01/15 10:00:00 main.go:54: Config loaded: monitoring process "java.exe", threshold 2000 MB, check every 60s
2024/01/15 10:01:00 main.go:155: OK: java.exe (PID 1234) memory 1024.5 MB (threshold: 2000 MB)
2024/01/15 10:02:00 main.go:128: ALERT: java.exe (PID 1234) memory 2048.0 MB exceeds threshold 2000 MB
2024/01/15 10:02:00 main.go:152: Alert email sent to [admin@example.com]
```

邮件告警内容示例：

```
Time: 2024-01-15 10:02:00
Host: SRV-APP-01
Process: java.exe (PID 1234)
Memory: 2048.0 MB
Threshold: 2000 MB
```

## 项目结构

```
heap_alarm/
├── main.go                  # 主入口，配置加载，控制台运行循环
├── main_windows.go          # Windows 服务管理命令（install/start/stop 等）
├── main_windows_stub.go     # 非 Windows 平台桩代码
├── config.toml              # 配置文件
├── config/
│   └── config.go            # TOML 配置加载与校验
├── monitor/
│   └── monitor.go           # 进程发现与内存信息获取
├── alert/
│   └── alert.go             # SMTP 邮件发送与告警冷却
└── service/
    └── service.go           # Windows 服务实现
```

## 跨平台说明

- **Windows**: 支持控制台和 Windows 服务两种运行模式
- **macOS/Linux**: 仅支持控制台模式（service 相关的桩代码返回错误）

## 告警冷却机制

- 当进程内存超过阈值时发送邮件，之后进入冷却期
- 冷却期内即使仍超阈值也不会重复发送邮件
- 当进程内存回落到阈值以下时自动重置冷却状态
- 多进程场景下每个 PID 拥有独立的冷却计时器
