<!-- source: charge-callback.md | locale: es | updated: 2026-10-23 -->
# Devolución de llamada de cargo API

> Idiomas: [简体中文](charge-callback.md) · [English](charge-callback.en.md) · [繁體中文](charge-callback.zh-TW.md) · [Русский](charge-callback.ru.md) · [日本語](charge-callback.ja.md) · [한국어](charge-callback.ko.md) · [Deutsch](charge-callback.de.md) · [Français](charge-callback.fr.md) · Español · [Português](charge-callback.pt.md) · [Türkçe](charge-callback.tr.md) · [Bahasa Indonesia](charge-callback.id.md) · [ไทย](charge-callback.th.md) · [Tiếng Việt](charge-callback.vi.md) · [العربية](charge-callback.ar.md) · [فارسی](charge-callback.fa.md)

Para **plataformas de pago/integradores de sitios**: después de que un usuario paga en un sitio montado, el servidor de la plataforma de pago notifica al panel.

## 1. Punto final

```
POST /api/v1/charge/callback/{site_id}
Content-Type: application/json
```

`site_id` es el ID del "sitio montado" del lado del administrador (entero). La devolución de llamada es de servidor a servidor y utiliza **firma HMAC** más una lista de IP permitidas opcional en lugar de autenticación de portador.

## 2. Cuerpo de la solicitud

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

| campo | Descripción |
|---|---|
| `trade_no` | Número de transacción del lado del panel (emitido en el momento de la creación del pedido, prefijo `CF`) |
| `site_trade_no` | Número de pedido de la plataforma de pago (firmado) |
| `status` | Estado comercial, **no firmado** |
| `amount` / `currency` | Importe del pedido (centavos enteros) y moneda (ambas firmadas) |
| `timestamp` | segundos Unix; rechazado fuera de la ventana (firmado) |
| `nonce` | Protección de reproducción: INSERTO atómico, cada nonce procesado una vez (firmado) |
| `paid_*` / `method` / `exchange_rate` / `paid_at` | Información de pago real, **no firmada** (integridad cubierta por `body_hash`) |
| `sign` | Firma HMAC-SHA256, codificada en hexadecimal (calculada sobre el cuerpo sin `sign`) |
| `extra` | Campos de extensión |

## 3. Algoritmo de firma (v3.2 / `version: "v1"`)

1. Construya el **cuerpo bruto sin `sign`** y calcule `body_hash = SHA256(raw body bytes)` (hexadecimal en minúsculas).
   > El panel también acepta el esquema heredado que codifica el cuerpo completo, incluido `sign`; Las nuevas integraciones deben seguir el esquema documentado.
2. Cree la carga útil de firma (**claves en orden lexicográfico**, semántica Go `json.Marshal`):

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
   > El secreto es el secreto de devolución de llamada del sitio montado en la interfaz de usuario del administrador (después de la rotación, el secreto anterior se conserva durante una ventana de transición).

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

## 4. Cadena de validación del lado del panel (determina los errores que obtienes)

1. El sitio existe y su dominio está verificado → 404 / 403
2. Lista de IP permitidas (si se configura `callback_whitelist`, las IP que no figuran en la lista están **bloqueadas**; de lo contrario, se omiten y solo se verifica la firma)
3. Ventana `timestamp` + protección de reproducción `nonce` (inserto atómico)
4. Verificación HMAC (acepta secretos tanto activos como heredados)
5. El pedido existe y pertenece al sitio → idempotencia (devoluciones ya pagadas exitosas) → **validación de bloqueo de monto y moneda** → publicación transaccional (libro mayor de saldo + estado del pedido)

El procesamiento es **idempotent**: las devoluciones de llamada repetidas con el mismo `trade_no` no se publican dos veces. Las anomalías de firma/tasa se registran como "eventos de devolución de llamada", que los administradores pueden inspeccionar y volver a intentar manualmente.

## 5. Respuestas

- Éxito: 2xx (después de que el panel haya publicado el pago)
- Fallo: 4xx/5xx + `{ "code": "...", "message": "..." }`; Los códigos típicos incluyen `INVALID_SIGNATURE`, `IP_BLOCKED`, `ORDER_NOT_FOUND`, falta de coincidencia de cantidades, etc.

> Cada devolución de llamada lleva `X-Request-ID` (generado por el panel si está ausente); los registros y eventos son rastreables de extremo a extremo. Proporcione esta identificación a su administrador cuando solucione problemas.

## 6. Pruebas y solución de problemas

- Los administradores pueden obtener el **documento de integración** de cada sitio: `GET /api/v1/charge-sites/{id}/integration-doc` (incluye ejemplos de devolución de llamada reales de URL del sitio).
- Eventos de devolución de llamada: `GET /api/v1/charge-callback-events`; reintentos: `GET /api/v1/charge-callback-retries`, `POST /api/v1/charge-callback-retries/{id}/retry`.
- Rotación secreta: `POST /api/v1/charge-sites/{id}/rotate-secret` (el secreto anterior se mantiene durante una ventana de transición).
