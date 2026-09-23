# Google Public CA 配置指南（EAB 自动申请）

> 适用版本：面板 v0.12.101+ ｜ 相关页面：**证书 → EAB 配置**
> 相关代码：`internal/service/certificate/eab_google.go`、`internal/api/handlers/certificates/eab_config.go`

## 1. 为什么 Google 必须要 EAB

| CA | ACME 是否强制 EAB | EAB 怎么来 |
|---|---|---|
| Let's Encrypt | ❌ 不需要 | —— |
| ZeroSSL | ✅ 必须 | **一个邮箱即可换**（`api.zerossl.com/acme/eab-credentials-email`，acme.sh 同机制），面板可一键自动领取 |
| **Google Public CA** | ✅ **必须** | **必须用 GCP 授权去创建**（不能靠邮箱换）→ 本文重点 |

也就是说：Google 这条路**绕不开"授权"**——要么用服务账号 JSON，要么用临时 access token。

## 2. Google 官方硬性规则（务必先读）

| 规则 | 说明 |
|---|---|
| **7 天内必须使用** | EAB 创建后 7 天内不用就**作废**，需要重新创建 |
| **只能注册一个账号** | 一把 EAB 只能注册一个 ACME 账号，注册成功后该 EAB **即失效**，不能重复用 |
| **ACME 账号不过期** | 用它注册出来的 ACME 账号**长期有效** → **续期不需要 EAB** |
| ⚠ **删除 GCP 项目 = 账号全废** | 删项目会让该项目下**所有 ACME 账号立即失效**，**无法再续期** |

> 结论：**EAB 只是一次性的"入网凭证"，真正长期的是 ACME 账号**。
> 所以面板会把账号落盘到 `data/acme/accounts/<ca>.json` —— **这个目录必须备份**。

## 3. 一次性 GCP 准备（5 步）

```bash
# ① 选定/创建项目，并设为当前项目
gcloud projects list
gcloud config set project <你的项目ID>          # 例如 my-project-123456

# ② 启用 Public CA API（不启用会报 SERVICE_DISABLED）
gcloud services enable publicca.googleapis.com

# ③ 创建服务账号（面板用它的密钥去申请 EAB）
gcloud iam service-accounts create corefusion-eab \
  --display-name="CoreFusion EAB"

# ④ 授予"只能创建 EAB"的最小角色（不要给 Owner/Editor）
gcloud projects add-iam-policy-binding <你的项目ID> \
  --member="serviceAccount:corefusion-eab@<你的项目ID>.iam.gserviceaccount.com" \
  --role="roles/publicca.externalAccountKeyCreator"

# ⑤ 下载服务账号密钥 JSON（这个文件就是要粘进面板的内容）
gcloud iam service-accounts keys create ./corefusion-eab.json \
  --iam-account=corefusion-eab@<你的项目ID>.iam.gserviceaccount.com
```

> 等价的手工方式（等价，任选）：GCP Console → IAM 与管理 → 服务账号 → 建号 → 授角色 → 密钥 → 创建新密钥（JSON）。

## 4. 在面板里配置（两种方式）

进入 **证书 → EAB 配置 → 新增 EAB 配置**，CA 选 **Google Public CA**：

### 方式 A（推荐）：粘服务账号 JSON

| 字段 | 填什么 |
|---|---|
| GCP 项目 ID | 第 3 步的项目 ID，如 `my-project-123456` |
| 服务账号 JSON | 把 `corefusion-eab.json` **整个文件内容**粘进来 |
| 临时 Access Token | 留空 |
| 名称 | 自动形如 `Google Public CA - <项目ID>`，可改 |

点 **自动申请 EAB** → 成功后 KID/HMAC-Key 自动填入并保存，之后签发选 `CA = Google Public CA` **即可**（EAB 按 CA 自动关联，不用手选）。

> 服务账号 JSON 会**加密写入面板数据库的 EAB 配置里**（`json:"-"`，**永不返回前端**），
> 目的是：**7 天过期后能自动再申请一把**，不必每次人工去 GCP 点。

### 方式 B：临时 Access Token（只试一次）

```bash
gcloud auth print-access-token        # 约 1 小时有效
```

把它粘到"临时 Access Token"，项目 ID 照填，点 **自动申请 EAB**。
⚠ 这个 token **不会入库**，过期就要重新取 → 适合临时验证，不适合长期用。

### 命令行等价操作（可选，用于比对）

```bash
gcloud publicca external-account-keys create
# 返回 keyId / b64MacKey，对应 ACME 的 --eab-kid / --eab-hmac-key
```

## 4.5 为什么是 JSON，而不是"一条 key"？

这是最常被问的点，先把两种"key"分开：

| 你可能以为要的 | 实际要的 |
|---|---|
| **API Key**（`AIza…` 那串） | ❌ **没用** —— API Key 只是"公开标识"，**不携带身份**；而创建 EAB 需要 **IAM 权限**，必须用能证明身份的凭据 |
| —— | ✅ **服务账号密钥**：GCP「IAM → 服务账号 → 密钥 → 创建新密钥 → **JSON**」下载的那个**文件** |

要点：**那个 JSON 文件本身就是"一把 key"** —— 只是 GCP 把"账号标识（`client_email`）+ 私钥（`private_key`）"
打包成一个文件交付。面板拿它签 JWT → 换 OAuth2 access token → 再调 Public CA API。
所以"粘 JSON"≠ 要填很多东西，**粘整个文件内容即可**（面板也会自动补齐 `type` / `token_uri` 这类可省字段）。

**真正的 EAB 才是两条字符串**（`EAB-KID` + `EAB-HMAC-Key`），那是**申请的结果**，面板自动填好，不用手抄。

### 不想给这个文件？还有两条路

| 方式 | 怎么用 | 代价 |
|---|---|---|
| **临时 Access Token** | `gcloud auth print-access-token` → 粘到「临时 Access Token」 | 约 1 小时有效、**不落库** → 只适合临时验证，不能自动再申请 |
| **纯手填 KID/HMAC** | 本地 `gcloud publicca external-account-keys create` → 把 `keyId` / `b64MacKey` 填进「EAB-KID / EAB-HMAC-Key」 | 不用给任何凭据 ✓；但 Google 的 EAB 是 **7 天 + 一次性** → 过期要再点一次 |

> 面板推荐"服务账号 JSON"，唯一原因是：**EAB 7 天会过期**，存了服务账号才能**自动再申请**，不必人工回来点。

## 5. 自动注册与续期的关系（重点）

```
面板：选 CA=Google Public CA
  ├─ 有可用 EAB（配置里存在）→ 直接用
  └─ 没有/已过期 → 用**存过的服务账号 JSON 现场再申请一把**（自动注册）
        │
节点/面板首次签发 ── 用 EAB 注册 ACME 账号（一次性！）
        │
账号落盘：data/acme/accounts/google.json（面板） / <Agent data_dir>/acme/accounts/google.json（节点）
        │
之后每次续期 ── 只用账号密钥，**再也不需要 EAB**
```

因此：

- **首次签发即自动注册**，不需要人工先去 GCP 点一次；
- 续期**不会**再消耗 EAB（前提：账号文件在、数据目录稳定）；
- ⚠ 节点侧 ACME 数据目录已固定为绝对路径（默认 `/var/lib/corefusion-agent/acme`），
  **不要随便改**，否则会被当成新账号 → 又要一把新 EAB。

## 6. 排错对照表

| 现象 / 报错 | 原因 | 处理 |
|---|---|---|
| `PERMISSION_DENIED`（403） | 服务账号没有 `roles/publicca.externalAccountKeyCreator` | 回到第 3 步 ④ |
| `SERVICE_DISABLED`（403） | 项目没启用 `publicca.googleapis.com` | 第 3 步 ② |
| `NOT_FOUND`（404） | 项目 ID 填错 / 项目已删除 | 核对项目 ID |
| 签发报 `externalAccountRequired` | 该 CA 没拿到 EAB | 面板里先申请/配置 EAB |
| 签发报 `Accounts must have at least one contact.` | 注册时没带邮箱 | 证书页配置 ACME 账号邮箱（面板会自动取默认账号） |
| 续期时又要 EAB / 报注册失败 | ACME 账号文件丢了（换了数据目录 / 被清理） | **恢复备份**；没有备份就只能用服务账号再申请一把新 EAB |
| 证书签发者从 Google 变成 Let's Encrypt | 续期指令没带 CA（老版本 bug，v0.12.101 已修） | 升级并重签一次 |

## 7. 安全与运维建议

1. **最小权限**：服务账号只给 `roles/publicca.externalAccountKeyCreator`，不要给 Owner/Editor。
2. **别删项目**：删了 GCP 项目 → 该项目关联的 ACME 账号全部失效 → 证书无法续期。
3. **务必备份** `data/acme/accounts/`（这是账号身份本身）与面板数据库里的 EAB 配置。
4. **EAB 过期不用慌**：只要 ACME 账号文件在，续期就不受影响；只有"要注册新账号"时才需要新 EAB。
5. 服务账号密钥若怀疑泄漏：GCP 里删掉该密钥 → 面板重新粘一份新 JSON 即可。

## 8. 相关接口（便于脚本化）

| 接口 | 说明 |
|---|---|
| `POST /api/v1/eab-configs/fetch-google` | 用 GCP 授权创建 EAB。body：`{project_id, service_account_json?, access_token?}` |
| `POST /api/v1/eab-configs/fetch-zerossl` | 用邮箱向 ZeroSSL 领取 EAB。body：`{email}` |
| `GET  /api/v1/eab-configs` | 列出 EAB 配置（**不含** HMAC-Key 与服务账号 JSON，只回 `has_mac_key` / `has_google_sa`） |
