<!-- source: charge-callback.md | locale: fr | updated: 2026-10-23 -->
# Rappel de frais API

> Langues : [简体中文](charge-callback.md) · [English](charge-callback.en.md) · [繁體中文](charge-callback.zh-TW.md) · [Русский](charge-callback.ru.md) · [日本語](charge-callback.ja.md) · [한국어](charge-callback.ko.md) · [Deutsch](charge-callback.de.md) · Français · [Español](charge-callback.es.md) · [Português](charge-callback.pt.md) · [Türkçe](charge-callback.tr.md) · [Bahasa Indonesia](charge-callback.id.md) · [ไทย](charge-callback.th.md) · [Tiếng Việt](charge-callback.vi.md) · [العربية](charge-callback.ar.md) · [فارسی](charge-callback.fa.md)

Pour les **plateformes de paiement / intégrateurs de sites** : après qu'un utilisateur ait payé sur un site monté, le serveur de la plateforme de paiement en informe le panel.

## 1. Point de terminaison

```
POST /api/v1/charge/callback/{site_id}
Content-Type: application/json
```

`site_id` est l’ID du « site monté » côté administrateur (entier). Le rappel s'effectue de serveur à serveur et utilise la **signature HMAC** ainsi qu'une liste blanche IP facultative au lieu de l'authentification du porteur.

## 2. Corps de la demande

```json
{
  "trade_no":       "CF20261010120000abcd1234",
  "site_trade_no":  "payment-platform-order-no",
  "status":         "success",
  "amount":         990,
  "currency":       "CNY",
  "timestamp":      1791640000,
  "nonce":          "random-unique-string",
  "paid_currency":  "CNY",
  "paid_amount":    990,
  "method":         "alipay",
  "exchange_rate":  1.0,
  "paid_at":        "2026-10-10 12:00:01",
  "sign":           "hex(signature)",
  "extra":          {}
}
```

| Champ | Descriptif |
|---|---|
| `trade_no` | Numéro de transaction côté panneau (émis lors de la création de la commande, préfixe `CF`) |
| `site_trade_no` | Numéro de commande de la plateforme de paiement (signé) |
| `status` | Statut de l'entreprise, **non signé** |
| `amount` / `currency` | Montant de la commande (centimes entiers) et devise (tous deux signés) |
| `timestamp` | Secondes Unix ; rejeté hors de la fenêtre (signé) |
| `nonce` | Protection contre la relecture : INSERT atomique, chaque nonce traité une fois (signé) |
| `paid_*` / `method` / `exchange_rate` / `paid_at` | Informations de paiement réelles, **non signées** (intégrité couverte par `body_hash`) |
| `sign` | Signature HMAC-SHA256, codée en hexadécimal (calculée sur le corps sans `sign`) |
| `extra` | Champs d'extension |

## 3. Algorithme de signature (v3.2 / `version: "v1"`)

1. Construisez le **corps brut sans `sign`** et calculez `body_hash = SHA256(raw body bytes)` (hexadécimal minuscule).
   > Le panneau accepte également le schéma hérité qui hache le corps complet, y compris `sign` ; les nouvelles intégrations doivent suivre le schéma documenté.
2. Créez la charge utile de signature (**clés dans l'ordre lexicographique**, sémantique Go `json.Marshal`) :

```json
{
  "amount": 990,
  "body_hash": "<hash from step 1>",
  "currency": "CNY",
  "nonce": "<nonce>",
  "site_trade_no": "<site_trade_no>",
  "timestamp": 1791640000,
  "trade_no": "<trade_no>",
  "version": "v1"
}
```

3. `sign = hex( HMAC_SHA256( key=callback secret, msg=JSON bytes from step 2) )`.
   > Le secret est le secret de rappel du site monté dans l'interface utilisateur d'administration (après la rotation, le secret précédent est conservé pendant une fenêtre de transition).

```python
import hashlib, hmac, json

def sign_callback(body: dict, secret: str) -> str:
    # 1) body_hash first: body must not contain sign here
    body_hash = hashlib.sha256(json.dumps(body, separators=(",", ":"), ensure_ascii=False).encode()).hexdigest()
    # 2) signing payload (key order irrelevant: json.dumps(sort_keys=True))
    payload = {
        "amount": body["amount"], "body_hash": body_hash,
        "currency": body["currency"], "nonce": body["nonce"],
        "site_trade_no": body["site_trade_no"], "timestamp": body["timestamp"],
        "trade_no": body["trade_no"], "version": "v1",
    }
    msg = json.dumps(payload, separators=(",", ":"), sort_keys=True).encode()
    return hmac.new(secret.encode(), msg, hashlib.sha256).hexdigest()
```

## 4. Chaîne de validation côté panneau (détermine les erreurs que vous obtenez)

1. Le site existe et son domaine est vérifié → 404 / 403
2. Liste blanche d'adresses IP (si `callback_whitelist` est configuré, les adresses IP non répertoriées sont **bloquées en dur** ; sinon ignorées et seule la signature est vérifiée)
3. Fenêtre `timestamp` + protection contre la relecture `nonce` (INSERT atomique)
4. Vérification HMAC (accepte les secrets actifs et hérités)
5. La commande existe et appartient au site → idempotence (succès des retours déjà payés) → **validation du montant et de la devise** → validation transactionnelle (livre de solde + statut de la commande)

Le traitement est **idempotent** : les rappels répétés avec le même `trade_no` ne sont pas publiés deux fois. Les anomalies de signature/taux sont enregistrées sous forme d’« événements de rappel », que les administrateurs peuvent inspecter et réessayer manuellement.

## 5. Réponses

- Succès : 2xx (après que le panel ait validé le paiement)
- Échec : 4xx/5xx + `{ "code": "...", "message": "..." }` ; les codes typiques incluent `INVALID_SIGNATURE`, `IP_BLOCKED`, `ORDER_NOT_FOUND`, inadéquation de montant, etc.

> Chaque rappel porte `X-Request-ID` (généré par le panneau en cas d'absence) ; les journaux et les événements sont traçables de bout en bout. Donnez cet ID à votre administrateur lors du dépannage.

## 6. Tests et dépannage

- Les administrateurs peuvent récupérer le **document d'intégration** de chaque site : `GET /api/v1/charge-sites/{id}/integration-doc` (inclut les exemples de rappel URL réels du site).
- Événements de rappel : `GET /api/v1/charge-callback-events` ; tentatives : `GET /api/v1/charge-callback-retries`, `POST /api/v1/charge-callback-retries/{id}/retry`.
- Rotation du secret : `POST /api/v1/charge-sites/{id}/rotate-secret` (le secret précédent est conservé pour une fenêtre de transition).
