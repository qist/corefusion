<!-- source: LEGAL.md | locale: zh-TW | updated: 2026-10-23 -->
# 法律與合規說明

> 語言： [简体中文](LEGAL.md) · [English](LEGAL.en.md) · 繁體中文 · [Русский](LEGAL.ru.md) · [日本語](LEGAL.ja.md) · [한국어](LEGAL.ko.md) · [Deutsch](LEGAL.de.md) · [Français](LEGAL.fr.md) · [Español](LEGAL.es.md) · [Português](LEGAL.pt.md) · [Türkçe](LEGAL.tr.md) · [Bahasa Indonesia](LEGAL.id.md) · [ไทย](LEGAL.th.md) · [Tiếng Việt](LEGAL.vi.md) · [العربية](LEGAL.ar.md) · [فارسی](LEGAL.fa.md)

> 本文件說明 CoreFusion 的許可證、商業使用政策、第三方元件義務以及使用者在各地區應自行關注的合規事項。
>
> **本文件不構成法律意見。** 具體業務是否合規取決於部署地區、服務物件和運營模式，請諮詢當地執業律師。

## 1. 專案許可證

CoreFusion 以 **GNU Affero General Public License v3.0（AGPL-3.0）** 釋出，全文見 [LICENSE](./LICENSE)。

```
Copyright (C) 2026 qist

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published
by the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
```

## 2. 商業使用與商業託管：明確允許

**任何個人或組織都可以將 CoreFusion 用於商業目的，包括但不限於：**

- 自建自用、內部部署；
- 對外提供收費的代理/節點運營服務；
- 以 SaaS、託管、代運維、系統整合等方式向第三方提供服務並收費；
- 二次開發、定製、分發（包括隨硬體或整體方案交付）。

**上述商業使用和商業託管無需向本專案另行申請授權，也無需支付任何許可費或分成。**

唯一需要遵守的是 AGPL-3.0 本身的義務，核心有兩條：

1. **分發二進位制時**（§4–§6）：向接收者提供 Corresponding Source（完整對應原始碼），或按 §6 提供有效的書面獲取途徑；保留版權宣告和許可證文字。
2. **修改後透過網路提供服務時**（§13）：如果釋出了修改版，並讓第三方透過網路與修改版互動，必須向這些使用者**免費提供修改版的 Corresponding Source**（例如在面板顯著位置放置原始碼下載/倉庫連結）。

補充說明：

- **未修改**地原樣部署並對外提供服務，不因"託管"這一行為觸發 §13 的原始碼提供義務；但分發二進位制仍適用第 1 條。
- 收費本身不被禁止；AGPL 約束的是**原始碼提供**，而不是商業模式。修改版對外提供服務時不能以"商業/閉源運營"為由拒絕履行 §13。
- 如果你希望獲得**不受 AGPL 約束的閉源商業授權**，需要單獨聯絡版權人；注意本專案連結了第三方 GPL-3.0 元件（見第 3 節），任何閉源版本都必須先移除這些元件。

## 3. 第三方元件與再分發

主要第三方元件及其許可證見 [THIRD_PARTY_LICENSES.md](./THIRD_PARTY_LICENSES.md)。

特別提示：

- 本專案透過 `github.com/xtls/xray-core` 間接連結了 **GPL-3.0-or-later** 的 `github.com/sagernet/sing` 和 `sing-shadowsocks`。因此**分發面板或 Agent 二進位制時，必須同時滿足 GPL-3.0 的對應原始碼提供義務**。
- 面板執行時可下載/呼叫 Xray-core、sing-box、Caddy 等獨立程式。這些程式**不在本倉庫內分發**，但如果你在交付物中一併分發，必須遵守各自的許可證（尤其是 sing-box 的 GPL-3.0：需提供其對應原始碼）。Hysteria2、TUIC 是 sing-box 承載的協議，不是獨立二進位制。
- 專案附帶的 GeoIP/GeoSite 資料（如 GeoLite2、`geoip.dat`、`geosite.dat`）適用各自的資料許可；商用前請確認資料來源的授權條款。

## 4. 地區合規與使用者責任

CoreFusion 是**通用軟體**：它不提供任何代理或網路服務，不內建節點、頻寬或憑據，也不代收任何費用。實際運營由部署者負責。

部署者/運營者應自行確認其業務在所在司法轄區是否需要許可、備案或安全評估，包括但不限於：

- **中國大陸**：增值電信業務經營許可（ICP/IDC/ISP 等）、ICP 備案、網路安全等級保護、跨境資料傳輸安全評估，以及《網路安全法》《資料安全法》《個人資訊保護法》下的義務。不得將本軟體用於違反當地法律法規的用途。
- **歐盟/英國**：GDPR / UK GDPR、ePrivacy、數字服務法（DSA）下的資料處理者/控制者義務、日誌與 Cookie 合規。
- **美國**：各州隱私法、加密軟體出口管制（EAR）與制裁名單篩查。
- **其他地區**：當地電信、加密通訊、內容管理、資料本地化和消費者保護法規。

使用者承諾不將本軟體用於：

- 違反適用法律或出口管制/制裁規定的活動；
- 侵犯他人智慧財產權、隱私權或其他合法權益；
- 傳播違法內容、實施網路攻擊、欺詐或其他犯罪活動。

**因使用或誤用本軟體產生的任何法律責任，由使用者自行承擔。**

## 5. 資料保護與隱私

面板在執行過程中可能儲存賬號、聯絡方式、訂單、支付回撥、訂閱、流量統計、登入與操作日誌等資料。運營者作為資料處理者/控制者，應自行：

- 向使用者提供隱私政策並取得必要授權；
- 遵循最小必要原則，設定合理的日誌與統計資料保留期限（專案提供日誌清理策略，需運營者配置）；
- 支援使用者查詢、更正、刪除其個人資訊的請求；
- 對資料庫、備份、證書私鑰和支付憑據採取訪問控制與加密措施；
- 在跨境部署時完成相應的資料傳輸合規評估。

## 6. 免責宣告

本軟體按 AGPL-3.0 第 15–17 條"按原樣"提供，不附帶任何明示或默示的擔保，包括但不限於適銷性、特定用途適用性和不侵權的擔保。版權人不對因使用本軟體產生的直接、間接、偶然或後果性損失承擔責任。

## 7. 聯絡

- 專案地址：<https://github.com/qist/corefusion>
- 商業授權 / 合規問題：請透過專案 Issue 或倉庫主頁公佈的聯絡方式溝通。
