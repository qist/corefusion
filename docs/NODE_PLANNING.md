# CoreFusion 测试环境节点规划文档

> 版本日期:2026-08-04
> 作者:运维/测试规划
> 状态:规划定稿(9 节点 K8S 已关闭并禁用自启,节点重新规划为 CoreFusion 用途)

---

## 0. 背景与前提

- 现有 9 个节点(原运行二进制 Kubernetes,现已关闭并禁用自启,重启后无 K8S 进程)重新规划为 **CoreFusion 项目功能测试** 用途。
- Panel 复用现有开发机 `127.0.0.1`(该机真实 IP 为 `172.18.173.88`,对外通过 `127.0.0.1` 访问)。
- 包下载统一走代理 `127.0.0.1:7890`(节点本身无外网直连能力)。
- SSH 访问:以 `root` 身份可直连 9 个节点 IP。

### 节点清单

| 编号 | IP | 原角色 | 现状 |
|------|-----|--------|------|
| N1 | 192.168.2.175 | 原 K8S master/etcd | 已清 K8S,重规划 |
| N2 | 192.168.2.176 | 原 K8S master/etcd | 已清 K8S,重规划 |
| N3 | 192.168.2.177 | 原 K8S master/etcd | 已清 K8S,重规划 |
| N4 | 192.168.2.185 | 原 K8S worker | 已清 K8S,重规划 |
| N5 | 192.168.2.187 | 原 K8S worker | 已清 K8S,重规划 |
| N6 | 192.168.3.62  | 原 K8S worker | 已清 K8S,重规划 |
| N7 | 192.168.3.70  | 原 K8S worker | 已清 K8S,重规划 |
| N8 | 192.168.3.100 | 原 K8S worker | 已清 K8S,重规划 |
| N9 | 192.168.2.50  | 原 K8S worker | 已清 K8S,已部署为独立节点运行中 |

---

## 1. 节点用途规划(测试 CoreFusion,按节点类型)

> 规划原则:Panel 复用现有开发机;9 个节点按 CoreFusion 真实节点类型
> (`internal/model/entity/node_server.go` 中 `NodeType*` 枚举)分配,覆盖
> 主备 / 镜像 / 链式入口 / 链式出口 / 独立 / 数据库 全类型,用于功能测试。

### 1.1 角色分配(按节点类型)

| 编号 | IP | 节点类型(NodeType) | 部署内容 | 用途说明 |
|------|-----|--------------------|----------|----------|
| P  | 127.0.0.1(真实 IP 172.18.173.88,对外访问 127.0.0.1) | **Panel / 管理面板** | CoreFusion Panel + Web 前端 | 复用现有开发机,管理面 + API 入口 |
| N1 | 192.168.2.175 | `primary_backup`(主) | Agent + 代理核心 | 主备主节点,Partner=192.168.2.176 |
| N2 | 192.168.2.176 | `primary_backup`(备) | Agent + 代理核心 | 主备备节点,Partner=192.168.2.175,自动故障切换 |
| N3 | 192.168.2.177 | `mirror` | Agent + 代理核心 | 镜像节点,1对1 镜像主节点 N6(192.168.3.62),`PartnerServerID` 指向 .3.62,流量合并归 .3.62 |
| N4 | 192.168.2.185 | `chain_entry` | Agent + 代理核心 | 链式代理入口(国内中转) |
| N5 | 192.168.2.187 | `chain_exit` | Agent + 代理核心 | 链式代理出口(国际出口,`billing_enabled=false`) |
| N6 | 192.168.3.62  | `standalone` | Agent + 代理核心 | 独立服务节点,被 N3 镜像 |
| N7 | 192.168.3.70  | 数据库(MySQL) | MySQL 8.x | 主业务库,数据迁移源 |
| N8 | 192.168.3.100 | 数据库(PostgreSQL) | PostgreSQL 15+ | 数据迁移目标/对账库 |
| N9 | 192.168.2.50  | `standalone` | Agent + 监控采集 | 独立节点,已部署运行 |

> 说明:
> - 链式拓扑:`chain_entry(N4)` → `chain_exit(N5)`,出口不计费(`ShouldBill()` 对 chain_exit 返回 false)。
> - 主备:`primary_backup` 主(N1)与备(N2)通过 `PartnerServerID` 互指,`FailoverRole` 分别为 primary/backup。
> - 镜像:`mirror(N3, 192.168.2.177)` 1对1 镜像主节点 `standalone(N6, 192.168.3.62)`,`PartnerServerID` 指向 N6,配置同步 + 流量合并归 N6。
> - 数据库:N7(MySQL)、N8(PostgreSQL)即"拿一个节点部署 MySQL、一个节点部署 PGSQL 测试数据迁移"的落点(分两节点更贴近真实迁移;若需同节点双库可改 N7 同装)。
> - Panel 沿用开发机 127.0.0.1(真实 IP 172.18.173.88),对外通过 127.0.0.1 访问,不在 9 节点之列。

### 1.2 组件依赖关系(简化)

```
Panel(127.0.0.1, 真实 IP 172.18.173.88, 对外访问 127.0.0.1)
   │  gRPC/configsync
   ├── primary_backup 主(N1) ⇄ 备(N2)   # 故障切换
   │        └── mirror(N3) ──镜像──> standalone(N6)   # 1对1 镜像,流量合并归 N6
   ├── chain_entry(N4) ──> chain_exit(N5)   # 链式代理,出口不计费
   ├── standalone(N6)
   └── standalone/观测(N9)
   └── DB: MySQL(N7) / PostgreSQL(N8)
```

### 1.3 网络与代理

- 节点无外网:所有 `yum/apt/wget/curl` 下载统一通过 `127.0.0.1:7890` 代理。
- 节点内配置(示例):
  ```bash
  export http_proxy=http://127.0.0.1:7890
  export https_proxy=http://127.0.0.1:7890
  ```
- 数据库端口(N7:3306 / N8:5432)默认仅节点间互通,不外放;如需从外部迁移工具连接,经 `127.0.0.1` 代理或跳板。

---

## 2. 数据库节点部署与数据迁移测试(已完成,2026-08-04)

### 2.1 N7 部署 MySQL(已完成)

- 节点 `192.168.3.70`(Rocky Linux 8.9),经代理 `127.0.0.1:7890` 用 `dnf` 安装 `mysql-server` 8.0.46。
- 初始化库 `corefusion`,账号 `corefusion/Cf@2026test`,监听 3306,已开机自启。

### 2.2 N8 部署 PostgreSQL(已完成)

- 节点 `192.168.3.100`(Rocky Linux 10.1),经代理安装 `postgresql-server` 16.14。
- `postgresql-setup --initdb` 初始化,`pg_hba.conf` 改为 `scram-sha-256` 密码认证、`listen_addresses='*'`,库 `corefusion`、账号 `corefusion/Cf@2026test`。

### 2.3 数据迁移验证(已完成,核心结论)

CoreFusion 自带跨库迁移能力(`internal/store/db.MigrateTo` + `internal/migration.AutoMigrate`)。
验证路径(源=开发机 SQLite,16 万行真实数据):

```
SQLite ──> MySQL(N7) ──> PostgreSQL(N8) ──> MySQL(N7)   # 三向相互迁移
```

- 各向均成功,关键表行数一致(node_servers=3 / inbound_configs=48 / proxy_sites=11 /
  subscription_access_logs=445 / portal_templates=1 / server_metrics≈160623)。
- `server_metrics` 等差 ≤200 行属运行期实时写入,非迁移丢失。
- **三种后端均实测可正常运行**:Panel 分别以 SQLite / MySQL / PostgreSQL 为 `CF_DB_DSN`
  启动,HTTP 200、configsync / SpeedTest 等功能日志正常。

### 2.4 迁移过程中修复的跨库兼容性 Bug(已合入代码)

| # | 问题 | 原因 | 修复 |
|---|------|------|------|
| 1 | MySQL→PG 建表失败 `type longtext does not exist` | `Article.Content` 写死 `type:longtext`(MySQL 专有) | entity 长文本统一 `type:longtext`,PG 端 `CREATE DOMAIN longtext AS text` 别名 |
| 2 | `migration` 索引语句 MySQL 报 1064 | 用了 PG 语法 `DROP INDEX IF EXISTS` / `CREATE INDEX IF NOT EXISTS` | 按 `Dialector.Name()` 分支生成 MySQL/PG 语法,幂等忽略 1091/1061 |
| 3 | `portal_templates.config` 报 1406 Data too long | MySQL `text` 上限 64KB,实际数据 107KB | 长文本字段统一 `type:longtext`(MySQL 4GB);PG 用 domain 别名 |
| 4 | MySQL→PG 报 `unable to encode 0 into bool` | MySQL bool 读出为 `int64`,PG driver 拒绝 | `MigrateTo` 拷贝时按目标 bool 列把 `int64(0/1)` 转 `bool` |
| 5 | 源多列(如废弃 `enable_http3`)导致整批插入失败丢数据 | `MigrateTo` 整行插入,目标无该列即 1054 | 拷贝前只保留目标实际存在的列 |

> 涉及文件:`internal/model/entity/*.go`、`internal/store/db/db.go`、`internal/migration/migration.go`。
> 测试工具:`cmd/dbtest`(三向迁移)、`cmd/dbcheck`(行数核对)。

---

## 3. 待确认项

- [ ] N7/N8 是否严格"同一节点双库"还是"两节点各一库"(本文按两节点各一库规划,可调整)。
