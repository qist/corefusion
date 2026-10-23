<!-- source: oauth-google-verification.md | locale: en | updated: 2026-10-23 -->
# Google OAuth Verification Guide

> Languages: [简体中文](oauth-google-verification.md) · English · [繁體中文](oauth-google-verification.zh-TW.md) · [Русский](oauth-google-verification.ru.md) · [日本語](oauth-google-verification.ja.md) · [한국어](oauth-google-verification.ko.md) · [Deutsch](oauth-google-verification.de.md) · [Français](oauth-google-verification.fr.md) · [Español](oauth-google-verification.es.md) · [Português](oauth-google-verification.pt.md) · [Türkçe](oauth-google-verification.tr.md) · [Bahasa Indonesia](oauth-google-verification.id.md) · [ไทย](oauth-google-verification.th.md) · [Tiếng Việt](oauth-google-verification.vi.md) · [العربية](oauth-google-verification.ar.md) · [فارسی](oauth-google-verification.fa.md)

> Scope: Google sign-in for the CoreFusion portal (`internal/service/oauth`, scopes: `openid`, `email`, `profile`).
> GitHub comparison at the end (GitHub has no verification flow).

## 1. Testing vs production mode

The Google OAuth consent screen has two publishing states:

| State | Who can sign in | Verification required |
|---|---|---|
| **Testing** | Only Google accounts in the “test users” list | No |
| **In production** | Any Google account | **Yes** (see below) |

Once “System settings → OAuth” is enabled, the portal sign-in page shows the Google button to every visitor;
if the app is still in testing, visitors not on the test-user list see
`Error 403: access_denied` (app not verified / you are not a test user).
To offer Google sign-in publicly, **switch the app to production and pass verification**.

## 2. Hard requirements for verification

Google's review (basic verification for non-restricted scopes) requires:

1. **Home page**: a publicly accessible URL on the authorized domain (same domain or a subdomain).
2. **Privacy Policy link**: must be **publicly accessible** (no sign-in) and explain what data you collect and how you use/store/delete it. For Google sign-in it must at least cover:
   the Google account data collected (email, name, avatar), the purpose (creating/signing in to the portal account), no sale to third parties,
   and how users can delete their account and data.
3. **Terms of Service link**: publicly accessible, describing the service rules.
4. **Authorized domains**: register the app's domain in the consent screen (e.g. `example.com`); home/privacy/terms links must be on it or a subdomain.
5. **App logo (optional but recommended)**: 120×120px.
6. **Scope justification**: this project only uses `openid / email / profile` (non-restricted, non-sensitive),
   which falls under basic verification — no per-scope justification and **no third-party security assessment**.
   (Restricted scopes such as full Gmail access would require them; this project does not use any.)

Verification entry: [Google Cloud Console](https://console.cloud.google.com/) →
**APIs & Services → OAuth consent screen → Publish App → Submit for verification**.
Basic verification usually completes within days; you can operate in testing mode in the meantime.

## 3. What to configure for this project

### 3.1 Authorized redirect URI

The callback is generated dynamically from the domain that starts the sign-in (see `Authorize` in `internal/api/handlers/oauth/handler.go`):

```
https://<portal-domain>/api/v1/client-auth/oauth/google/callback
```

- Sign-in started from the portal domain (e.g. `https://p.example.com`) → register `https://p.example.com/...`;
- Started from the panel domain `/p` → register `https://panel.example.com/...`.
- Google allows **multiple** redirect URIs; register both domains.
- ⚠ Google requires redirect URIs to be **https** (except `http://localhost`);
  the reverse proxy must forward `X-Forwarded-Host` / `X-Forwarded-Proto`, otherwise the panel builds an
  http or internal redirect_uri → Google rejects it (`redirect_uri_mismatch`).

After setting “System settings → OAuth → Portal callback domain”, the callback URL shown on the page is the one to paste into Google.

### 3.2 Privacy Policy / Terms links on the portal

The sign-in card includes “Privacy Policy · Terms of Service” links (shown once configured), and both documents have **standalone pages**
(the dialog and the standalone page share the same content):

- Main panel domain: `https://<panel-domain>/p/legal/privacy`, `/p/legal/terms`
- Dedicated portal domain (root deployment): `https://<portal-domain>/legal/privacy`, `/legal/terms`
- Supports `?lang=xx`; without it, the browser's `Accept-Language` is used, then the portal default.
- Pages are **server-rendered** (no JS), which helps Google's verification crawler.

**Configuration (either way; links win)**:

1. **Content mode (recommended, works out of the box)**: portal editor → Settings → General → Compliance documents →
   choose “Edit language” and paste the §4 templates into “Privacy policy content” / “Terms of service content”
   (**Markdown, edited per language**) → after saving, the sign-in card shows links that open a **dialog**;
   the editor's “standalone link” can be entered directly in the Google consent screen.
2. **External link mode**: enter “Privacy policy URL” / “Terms of service URL” (pointing to pages you host) → opens in a new tab.

Content and links are delivered via `/api/v1/public/portal-config` (`privacy_policy_content` /
`terms_of_service_content` are **per-language Markdown maps** `{"zh-CN":"...","en":"..."}`;
`privacy_policy_url` / `terms_of_service_url` are optional external links). **They must be accessible without signing in** ✓.

### 3.3 Panel-side switch

`System settings → OAuth`: enable Google and enter Client ID / Client Secret (the secret is encrypted with AES-256-GCM at rest).
The Google button appears on the portal sign-in page only when everything is configured (`/api/v1/client-auth/oauth/providers` returns enabled).

## 4. Privacy Policy / Terms templates

> Minimal compliance templates you can adapt. Before use: replace the `【】` placeholders, add/remove clauses for your business,
> and have the operating entity review them. These templates are not legal advice.

### 4.1 Privacy Policy (template)

```markdown
# Privacy Policy

Last updated: [DATE]

[OPERATOR NAME] ("we") operates [PORTAL NAME] ("the Service"). We respect your privacy;
this policy explains what information we collect and how we use it.

## 1. Information we collect
- **Account information**: the email address and password you provide at registration (stored only as a one-way hash);
- **Google sign-in information**: when you choose "Sign in with Google", we receive, with your authorization,
  your Google account email, display name and avatar URL (scopes: openid, email, profile);
- **Service data**: subscription configuration, traffic usage, device IP allow-list, tickets and order records.

## 2. How we use information
Only to: create and sign in to your account, provide the proxy subscription service, bill you and provide support,
and for security controls (sign-in protection, IP allow-list). We **do not** sell your personal information.

## 3. Third-party services
- **Google**: receives account information within the authorized scope only when you actively use "Sign in with Google";
- Payment providers: receive order information only when you initiate a payment.

## 4. Storage and security
Data is stored on our servers; credentials are encrypted at rest and all traffic uses TLS.

## 5. Your rights
You can update your profile/password and end sessions in the portal at any time; to delete your account and all data,
contact [CONTACT EMAIL] and we will process the request within [N] business days.

## 6. Policy changes
Material changes will be announced in the portal notice board.
```

### 4.2 Terms of Service (template)

```markdown
# Terms of Service

Last updated: [DATE]

## 1. Service
The Service provides [PROXY / NETWORK ACCESS] to registered users; specific plans are described on the store page.

## 2. Accounts
- You are responsible for the security of your account and password; for accounts created with Google sign-in, credentials are managed by Google;
- You may not lend or transfer your account, or use the Service for any unlawful purpose (including but not limited to:
  unauthorized access to others' systems, distributing illegal content, or circumventing mandatory legal requirements).

## 3. Payment and refunds
Plans are billed [monthly / by usage]; traffic and device limits follow the plan description;
[REFUND POLICY: e.g. full refund within 7 days if unused].

## 4. Changes and termination
We may suspend or terminate the Service due to force majeure or your breach of these terms; deleting an account deletes the related data.

## 5. Disclaimer
The Service is provided "as is"; to the extent permitted by law, we are not liable for indirect losses caused by use of the Service.

## 6. Miscellaneous
These terms are governed by the laws of [JURISDICTION]. Contact: [CONTACT EMAIL].
```

## 5. Pre-launch checklist

- [ ] OAuth consent screen: app name / logo / home page / privacy policy / terms all present; domain registered
- [ ] Privacy Policy and Terms pages **accessible without signing in** (portal custom page or standalone static page)
- [ ] Both links visible on the portal sign-in card/footer
- [ ] redirect URI matches the actual domain and is https
- [ ] Publish App → submit for verification; the test-user restriction is lifted after approval
- [ ] Panel OAuth settings: enabled=true with Client ID/Secret filled (providers endpoint returns enabled:true)

## 6. GitHub comparison

GitHub OAuth Apps have **no** publishing state or verification flow: create the app, set the callback URL, and it works publicly.
Note that a GitHub OAuth App accepts only **one** Authorization callback URL (prefix matching); for multiple domains,
use the multi-URI approach from Google or create a separate app per portal domain.
