<!-- source: subscription.md | locale: pt | updated: 2026-10-23 -->
# Assinatura API (pública)

> Idiomas: [简体中文](subscription.md) · [English](subscription.en.md) · [繁體中文](subscription.zh-TW.md) · [Русский](subscription.ru.md) · [日本語](subscription.ja.md) · [한국어](subscription.ko.md) · [Deutsch](subscription.de.md) · [Français](subscription.fr.md) · [Español](subscription.es.md) · Português · [Türkçe](subscription.tr.md) · [Bahasa Indonesia](subscription.id.md) · [ไทย](subscription.th.md) · [Tiếng Việt](subscription.vi.md) · [العربية](subscription.ar.md) · [فارسی](subscription.fa.md)

Busca a lista de nós de assinatura de um usuário. **Este é o endpoint mais usado pelos usuários finais**; nenhum cabeçalho é necessário — o acesso é concedido pelo token no URL.

## 1. Ponto final

| Implantação | URL |
|---|---|
| Painel principal | `GET https://<panel-domain>/sub/<token>` |
| Porta de assinatura dedicada | `GET https://<sub-domain>:<port>/<token>` (sem prefixo `/sub`) |

`<token>` é o token de acesso exclusivo de uma assinatura, gerado pelo lado administrativo (visível em “Minhas assinaturas” no portal).

## 2. Parâmetros de consulta

| Parâmetro | Descrição |
|---|---|
| `format` | Formato de saída; substitui o padrão de assinatura. Veja abaixo |
| `config` | Nome do modelo de configuração (definido em “Formatos de assinatura” na interface do administrador). Deve usar o mesmo formato do `format` desta solicitação, caso contrário ele será ignorado |
| `platform` | Combine um modelo de configuração por nome de plataforma (usado quando `config` é omitido); o formato também deve corresponder |

### Valores `format` suportados

| Valor | Saída | Clientes |
|---|---|---|
| `base64_uri` (padrão) | Links de compartilhamento unidos por nova linha, codificados em base64 como um todo | V2RayN, Shadowrocket, Surge e outros clientes genéricos |
| `raw_uri` | Links de compartilhamento unidos por nova linha, texto simples | Clientes que precisam de links simples/depuração |
| `mihomo_yaml` | Configuração completa do Clash/Mihomo YAML (com grupos de proxy, regras) | Clash.Meta / mihomo / Clash Verge Rev |
| `mihomo_proxies_yaml` | YAML apenas com a lista de nós `proxies:` | fonte do provedor de proxy mihomo (com atualização agendada) |
| `singbox_json` | Configuração completa sing-box JSON | sing-box/SFI/FLClash (núcleo sing-box) |
| `xray_json` | Configuração completa Xray JSON | Núcleo Xray |

> A compatibilidade de protocolo/formato se aplica: por exemplo, `anytls` aparece apenas nos formatos sing-box/Mihomo; Os nós REALITY em um servidor sing-box não aparecem nas assinaturas Xray/sing-box JSON. Os nós que o cliente não consegue reconhecer são descartados automaticamente em vez de falharem em toda a assinatura.

## 3. Cabeçalhos de resposta

| Cabeçalho | Exemplo | Descrição |
|---|---|---|
| `Subscription-Userinfo` | `upload=1024; download=2048; total=1073741824; expire=1760000000` | Uso e expiração (bytes/segundos Unix). **Também enviado em 403**, para que os clientes possam mostrar o valor restante |
| `Profile-Update-Interval` | `6` | Intervalo de atualização sugerido (horas) |
| `Profile-Title` | `CoreFusion` | Nome do perfil (mostrado por Clash Verge Rev / V2RayN, etc.) |
| `Profile-Web-Page-Url` | `https://…/portal` | Link da página do portal (“abrir página web” nos clientes) |
| `Content-Type` | Por formato: `text/plain` / `text/yaml` / `application/json` |  |
| `Content-Disposition` | `attachment; filename="subscription.txt"` |  |

## 4. Códigos de status

| HTTP | código | Significado |
|---|---|---|
| 200 | — | Normal (o corpo é a assinatura renderizada em `format`) |
| 400 | `MISSING_TOKEN` | Nenhum token no URL |
| 403 | (texto simples) | Assinatura suspensa/expirada/inativa. A mensagem é localizada (zh/en/ja/ko) via `Accept-Language` e é um motivo de uma linha |
| 404 | `SUBSCRIPTION_NOT_FOUND` | O token não existe |

> Assinaturas revogadas são tratadas como inexistentes (404).

## 5. Exemplos

```bash
# 1) Fetch with the default format
curl -L "https://panel.example.com/sub/abcd1234efgh5678"

# 2) Request Mihomo YAML with the "clash-global" config template
curl -L "https://panel.example.com/sub/abcd1234efgh5678?format=mihomo_yaml&config=clash-global"

# 3) Node list only (for a mihomo proxy-provider)
curl -L "https://panel.example.com/sub/abcd1234efgh5678?format=mihomo_proxies_yaml"

# 4) Check usage (headers only)
curl -sI "https://panel.example.com/sub/abcd1234efgh5678" | grep -i subscription-userinfo
```

Assinatura dinâmica Clash/Mihomo (atualização periódica):

```yaml
proxy-providers:
  subscription-nodes:
    type: http
    url: "https://panel.example.com/sub/abcd1234efgh5678?format=mihomo_proxies_yaml"
    interval: 3600
    health-check:
      enable: true
      url: https://www.gstatic.com/generate_204
      interval: 300

proxy-groups:
  - name: PROXY
    type: select
    use:
      - subscription-nodes
```

## 6. Notas

- **Formato e configuração estão fortemente acoplados**: o modelo selecionado por `config`/`platform` deve corresponder ao `format` desta solicitação; modelos incompatíveis são ignorados — isso evita a injeção de regras Mihomo em sing-box/Xray JSON.
- **Limitação de taxa**: o endpoint da assinatura tem seu próprio limite (`SubscribeRateLimit`); clientes abusivos são rejeitados.
- **Log de acesso**: cada busca registra informações de IP/UA/CDN, visíveis em “Registro de acesso de assinatura” na UI do administrador.
- Sem um domínio configurado em uma porta de assinatura dedicada, são utilizados os `X-Forwarded-Proto` / `X-Forwarded-Host` da solicitação; certifique-se de que seu proxy reverso encaminhe ambos os cabeçalhos.
