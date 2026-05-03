# Chiu-PC-server — 湫创·灵境 AI 后端服务

## 项目简介

Go 后端服务，为 "湫创·灵境 AI" 平台提供认证授权、任务调度、设备管理、WebSocket 实时通信和数据持久化。作为中央调度器接收 PC 客户端的去重任务，通过 WebSocket 下发到指定设备执行，并汇总进度和结果。

## 技术栈

| 技术 | 用途 |
|------|------|
| Go 1.25 | 后端语言 |
| Gin (v1.11.0) | HTTP 框架 |
| GORM (v1.31.0) + MySQL | ORM + 数据库 |
| go-redis (v8) | Redis 缓存 |
| gorilla/websocket (v1.5.3) | WebSocket |
| golang-jwt (v5) | JWT 鉴权 |
| bcrypt (golang.org/x/crypto) | 密码哈希 |
| swaggo (swag + gin-swagger) | API 文档 |
| YAML (gopkg.in/yaml.v3) | 配置管理 |

## 目录结构

```
Chiu-PC-server/
├── main.go                      # 入口：初始化配置/DB/Redis/WS/Scheduler → 启动 HTTP
├── go.mod / go.sum              # 模块: ffmpegserver
│
├── config/
│   ├── global.go                # GlobalConfig 结构体（Server/Ws/Task/Mysql/Redis/JWT/Log/ApiDoc）
│   ├── init.go                  # 根据 ENV 加载对应 YAML 配置
│   ├── config.yaml              # 默认配置
│   ├── config.dev.yaml          # 开发环境配置
│   └── config.prod.yaml         # 生产环境配置
│
├── API/                         # HTTP API 层
│   ├── init.go                  # Gin 路由注册（所有端点挂载到 /api 组）
│   │
│   ├── middleware/
│   │   ├── cors.go              # 全源 CORS 中间件
│   │   ├── auth.go              # JWT Bearer Token 鉴权（含白名单路径）
│   │   └── hmac.go              # HMAC-SHA256 请求体签名验证
│   │
│   ├── login/
│   │   ├── login.go             # POST /api/auth/login, /refresh, /check
│   │   └── avatar.go            # 默认头像 PNG 生成（22 个）
│   │
│   ├── device/
│   │   └── device.go            # GET/PUT/DELETE /api/devices
│   │
│   ├── dashboard/
│   │   └── dashboard.go         # GET /api/dashboard（概览/日统计/设备统计/最近任务）
│   │
│   └── video_dedup/
│       └── task.go              # POST/GET/DELETE /api/video-dedup/tasks, /stats
│
├── types/                       # 请求/响应数据定义
│   └── login/
│       └── login.go             # PostLogin, LoginResponse, UserInfo 等
│
├── model/                       # GORM 数据模型
│   ├── user.go                  # 用户表 users
│   ├── pc_device.go             # 设备表 pc_devices
│   ├── video_dedup_task.go      # 任务表 video_dedup_tasks
│   └── task_daily_stat.go       # 日统计表 task_daily_stats
│
├── service/                     # 业务逻辑层
│   ├── device/
│   │   └── device.go            # 设备连接/状态回调
│   │
│   └── video_dedup/
│       ├── task.go              # 任务创建：XOR 解密 → AES 重加密 → 入库
│       ├── command.go           # AES-256-CBC 加解密（EncryptCommand）
│       ├── scheduler.go         # 定时调度器：每 5s 轮询等待任务 → WS 下发
│       └── stats.go             # 统计服务（占位）
│
│   └── ws/                      # WebSocket 子系统
│       ├── server.go            # WS HTTP 服务器（独立端口 9903），认证升级握手
│       ├── hub.go               # 连接管理器（按 UserID 分组），消息广播
│       ├── client.go            # WS 客户端连接封装
│       ├── router.go            # 消息路由：progress/complete/error/log
│       └── registry.go          # 消息处理器注册模式
│
├── public/                      # 基础设施
│   ├── sql/
│   │   ├── sql.go               # MySQL 连接（GORM）
│   │   └── migrate.go           # 自动迁移（建表/加列/删冗余/删多余表）
│   └── redis/
│       └── redis.go             # Redis 客户端封装
│
├── utils/                       # 工具函数
│   ├── crypto.go                # bcrypt + JWT（access/refresh token 生成与验证）
│   ├── hmac.go                  # HMAC-SHA256 签名与验证
│   └── transport.go             # XOR 解密（DecryptTransport，用于接收客户端加密命令）
│
├── docs/                        # Swagger 自动生成文档
│
├── public/                      # 静态资源
│   └── avatar/                  # 默认头像图片（启动时生成）
│
└── bin/                         # 构建输出
```

## API 接口规范

### REST API（HTTP :9902）

所有 API 以 `/api` 为前缀，经过 `CORS → HMAC → JWT` 中间件链。

#### 公开端点（无需鉴权）

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/health` | 健康检查 |
| `POST` | `/api/auth/login` | 登录/自动注册 |
| `POST` | `/api/auth/refresh` | 刷新 access_token |
| `POST` | `/api/auth/check` | 验证 token（可选鉴权） |
| `GET` | `/swagger/*` | Swagger API 文档（开发/默认环境） |
| `GET` | `/avatar/*` | 静态头像文件 |

#### 需鉴权端点（JWT Bearer Token）

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/dashboard` | 仪表盘概览（?range=7d\|30d\|all） |
| `GET` | `/api/devices` | 设备列表 |
| `PUT` | `/api/devices/:id` | 修改设备名 `{"device_name": "..."}` |
| `DELETE` | `/api/devices/:id` | 删除设备 |
| `POST` | `/api/video-dedup/tasks` | 创建任务 `{"pc_code","tasks":[...],"output_dir"}` |
| `GET` | `/api/video-dedup/tasks` | 任务列表（分页 + 状态/设备过滤） |
| `GET` | `/api/video-dedup/tasks/stats` | 任务统计 |
| `GET` | `/api/video-dedup/tasks/:id` | 任务详情 |
| `DELETE` | `/api/video-dedup/tasks` | 批量软删除 `{"task_ids": [int64]}` |

### WebSocket API（:9903）

**连接流程：** HTTP 升级 → 5s 内发送 `auth` → 认证成功后持续通信

#### 客户端 → 服务端

| 消息类型 | 载荷 | 说明 |
|----------|------|------|
| `auth` | `{"token","pc_code"}` | JWT 认证（必须 5s 内发送） |
| `ping` | — | 心跳 |
| `dedup_progress` | `{task_id,stage,percent,frame,speed}` | 任务进度 |
| `dedup_complete` | `{task_id,output_path}` | 任务完成 |
| `dedup_error` | `{task_id,error}` | 任务错误 |
| `dedup_log` | `{task_id,line}` | 实时日志 |

#### 服务端 → 客户端

| 消息类型 | 载荷 | 说明 |
|----------|------|------|
| `auth_success` | — | 认证成功 |
| `auth_fail` | `{"error"}` | 认证失败 |
| `dedup_execute` | `{task_id,encrypted_arg,trf_name,output_dir}` | 下发任务执行 |
| `dedup_progress/complete/error/log` | 同上 | 广播给同一用户的其他设备 |

## 中间件链（执行顺序）

```
1. CORS → 允许所有来源，暴露 X-Signature 头
2. HMAC → 验证 POST/PUT/DELETE 的 X-Signature（白名单放行 auth/health/swagger/avatar）
3. JWT → 解析 Bearer Token，注入 user_id（白名单同上）
```

## 数据模型

### users（用户）
| 字段 | 类型 | 说明 |
|------|------|------|
| id | int32 PK | 主键 |
| account | varchar(50) UNIQUE | 账号 |
| password | varchar(255) | bcrypt 哈希（JSON 隐藏） |
| nick_name | varchar(50) | 昵称 |
| avatar | varchar(255) | 头像路径 |
| role | int | 0=用户, 66=管理员, 888=超级管理员 |
| login_ip / login_time | varchar(45) / bigint | 最后登录信息 |
| creation_time / update_time | bigint | 时间戳 |

### pc_devices（设备）
| 字段 | 类型 | 说明 |
|------|------|------|
| id | bigint PK | 主键 |
| user_id | int INDEX | 所属用户 |
| pc_code | varchar(64) UNIQUE | 设备硬件指纹 |
| device_name | varchar(128) | 用户自定义名称 |
| ip | varchar(45) | 设备 IP |
| is_current | tinyint(1) | 当前活动设备标记 |
| last_active | bigint | 最后活跃时间 |

### video_dedup_tasks（去重任务）
| 字段 | 类型 | 说明 |
|------|------|------|
| id | bigint PK | 主键 |
| user_id | int INDEX | 所属用户 |
| pc_code | varchar(64) INDEX | 目标设备 |
| input_file_path | text | 源视频路径 |
| output_dir / output_path | varchar | 输出目录/路径 |
| encrypted_arg | text | AES-256-CBC 加密参数（JSON 隐藏） |
| trf_name | varchar(128) | vidstab 变换文件名 |
| status | int | 0=等待, 1=运行中, 2=完成, 3=错误, 4=已取消 |
| progress | int | 0-100 |
| stage | varchar(64) | 当前处理阶段 |
| concurrent_lock | tinyint(1) | 并发锁定标记 |
| error_msg | text | 错误信息 |
| deleted_at | timestamp | 软删除时间 |

### task_daily_stats（日统计）
| 字段 | 类型 | 说明 |
|------|------|------|
| id | bigint PK | 主键 |
| user_id | int INDEX | 用户（与 date 组合索引） |
| date | varchar(10) | YYYY-MM-DD |
| total/completed/failed/running/waiting/cancelled | bigint | 各项计数 |

## 命名规范

- **包结构**: API handlers 按模块分包（`login/`, `device/`, `dashboard/`, `video_dedup/`）
- **文件命名**: 见名知义，小写蛇形（如 `video_dedup_task.go`, `pc_device.go`）
- **API handler 函数**: `GetXxx`, `PostXxx`, `PutXxx`, `DeleteXxx`
- **Service 函数**: `CreateXxx`, `GetXxx`, `HandleXxx`
- **Model 结构体**: `PascalCase` 单数（如 `VideoDedupTask`, `PcDevice`）
- **JSON 字段**: `snake_case`（如 `pc_code`, `access_token`）
- **配置文件**: `config.{env}.yaml`

## 与 ffmpeg_go 的交互

Chiu-PC-server 作为服务端，ffmpeg_go 作为 PC 客户端，交互如下：

1. **ffmpeg_go 发起认证** → `POST /api/auth/login` → 获取 JWT token
2. **ffmpeg_go 连接 WS** → 发送 `auth` 消息 → 服务器注册设备在线
3. **ffmpeg_go 创建任务** → `POST /api/video-dedup/tasks`（初次 XOR 加密）→ 服务端重加密为 AES 存储
4. **调度器下发** → 每 5s 轮询 → 向目标设备 WS 发送 `dedup_execute`
5. **ffmpeg_go 执行** → 解密 AES 命令 → 运行 FFmpeg → 上报进度/完成/错误
6. **前端查看** → 同一用户的其他 Web 客户端通过 WS 广播接收实时进度

## 开发注意事项

1. **启动**: `ENV=dev go run main.go`（默认端口 9902，WS 端口 9903）
2. **编译**: `go build -o bin/chiu_pc_server.exe`
3. **配置管理**: 通过 `ENV` 环境变量切换配置，三个 YAML 文件共享相同的 MySQL/Redis 远程连接
4. **数据库**: MySQL 连接 `193.218.201.251`，自动迁移在启动时运行
5. **任务调度**: 默认每 PC 并发 2 个任务，`concurrent_lock=true` 的任务会阻塞所有其他任务
6. **安全**: HMAC 签名 + JWT 双层校验，传输层 XOR + AES-256-CBC 两级加密
7. **修改 API 端点**：需同步更新 ffmpeg_go 和前端 Web 应用的调用代码
8. **WS 消息格式**：增减消息类型时需同步更新 `service/ws/` 下的 router 和 registry
9. **Swagger 文档**：通过 `swaggo` 自动生成，修改 API 后运行 `swag init`
