<!-- source: subscription.md | locale: pt | updated: 2026-11-12 -->
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
| `providers` | Somente `mihomo_yaml`: `1` / `true` ativa o **modo Provedor** (emitir `proxy-providers` e referenciá-los via `use`); padrão ou qualquer outro valor mantém os nós alinhados em `proxies`. Veja §6 |

### Valores `format` suportados

| Valor | Saída | Clientes |
|---|---|---|
| `base64_uri` (padrão) | Links de compartilhamento unidos por nova linha, codificados em base64 como um todo | V2RayN, Shadowrocket, Surge e outros clientes genéricos |
| `raw_uri` | Links de compartilhamento unidos por nova linha, texto simples | Clientes que precisam de links simples/depuração |
| `mihomo_yaml` | Configuração completa do Clash/Mihomo YAML (com grupos de proxy, regras) | Clash.Meta / mihomo / Clash Verge Rev |
| `mihomo_proxies_yaml` | YAML apenas com a lista de nós `proxies:` | fonte do provedor de proxy mihomo (com atualização agendada) |
| `singbox_json` | Configuração completa do sing-box JSON | sing-box / SFI / FLClash (núcleo sing-box) |
| `xray_json` | Configuração completa Xray JSON | Núcleo Xray |

> A compatibilidade de protocolo/formato se aplica: por exemplo, `anytls` aparece apenas nos formatos sing-box/Mihomo; Os nós REALITY em um servidor sing-box não aparecem nas assinaturas Xray/sing-box JSON. Os nós que o cliente não consegue reconhecer são descartados automaticamente em vez de falharem em toda a assinatura.

## 3. Cabeçalhos de resposta

| Cabeçalho | Exemplo | Descrição |
|---|---|---|
| `Subscription-Userinfo` | `upload=1024; download=2048; total=1073741824; expire=1760000000` | Uso e expiração (bytes/segundos Unix). **Também enviado em 403**, para que os clientes possam mostrar o valor restante |
| `Profile-Update-Interval` | `6` | Intervalo de atualização sugerido (horas) |
| `Profile-Title` | `CoreFusion` | Nome do perfil (mostrado por Clash Verge Rev / V2RayN, etc.) |
| `Profile-Web-Page-Url` | `https://portal.example.com` | Portal URL (“página web aberta” em clientes; caminho raiz quando o portal possui porta própria, caso contrário /p) |
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

# 5) Clash dynamic subscription: config template + Provider mode
curl -L "https://sub.example.com/abcd1234efgh5678?format=clash&config=out&providers=1"

# 6) sing-box / Xray with their own config templates (rules come from the template)
curl -L "https://sub.example.com/abcd1234efgh5678?format=singbox_json&config=sing-box-home"
curl -L "https://sub.example.com/abcd1234efgh5678?format=xray_json&config=xray-home"
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

## 6. Modo provedor e vários domínios de assinatura

### 6.1 O que `providers=1` emite

Com `format=mihomo_yaml&providers=1` (ou `format=clash&providers=1`) o YAML:

- não insere mais nós no nível superior — ele emite `proxy-providers:`;
- grupos de proxy fazem referência a fontes de nós via `use: [<provider-key>...]`;
- qualquer `proxy-providers` escrito em um formato de assinatura compartilhada (`mihomo_configs.config_json`) é **ignorado** (uma configuração compartilhada atende a vários planos; provedores codificados vazariam entre os planos). Grupo → mapeamentos de origem estão em “Gerenciamento de plano → grupo → mapeamento de origem”.

Tipos de provedores:

| Gentil | Chave | Notas |
|---|---|---|
| Interno | Nome do grupo de nós higienizado | Gerado por grupo de nós do plano; contém apenas os nós desse grupo |
| Reservado | `subscription-nodes` | Todos os nós do plano; sua fonte URL **omite** o parâmetro `provider` |
| Externo | Nome configurado no plano | Fonte externa (`http` YAML ou `file`) |

Campos comuns por provedor:

- `url` — aponta para esta assinatura com `format=mihomo_proxies_yaml`, herdando `config`/`platform` da solicitação principal; provedores internos acrescentam `provider=<key>`;
- `path` — **sempre derivado de “padrões internos do provedor → modelo de caminho de cache”** (padrão `./providers/{key}.yaml`, `{key}` substituído). Os fornecedores externos também não podem definir o seu próprio caminho;
- `health-check` — `enable / url / interval`; os provedores internos herdam os padrões, os provedores externos configuram no plano;
- `interval` — intervalo de atualização em segundos.

### 6.2 Qual domínio vai para o provedor URL (multidomínio)

“Centro de assinaturas → Terminais de assinatura” pode conter **vários** registros (domínio + caminho + ativado). O provedor `url` escolhe o domínio nesta ordem:

1. **O domínio que esta solicitação realmente usou**: o primeiro valor de `X-Forwarded-Host` (porta removida), caso contrário, `Host`; deve corresponder a um endpoint **ativado**;
2. Se nada corresponder, volte para o endpoint habilitado com o **ID mais baixo**;
3. O caminho vem do terminal correspondente (`path`, padrão `/`), produzindo `https://<domain><path>/<token>?format=mihomo_proxies_yaml...`.

Consequências:

- Um cliente que busca um novo domínio obtém esse domínio dentro do provedor — **não é necessária nenhuma alteração no painel**;
- Um domínio antigo ainda habilitado na lista não está mais codificado;
- Se um cliente armazenou em cache o provedor antigo URL, atualizar a assinatura (ou limpar o cache do provedor/reiniciar o cliente) fará a troca.

Os proxies reversos devem encaminhar `X-Forwarded-Proto` e `X-Forwarded-Host`; caso contrário, o endpoint de fallback será usado.

## 7. Notas

- **O formato e a configuração estão fortemente acoplados**: o modelo selecionado por `config`/`platform` deve corresponder ao `format` desta solicitação; modelos incompatíveis são ignorados — isso evita a injeção de regras Mihomo em sing-box/Xray JSON.
- **`providers=1` afeta apenas Mihomo YAML**: sing-box / Xray / base64 / raw ignore-o (os nós permanecem alinhados).
- **Limitação de taxa**: o endpoint da assinatura tem seu próprio limite (`SubscribeRateLimit`); clientes abusivos são rejeitados.
- **Log de acesso**: cada busca registra informações de IP / UA / CDN, visíveis em “Registro de acesso de assinatura” na UI do administrador.
- Sem domínio configurado em porta de assinatura dedicada, são utilizados os `X-Forwarded-Proto`/`X-Forwarded-Host` da solicitação; certifique-se de que seu proxy reverso encaminhe ambos os cabeçalhos.
- Em implantações de vários domínios, mantenha a lista de endpoints alinhada com os domínios que os clientes realmente usam; ver §6.2.
