# CoreFusion Agent 编译与部署指南

## 1. 环境要求

| 组件 | 版本要求 |
|------|---------|
| Go | >= 1.26.2 |
| Node.js | >= 18 |
| npm | >= 9 |
| SQLite | >= 3.35 (支持 ON CONFLICT) |
| OS | Linux (amd64 / arm64) |

---

## 2. 项目结构

```
corefusion/
├── cmd/
│   ├── corefusion/          # 面板后端主程序
│   │   └── main.go
│   └── agent/               # Agent 远程节点守护进程
│       └── main.go
├── internal/
│   ├── app/embed/           # 前端嵌入 (go:embed)
│   │   ├── embed.go         # //go:embed all:dist — 将前端打包进 Go 二进制
│   │   └── dist/            # 前端构建产物（编译前需从 web/admin/dist 复制）
│   ├── runtime/agent/       # Agent 核心逻辑
│   │   ├── proto/           # gRPC proto 定义 (AgentService, HeartbeatService)
│   │   ├── manager.go       # NodeServerManager (面板侧 gRPC 服务端 + 反向拨号客户端)
│   │   ├── local.go         # LocalAgent (本地节点适配器)
│   │   ├── ca.go            # mTLS CA 管理
│   │   └── install_script.go # Agent 安装脚本生成器
│   └── ...
├── web/
│   ├── admin/               # 管理面板前端 (Vue3 + Element Plus)
│   └── portal/              # 客户端前端 (预编译)
├── data/                    # 运行时数据目录
│   ├── sqlite/              # SQLite 数据库 (app.db)
│   ├── binaries/            # 核心二进制 (xray, singbox, ...)
│   ├── certs/               # mTLS 证书
│   ├── runtime/             # 核心配置文件 (xray.jsonc, ...)
│   └── logs/                # 日志
├── go.mod
└── .env                     # 环境配置 (SAFE_PATHS=data 目录路径)
```

---

## 3. 面板编译与启动（前后端一体）

> **重要**：前端通过 `//go:embed` 嵌入到 Go 二进制中（`internal/app/embed/embed.go`），
> 因此编译顺序必须是：**前端构建 → 复制到 embed 目录 → Go 编译**。

### 3.1 一键编译（前端 + 后端）

```bash
cd /opt/corefusion

# 1. 构建前端
cd web/admin && npm run build && cd ../..

# 2. 复制前端产物到 Go embed 目录
rm -rf internal/app/embed/dist/*
cp -r web/admin/dist/* internal/app/embed/dist/
cp -r web/portal internal/app/embed/dist/

# 3. 编译面板后端（前端已嵌入二进制）
go build -o /tmp/corefusion ./cmd/corefusion/
```

编译产物：`/tmp/corefusion`（包含前端 + 后端的单体二进制）

### 3.2 配置文件 (.env)

```bash
# /opt/corefusion/.env
SAFE_PATHS=/opt/corefusion/data
```

| 字段 | 说明 | 默认值 |
|------|------|--------|
| `SAFE_PATHS` | 数据目录（SQLite、证书、二进制、日志） | `data` (相对路径) |

### 3.3 启动

```bash
cd /opt/corefusion

# 前台启动（开发调试）
/tmp/corefusion server

# 后台启动（生产环境）
nohup /tmp/corefusion server > /tmp/corefusion.log 2>&1 &
```

### 3.4 默认监听端口

| 端口 | 用途 |
|------|------|
| `8080` | HTTP API + 管理面板前端 |
| `8081` | 订阅服务（可选） |
| `18443` | Agent gRPC 服务端（mTLS，接收 Agent 连接） |

### 3.5 默认管理员账号

面板首次启动时自动创建：

| 字段 | 值 |
|------|-----|
| 用户名 | `admin` |
| 密码 | `admin123` |

> 登录后请立即修改密码。

### 3.6 验证面板启动

```bash
# 检查进程
ps aux | grep corefusion | grep -v grep

# 检查端口
ss -tlnp | grep -E '8080|18443'

# 测试 API
curl -s http://127.0.0.1:8080/api/v1/auth/login \
  -X POST \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}' | jq .

# 预期输出包含 access_token
```

---

## 4. Agent 编译与部署

### 4.1 编译

```bash
cd /opt/corefusion

# 编译 Agent 二进制
go build -o data/binaries/active/corefusion-agent/corefusion-agent ./cmd/agent/
```

编译产物：`data/binaries/active/corefusion-agent/corefusion-agent`

### 4.2 交叉编译（远程节点架构不同时）

```bash
# 目标 Linux ARM64
CGO_ENABLED=1 GOOS=linux GOARCH=arm64 \
  go build -o corefusion-agent-linux-arm64 ./cmd/agent/

# 目标 Linux AMD64
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
  go build -o corefusion-agent-linux-amd64 ./cmd/agent/
```

> 注意：Agent 使用 SQLite (CGO)，交叉编译时需设置 `CGO_ENABLED=0` 使用 pure Go 驱动。
> 如果目标架构与编译机相同，直接 `go build` 即可（保留 CGO）。

### 4.3 Agent 配置文件

在远程节点上创建 `/opt/corefusion-agent/config.yaml`：

```yaml
# /opt/corefusion-agent/config.yaml

panel:
  host: "127.0.0.1"       # 面板 IP
  port: 8080                   # 面板 HTTP 端口 (用于 Bootstrap REST API)
  grpc_port: 18443             # 面板 gRPC 端口 (用于 mTLS 心跳)
  server_id: 2                 # 面板中注册的节点服务器 ID

bootstrap:
  token: "<在面板节点服务器页面生成的 Bootstrap Token>"

agent:
  listen_port: 18443           # Agent gRPC 服务端端口 (面板反向拨号连接此端口)
  log_level: info
  data_dir: "/opt/corefusion-agent/data"
```

### 4.4 在面板上添加远程节点

1. 登录面板 `http://<面板IP>:8080`
2. 用户名 `admin`，密码 `admin123`
3. 进入 **节点服务器** 页面
4. 点击 **添加服务器**
5. 填写服务器名称、IP 地址（远程节点内网 IP）
6. 点击 **生成 Bootstrap Token** — 复制 Token 填入 Agent 配置文件

### 4.5 部署 Agent 到远程节点

#### 方式一：手动部署（推荐开发调试）

```bash
# 1. 在面板机上编译 Agent
cd /opt/corefusion
go build -o data/binaries/active/corefusion-agent/corefusion-agent ./cmd/agent/

# 2. 创建远程节点目录
ssh root@<REMOTE_IP> "mkdir -p /opt/corefusion-agent/data"

# 3. 上传 Agent 二进制
scp data/binaries/active/corefusion-agent/corefusion-agent \
  root@<REMOTE_IP>:/opt/corefusion-agent/corefusion-agent

# 4. 上传配置文件（先在远程节点创建 config.yaml，内容见 4.3）
#    或者直接用 SSH 写入
ssh root@<REMOTE_IP> "cat > /opt/corefusion-agent/config.yaml << 'EOF'
panel:
  host: \"<PANEL_IP>\"
  port: 8080
  grpc_port: 18443
  server_id: 2
bootstrap:
  token: \"<BOOTSTRAP_TOKEN>\"
agent:
  listen_port: 18443
  log_level: info
  data_dir: \"/opt/corefusion-agent/data\"
EOF"

# 5. 启动 Agent
ssh root@<REMOTE_IP> "cd /opt/corefusion-agent && \
  nohup ./corefusion-agent -config config.yaml > /tmp/agent.log 2>&1 &"

# 6. 检查日志
ssh root@<REMOTE_IP> "tail -20 /tmp/agent.log"
```

#### 方式二：systemd 服务（推荐生产环境）

在远程节点创建 `/etc/systemd/system/corefusion-agent.service`：

```ini
[Unit]
Description=CoreFusion Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/opt/corefusion-agent/corefusion-agent -config /opt/corefusion-agent/config.yaml
Restart=on-failure
RestartSec=5
LimitNOFILE=65535
WorkingDirectory=/opt/corefusion-agent

[Install]
WantedBy=multi-user.target
```

```bash
systemctl daemon-reload
systemctl enable corefusion-agent
systemctl start corefusion-agent

# 查看状态
systemctl status corefusion-agent

# 查看日志
journalctl -u corefusion-agent -f
```

### 4.6 Agent 启动日志（正常输出示例）

```
2026/07/02 10:42:28 [Agent] version=1.0.0-dev, server_id=2, panel=127.0.0.1:18443, data_dir=/opt/corefusion-agent/data
2026/07/02 10:42:28 database initialized at /opt/corefusion-agent/data/sqlite/agent.db (WAL mode)
2026/07/02 10:42:28 [Agent] using existing mTLS certificate
2026/07/02 10:42:28 [Agent] process.Manager started (Guardian auto-restart active)
2026/07/02 10:42:28 [Agent] gRPC server listening on :18443 (mTLS)
2026/07/02 10:42:28 [Agent] connected to panel, heartbeat stream active
2026/07/02 10:42:29 reconcile: done, 0 processes managed
```

关键日志含义：

| 日志 | 说明 |
|------|------|
| `using existing mTLS certificate` | Bootstrap 成功，已持有有效 mTLS 证书 |
| `process.Manager started (Guardian auto-restart active)` | 本地进程守护已启动（核心崩溃自动重启） |
| `gRPC server listening on :18443 (mTLS)` | Agent gRPC 服务端已启动，等待面板反向拨号 |
| `connected to panel, heartbeat stream active` | 已连接面板，双向心跳流已建立 |

### 4.7 面板侧验证日志

面板启动后，Agent 连接成功时日志输出：

```
[NodeServerManager] server 2 (1.0.0-dev) connected
[NodeServerManager] reverse-dial to agent 192.168.2.50:18443 (server 2) established
```

| 日志 | 说明 |
|------|------|
| `server N connected` | Agent 心跳流已接入面板 |
| `reverse-dial to agent ... established` | 面板已反向拨号到 Agent gRPC 服务端，可下发指令 |

### 4.8 API 验证

```bash
# 获取 Token
TOKEN=$(curl -s http://127.0.0.1:8080/api/v1/auth/login \
  -X POST \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.data.access_token')

# 查看节点服务器列表（确认 agent_status=online）
curl -s http://127.0.0.1:8080/api/v1/node-servers \
  -H "Authorization: Bearer $TOKEN" | jq '.data.items[] | {id, name, hostname, agent_status, agent_version}'

# 查看远程节点核心状态
curl -s http://127.0.0.1:8080/api/v1/node-servers/2/status \
  -H "Authorization: Bearer $TOKEN" | jq .

# 查看远程节点已安装二进制
curl -s http://127.0.0.1:8080/api/v1/node-servers/2/binaries \
  -H "Authorization: Bearer $TOKEN" | jq .
```

---

## 5. 前端开发与构建

### 5.1 开发模式（热更新，不需编译后端）

```bash
cd /opt/corefusion/web/admin

# 安装依赖
npm install

# 启动开发服务器（端口 3000，自动代理 /api 到 localhost:8080）
npm run dev
```

访问 `http://localhost:3000`，使用 `admin` / `admin123` 登录。

`vite.config.js` 已配置代理：
```
/api → http://localhost:8080
```

> 开发模式下前端独立运行，面板后端只需单独启动（`/tmp/corefusion server`）。

### 5.2 生产构建（嵌入到 Go 二进制）

前端通过 `//go:embed all:dist`（`internal/app/embed/embed.go`）嵌入到面板 Go 二进制中。
构建步骤见 [3.1 一键编译](#31-一键编译前端--后端)，核心流程：

```bash
# 1. 构建前端产物
cd /opt/corefusion/web/admin && npm run build && cd ../..

# 2. 复制到 Go embed 目录（Go 编译时会读取此目录）
rm -rf internal/app/embed/dist/*
cp -r web/admin/dist/* internal/app/embed/dist/
cp -r web/portal internal/app/embed/dist/

# 3. 编译 Go 二进制（前端已嵌入）
go build -o /tmp/corefusion ./cmd/corefusion/
```

> 前端代码修改后，必须重新执行以上三步才能生效。
> 仅修改后端 Go 代码时，只需执行第 3 步。

---

## 6. 完整启动流程（从零开始）

### 6.1 面板侧

```bash
# 1. 构建前端
cd /opt/corefusion/web/admin && npm install && npm run build && cd ../..

# 2. 复制前端到 Go embed 目录
rm -rf internal/app/embed/dist/*
cp -r web/admin/dist/* internal/app/embed/dist/
cp -r web/portal internal/app/embed/dist/

# 3. 编译面板后端（前端已嵌入）
go build -o /tmp/corefusion ./cmd/corefusion/

# 4. 启动面板
cd /opt/corefusion && /tmp/corefusion server

# 5. 验证
curl -s http://127.0.0.1:8080/api/v1/auth/login \
  -X POST -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.data.access_token'
```

### 6.2 远程节点侧

```bash
# 1. 在面板 Web UI 添加节点服务器，获取 Bootstrap Token

# 2. 在面板机上编译 Agent
cd /opt/corefusion
go build -o data/binaries/active/corefusion-agent/corefusion-agent ./cmd/agent/

# 3. 部署到远程节点
ssh root@<REMOTE_IP> "mkdir -p /opt/corefusion-agent/data"
scp data/binaries/active/corefusion-agent/corefusion-agent \
  root@<REMOTE_IP>:/opt/corefusion-agent/corefusion-agent

# 4. 创建配置文件（替换 <PANEL_IP> 和 <TOKEN>）
ssh root@<REMOTE_IP> "cat > /opt/corefusion-agent/config.yaml << 'EOF'
panel:
  host: \"<PANEL_IP>\"
  port: 8080
  grpc_port: 18443
  server_id: 2
bootstrap:
  token: \"<TOKEN>\"
agent:
  listen_port: 18443
  log_level: info
  data_dir: \"/opt/corefusion-agent/data\"
EOF"

# 5. 启动 Agent
ssh root@<REMOTE_IP> "cd /opt/corefusion-agent && chmod +x corefusion-agent && \
  nohup ./corefusion-agent -config config.yaml > /tmp/agent.log 2>&1 &"

# 6. 验证连接
ssh root@<REMOTE_IP> "tail -10 /tmp/agent.log"
# 应看到 "connected to panel, heartbeat stream active"
```

### 6.3 验证面板侧

```bash
# 检查节点在线状态
curl -s http://127.0.0.1:8080/api/v1/node-servers \
  -H "Authorization: Bearer $(curl -s http://127.0.0.1:8080/api/v1/auth/login \
    -X POST -H 'Content-Type: application/json' \
    -d '{"username":"admin","password":"admin123"}' | jq -r '.data.access_token')" \
  | jq '.data.items[] | {id, name, agent_status}'
```

预期输出：
```json
{"id": 1, "agent_status": "online"}
{"id": 2, "agent_status": "online"}
```

---

## 7. 日常更新流程

当修改了 Agent 或面板代码后，需要重新编译并部署。

### 7.1 更新面板（前端 + 后端）

```bash
cd /opt/corefusion

# 1. 构建前端
cd web/admin && npm run build && cd ../..

# 2. 复制前端到 Go embed 目录
rm -rf internal/app/embed/dist/*
cp -r web/admin/dist/* internal/app/embed/dist/
cp -r web/portal internal/app/embed/dist/

# 3. 编译面板后端
go build -o /tmp/corefusion ./cmd/corefusion/

# 4. 停止旧进程
kill -9 $(lsof -t -i :8080) 2>/dev/null; sleep 2

# 5. 启动新进程
nohup /tmp/corefusion server > /tmp/corefusion.log 2>&1 &
```

> 仅修改后端 Go 代码时，跳过步骤 1-2，直接执行 3-5。

### 7.2 更新远程节点 Agent

```bash
cd /opt/corefusion

# 1. 编译新 Agent
go build -o data/binaries/active/corefusion-agent/corefusion-agent ./cmd/agent/

# 2. 停止远程节点上的旧 Agent
ssh root@<REMOTE_IP> "pkill -f corefusion-agent"

# 3. 上传新二进制
scp data/binaries/active/corefusion-agent/corefusion-agent \
  root@<REMOTE_IP>:/opt/corefusion-agent/corefusion-agent

# 4. 启动新 Agent
ssh root@<REMOTE_IP> "cd /opt/corefusion-agent && \
  nohup ./corefusion-agent -config config.yaml > /tmp/agent.log 2>&1 &"

# 5. 验证
ssh root@<REMOTE_IP> "tail -10 /tmp/agent.log"
```

> 如果使用 systemd 管理，将步骤 2/4 替换为 `systemctl stop corefusion-agent` / `systemctl start corefusion-agent`。

### 7.3 仅更新前端

前端通过 `//go:embed` 编译进 Go 二进制，修改前端后必须重新编译后端：

```bash
cd /opt/corefusion

# 1. 构建前端
cd web/admin && npm run build && cd ../..

# 2. 复制到 embed 目录
rm -rf internal/app/embed/dist/*
cp -r web/admin/dist/* internal/app/embed/dist/
cp -r web/portal internal/app/embed/dist/

# 3. 重新编译后端（前端会嵌入）
go build -o /tmp/corefusion ./cmd/corefusion/

# 4. 重启面板
kill -9 $(lsof -t -i :8080) 2>/dev/null; sleep 2
nohup /tmp/corefusion server > /tmp/corefusion.log 2>&1 &
```

> 前端修改无法热生效——必须重新编译整个 Go 二进制并重启面板。

---

## 8. 架构概述

### 8.1 通信拓扑

```
┌─────────────────────────────────────────────────────┐
│                    面板 (Panel)                       │
│                                                     │
│  HTTP API (:8080)  ←── 浏览器 (管理界面)             │
│  gRPC Server (:18443) ←── Agent 心跳上报             │
│  gRPC Client (反向拨号) ──→ Agent gRPC Server        │
│                                                     │
│  SQLite (app.db) — 全局配置、用户、套餐、节点信息     │
│  process.Manager — 本地节点核心进程管理               │
└─────────────────────────────────────────────────────┘
          │                              │
          │ Heartbeat Stream              │ Reverse Dial (gRPC)
          │ (Agent → Panel)               │ (Panel → Agent)
          ▼                              ▼
┌──────────────────────┐     ┌──────────────────────┐
│   节点 1 (Agent)      │     │   节点 2 (Agent)      │
│                      │     │                      │
│  Heartbeat → Panel   │     │  Heartbeat → Panel   │
│  AgentService gRPC   │     │  AgentService gRPC   │
│  process.Manager     │     │  process.Manager     │
│  + Guardian          │     │  + Guardian          │
│                      │     │                      │
│  SQLite (agent.db)   │     │  SQLite (agent.db)   │
│  runtime/xray.jsonc  │     │  runtime/xray.jsonc  │
└──────────────────────┘     └──────────────────────┘
```

### 8.2 Agent 职责

| 职责 | 说明 |
|------|------|
| Bootstrap | 用 Bootstrap Token 向面板换取 mTLS 证书 |
| 心跳上报 | 每 15 秒发送心跳，每 30 秒上报核心状态 |
| 配置同步 | 接收面板下发的核心配置文件，写入本地 runtime 目录 |
| 进程守护 | process.Manager + Guardian 自动重启崩溃的核心 |
| 用户热更新 | 通过 Xray gRPC API 增删用户，不重启核心 |
| 代理管理 | Caddy/Nginx 配置同步与进程控制 |

### 8.3 配置热更新 vs 重启

| 操作类型 | 方式 | 是否重启核心 |
|---------|------|-------------|
| 入站 (Inbounds) | Xray gRPC API (AddInbound/RemoveInbound) | 否（热更新） |
| 出站 (Outbounds) | Xray gRPC API (AddOutbound/RemoveOutbound) | 否（热更新） |
| 路由 (Routing) | Xray gRPC API (AddRule/RemoveRule) | 否（热更新） |
| 用户增删 | Xray gRPC API (AlterInbound) | 否（热更新） |
| 日志/DNS/策略等 | 配置文件写入 + 核心重启 | 是（若核心运行中） |
| 配置文件持久化 | configsync 定时器（5 分钟）同步到文件 | 否 |

### 8.4 基于节点的数据隔离 (Per-Node Scoping)

服务管理 (Services) 和规则管理 (Geo Rules) 已从全局模式迁移为基于节点的架构。
所有数据和 API 路由均按 `server_id` 隔离，确保每个节点的配置互不影响。

#### 数据模型

以下实体新增 `server_id` 字段（`int64`，默认值 `1` = 本地节点）：

| 实体 | 表名 | 说明 |
|------|------|------|
| `InboundService` | `inbound_services` | 入站服务（核心进程管理） |
| `RuleSource` | `rule_sources` | Geo 规则来源（GitHub repo / 直接 URL） |
| `GeoRuleFile` | `geo_rule_files` | Geo 规则文件（geoip.dat / geosite.dat 等） |

#### API 路由

所有服务管理和规则管理 API 已迁移至 `/node-servers/:id/` 下：

```
# 服务管理（按节点）
GET    /api/v1/node-servers/:id/services                    # 列出节点的服务
POST   /api/v1/node-servers/:id/services                    # 创建服务
GET    /api/v1/node-servers/:id/services/:serviceId         # 获取服务详情
PUT    /api/v1/node-servers/:id/services/:serviceId         # 更新服务
DELETE /api/v1/node-servers/:id/services/:serviceId         # 删除服务

# 进程管理（按节点）
POST   /api/v1/node-servers/:id/services/:serviceId/start   # 启动核心
POST   /api/v1/node-servers/:id/services/:serviceId/stop    # 停止核心
POST   /api/v1/node-servers/:id/services/:serviceId/restart # 重启核心
GET    /api/v1/node-servers/:id/services/process-status      # 所有核心状态
PUT    /api/v1/node-servers/:id/services/:serviceId/guardian # 守护配置

# 规则管理（按节点）
GET    /api/v1/node-servers/:id/geo-rules/sources            # 列出规则来源
POST   /api/v1/node-servers/:id/geo-rules/sources            # 创建规则来源
PUT    /api/v1/node-servers/:id/geo-rules/sources/:sourceId  # 更新规则来源
DELETE /api/v1/node-servers/:id/geo-rules/sources/:sourceId  # 删除规则来源
GET    /api/v1/node-servers/:id/geo-rules/files              # 列出规则文件
GET    /api/v1/node-servers/:id/geo-rules/status             # 规则文件状态
POST   /api/v1/node-servers/:id/geo-rules/sync-all           # 同步所有来源
```

#### 远程节点操作

`ProcessHandler` 通过 `NodeOperator` 接口支持远程节点操作：
- **本地节点** (`is_local=1`)：直接调用 `process.Manager` 管理进程
- **远程节点**：通过 gRPC 向 Agent 发送 `ControlCore` 指令

```go
// ProcessHandler 中的远程节点路由逻辑
if h.nodeOperator != nil && !h.nodeOperator.IsLocalServer(ctx, svc.ServerID) {
    // 远程节点：通过 gRPC 控制 Agent
    h.nodeOperator.ControlCore(ctx, svc.ServerID, svc.CoreType, "start")
} else {
    // 本地节点：直接调用 process.Manager
    h.processMgr.Start(ctx, serviceID, svc.CoreType)
}
```

#### Agent 默认配置生成

Agent 首次启动时，如果面板尚未同步配置，`process.Manager` 会调用 `SetConfigGenerator`
生成最小可用配置。每种核心类型 (`xray`, `singbox`, `hysteria2`, `tuic`) 都有对应的默认配置。
面板通过 `SyncConfig` 下发完整配置后会覆盖这些默认值。

---

## 9. 常见问题

### Q: Agent 显示 offline

**检查步骤：**

1. 确认 Agent 进程在运行：
   ```bash
   ssh root@<REMOTE_IP> "ps aux | grep corefusion-agent | grep -v grep"
   ```

2. 检查 Agent 日志：
   ```bash
   ssh root@<REMOTE_IP> "tail -30 /tmp/agent.log"
   ```

3. 确认面板 gRPC 端口 (18443) 可达：
   ```bash
   ssh root@<REMOTE_IP> "nc -zv <PANEL_IP> 18443"
   ```

4. 确认 mTLS 证书未过期（有效期 10 天，面板自动续签）：
   ```bash
   ssh root@<REMOTE_IP> "openssl x509 -in /opt/corefusion-agent/data/certs/agent-cert.pem -noout -dates"
   ```

5. 如果证书过期或损坏，删除证书重新 Bootstrap：
   ```bash
   ssh root@<REMOTE_IP> "rm /opt/corefusion-agent/data/certs/*.pem"
   # 在面板上重新生成 Bootstrap Token，更新 config.yaml，重启 Agent
   ```

### Q: 面板重启后 Agent 不自动重连

gRPC TCP 连接断开检测有延迟（默认依赖 TCP keepalive）。**临时方案**：重启 Agent 进程。后续优化可增加 gRPC keepalive 探测。

### Q: `server N not connected` 错误

面板反向拨号失败。检查：

1. 远程节点 `agent.listen_port` (默认 18443) 是否被防火墙拦截
2. 面板能否访问远程节点的 18443 端口：
   ```bash
   nc -zv <REMOTE_IP> 18443
   ```
3. 远程节点的 `internal_ip` 或 `public_ip` 在面板数据库中是否正确：
   ```bash
   sqlite3 data/sqlite/app.db "SELECT id, hostname, internal_ip, public_ip, agent_port FROM node_servers;"
   ```

### Q: SQLite `ON CONFLICT` 错误

SQLite 版本过低（< 3.35）。升级 SQLite 或使用 PostgreSQL。面板启动时迁移会自动处理索引。

### Q: 编译报错 `module not found`

```bash
cd /opt/corefusion
go mod tidy
go mod download
```

---

## 10. 端口速查表

| 端口 | 协议 | 方向 | 用途 |
|------|------|------|------|
| 8080 | HTTP | 浏览器 → 面板 | 管理界面 + REST API |
| 8081 | HTTP | 客户端 → 面板 | 订阅服务 |
| 18443 | gRPC (mTLS) | Agent → 面板 | 心跳上报流 |
| 18443 | gRPC (mTLS) | 面板 → Agent | 反向拨号 (AgentService RPC) |
| 10085 | gRPC | 面板/Agent → Xray | Xray 内部 API (入站/出站/路由热更新) |
| 3000 | HTTP | 开发者 → Vite | 前端开发服务器 (仅开发环境) |

---

## 11. 快速参考命令

### 一键编译并重启面板（前端 + 后端）

```bash
cd /opt/corefusion/web/admin && npm run build 2>&1 | tail -2 && \
cd /opt/corefusion && rm -rf internal/app/embed/dist/* && \
cp -r web/admin/dist/* internal/app/embed/dist/ && \
cp -r web/portal internal/app/embed/dist/ && \
go build -o /tmp/corefusion ./cmd/corefusion 2>&1 && \
kill -9 $(lsof -t -i :8080) 2>/dev/null; sleep 2; \
/tmp/corefusion server > /tmp/corefusion.log 2>&1 & sleep 5; echo "done"
```

### 仅编译后端（前端无改动）

```bash
cd /opt/corefusion && go build -o /tmp/corefusion ./cmd/corefusion/ && \
kill -9 $(lsof -t -i :8080) 2>/dev/null; sleep 2; \
/tmp/corefusion server > /tmp/corefusion.log 2>&1 & sleep 3; echo "done"
```

### 编译并部署 Agent 到远程节点

```bash
cd /opt/corefusion && \
go build -o data/binaries/active/corefusion-agent/corefusion-agent ./cmd/agent/ && \
ssh root@<REMOTE_IP> "pkill -f corefusion-agent" 2>/dev/null; sleep 2; \
scp data/binaries/active/corefusion-agent/corefusion-agent root@<REMOTE_IP>:/opt/corefusion-agent/corefusion-agent && \
ssh root@<REMOTE_IP> "cd /opt/corefusion-agent && nohup ./corefusion-agent -config config.yaml > /tmp/agent.log 2>&1 &" && \
sleep 3 && ssh root@<REMOTE_IP> "tail -10 /tmp/agent.log"
```
