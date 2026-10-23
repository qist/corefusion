<!-- source: LEGAL.md | locale: en | updated: 2026-10-23 -->
# Legal and Compliance Notes

> Languages: [简体中文](LEGAL.md) · English · [繁體中文](LEGAL.zh-TW.md) · [Русский](LEGAL.ru.md) · [日本語](LEGAL.ja.md) · [한국어](LEGAL.ko.md) · [Deutsch](LEGAL.de.md) · [Français](LEGAL.fr.md) · [Español](LEGAL.es.md) · [Português](LEGAL.pt.md) · [Türkçe](LEGAL.tr.md) · [Bahasa Indonesia](LEGAL.id.md) · [ไทย](LEGAL.th.md) · [Tiếng Việt](LEGAL.vi.md) · [العربية](LEGAL.ar.md) · [فارسی](LEGAL.fa.md)

> This document explains CoreFusion's license, commercial-use policy, third-party obligations, and the compliance points users should watch in their own jurisdictions.
>
> **This document is not legal advice.** Compliance depends on your deployment region, customers and operating model; consult a qualified local lawyer.
>
> *This translation is provided for convenience. If it differs from the Chinese original, the Chinese version prevails.*

## 1. Project license

CoreFusion is released under the **GNU Affero General Public License v3.0 (AGPL-3.0)**; see [LICENSE](./LICENSE) for the full text.

```
Copyright (C) 2026 qist

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published
by the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
```

## 2. Commercial use and commercial hosting: expressly allowed

**Any individual or organization may use CoreFusion for commercial purposes, including but not limited to:**

- Self-hosting and internal deployment;
- Running a paid proxy/node service for third parties;
- Providing SaaS, hosting, managed operations or systems integration to third parties for a fee;
- Secondary development, customization and distribution (including delivery with hardware or as part of a solution).

**The above commercial use and commercial hosting require no separate authorization and no license fee or revenue share.**

The only obligations are those of AGPL-3.0 itself, mainly two:

1. **When distributing binaries** (§4–§6): provide recipients with the Corresponding Source (complete corresponding source), or an effective written offer under §6; retain copyright notices and the license text.
2. **When providing a modified version over a network** (§13): if you publish a modified version and let third parties interact with it over a network, you must **offer those users the Corresponding Source of the modified version free of charge** (for example, a prominent source download/repository link in the panel).

Additional notes:

- Deploying an **unmodified** copy and serving users does not, by itself, trigger the §13 source obligation; distributing binaries still falls under item 1.
- Charging money is not prohibited; AGPL governs **source availability**, not the business model. You cannot refuse §13 for a modified version simply because you operate commercially or closed-source.
- If you want a **closed-source commercial license not subject to AGPL**, contact the copyright holder separately. Note that this project links third-party GPL-3.0 components (see section 3); any closed-source version must remove them first.

## 3. Third-party components and redistribution

The main third-party components and their licenses are listed in [THIRD_PARTY_LICENSES.md](./THIRD_PARTY_LICENSES.md).

Special notes:

- Through `github.com/xtls/xray-core`, this project indirectly links **GPL-3.0-or-later** `github.com/sagernet/sing` and `sing-shadowsocks`. Therefore **distributing panel or Agent binaries also requires satisfying GPL-3.0's corresponding-source obligations**.
- At runtime the panel may download or invoke standalone programs such as Xray-core, sing-box and Caddy. These are **not distributed in this repository**, but if you ship them with your deliverables you must comply with their licenses (especially sing-box's GPL-3.0: provide its corresponding source). Hysteria2 and TUIC are protocols carried by sing-box, not standalone binaries.
- Bundled GeoIP/GeoSite data (GeoLite2, `geoip.dat`, `geosite.dat`) is subject to its own data licenses; verify the terms before commercial use.

## 4. Regional compliance and user responsibility

CoreFusion is **general-purpose software**: it does not provide any proxy or network service, ships no nodes, bandwidth or credentials, and collects no fees. Operations are the deployer's responsibility.

Deployers/operators must determine whether their business requires licenses, filings or security assessments in their jurisdiction, including but not limited to:

- **Mainland China**: value-added telecom business licenses (ICP/IDC/ISP, etc.), ICP filing, MLPS, cross-border data transfer security assessment, and obligations under the Cybersecurity Law, Data Security Law and Personal Information Protection Law. Do not use this software for purposes that violate local laws.
- **EU/UK**: GDPR / UK GDPR, ePrivacy, Digital Services Act (DSA) processor/controller obligations, logging and cookie compliance.
- **United States**: state privacy laws, encryption export controls (EAR) and sanctions screening.
- **Other regions**: local telecom, encrypted communications, content management, data localization and consumer protection rules.

Users agree not to use this software for:

- Activities that violate applicable law or export control/sanctions rules;
- Infringing intellectual property, privacy or other lawful rights;
- Distributing illegal content, network attacks, fraud or other criminal activity.

**Any legal liability arising from use or misuse of this software rests with the user.**

## 5. Data protection and privacy

The panel may store accounts, contact details, orders, payment callbacks, subscriptions, traffic statistics and sign-in/operation logs. As processor/controller, operators must:

- Provide users with a privacy policy and obtain necessary consent;
- Follow data minimization and set reasonable retention periods for logs and statistics (the project provides log cleanup policies that operators must configure);
- Support requests to access, correct and delete personal information;
- Apply access control and encryption to databases, backups, certificate private keys and payment credentials;
- Complete cross-border data transfer assessments when deploying across borders.

## 6. Disclaimer

This software is provided “as is” under AGPL-3.0 §§15–17, without any express or implied warranty, including merchantability, fitness for a particular purpose and non-infringement. The copyright holder is not liable for direct, indirect, incidental or consequential damages arising from use of this software.

## 7. Contact

- Project: <https://github.com/qist/corefusion>
- Commercial licensing / compliance questions: use the project Issues or the contact details published on the repository page.
