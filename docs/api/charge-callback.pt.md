<!-- source: charge-callback.md | locale: pt | updated: 2026-10-23 -->
# Retorno de chamada de cobrança API

> Idiomas: [简体中文](charge-callback.md) · [English](charge-callback.en.md) · [繁體中文](charge-callback.zh-TW.md) · [Русский](charge-callback.ru.md) · [日本語](charge-callback.ja.md) · [한국어](charge-callback.ko.md) · [Deutsch](charge-callback.de.md) · [Français](charge-callback.fr.md) · [Español](charge-callback.es.md) · Português · [Türkçe](charge-callback.tr.md) · [Bahasa Indonesia](charge-callback.id.md) · [ไทย](charge-callback.th.md) · [Tiếng Việt](charge-callback.vi.md) · [العربية](charge-callback.ar.md) · [فارسی](charge-callback.fa.md)

Para **plataformas de pagamento/integradores de sites**: depois que um usuário paga em um site montado, o servidor da plataforma de pagamento notifica o painel.

## 1. Ponto final

```
POST /api/v1/charge/callback/{site_id}
Content-Type: application/json
```

`site_id` é o ID do “site montado” do lado do administrador (inteiro). O retorno de chamada é de servidor para servidor e usa **assinatura HMAC** além de uma lista de permissões de IP opcional em vez de autenticação de portador.

## 2. Solicitar corpo

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

| Campo | Descrição |
|---|---|
| `trade_no` | Número da transação no painel (emitido na criação do pedido, prefixo `CF`) |
| `site_trade_no` | Número do pedido da plataforma de pagamento (assinado) |
| `status` | Status comercial, **não assinado** |
| `amount`/`currency` | Valor do pedido (centavos inteiros) e moeda (ambos assinados) |
| `timestamp` | Segundos Unix; rejeitado fora da janela (assinado) |
| `nonce` | Proteção de repetição: INSERT atômico, cada nonce processado uma vez (assinado) |
| `paid_*`/`method`/`exchange_rate`/`paid_at` | Informações reais de pagamento, **não assinadas** (integridade coberta por `body_hash`) |
| `sign` | Assinatura HMAC-SHA256, codificada em hexadecimal (calculada no corpo sem `sign`) |
| `extra` | Campos de extensão |

## 3. Algoritmo de assinatura (v3.2/`version: "v1"`)

1. Construa o **corpo bruto sem `sign`** e calcule `body_hash = SHA256(raw body bytes)` (hexadecimal minúsculo).
   > O painel também aceita o esquema legado que faz hash de todo o corpo, incluindo `sign`; novas integrações devem seguir o esquema documentado.
2. Crie a carga de assinatura (**chaves em ordem lexicográfica**, semântica Go `json.Marshal`):

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
   > O segredo é o segredo de retorno de chamada do site montado na UI administrativa (após a rotação, o segredo anterior é mantido por uma janela de transição).

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

## 4. Cadeia de validação no painel (determina os erros que você obtém)

1. O site existe e seu domínio foi verificado → 404/403
2. Lista de permissões de IP (se `callback_whitelist` estiver configurado, os IPs não listados serão **bloqueados**; caso contrário, serão ignorados e somente a assinatura será verificada)
3. Janela `timestamp` + proteção de repetição `nonce` (INSERT atômico)
4. Verificação HMAC (aceita segredos ativos e legados)
5. O pedido existe e pertence ao site → idempotência (já pago retorna com sucesso) → **validação de valor e bloqueio de moeda** → lançamento transacional (razão de saldo + status do pedido)

O processamento é **idempotente**: retornos de chamada repetidos com o mesmo `trade_no` não são postados duas vezes. Anomalias de assinatura/taxa são registradas como “eventos de retorno de chamada”, que os administradores podem inspecionar e tentar novamente manualmente.

## 5. Respostas

- Sucesso: 2xx (após o painel postar o pagamento)
- Falha: 4xx/5xx + `{ "code": "...", "message": "..." }`; códigos típicos incluem `INVALID_SIGNATURE`, `IP_BLOCKED`, `ORDER_NOT_FOUND`, incompatibilidade de valor, etc.

> Cada retorno de chamada carrega `X-Request-ID` (gerado pelo painel se estiver ausente); logs e eventos são rastreáveis ​​de ponta a ponta. Forneça esse ID ao seu administrador ao solucionar problemas.

## 6. Teste e solução de problemas

- Os administradores podem buscar o **documento de integração** de cada site: `GET /api/v1/charge-sites/{id}/integration-doc` (inclui exemplos reais de retorno de chamada do site URL).
- Eventos de retorno de chamada: `GET /api/v1/charge-callback-events`; novas tentativas: `GET /api/v1/charge-callback-retries`, `POST /api/v1/charge-callback-retries/{id}/retry`.
- Rotação secreta: `POST /api/v1/charge-sites/{id}/rotate-secret` (o segredo anterior é mantido por uma janela de transição).
