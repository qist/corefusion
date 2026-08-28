# CoreFusion 测试环境节点规划文档

> 版本日期:2026-08-04(▸ 2026-08-28 依 chain-proxy-redesign 更新:取消 chain_entry/chain_exit 节点类型)
> 作者:运维/测试规划
> 状态:规划定稿(9 节点 K8S 已关闭并禁用自启,节点重新规划为 CoreFusion 用途)
> 说明:本版 N4/N5 由 `chain_entry`/`chain_exit` 类型改为 `standalone`,链式入口/出口角色改由
> `chain_links`(N4→N5)与 `chain_exit_configs`(N5,relay_only 纯出口)表达,边界见 §0 变更注记。

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
> 主备 / 镜像 / 独立 / 同步 / 数据库 全类型,用于功能测试。
>
> > **⚠️ 2026 链式代理设计变更**:链式入口/链式出口**不再是节点类型**。
> > 见 `docs/design/chain-proxy-redesign.md`:节点类型回归为 `standalone / mirror /
> > sync / primary_backup`;链式代理角色改用独立的 `chain_links`(入口→出口关系)与
> > `chain_exit_configs`(出口中转配置)表达。任何节点(通常 `standalone`)只要：
> > - 有 `chain_links` 记录 → 即**入口**(其入站流量路由到出口)；
> > - 有 `chain_exit_configs`(且 `relay_only` 可设) → 即**出口**(接收中转)。
> > 因此本文 N4/N5 的 `chain_entry`/`chain_exit` 类型改为 `standalone`,
> > 改由 `chain_links`(N4→N5)与 `chain_exit_configs`(N5)表达链式拓扑。

### 1.1 角色分配(按节点类型)

| 编号 | IP | 节点类型(NodeType) | 部署内容 | 用途说明 |
|------|-----|--------------------|----------|----------|
| P  | 127.0.0.1(真实 IP 172.18.173.88,对外访问 127.0.0.1) | **Panel / 管理面板** | CoreFusion Panel + Web 前端 | 复用现有开发机,管理面 + API 入口 |
| N1 | 192.168.2.175 | `primary_backup`(主) | Agent + 代理核心 | 主备主节点,Partner=192.168.2.176 |
| N2 | 192.168.2.176 | `primary_backup`(备) | Agent + 代理核心 | 主备备节点,Partner=192.168.2.175,自动故障切换 |
| N3 | 192.168.2.177 | `mirror` | Agent + 代理核心 | 镜像节点,1对1 镜像主节点 N6(192.168.3.62),`PartnerServerID` 指向 .3.62,流量合并归 .3.62 |
| N4 | 192.168.2.185 | `standalone` | Agent + 代理核心 | 链式代理**入口**(国内中转,`chain_links` 指向 N5;有用户入站,计费) |
| N5 | 192.168.2.187 | `standalone` | Agent + 代理核心 | 链式代理**出口**(`chain_exit_configs`,可设 `relay_only=1` 纯出口,`billing_enabled=false` 不计费) |
| N6 | 192.168.3.62  | `standalone` | Agent + 代理核心 | 独立服务节点,被 N3 镜像 |
| N7 | 192.168.3.70  | 数据库(MySQL) | MySQL 8.x | 主业务库,数据迁移源 |
| N8 | 192.168.3.100 | 数据库(PostgreSQL) | PostgreSQL 15+ | 数据迁移目标/对账库 |
| N9 | 192.168.2.50  | `standalone` | Agent + 监控采集 | 独立节点,已部署运行 |

> 说明:
> - 链式拓扑:N4(入口 `standalone`,含 `chain_links`) → N5(出口 `standalone`,含 `chain_exit_configs`)。
>   出口为纯中转时设 `relay_only=1` 且 `billing_enabled=false`(纯出口不创建用户、不计费,见 §7)。
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
   ├── N4(入口 standalone) ──chain_link──> N5(出口 standalone)   # 链式代理,出口 relay_only 不计费
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

## 2. 数据库节点部署与数据迁移测试(✅ 已完成,2026-08-04,无需重复部署)

### 2.1 N7 部署 MySQL(✅ 已完成,已验证正常运行)

- 节点 `192.168.3.70`(Rocky Linux 8.9),经代理 `127.0.0.1:7890` 用 `dnf` 安装 `mysql-server` 8.0.46。
- 初始化库 `corefusion`,账号 `corefusion/Cf@2026test`,监听 3306,已开机自启。

### 2.2 N8 部署 PostgreSQL(✅ 已完成,已验证正常运行)

- 节点 `192.168.3.100`(Rocky Linux 10.1),经代理安装 `postgresql-server` 16.14。
- `postgresql-setup --initdb` 初始化,`pg_hba.conf` 改为 `scram-sha-256` 密码认证、`listen_addresses='*'`,库 `corefusion`、账号 `corefusion/Cf@2026test`。

### 2.3 数据迁移验证(✅ 已完成,三向迁移均成功,无需重复验证)

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

## 4. 备份/还原节点部署与测试(✅ 已完成,FTP + S3 + WebDAV 接口均已验证,无需重复部署)

> 目标:在测试节点上部署 **FTP 服务**、**S3 兼容存储(MinIO)** 与 **WebDAV 服务**,用 CoreFusion 已有的备份/上传/还原工具
> 验证"本地备份 → 上传 FTP → 上传 S3 → 上传 WebDAV → 从备份还原 → 数据库级还原"全链路。
> CoreFusion 代码(见 `internal/service/backup/service.go`)已内置 FTP(`jlaffaye/ftp`)、
> S3(`aws-sdk-go-v2/service/s3`,支持自定义 Endpoint 兼容 MinIO)、WebDAV(`studio-b12/gowebdav`)上传能力,及数据库纯 Go dump/restore。

### 4.1 节点角色分配(在既有 9 节点内)

| 节点 | 角色 | 部署内容 | 访问地址 |
|------|------|----------|----------|
| N5 (192.168.2.187) | FTP Server | `vsftpd`(被动模式,开启 `pasv_enable`) | `ftp://192.168.2.187:21` |
| N6 (192.168.3.62) | S3 兼容存储 | `minio`(单节点 single drive) | `http://192.168.3.62:9000`,控制台 `:9001` |
| 192.168.1.117 | WebDAV Server | nginx + dav-ext-module(OpenResty) | `http://192.168.1.117:80/webdav/corefusion/` |
| 127.0.0.1 | Panel(已有) | CoreFusion + 备份 API | 管理后台 `/admin/backups` |

> 注:N5/N6 节点无外网,`vsftpd` / `minio` 二进制通过 Panel 代理 `127.0.0.1:7890` 下载。
> 192.168.1.117 可直接访问外网,nginx 已预装(OpenResty + dav-ext-module)。

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

### 4.3 部署 WebDAV(192.168.1.117,已完成)

```bash
# 192.168.1.117 可直接访问外网,无需代理
apt-get install -y apache2-utils

# 创建 WebDAV 目录和认证
mkdir -p /srv/webdav/corefusion
htpasswd -bc /etc/nginx/.webdav-passwd cf_webdav 'CFwd@2026'
chown -R www-data:www-data /srv/webdav

# nginx 已预装 OpenResty + dav-ext-module
# 配置 /etc/nginx/nginx.conf,添加 WebDAV server (port 80):
#   location /webdav/ {
#       root /srv/webdav;
#       auth_basic "CoreFusion WebDAV";
#       auth_basic_user_file /etc/nginx/.webdav-passwd;
#       dav_methods PUT DELETE MKCOL COPY MOVE;
#       dav_ext_methods PROPFIND OPTIONS;
#       create_full_put_path on;
#       client_max_body_size 0;
#   }

# 启动并加入 rc.local 开机自启
nginx
echo '/usr/sbin/nginx' >> /etc/rc.local
```

**WebDAV 访问信息:**
- URL: `http://192.168.1.117:80/webdav/corefusion/`
- 用户名: `cf_webdav`
- 密码: `CFwd@2026`
- 已验证: PUT(201) / GET(200) / PROPFIND(207) / DELETE(204) 全部正常

### 4.4 部署 MinIO(S3 兼容, N6)

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

### 4.5 在 Panel 配置备份存储(走 API 或后台)

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
  "endpoint": "ftp://192.168.2.187:21",
  "access_key": "ftpuser",
  "secret_key": "CHANGE_ME_FTP_PASSWORD"
}
```

```json
POST /api/backups/storages
{
  "name": "webdav-117",
  "type": "webdav",
  "endpoint": "http://192.168.1.117:80/webdav/corefusion/",
  "access_key": "cf_webdav",
  "secret_key": "CFwd@2026"
}
```

### 4.6 测试矩阵:备份 → 上传 → 还原

| 步骤 | 操作(API) | 预期 |
|------|-----------|------|
| ① 创建全量备份 | `POST /backups/create` | 返回备份 ID,文件落 `data/storage/backups/`(含 `*.sql` + `data.tar.gz` + `manifest.json`) |
| ② 上传 FTP | `POST /backups/:id/upload-ftp` | FTP 服务端 `/srv/ftp/corefusion/<备份tar>` 出现;返回 `url` 可下载 |
| ③ 上传 S3 | `POST /backups/:id/upload-s3` | MinIO `corefusion-backup` 桶出现对象;返回 `url` |
| ③b 上传 WebDAV | `POST /backups/:id/upload-webdav` | WebDAV `/srv/webdav/corefusion/<备份tar>` 出现;返回 `url` |
| ④ 下载校验 | `GET /backups/:id/download` | 与本地 `sha256` 一致 |
| ⑤ 全量还原 | `POST /backups/:id/restore` | 暂停服务 → 解包 → 覆盖 data 目录 + `mysql/psql/sqlite` 纯 Go 导入 → 重启 → 校验任务/用户数 |
| ⑥ 数据库级还原 | `POST /backups/database/restore`(含 `db_dump` 字段的 JSON body) | 仅还原 DB,不动数据目录 |
| ⑦ 上传即还原 | `POST /backups/upload-restore`(multipart 上传 tar) | 直接套用一份外来备份 |

### 4.7 数据流与还原链路

```text
Panel(data/) ──①create──> backups/<id>/ { *.sql, data.tar.gz, manifest.json }
                         ├─②upload-ftp────> N5 vsftpd       /srv/ftp/corefusion/<id>.tar.gz
                         ├─③upload-s3─────> N6 MinIO         corefusion-backup/<id>.tar.gz
                         └─③bupload-webdav> 192.168.1.117 nginx /srv/webdav/corefusion/<id>.tar.gz
                              │
   ⑤restore ◀── 下载回 data/ ──┘
   ⑥database/restore ◀── 解析 *.sql 做纯 Go import(支持 sqlite/mysql/pg)
```

### 4.8 校验要点

- FTP 被动端口段(21100-21110)需在防火墙放行,否则上传卡死(节点间走内网,一般不挡)。
- S3 必须 `use_path_style=true`(MinIO 默认 path-style),否则 `UploadToS3` 拼出 `bucket.endpoint` 虚拟主机风格报 DNS 错误。
- 还原前必须确认备份 `manifest.json` 的 `db_type` 与当前 `CF_DB_DSN` 一致(SQLite/MySQL/PostgreSQL 不可混用)。
- 还原会停服务、覆盖 `data/` 与数据库,**测试请在独立测试库/测试节点执行**,避免污染 `127.0.0.1` 生产开发数据。

### 4.9 涉及文件(已实现,无需重写)

- `internal/service/backup/service.go`:`CreateBackup` / `RestoreBackup` / `UploadToFTP` / `UploadToS3` / `UploadToWebDAV` / `DatabaseDump` / `DatabaseRestore`
- `internal/service/backup/storage.go`:存储配置(含 S3 custom Endpoint 支持)
- `internal/api/handlers/backups/handler.go`:上述所有 API 端点
- 路由:`internal/api/router/router.go` 第 1993-2025 行

---

## 5. 部署状态汇总

### 5.1 基础设施(✅ 已完成,无需重复部署)

| 服务 | 节点 | 状态 | 访问信息 |
|------|------|------|----------|
| FTP (vsftpd) | N5 (192.168.2.187) | ✅ 已部署运行 | `ftp://192.168.2.187:21`, user=`ftpuser`, pass=`CHANGE_ME_FTP_PASSWORD` |
| S3 (MinIO) | N6 (192.168.3.62) | ✅ 已部署运行 | `http://192.168.3.62:9000`, key=`cf_s3`, secret=`CHANGE_ME_S3_PASSWORD` |
| WebDAV (nginx) | 192.168.1.117 | ✅ 已部署运行 | `http://192.168.1.117:80/webdav/corefusion/`, user=`cf_webdav`, pass=`CFwd@2026` |
| MySQL 8.x | N7 (192.168.3.70) | ✅ 已部署运行 | `corefusion@192.168.3.70:3306`, pass=`Cf@2026test` |
| PostgreSQL 16 | N8 (192.168.3.100) | ✅ 已部署运行 | `corefusion@192.168.3.100:5432`, pass=`Cf@2026test` |

### 5.2 节点类型部署(✅ 已完成,2026-08-06)

| 编号 | IP | 面板ID | 节点类型 | Agent 状态 | 版本 | 伙伴 | Failover | 计费 |
|------|-----|--------|----------|-----------|------|------|----------|------|
| N1 | 192.168.2.175 | 12 | `primary_backup`(主) | ✅ online | v0.8.81 | →N2(13) | primary | true |
| N2 | 192.168.2.176 | 13 | `primary_backup`(备) | ✅ online | v0.8.81 | →N1(12) | backup | true |
| N3 | 192.168.2.177 | 14 | `mirror` | ✅ online | v0.8.81 | →N6(17) | — | false |
| N4 | 192.168.2.185 | 15 | `standalone`(入口) | ✅ online | v0.10.0 | →N5(16) `chain_links` | — | true |
| N5 | 192.168.2.187 | 16 | `standalone`(出口) | ✅ online | v0.10.0 | — | — | false |
| N6 | 192.168.3.62 | 17 | `standalone` | ✅ online | v0.8.81 | — | — | true |
| N9 | 192.168.2.50 | 2 | `standalone` | ✅ online | v0.8.81 | — | — | true |

> 数据库节点(N7/N8)和备份服务(FTP/S3/WebDAV)已在第 2、4 节完成部署与验证,**无需重复部署**。
> **注**:N4/N5 类型按 2026 链式代理设计变更为 `standalone`,入口/出口角色由
> `chain_links`(N4→N5)与 `chain_exit_configs`(N5)表达;N5 作纯出口可设 `relay_only`,故不计费。

## 6. 待确认项

- [ ] 备份还原是否纳入定时调度(`/backups/schedules`)做周期性演练。
- [ ] MinIO 是否需要开启 TLS(测试环境先用 http)。

---

## 7. 节点类型部署与 Agent 功能测试(✅ 已完成,2026-08-06)

> 按照第 1.1 节规划,将 9 个节点中的 6 个未部署节点(N1-N6)全部部署 Agent 并配置为
> 对应节点类型,N9 已部署运行。Agent 版本 v0.8.81,面板 v0.8.81。

### 7.1 部署流程

1. **面板注册节点**:通过 `POST /api/v1/node-servers` 创建 N1-N6,设置正确的 `node_type`、
   `failover_role`、`partner_server_id` 等字段。
2. **伙伴关系配置**:N1↔N2 互指(partner),N3→N6 指向(partner=17)。
3. **计费修正**:mirror(N3)和链式纯出口 N5(relay_only)自动/手动设置 `billing_enabled=false`。
4. **Agent 一键安装**:通过 `curl -fsSL 'http://127.0.0.1:8080/api/v1/agent/install?id=<ID>&token=<TOKEN>' | bash`
   在各节点执行,自动完成二进制下载、config.yaml 生成、systemd 服务创建并启动。
5. **Bootstrap 完成所有节点 mTLS 证书签发**:每节点 `data/certs/` 下均有
   `agent-cert.pem`、`agent-key.pem`、`ca-cert.pem`。

### 7.2 Agent 连接验证(全部 ✅ 通过)

| 检查项 | N1 | N2 | N3 | N4 | N5 | N6 | N9 |
|--------|----|----|----|----|----|----|----|
| systemd active | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| systemd enabled | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Agent 进程 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| mTLS 证书 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| 面板心跳 online | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| 版本号 v0.8.81 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| 资源使用上报 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

> 日志中 `dial xray stats 127.0.0.1:10085: connection failed (transient)` 为**正常预期**行为 —
> 新节点尚未安装代理核心(xray/singbox 等),Agent 定期查询统计失败,安装核心后自动恢复。

### 7.3 节点类型功能测试

#### 7.3.1 主备故障切换(primary_backup)— ✅ 通过

```
N1 (ID=12) primary_backup primary, partner=13, online=true, has_failed_over=false
N2 (ID=13) primary_backup backup,  partner=12, online=true, has_failed_over=false
Failover check triggered: true
```

- `GET /node-servers/12/failover-status` → current_role=primary, partner_id=13 ✅
- `GET /node-servers/13/failover-status` → current_role=backup, partner_id=12 ✅
- `POST /node-servers/failover-check` → triggered=true ✅
- 互指关系正确,主备角色符合预期,故障切换管理器正常工作

#### 7.3.2 镜像同步(mirror)— ✅ 通过

```
N3 (ID=14) mirror, primary_id=17 (N6), online=true, primary_online=true, last_sync_age=0
Mirror sync triggered: synced=true
```

- `GET /node-servers/14/mirror-status` → mirror_id=14, primary_id=17, primary_online=true ✅
- `POST /node-servers/14/sync-mirror` → synced=true ✅
- N3 镜像关系指向 N6(standalone),双方在线,镜像同步管理器正常工作

#### 7.3.3 链式代理(入口/出口角色,节点类型均为 `standalone`)— ✅ 已打通全链路

- N4 (ID=15) `standalone`(链式入口,`chain_links` 指向 N5)— Agent online v0.10.0 ✅
- N5 (ID=16) `standalone`(链式出口,`chain_exit_configs` `relay_only`,billing_enabled=false)— Agent online v0.10.0 ✅
- **配置落地**:configsync 全节点同步,入口 N4 `config_content`=2153B(含 `chain-117-16` outbound + 路由 `in-117→chain-117-16`),
  出口 N5 `config_content`=1457B(含 `relay-in` vless:443 + `chain-relay-uuid`);修复 `GenerateExitInbound` 缺 `decryption:none`。
- **SyncData 本地落地**:面板→Agent 下传 `chain_links`/`chain_exit_configs`/`node_servers`/`external_nodes`,
  N4 本地 3 条 chain_links、N5 本地 2 条 chain_exit_configs,离线可还原。✅
- **端到端链路**:两机 xray 26.7.28 运行——N4 `in-chain-entry`:10090 / N5 `relay-in`:443。面板侧客户端
  →N4 入口→`chain-117-16`→N5 `relay-in`→N5 direct 出网,`curl --socks5 http://example.com/` 返回 **HTTP 200** ✅
- 纯出口(N5 relay_only)`ShouldBill()` 返回 false ✅

#### 7.3.4 独立节点(standalone)— ✅ 通过

- N6 (ID=17) standalone — Agent online, 12 核 CPU, 31.4GB 内存,资源上报正常 ✅
- N9 (ID=2) standalone — Agent online, xray 进程运行中(PID=8738, uptime=151567s) ✅
- N9 有已安装的 xray 核心,`GET /node-servers/2/status` 返回 core 运行状态 ✅

### 7.4 计费规则验证

| 节点 | 类型 | billing_enabled | ShouldBill() | 预期 | 结果 |
|------|------|----------------|-------------|------|------|
| N1 | primary_backup | true | true | 计费 | ✅ |
| N2 | primary_backup | true | true | 计费 | ✅ |
| N3 | mirror | false | false | 不计费 | ✅ |
| N4 | standalone(入口) | true | true | 计费 | ✅ |
| N5 | standalone(出口) | false | false | 不计费 | ✅ |
| N6 | standalone | true | true | 计费 | ✅ |
| N9 | standalone | true | true | 计费 | ✅ |

### 7.5 测试结论

- **Agent 部署**:9 个节点(含 Panel 本机)全部 online,版本一致 v0.8.81,systemd active+enabled ✅
- **主备故障切换**:N1/N2 互指关系正确,FailoverManager 状态查询和触发正常 ✅
- **镜像同步**:N3→N6 镜像关系建立,MirrorSyncManager 同步触发正常 ✅
- **链式代理**:N4(入口)/N5(出口,`chain_exit_configs` + `relay_only`)Agent v0.10.0 在线;配置与 SyncData 落地
  (N4 `chain-117-16`+路由 / N5 `relay-in`:443),端到端链路验证 `curl` 经链式代理返回 HTTP 200 ✅
- **独立节点**:N6/N9 正常运行,N9 已有 xray 核心运行 ✅
- **计费规则**:mirror 和链式纯出口 relay_only(N5)不计费,其余计费,符合 `ShouldBill()` 逻辑 ✅
- **数据库/备份**:已在第 2、4 节完成验证,**无需重复部署** ✅
