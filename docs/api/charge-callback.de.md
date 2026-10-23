<!-- source: charge-callback.md | locale: de | updated: 2026-10-23 -->
# Kostenpflichtiger Rückruf API

> Sprachen: [简体中文](charge-callback.md) · [English](charge-callback.en.md) · [繁體中文](charge-callback.zh-TW.md) · [Русский](charge-callback.ru.md) · [日本語](charge-callback.ja.md) · [한국어](charge-callback.ko.md) · Deutsch · [Français](charge-callback.fr.md) · [Español](charge-callback.es.md) · [Português](charge-callback.pt.md) · [Türkçe](charge-callback.tr.md) · [Bahasa Indonesia](charge-callback.id.md) · [ไทย](charge-callback.th.md) · [Tiếng Việt](charge-callback.vi.md) · [العربية](charge-callback.ar.md) · [فارسی](charge-callback.fa.md)

Für **Zahlungsplattformen/Site-Integratoren**: Nachdem ein Benutzer auf einer bereitgestellten Site bezahlt, benachrichtigt der Server der Zahlungsplattform das Panel.

## 1. Endpunkt

```
POST /api/v1/charge/callback/{site_id}
Content-Type: application/json
```

`site_id` ist die admin-seitige „gemountete Site“-ID (Ganzzahl). Der Rückruf erfolgt von Server zu Server und verwendet **HMAC-Signierung** sowie eine optionale IP-Zulassungsliste anstelle der Bearer-Authentifizierung.

## 2. Anforderungstext

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

| Feld | Beschreibung |
|---|---|
| `trade_no` | Panelseitige Transaktionsnummer (wird bei der Auftragserstellung vergeben, Präfix `CF`) |
| `site_trade_no` | Bestellnummer der Zahlungsplattform (unterschrieben) |
| `status` | Geschäftsstatus, **nicht unterzeichnet** |
| `amount` / `currency` | Bestellbetrag (ganzzahlige Cent) und Währung (beide mit Vorzeichen) |
| `timestamp` | Unix-Sekunden; außerhalb des Fensters abgelehnt (unterschrieben) |
| `nonce` | Wiedergabeschutz: atomares INSERT, jede Nonce wird einmal verarbeitet (signiert) |
| `paid_*` / `method` / `exchange_rate` / `paid_at` | Tatsächliche Zahlungsinformationen, **nicht signiert** (Integrität abgedeckt durch `body_hash`) |
| `sign` | HMAC-SHA256-Signatur, hexadezimal kodiert (berechnet über den Körper ohne `sign`) |
| `extra` | Erweiterungsfelder |

## 3. Signaturalgorithmus (v3.2 / `version: "v1"`)

1. Erstellen Sie den **Rohkörper ohne `sign`** und berechnen Sie `body_hash = SHA256(raw body bytes)` (kleingeschriebenes Hex).
   > Das Panel akzeptiert auch das Legacy-Schema, das den gesamten Körper einschließlich `sign` hasht; Neue Integrationen sollten dem dokumentierten Schema folgen.
2. Erstellen Sie die Signierungsnutzlast (**Schlüssel in lexikografischer Reihenfolge**, Go `json.Marshal`-Semantik):

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
   > Das Geheimnis ist das Rückrufgeheimnis der bereitgestellten Site in der Admin-Benutzeroberfläche (nach der Rotation bleibt das vorherige Geheimnis für ein Übergangsfenster erhalten).

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

## 4. Panelseitige Validierungskette (bestimmt die Fehler, die Sie erhalten)

1. Die Website existiert und ihre Domain ist verifiziert → 404/403
2. IP-Zulassungsliste (wenn `callback_whitelist` konfiguriert ist, werden nicht aufgeführte IPs **hart blockiert**; andernfalls werden sie übersprungen und nur die Signatur überprüft)
3. `timestamp`-Fenster + `nonce`-Wiedergabeschutz (atomarer INSERT)
4. HMAC-Verifizierung (akzeptiert sowohl aktive als auch ältere Geheimnisse)
5. Bestellung existiert und gehört zur Site → Idempotenz (bereits bezahlte Retouren erfolgreich) → **Validierung der Betrags- und Währungssperre** → Transaktionsbuchung (Saldobuch + Bestellstatus)

Die Verarbeitung ist **idempotent**: Wiederholte Rückrufe mit demselben `trade_no` werden nicht zweimal gepostet. Signatur-/Ratenanomalien werden als „Rückrufereignisse“ aufgezeichnet, die Administratoren überprüfen und manuell wiederholen können.

## 5. Antworten

- Erfolg: 2xx (nachdem das Panel die Zahlung gebucht hat)
- Fehler: 4xx/5xx + `{ "code": "...", "message": "..." }`; Zu den typischen Codes gehören `INVALID_SIGNATURE`, `IP_BLOCKED`, `ORDER_NOT_FOUND`, Mengeninkongruenz usw.

> Jeder Rückruf trägt `X-Request-ID` (wird vom Panel generiert, falls nicht vorhanden); Protokolle und Ereignisse sind durchgängig nachvollziehbar. Geben Sie diese ID bei der Fehlerbehebung an Ihren Administrator weiter.

## 6. Testen und Fehlerbehebung

- Administratoren können das **Integrationsdokument** jeder Site abrufen: `GET /api/v1/charge-sites/{id}/integration-doc` (einschließlich der tatsächlichen Callback-Beispiele URL der Site).
- Rückrufereignisse: `GET /api/v1/charge-callback-events`; Wiederholungsversuche: `GET /api/v1/charge-callback-retries`, `POST /api/v1/charge-callback-retries/{id}/retry`.
- Geheime Rotation: `POST /api/v1/charge-sites/{id}/rotate-secret` (das vorherige Geheimnis wird für ein Übergangsfenster aufbewahrt).
