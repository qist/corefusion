<!-- source: cert-google-eab.md | locale: en | updated: 2026-10-23 -->
# Google Public CA Guide (automatic EAB issuance)

> Languages: [简体中文](cert-google-eab.md) · English · [繁體中文](cert-google-eab.zh-TW.md) · [Русский](cert-google-eab.ru.md) · [日本語](cert-google-eab.ja.md) · [한국어](cert-google-eab.ko.md) · [Deutsch](cert-google-eab.de.md) · [Français](cert-google-eab.fr.md) · [Español](cert-google-eab.es.md) · [Português](cert-google-eab.pt.md) · [Türkçe](cert-google-eab.tr.md) · [Bahasa Indonesia](cert-google-eab.id.md) · [ไทย](cert-google-eab.th.md) · [Tiếng Việt](cert-google-eab.vi.md) · [العربية](cert-google-eab.ar.md) · [فارسی](cert-google-eab.fa.md)

> Applies to: panel v0.12.101+ | Page: **Certificates → EAB configuration**
> Code: `internal/service/certificate/eab_google.go`, `internal/api/handlers/certificates/eab_config.go`

## 1. Why Google requires EAB

| CA | EAB mandatory for ACME? | How to get EAB |
|---|---|---|
| Let's Encrypt | ❌ No | — |
| ZeroSSL | ✅ Yes | **Trade an email for one** (`api.zerossl.com/acme/eab-credentials-email`, same as acme.sh); the panel can fetch it automatically |
| **Google Public CA** | ✅ **Yes** | **Must be created with GCP authorization** (an email is not enough) → the focus of this page |

In other words, Google **cannot avoid "authorization"**: either a service-account JSON file or a temporary access token.

## 2. Google's hard rules (read first)

| Rule | Description |
|---|---|
| **Use within 7 days** | An EAB key **expires** if unused within 7 days and must be recreated |
| **One account only** | One EAB key registers exactly one ACME account; after registration the key **is invalidated** and cannot be reused |
| **ACME accounts do not expire** | The ACME account created with it is **long-lived** → **renewals do not need EAB** |
| ⚠ **Deleting the GCP project kills all accounts** | Deleting the project immediately invalidates **all ACME accounts** under it and **renewal becomes impossible** |

> Conclusion: **EAB is a one-time "onboarding credential"; the ACME account is what lasts.**
> The panel stores accounts at `data/acme/accounts/<ca>.json` — **back up this directory**.

## 3. Three paths (start with path A — no menu hunting)

> The Cloud Console entry points have changed several times, so direct links and Cloud Shell come first; menu paths are fallbacks.

### Path A (easiest, recommended first): one command in GCP Cloud Shell

Cloud Shell is a browser terminal with **gcloud preinstalled and already signed in**; no local setup and no menus.

1. Open <https://console.cloud.google.com/cloudshell> (or the `>_` icon in the top-right of the console)
2. Select the project (the project ID is visible in the project picker at the top):
   ```bash
   gcloud config set project <your-project-id>
   ```
3. Enable the Public CA API (**equivalent** to enabling it in the API Library):
   ```bash
   gcloud services enable publicca.googleapis.com
   ```
4. Create the EAB (**these are the two strings you need**):
   ```bash
   gcloud publicca external-account-keys create
   ```
   In the output: `keyId` → the panel's **EAB-KID**; `b64MacKey` → the panel's **EAB-HMAC-Key**
5. Back in the panel: **Certificates → EAB configuration → Add EAB configuration → CA: Google Public CA** →
   enter the two values into `EAB-KID` / `EAB-HMAC-Key` → save ✓

> ⚠ Cost: Google EAB keys are **7-day, one-time** ✗ → after 7 days (or when switching accounts) **repeat step 4**.
> To avoid that, use path B (store a service account once and let the panel re-request automatically).

### Path B (one-time setup, then automatic): service-account key JSON

**① Enable the Public CA API** (direct link is easiest)
<https://console.cloud.google.com/apis/library/publicca.googleapis.com?project=YOUR_PROJECT_ID> → click “Enable”
Fallback: ☰ → **APIs & Services → Library** → search `Public CA` → open → “Enable”

**② Create a service account**
<https://console.cloud.google.com/iam-admin/serviceaccounts?project=YOUR_PROJECT_ID>
→ “**+ Create service account**” → any name (e.g. `corefusion-eab`) → “Create and continue” → you may **skip** the role step → “Done”
Fallback: ☰ → **IAM & Admin → Service accounts**

**③ Grant the minimal role** (easiest on the same page)
Click the newly created account → **“Permissions” tab** → “Grant access” → new principal: the **service account email**
→ search the role for `publicca` → choose **“Public CA External Account Key Creator”** → save
Fallback: ☰ → **IAM & Admin → IAM** → “Grant access” → same email + role
⚠ **Do not grant Owner/Editor** ✗

**④ Download the key JSON**
Service accounts → open the account → **“Keys” tab** → “**Add key**” → “**Create new key**” → **JSON** → create
(the browser downloads a `xxx.json`; or row `⋮` → “Manage keys” → same flow)

**⑤ Back in the panel**: enter the **project ID** and paste the **entire content** of the JSON into “Service account key file” → click “Request EAB automatically”
→ afterwards the panel **re-requests EAB automatically** when it expires ✓, no more GCP visits ✓

### Path C (one-off test only): paste an access token

In Cloud Shell:
```bash
gcloud auth print-access-token
```
Paste the output into the panel's “Temporary access token” plus the project ID → click “Request EAB automatically”
⚠ Valid for about 1 hour and **not stored** ✓ → verification only, no automatic re-request ✗

### Cheat sheet when you cannot find an entry point (console menus change)

| Task | Direct link (most reliable) | Fallback: menu |
|---|---|---|
| Enable the Public CA API | `https://console.cloud.google.com/apis/library/publicca.googleapis.com?project=<PROJECT_ID>` | ☰ → APIs & Services → Library → search `Public CA` |
| Service accounts | `https://console.cloud.google.com/iam-admin/serviceaccounts?project=<PROJECT_ID>` | ☰ → IAM & Admin → Service accounts |
| Grant role | `https://console.cloud.google.com/iam-admin/iam?project=<PROJECT_ID>` | ☰ → IAM & Admin → IAM → Grant access |
| Cloud Shell | `https://console.cloud.google.com/cloudshell` | `>_` icon in the top-right |

> Tip: replace `<PROJECT_ID>` / `YOUR_PROJECT_ID` with the real project ID (like `my-project-123456`) to jump straight to the page.

## 4. Configuring the panel (two ways)

Go to **Certificates → EAB configuration → Add EAB configuration** and choose **Google Public CA**:

### Option A (recommended): paste the service-account JSON

| Field | Value |
|---|---|
| GCP project ID | The project ID from section 3, e.g. `my-project-123456` |
| Service account JSON | Paste the **entire content** of `corefusion-eab.json` |
| Temporary access token | Leave empty |
| Name | Auto-generated as `Google Public CA - <project-id>`; editable |

Click **Request EAB automatically** → on success the KID/HMAC key are filled in and saved; later just choose `CA = Google Public CA` when issuing (EAB is associated per CA; no manual selection).

> The service-account JSON is **encrypted into the panel database's EAB configuration** (`json:"-"`, **never returned to the frontend**).
> The purpose is to **re-request a key automatically after the 7-day expiry** instead of manual GCP visits.

### Option B: temporary access token (one-off)

```bash
gcloud auth print-access-token        # valid ~1 hour
```

Paste it into “Temporary access token”, fill the project ID, click **Request EAB automatically**.
⚠ This token is **not stored** and must be re-obtained after expiry → fine for a one-off test, not for long-term use.

### Command-line equivalent (optional, for comparison)

```bash
gcloud publicca external-account-keys create
# returns keyId / b64MacKey, corresponding to ACME's --eab-kid / --eab-hmac-key
```

## 4.5 Why JSON instead of "a single key"?

This is the most common question. Separate the two kinds of “key”:

| What you might expect | What is actually needed |
|---|---|
| **API key** (the `AIza…` string) | ❌ **Useless** — an API key is only a public identifier and **carries no identity**; creating EAB needs **IAM permissions**, so identity-asserting credentials are required |
| — | ✅ **Service-account key**: the **file** downloaded from GCP “IAM → Service accounts → Keys → Create new key → **JSON**” |

Key point: **the JSON file itself is "a key"** — GCP packages the account identity (`client_email`) plus the private key (`private_key`) into one file. The panel signs a JWT with it → exchanges for an OAuth2 access token → calls the Public CA API.
So “paste JSON” ≠ filling many fields; **paste the whole file** (the panel also fills optional fields such as `type` / `token_uri`).

**The real EAB is the two strings** (`EAB-KID` + `EAB-HMAC-Key`), which are the **result** of the request; the panel fills them in automatically.

### Do not want to hand over the file? Two alternatives

| Method | How | Cost |
|---|---|---|
| **Temporary access token** | `gcloud auth print-access-token` → paste into “Temporary access token” | ~1 hour validity, **not stored** → one-off only, no automatic re-request |
| **Manual KID/HMAC** | Locally run `gcloud publicca external-account-keys create` → enter `keyId` / `b64MacKey` into “EAB-KID / EAB-HMAC-Key” | No credentials handed over ✓; but Google EAB is **7-day, one-time** → repeat after expiry |

> The panel recommends the service-account JSON for one reason: **EAB expires after 7 days**, and only a stored service account allows **automatic re-request** without manual work.

## 4.6 FAQ: can I use an API key?

**No.** This is not an implementation choice — **Google explicitly rejects it** (verified both ways):

```bash
# 1) Query parameter
curl -X POST "https://publicca.googleapis.com/v1/projects/<ID>/locations/global/externalAccountKeys?key=AIza..." -d '{}'
# 2) x-goog-api-key header recommended by the official docs
curl -X POST "https://publicca.googleapis.com/v1/projects/<ID>/locations/global/externalAccountKeys" \
     -H "x-goog-api-key: AIza..." -d '{}'
```

Both return:

```json
{"error":{"code":401,"status":"UNAUTHENTICATED",
  "message":"API keys are not supported by this API. Expected OAuth2 access token
             or other authentication credentials that assert a principal."}}
```

For comparison, a **(fake) OAuth Bearer** returns
`Request had invalid authentication credentials. Expected OAuth 2 access token...` —
so OAuth is recognized while **API keys are rejected outright** ✗.

**Why**: creating EAB is an **administrative operation** that requires knowing “who you are and whether you have `roles/publicca.externalAccountKeyCreator`” ✗
— an API key only identifies a project/application ✓ and **does not express a principal identity** ✓.
(Google's "Best practices for managing API keys" recommends **IAM + short-lived service-account credentials** for administrative scenarios ✓ — the same approach used here ✓.)

**So there are only three paths** ✓ (see §3): service-account JSON ✓ / temporary access token ✓ /
or **no credentials at all** — run `gcloud publicca external-account-keys create` in Cloud Shell ✓
and enter `keyId` / `b64MacKey` into the panel ✓ (**use this if GCP IAM blocks you** ✓).

## 5. Automatic registration and renewals (important)

```
Panel: CA = Google Public CA
  ├─ usable EAB present in the config → use it
  └─ missing/expired → request a new one on the spot with the stored service-account JSON (auto-registration)
        │
First issuance on the node/panel ── registers the ACME account with EAB (one-time!)
        │
Account persisted: data/acme/accounts/google.json (panel) / <Agent data_dir>/acme/accounts/google.json (node)
        │
Every later renewal ── uses the account key only; **EAB is never needed again**
```

Therefore:

- **The first issuance registers automatically**; no manual GCP step is needed;
- Renewals **do not** consume EAB (as long as the account file exists and the data directory stays stable);
- ⚠ The Agent-side ACME data directory is a fixed absolute path (default `/var/lib/corefusion-agent/acme`);
  **do not change it casually**, or it will be treated as a new account → another EAB key required.

## 6. Troubleshooting

| Symptom / error | Cause | Fix |
|---|---|---|
| `PERMISSION_DENIED` (403) | Service account lacks `roles/publicca.externalAccountKeyCreator` | Go back to section 3 ④ |
| `SERVICE_DISABLED` (403) | `publicca.googleapis.com` not enabled for the project | Section 3 ② |
| `NOT_FOUND` (404) | Wrong project ID / project deleted | Verify the project ID |
| Issuance reports `externalAccountRequired` | This CA has no EAB | Request/configure EAB in the panel first |
| Issuance reports `Accounts must have at least one contact.` | No email was provided during registration | Set the ACME account email on the certificate page (the panel uses the default account) |
| Renewal asks for EAB again / registration fails | ACME account file lost (data directory changed / cleaned) | **Restore the backup**; without one, request a new EAB using the service account |
| Issuer changes from Google to Let's Encrypt | Renewal command missing the CA (old bug, fixed in v0.12.101) | Upgrade and reissue |

## 7. Security and operations recommendations

1. **Least privilege**: grant the service account only `roles/publicca.externalAccountKeyCreator`, never Owner/Editor.
2. **Do not delete the project**: deleting the GCP project invalidates all its ACME accounts → certificates cannot renew.
3. **Always back up** `data/acme/accounts/` (the account identity itself) and the panel database's EAB configuration.
4. **Do not panic about EAB expiry**: as long as the ACME account file exists, renewals are unaffected; a new EAB is only needed to register a new account.
5. If the service-account key may have leaked: delete that key in GCP and paste a new JSON into the panel.

## 8. Related endpoints (for scripting)

| Endpoint | Description |
|---|---|
| `POST /api/v1/eab-configs/fetch-google` | Create an EAB using GCP authorization. Body: `{project_id, service_account_json?, access_token?}` |
| `POST /api/v1/eab-configs/fetch-zerossl` | Obtain a ZeroSSL EAB with an email. Body: `{email}` |
| `GET  /api/v1/eab-configs` | List EAB configurations (**without** the HMAC key or service-account JSON; only `has_mac_key` / `has_google_sa`) |
