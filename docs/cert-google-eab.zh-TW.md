<!-- source: cert-google-eab.md | locale: zh-TW | updated: 2026-10-23 -->
# Google Public CA 配置指南（EAB 自動申請）

> 語言： [简体中文](cert-google-eab.md) · [English](cert-google-eab.en.md) · 繁體中文 · [Русский](cert-google-eab.ru.md) · [日本語](cert-google-eab.ja.md) · [한국어](cert-google-eab.ko.md) · [Deutsch](cert-google-eab.de.md) · [Français](cert-google-eab.fr.md) · [Español](cert-google-eab.es.md) · [Português](cert-google-eab.pt.md) · [Türkçe](cert-google-eab.tr.md) · [Bahasa Indonesia](cert-google-eab.id.md) · [ไทย](cert-google-eab.th.md) · [Tiếng Việt](cert-google-eab.vi.md) · [العربية](cert-google-eab.ar.md) · [فارسی](cert-google-eab.fa.md)

> 適用版本：面板 v0.12.101+ ｜ 相關頁面：**證書 → EAB 配置**
> 相關程式碼：`internal/service/certificate/eab_google.go`、`internal/api/handlers/certificates/eab_config.go`

## 1. 為什麼 Google 必須要 EAB

| CA | ACME 是否強制 EAB | EAB 怎麼來 |
|---|---|---|
| Let's Encrypt | ❌ 不需要 | —— |
| ZeroSSL | ✅ 必須 | **一個郵箱即可換**（`api.zerossl.com/acme/eab-credentials-email`，acme.sh 同機制），面板可一鍵自動領取 |
| **Google Public CA** | ✅ **必須** | **必須用 GCP 授權去建立**（不能靠郵箱換）→ 本文重點 |

也就是說：Google 這條路**繞不開"授權"**——要麼用服務賬號 JSON，要麼用臨時 access token。

## 2. Google 官方硬性規則（務必先讀）

| 規則 | 說明 |
|---|---|
| **7 天內必須使用** | EAB 建立後 7 天內不用就**作廢**，需要重新建立 |
| **只能註冊一個賬號** | 一把 EAB 只能註冊一個 ACME 賬號，註冊成功後該 EAB **即失效**，不能重複用 |
| **ACME 賬號不過期** | 用它註冊出來的 ACME 賬號**長期有效** → **續期不需要 EAB** |
| ⚠ **刪除 GCP 專案 = 賬號全廢** | 刪專案會讓該專案下**所有 ACME 賬號立即失效**，**無法再續期** |

> 結論：**EAB 只是一次性的"入網憑證"，真正長期的是 ACME 賬號**。
> 所以面板會把賬號落盤到 `data/acme/accounts/<ca>.json` —— **這個目錄必須備份**。

## 3. 三條路，挑一條（**先看路徑 A，完全不用找選單**）

> 控制檯的入口改過好幾次，所以下面**優先給直鏈和 Cloud Shell**，最後才給"點選單"的備用路徑。

### 路徑 A（最省事，推薦先跑通這個）：GCP Cloud Shell 跑一條命令

Cloud Shell 是瀏覽器裡的終端，**gcloud 已裝好且已登入**，不用本機裝任何東西，也不用找選單。

1. 開啟 <https://console.cloud.google.com/cloudshell>（控制檯右上角的 `>_` 圖示也行）
2. 選定專案（專案 ID 在控制檯頂部的專案選擇器裡能看到）：
   ```bash
   gcloud config set project <你的專案ID>
   ```
3. 啟用 Public CA API（**這一條就等於**在控制檯裡翻"API 庫"點啟用）：
   ```bash
   gcloud services enable publicca.googleapis.com
   ```
4. 建立 EAB（**這就是最終要的兩條字串**）：
   ```bash
   gcloud publicca external-account-keys create
   ```
   輸出裡：`keyId` → 面板的 **EAB-KID**；`b64MacKey` → 面板的 **EAB-HMAC-Key**
5. 回面板：**證書 → EAB 配置 → 新增 EAB 配置 → CA 選 Google Public CA** →
   把上面兩條**手填**進 `EAB-KID` / `EAB-HMAC-Key` → 儲存 → 完事 ✓

> ⚠ 代價：Google 的 EAB 是 **7 天 + 一次性** ✗ → 7 天后（或換新賬號時）要**再跑一次第 4 步**。
> 想免掉這個 → 走路徑 B（存一次服務賬號，之後面板自動再申請）。

### 路徑 B（一次性投入，之後全自動）：服務賬號金鑰 JSON

**① 啟用 Public CA API**（直鏈最省事）
<https://console.cloud.google.com/apis/library/publicca.googleapis.com?project=你的專案ID> → 點「啟用」
備用路徑：左上 ☰ → **API 和服務 → 庫** → 搜 `Public CA` → 點進去 → 「啟用」

**② 建立服務賬號**
<https://console.cloud.google.com/iam-admin/serviceaccounts?project=你的專案ID>
→ 頂部「**+ 建立服務賬號**」→ 名字隨意（如 `corefusion-eab`）→ 「建立並繼續」→ 角色那一步可**先跳過** → 「完成」
備用路徑：☰ → **IAM 和管理 → 服務賬號**

**③ 授予最小角色**（就在同一個頁面最方便）
服務賬號列表裡**點進剛建的那個** → **「許可權」標籤** → 「授予訪問許可權」→ 新主填**服務賬號郵箱**
→ 角色裡搜 `publicca` → 選 **「Public CA External Account Key Creator」** → 儲存
備用路徑：☰ → **IAM 和管理 → IAM** → 「授予訪問許可權」→ 同樣填郵箱 + 角色
⚠ **不要給 Owner/Editor** ✗

**④ 下載金鑰 JSON**
服務賬號列表 → 點進該賬號 → **「金鑰」標籤** → 「**新增金鑰**」→「**建立新金鑰**」→ 選 **JSON** → 建立
（瀏覽器會直接下載一個 `xxx.json`；或列表行右側 `⋮` → 「管理金鑰」→ 同上）

**⑤ 回面板**：填 **專案 ID** + 把那個 `xxx.json` 的**全部內容**粘進「服務賬號金鑰檔案」→ 點「自動申請 EAB」
→ 以後 EAB 過期面板會**自動再申請** ✓，不用再進 GCP ✓

### 路徑 C（只臨時試一次）：直接粘 access token

在 Cloud Shell 裡：
```bash
gcloud auth print-access-token
```
把輸出粘到面板的「臨時 Access Token」+ 填專案 ID → 點「自動申請 EAB」
⚠ 約 1 小時有效、**不落庫** ✓ → 只適合驗證一次，沒法自動再申請 ✗

### 找不到入口時的對照表（控制檯選單改過好幾次）

| 要做什麼 | 直鏈（最穩，推薦） | 備用：點選單 |
|---|---|---|
| 啟用 Public CA API | `https://console.cloud.google.com/apis/library/publicca.googleapis.com?project=<專案ID>` | ☰ → API 和服務 → 庫 → 搜 `Public CA` |
| 服務賬號 | `https://console.cloud.google.com/iam-admin/serviceaccounts?project=<專案ID>` | ☰ → IAM 和管理 → 服務賬號 |
| 授角色 | `https://console.cloud.google.com/iam-admin/iam?project=<專案ID>` | ☰ → IAM 和管理 → IAM → 授予訪問許可權 |
| Cloud Shell | `https://console.cloud.google.com/cloudshell` | 控制檯右上角 `>_` 圖示 |

> 提示：把連結裡的 `<專案ID>` / `你的專案ID` 換成真實專案 ID（形如 `my-project-123456`）即可直接跳到對應頁面。

## 4. 在面板裡配置（兩種方式）

進入 **證書 → EAB 配置 → 新增 EAB 配置**，CA 選 **Google Public CA**：

### 方式 A（推薦）：粘服務賬號 JSON

| 欄位 | 填什麼 |
|---|---|
| GCP 專案 ID | 第 3 步的專案 ID，如 `my-project-123456` |
| 服務賬號 JSON | 把 `corefusion-eab.json` **整個檔案內容**粘進來 |
| 臨時 Access Token | 留空 |
| 名稱 | 自動形如 `Google Public CA - <專案ID>`，可改 |

點 **自動申請 EAB** → 成功後 KID/HMAC-Key 自動填入並儲存，之後簽發選 `CA = Google Public CA` **即可**（EAB 按 CA 自動關聯，不用手選）。

> 服務賬號 JSON 會**加密寫入面板資料庫的 EAB 配置裡**（`json:"-"`，**永不返回前端**），
> 目的是：**7 天過期後能自動再申請一把**，不必每次人工去 GCP 點。

### 方式 B：臨時 Access Token（只試一次）

```bash
gcloud auth print-access-token        # 約 1 小時有效
```

把它粘到"臨時 Access Token"，專案 ID 照填，點 **自動申請 EAB**。
⚠ 這個 token **不會入庫**，過期就要重新取 → 適合臨時驗證，不適合長期用。

### 命令列等價操作（可選，用於比對）

```bash
gcloud publicca external-account-keys create
# 返回 keyId / b64MacKey，對應 ACME 的 --eab-kid / --eab-hmac-key
```

## 4.5 為什麼是 JSON，而不是"一條 key"？

這是最常被問的點，先把兩種"key"分開：

| 你可能以為要的 | 實際要的 |
|---|---|
| **API Key**（`AIza…` 那串） | ❌ **沒用** —— API Key 只是"公開標識"，**不攜帶身份**；而建立 EAB 需要 **IAM 許可權**，必須用能證明身份的憑據 |
| —— | ✅ **服務賬號金鑰**：GCP「IAM → 服務賬號 → 金鑰 → 建立新金鑰 → **JSON**」下載的那個**檔案** |

要點：**那個 JSON 檔案本身就是"一把 key"** —— 只是 GCP 把"賬號標識（`client_email`）+ 私鑰（`private_key`）"
打包成一個檔案交付。面板拿它籤 JWT → 換 OAuth2 access token → 再調 Public CA API。
所以"粘 JSON"≠ 要填很多東西，**粘整個檔案內容即可**（面板也會自動補齊 `type` / `token_uri` 這類可省欄位）。

**真正的 EAB 才是兩條字串**（`EAB-KID` + `EAB-HMAC-Key`），那是**申請的結果**，面板自動填好，不用手抄。

### 不想給這個檔案？還有兩條路

| 方式 | 怎麼用 | 代價 |
|---|---|---|
| **臨時 Access Token** | `gcloud auth print-access-token` → 粘到「臨時 Access Token」 | 約 1 小時有效、**不落庫** → 只適合臨時驗證，不能自動再申請 |
| **純手填 KID/HMAC** | 本地 `gcloud publicca external-account-keys create` → 把 `keyId` / `b64MacKey` 填進「EAB-KID / EAB-HMAC-Key」 | 不用給任何憑據 ✓；但 Google 的 EAB 是 **7 天 + 一次性** → 過期要再點一次 |

> 面板推薦"服務賬號 JSON"，唯一原因是：**EAB 7 天會過期**，存了服務賬號才能**自動再申請**，不必人工回來點。

## 4.6 FAQ：能不能用 API Key？

**不能。** 這不是"我們的實現選擇"，而是 **Google 明確拒絕** —— 實測（兩種寫法結果一致）：

```bash
# ① 查詢引數寫法
curl -X POST "https://publicca.googleapis.com/v1/projects/<ID>/locations/global/externalAccountKeys?key=AIza..." -d '{}'
# ② 官方文件推薦的 x-goog-api-key 頭寫法
curl -X POST "https://publicca.googleapis.com/v1/projects/<ID>/locations/global/externalAccountKeys" \
     -H "x-goog-api-key: AIza..." -d '{}'
```

兩者都返回：

```json
{"error":{"code":401,"status":"UNAUTHENTICATED",
  "message":"API keys are not supported by this API. Expected OAuth2 access token
             or other authentication credentials that assert a principal."}}
```

對照：換成**（假的）OAuth Bearer** 時報的是
`Request had invalid authentication credentials. Expected OAuth 2 access token...` ——
說明 OAuth 這條它會去認、而 **API Key 直接被拒** ✗。

**為什麼**：建立 EAB 是**管理類操作**，要判"你是誰、有沒有 `roles/publicca.externalAccountKeyCreator`" ✗
—— API Key 只是"哪個專案/哪個應用的標識" ✓，**不表達主體身份** ✓。
（《管理 API 金鑰的最佳實踐》那篇文件講的是"API Key 怎麼用才安全"，它建議管理類場景改用
**IAM + 短時服務賬號憑證** ✓ —— 與本頁做法一致 ✓。）

**所以只有三條路** ✓（見 §3）：服務賬號 JSON ✓ / 臨時 access token ✓ /
或者**根本不用憑據** —— 在 Cloud Shell 裡 `gcloud publicca external-account-keys create` ✓
把 `keyId` / `b64MacKey` 手填進面板 ✓（**卡在 GCP IAM 的話就走這條** ✓）。

## 5. 自動註冊與續期的關係（重點）

```
面板：選 CA=Google Public CA
  ├─ 有可用 EAB（配置裡存在）→ 直接用
  └─ 沒有/已過期 → 用**存過的服務賬號 JSON 現場再申請一把**（自動註冊）
        │
節點/面板首次簽發 ── 用 EAB 註冊 ACME 賬號（一次性！）
        │
賬號落盤：data/acme/accounts/google.json（面板） / <Agent data_dir>/acme/accounts/google.json（節點）
        │
之後每次續期 ── 只用賬號金鑰，**再也不需要 EAB**
```

因此：

- **首次簽發即自動註冊**，不需要人工先去 GCP 點一次；
- 續期**不會**再消耗 EAB（前提：賬號檔案在、資料目錄穩定）；
- ⚠ 節點側 ACME 資料目錄已固定為絕對路徑（預設 `/var/lib/corefusion-agent/acme`），
  **不要隨便改**，否則會被當成新賬號 → 又要一把新 EAB。

## 6. 排錯對照表

| 現象 / 報錯 | 原因 | 處理 |
|---|---|---|
| `PERMISSION_DENIED`（403） | 服務賬號沒有 `roles/publicca.externalAccountKeyCreator` | 回到第 3 步 ④ |
| `SERVICE_DISABLED`（403） | 專案沒啟用 `publicca.googleapis.com` | 第 3 步 ② |
| `NOT_FOUND`（404） | 專案 ID 填錯 / 專案已刪除 | 核對專案 ID |
| 簽發報 `externalAccountRequired` | 該 CA 沒拿到 EAB | 面板裡先申請/配置 EAB |
| 簽發報 `Accounts must have at least one contact.` | 註冊時沒帶郵箱 | 證書頁配置 ACME 賬號郵箱（面板會自動取預設賬號） |
| 續期時又要 EAB / 報註冊失敗 | ACME 賬號檔案丟了（換了資料目錄 / 被清理） | **恢復備份**；沒有備份就只能用服務賬號再申請一把新 EAB |
| 證書籤發者從 Google 變成 Let's Encrypt | 續期指令沒帶 CA（老版本 bug，v0.12.101 已修） | 升級並重籤一次 |

## 7. 安全與運維建議

1. **最小許可權**：服務賬號只給 `roles/publicca.externalAccountKeyCreator`，不要給 Owner/Editor。
2. **別刪專案**：刪了 GCP 專案 → 該專案關聯的 ACME 賬號全部失效 → 證書無法續期。
3. **務必備份** `data/acme/accounts/`（這是賬號身份本身）與面板資料庫裡的 EAB 配置。
4. **EAB 過期不用慌**：只要 ACME 賬號檔案在，續期就不受影響；只有"要註冊新賬號"時才需要新 EAB。
5. 服務賬號金鑰若懷疑洩漏：GCP 裡刪掉該金鑰 → 面板重新粘一份新 JSON 即可。

## 8. 相關介面（便於指令碼化）

| 介面 | 說明 |
|---|---|
| `POST /api/v1/eab-configs/fetch-google` | 用 GCP 授權建立 EAB。body：`{project_id, service_account_json?, access_token?}` |
| `POST /api/v1/eab-configs/fetch-zerossl` | 用郵箱向 ZeroSSL 領取 EAB。body：`{email}` |
| `GET  /api/v1/eab-configs` | 列出 EAB 配置（**不含** HMAC-Key 與服務賬號 JSON，只回 `has_mac_key` / `has_google_sa`） |
