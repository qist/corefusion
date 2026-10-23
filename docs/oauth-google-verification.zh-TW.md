<!-- source: oauth-google-verification.md | locale: zh-TW | updated: 2026-10-23 -->
# Google OAuth 正式授權（應用驗證）指南

> 語言： [简体中文](oauth-google-verification.md) · [English](oauth-google-verification.en.md) · 繁體中文 · [Русский](oauth-google-verification.ru.md) · [日本語](oauth-google-verification.ja.md) · [한국어](oauth-google-verification.ko.md) · [Deutsch](oauth-google-verification.de.md) · [Français](oauth-google-verification.fr.md) · [Español](oauth-google-verification.es.md) · [Português](oauth-google-verification.pt.md) · [Türkçe](oauth-google-verification.tr.md) · [Bahasa Indonesia](oauth-google-verification.id.md) · [ไทย](oauth-google-verification.th.md) · [Tiếng Việt](oauth-google-verification.vi.md) · [العربية](oauth-google-verification.ar.md) · [فارسی](oauth-google-verification.fa.md)

> 適用範圍：CoreFusion 門戶的 Google 第三方登入（`internal/service/oauth`，scopes：`openid`、`email`、`profile`）。
> GitHub 對照說明見文末（GitHub 無需驗證流程）。

## 1. 測試模式 vs 正式（釋出）模式

Google OAuth 同意螢幕（OAuth consent screen）有兩種釋出狀態：

| 狀態 | 誰能登入 | 是否需要驗證 |
|---|---|---|
| **測試（Testing）** | 僅"測試使用者"列表裡的 Google 賬號 | 否 |
| **正式（In production）** | 任何 Google 賬號 | **是**（見下） |

面板"系統設定 → OAuth"啟用後，門戶登入頁會對所有訪客顯示 Google 按鈕；
若此時應用仍是測試狀態，未加入測試使用者列表的訪客會看到
`Error 403: access_denied`（應用未驗證 / 您未被列為測試使用者）。
要對外開放 Google 登入，**必須把應用切到正式狀態並透過驗證**。

## 2. 正式授權（驗證）的硬性要求

Google 稽核 OAuth 應用（非受限 scope 的"基礎驗證"）要求：

1. **應用主頁（Home page）**：公共可訪問的 URL，須與授權域名一致（同域或隸屬域名）。
2. **隱私權政策（Privacy Policy）連結**：必須**公共可訪問**（無需登入即可開啟），
   說明你收集哪些資料、如何使用/儲存/刪除。Google 登入場景至少要覆蓋：
   收集的 Google 賬號資訊（郵箱、姓名、頭像）、用途（建立/登入門戶賬號）、不向第三方出售、
   使用者可登出/刪除賬號及資料的途徑。
3. **服務條款（Terms of Service）連結**：公共可訪問，說明服務使用規則。
4. **授權域名（Authorized domains）**：在同意螢幕裡登記應用所屬域名（如 `example.com`），
   主頁/隱私/條款連結的域名必須是其本身或其子域。
5. **應用徽標（可選但建議）**：120×120px。
6. **scopes 說明**：本專案只用 `openid / email / profile`（非受限、非敏感 scope），
   屬於"基本驗證"——不需要逐 scope 論證，也**不需要第三方安全評估**。
   （受限 scope 如 Gmail 全量讀寫才需要，本專案不涉及。）

驗證入口：[Google Cloud Console](https://console.cloud.google.com/) →
**API 和服務 → OAuth 同意螢幕 → Publish App（切換到正式）→ Submit for verification**。
基礎驗證一般數日內完成；未驗證期間仍可先用測試模式運營。

## 3. 本專案需要配置的位置

### 3.1 回撥地址（Authorized redirect URI）

回撥按"發起登入的域名"動態生成（見 `internal/api/handlers/oauth/handler.go` Authorize）：

```
https://<門戶訪問域名>/api/v1/client-auth/oauth/google/callback
```

- 從門戶域名（如 `https://p.example.com`）發起登入 → 註冊 `https://p.example.com/...` 的回撥；
- 從面板域名 `/p` 發起 → 註冊 `https://panel.example.com/...`。
- Google 允許配置**多個** redirect URI，兩套域名都登記即可。
- ⚠ Google 強制要求 redirect URI 為 **https**（`http://localhost` 除外）；
  反代必須傳對 `X-Forwarded-Host` / `X-Forwarded-Proto`，否則面板拼出的 redirect_uri
  會是 http 或內網地址 → Google 直接拒絕（`redirect_uri_mismatch`）。

面板裡"系統設定 → OAuth → 門戶回撥域名"填上門戶域名後，
頁面顯示的回撥地址就是需要粘到 Google 後臺的地址。

### 3.2 門戶頁上的隱私政策 / 服務條款連結

登入卡片底部自帶"隱私政策 · 服務條款"連結（配置後顯示 ✓），且兩份文件都有**獨立可訪問頁面**
（登入卡片彈窗與獨立頁面共用同一份內容）：

- 主面板域名：`https://<面板域名>/p/legal/privacy`、`/p/legal/terms`
- 獨立門戶域名（根路徑部署）：`https://<門戶域名>/legal/privacy`、`/legal/terms`
- 支援 `?lang=xx` 切換語言；不帶引數時按瀏覽器 `Accept-Language`，再回退門戶預設語言。
- 頁面為**服務端渲染**（不依賴 JS），便於 Google 驗證抓取。

**配置入口（二選一，連結優先）**：

1. **內容方式（推薦，開箱即用）**：門戶編輯器 → 設定 → 基礎設定 → 合規文件 →
   選擇「編輯語言」後把 §4 模板文字粘進"隱私政策內容"/"服務條款內容"
   （**Markdown，按語言分別編輯**）→ 儲存後登入卡片顯示連結，點選**彈窗展示**；
   編輯器裡的"獨立訪問連結"可直接填入 Google 同意螢幕。
2. **外鏈方式**：填"隱私政策連結"/"服務條款連結"（指向你託管的公共頁面）→ 點選新標籤開啟。

內容與連結經 `/api/v1/public/portal-config` 下發（`privacy_policy_content` /
`terms_of_service_content` 為**按語言的 markdown 對映** `{"zh-CN":"...","en":"..."}`；
`privacy_policy_url` / `terms_of_service_url` 為可選外鏈）。**要求：未登入也能開啟** ✓。

### 3.3 面板側開關

`系統設定 → OAuth`：啟用 Google、填 Client ID / Client Secret（Secret AES-256-GCM 加密落庫）。
配置齊全後門戶登入頁才會顯示 Google 按鈕（`/api/v1/client-auth/oauth/providers` 返回 enabled）。

## 4. 隱私權政策 / 服務條款模板

> 以下為可直接改編的最小合規模板。使用前：替換 `【】` 佔位、按實際業務增刪條款、
> 由運營主體審閱後釋出。本模板不構成法律意見。

### 4.1 隱私權政策（模板）

```markdown
# 隱私權政策

最後更新：【日期】

【運營主體名稱】（"我們"）運營【門戶名稱】（"本服務"）。我們尊重您的隱私，
本政策說明我們收集哪些資訊以及如何使用它們。

## 1. 我們收集的資訊
- **賬號資訊**：您註冊時提供的電子郵箱、密碼（僅以單向雜湊儲存）；
- **Google 登入資訊**：您選擇"使用 Google 登入"時，我們經 Google 授權獲取
  您的 Google 賬號郵箱、暱稱與頭像 URL（scopes: openid, email, profile）；
- **服務必要資料**：訂閱配置、流量用量、裝置 IP 白名單、工單與訂單記錄。

## 2. 我們如何使用資訊
僅用於：建立與登入您的賬號、提供代理訂閱服務、計費與客服、
安全風控（登入保護、IP 白名單）。我們**不會**出售您的個人資訊。

## 3. 第三方服務
- **Google**：僅在您主動使用"Google 登入"時接收授權範圍內的賬號資訊；
- 支付渠道：僅在您發起支付時接收訂單資訊。

## 4. 儲存與安全
資料儲存於我們的伺服器，憑據經加密儲存，傳輸全程 TLS。

## 5. 您的權利
您可以隨時在門戶內修改資料/密碼、登出會話；如需刪除賬號及其全部資料，
請聯絡 【聯絡郵箱】，我們將在【N】個工作日內處理。

## 6. 政策變更
重大變更將在門戶公告欄公示。
```

### 4.2 服務條款（模板）

```markdown
# 服務條款

最後更新：【日期】

## 1. 服務內容
本服務向註冊使用者提供【代理/網路接入】服務，具體套餐以商店頁面為準。

## 2. 賬號
- 您須對賬號與密碼安全負責；透過 Google 登入建立的賬號，其憑據由 Google 託管；
- 禁止出借、轉讓賬號，禁止將服務用於任何違法用途（包括但不限於：
  未經授權訪問他人系統、分發違法內容、繞過司法管轄的強制規定）。

## 3. 付費與退款
套餐按【月/量】計費，流量與裝置數以套餐說明為準；
【退款政策：如 7 天內未使用可全額退款 等】。

## 4. 服務變更與終止
我們可因不可抗力或您違反本條款而暫停/終止服務；賬號刪除時相關資料一併刪除。

## 5. 免責宣告
服務按"現狀"提供；對因使用本服務造成的間接損失，我們在法律允許範圍內不承擔責任。

## 6. 其他
本條款受【司法轄區】法律管轄。聯絡：【聯絡郵箱】。
```

## 5. 上線前自查清單

- [ ] OAuth 同意螢幕：應用名稱 / 徽標 / 主頁 / 隱私政策 / 條款 五項齊全，域名已登記
- [ ] 隱私政策與服務條款頁面**未登入可訪問**（門戶自定義頁或獨立靜態頁）
- [ ] 門戶登入卡片/頁尾可見兩個連結
- [ ] redirect URI 與實際訪問域名一致且為 https
- [ ] Publish App → 提交驗證，透過後測試使用者限制解除
- [ ] 面板 OAuth 設定：enabled=true 且 Client ID/Secret 已填（providers 介面返回 enabled:true）

## 6. GitHub 對照

GitHub OAuth App **沒有**釋出狀態/驗證流程：建立 App、填回撥 URL 即可對外用。
僅需注意 GitHub OAuth App 的 Authorization callback URL 只能填**一個**（按字首匹配），
多域名場景用 Google 那套多 URI 或為每個門戶域名建獨立 App。
