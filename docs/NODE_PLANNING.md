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

---

## 4. 备份/还原节点部署与测试(FTP + S3 接口)

> 目标:在测试节点上部署 **FTP 服务** 与 **S3 兼容存储(MinIO)**,用 CoreFusion 已有的备份/上传/还原工具
> 验证"本地备份 → 上传 FTP → 上传 S3 → 从备份还原 → 数据库级还原"全链路。
> CoreFusion 代码(见 `internal/service/backup/service.go`)已内置 FTP(`jlaffaye/ftp`)、
> S3(`aws-sdk-go-v2/service/s3`,支持自定义 Endpoint 兼容 MinIO)、WebDAV 上传能力,及数据库纯 Go dump/restore。

### 4.1 节点角色分配(在既有 9 节点内)

| 节点 | 角色 | 部署内容 | 访问地址 |
|------|------|----------|----------|
| N5 | FTP Server | `vsftpd`(被动模式,开启 `pasv_enable`) | `ftp://<N5>:21` |
| N6 | S3 兼容存储 | `minio`(单节点 single drive) | `http://<N6>:9000`,控制台 `:9001` |
| 127.0.0.1 | Panel(已有) | CoreFusion + 备份 API | 管理后台 `/admin/backups` |

> 注:节点无外网,`vsftpd` / `minio` 二进制通过 Panel 代理 `127.0.0.1:7890` 下载,或离线拷入 `/opt/corefusion/data`。

### 4.2 部署 FTP(N5)

```bash
# 经代理下载(节点无外网)
export http_proxy=http://127.0.0.1:7890 https_proxy=http://127.0.0.1:7890
apt-get update && apt-get install -y vsftpd

cat > /etc/vsftpd.conf <<'EOF'
listen=YES
listen_ipv6=NO
anonymous_enable=NO
local_enable=YES
write_enable=YES
chroot_local_user=YES
allow_writeable_chroot=YES
pasv_enable=YES
pasv_min_port=21100
pasv_max_port=21110
pasv_address=<N5-内网IP>
local_root=/srv/ftp
EOF

mkdir -p /srv/ftp/corefusion
useradd -m ftpuser && echo 'ftpuser:CHANGE_ME_FTP_PASSWORD' | chpasswd
chown -R ftpuser:ftpuser /srv/ftp
systemctl restart vsftpd && systemctl enable vsftpd
```

### 4.3 部署 MinIO(S3 兼容, N6)

```bash
export http_proxy=http://127.0.0.1:7890 https_proxy=http://127.0.0.1:7890
cd /opt/corefusion/data
wget https://dl.min.io/server/minio/release/linux-amd64/minio
chmod +x minio

# 单节点单盘,数据目录 /srv/minio,控制台 9001
export MINIO_ROOT_USER=cf_s3
export MINIO_ROOT_PASSWORD=CHANGE_ME_S3_PASSWORD
mkdir -p /srv/minio
nohup ./minio server /srv/minio --address :9000 --console-address :9001 > /srv/minio/minio.log 2>&1 &

# 建桶(用 mc 或 curl)
mc alias set local http://<N6>:9000 cf_s3 CHANGE_ME_S3_PASSWORD
mc mb local/corefusion-backup
```

### 4.4 在 Panel 配置备份存储(走 API 或后台)

CoreFusion 备份 API 支持 FTP / S3 / WebDAV 存储配置(`/backups/storages`)。示例 S3 存储配置:

```json
POST /api/backups/storages
{
  "name": "minio-n6",
  "type": "s3",
  "endpoint": "http://<N6>:9000",
  "bucket": "corefusion-backup",
  "access_key": "cf_s3",
  "secret_key": "CHANGE_ME_S3_PASSWORD",
  "use_path_style": true
}
```

```json
POST /api/backups/storages
{
  "name": "ftp-n5",
  "type": "ftp",
  "endpoint": "ftp://<N5>:21",
  "access_key": "ftpuser",
  "secret_key": "CHANGE_ME_FTP_PASSWORD"
}
```

### 4.5 测试矩阵:备份 → 上传 → 还原

| 步骤 | 操作(API) | 预期 |
|------|-----------|------|
| ① 创建全量备份 | `POST /backups/create` | 返回备份 ID,文件落 `data/storage/backups/`(含 `*.sql` + `data.tar.gz` + `manifest.json`) |
| ② 上传 FTP | `POST /backups/:id/upload-ftp` | FTP 服务端 `/srv/ftp/corefusion/<备份tar>` 出现;返回 `url` 可下载 |
| ③ 上传 S3 | `POST /backups/:id/upload-s3` | MinIO `corefusion-backup` 桶出现对象;返回 `url` |
| ④ 下载校验 | `GET /backups/:id/download` | 与本地 `sha256` 一致 |
| ⑤ 全量还原 | `POST /backups/:id/restore` | 暂停服务 → 解包 → 覆盖 data 目录 + `mysql/psql/sqlite` 纯 Go 导入 → 重启 → 校验任务/用户数 |
| ⑥ 数据库级还原 | `POST /backups/database/restore`(含 `db_dump` 字段的 JSON body) | 仅还原 DB,不动数据目录 |
| ⑦ 上传即还原 | `POST /backups/upload-restore`(multipart 上传 tar) | 直接套用一份外来备份 |

### 4.6 数据流与还原链路

```text
Panel(data/) ──①create──> backups/<id>/ { *.sql, data.tar.gz, manifest.json }
                         ├─②upload-ftp──> N5 vsftpd  /srv/ftp/corefusion/<id>.tar.gz
                         └─③upload-s3───> N6 MinIO    corefusion-backup/<id>.tar.gz
                              │
   ⑤restore ◀── 下载回 data/ ──┘
   ⑥database/restore ◀── 解析 *.sql 做纯 Go import(支持 sqlite/mysql/pg)
```

### 4.7 校验要点

- FTP 被动端口段(21100-21110)需在防火墙放行,否则上传卡死(节点间走内网,一般不挡)。
- S3 必须 `use_path_style=true`(MinIO 默认 path-style),否则 `UploadToS3` 拼出 `bucket.endpoint` 虚拟主机风格报 DNS 错误。
- 还原前必须确认备份 `manifest.json` 的 `db_type` 与当前 `CF_DB_DSN` 一致(SQLite/MySQL/PostgreSQL 不可混用)。
- 还原会停服务、覆盖 `data/` 与数据库,**测试请在独立测试库/测试节点执行**,避免污染 `127.0.0.1` 生产开发数据。

### 4.8 涉及文件(已实现,无需重写)

- `internal/service/backup/service.go`:`CreateBackup` / `RestoreBackup` / `UploadToFTP` / `UploadToS3` / `UploadToWebDAV` / `DatabaseDump` / `DatabaseRestore`
- `internal/service/backup/storage.go`:存储配置(含 S3 custom Endpoint 支持)
- `internal/api/handlers/backups/handler.go`:上述所有 API 端点
- 路由:`internal/api/router/router.go` 第 1993-2025 行

---

## 5. 待确认项(备份节点)

- [ ] FTP/S3 节点复用 N5/N6 还是单独新增测试机(本文按复用 N5/N6 规划)。
- [ ] 备份还原是否纳入定时调度(`/backups/schedules`)做周期性演练。
- [ ] MinIO 是否需要开启 TLS(测试环境先用 http)。
