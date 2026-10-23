<!-- source: portal-api.md | locale: pt | updated: 2026-10-23 -->
# Referência do Portal API (Cliente / Portal API)

> Idiomas: [简体中文](portal-api.md) · [English](portal-api.en.md) · [繁體中文](portal-api.zh-TW.md) · [Русский](portal-api.ru.md) · [日本語](portal-api.ja.md) · [한국어](portal-api.ko.md) · [Deutsch](portal-api.de.md) · [Français](portal-api.fr.md) · [Español](portal-api.es.md) · Português · [Türkçe](portal-api.tr.md) · [Bahasa Indonesia](portal-api.id.md) · [ไทย](portal-api.th.md) · [Tiếng Việt](portal-api.vi.md) · [العربية](portal-api.ar.md) · [فارسی](portal-api.fa.md)

> A referência de interface **completa** para o frontend do portal (`/p` ou uma porta de portal dedicada) e clientes de terceiros.
> Definições de rota: `internal/api/router/router.go` (`clientAuthGroup` / `clientAuthProtected` / `/api/v1/public/*`).

## 1. Convenções gerais

### 1.1 Base URL

| Implantação | Base URL |
|---|---|
| Motor do painel principal | `https://<panel-domain>/api/v1` (página do portal em `/p`) |
| Porta de portal dedicada | `https://<portal-domain>[:port]/api/v1` (página do portal em `/`) |
| Domínio de portal dedicado + proxy reverso | O mesmo que acima; o proxy deve encaminhar ambos os caminhos de página e `/api/v1` |

### 1.2 Autenticação

- Após o login, envie `Authorization: Bearer <access_token>`;
- Os tokens de acesso duram 24h, os tokens de atualização 7 dias;
- O JWT contém `tv` (token_version): logout / saída forçada aumenta-o e invalida todos os tokens antigos
 (`clients.token_version`, consulte `internal/service/clientauth/service.go`);
- Cada endpoint protegido passa por `ClientAuthMiddleware` (verificação de versão + proteção em modo de manutenção).

### 1.3 Envelope de resposta

A maioria dos terminais retorna `{ "code": 0, "data": ..., "message": "..." }`;
alguns pontos de extremidade da lista retornam `{ "items": [...], "total": N }` ou uma matriz simples (indicada por ponto de extremidade abaixo).
Erros retornam `{ "code": "ERROR_CODE", "message": "...", "request_id": "...", "can_retry": ... }`
(HTTP 4xx/5xx).

### 1.4 Unidades de quantidade

Os campos de pedido/saldo são **centavos inteiros**: `990` = ¥9,90 (`currency: "CNY"`).

### 1.5 Exemplo real (entrar → buscar perfil)

```bash
# Register (when registration_enabled=true)
curl -X POST https://<portal-domain>/api/v1/client-auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"plaintext-password","referral_code":""}'
# → 201 {"data":{"access_token":"<JWT>","refresh_token":"<JWT>","client":{...}}}

# Sign in
curl -X POST https://<portal-domain>/api/v1/client-auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"plaintext-password"}'
# → 200 same as above

# Fetch the current client profile (with token)
curl https://<portal-domain>/api/v1/client-auth/me \
  -H 'Authorization: Bearer <access_token>'
# → 200 {"data":{"id":27,"email":"user@example.com","nickname":"","status":"active","quota_bytes":10737418240,"used_bytes":0,...}}

# Fetch a subscription (no header, token in the path)
curl 'https://<portal-domain>/sub/<subscription_token>?format=mihomo_yaml'
# → 200 Mihomo YAML (or base64_uri / singbox_json / xray_json)

# Error example: expired token
# → 401 {"code":"errClientTokenRevoked","message":"...","request_id":"...","can_retry":false}
```

No 401, ligue para `POST /api/v1/client-auth/refresh` uma vez e tente novamente; se ainda falhar, limpe a sessão local (o modelo do portal já faz isso).

---

## 2. Endpoints públicos (sem login)

| Método | Caminho | Descrição |
|---|---|---|
| GET | `/p` (`/` em um mecanismo de portal dedicado) | HTML do portal (renderizado pelo servidor + tempo de execução Vue) |
| GET | `/api/v1/public/portal-config` | Configuração de tempo de execução do portal: nome do site, cores do tema, navegação, módulos, pacotes de idiomas, `registration_enabled`, `regions` (fonte da região de login), `privacy_policy_url` / `terms_of_service_url` / `privacy_policy_content` / `terms_of_service_content` (documentos de conformidade; conteúdo mapeado por idioma; páginas independentes `/p/legal/*` ✓) |
| GET | `/api/v1/public/settings` | Configurações públicas (atualmente `registration_enabled`) |
| GET | `/api/v1/public/subscription-endpoints` | Lista de endpoints de assinatura (matriz simples) |
| GET | `/api/v1/public/articles` | Artigos públicos (apenas publicados) |
| GET | `/api/v1/public/articles/:slug` | Detalhe do artigo |
| GET | `/api/v1/public/client-configs` | Configuração de download do cliente |
| GET | `/sub/:token` | Entrega de assinatura (`format` retorna base64_uri / raw_uri / mihomo_yaml / singbox_json / xray_json, etc.; revogado/ausente → 404, suspenso/expirado/inativo → 403. Consulte [subscription.md](portal-api.pt.md)) |
| GET | `/api/v1/client-auth/oauth/providers` | Visibilidade do provedor OAuth: `{"data":{"providers":[{"provider":"github","enabled":true},...]}}` |
| GET | `/api/v1/system/maintenance` | Status de manutenção (público). O portal mostra uma página de manutenção quando ativado |

> Tudo isso inclui a proteção de manutenção e a limitação de taxa pública API.

## 3. Autenticação/OAuth

| Método | Caminho | Autenticação | Descrição |
|---|---|---|---|
| POST | `/api/v1/client-auth/register` | — | Registre-se. Corpo: `{email, password, referral_code?}`. Retorna `{access_token, refresh_token, client}`; taxa de inscrição limitada |
| POST | `/api/v1/client-auth/login` | — | Faça login. Corpo: `{email, password}`. Mesma resposta; emite JWTs com `token_version` |
| POST | `/api/v1/client-auth/refresh` | — | Atualizar. Corpo: `{refresh_token}`. Retorna novos tokens de acesso/atualização (versão verificada; tokens de atualização antigos morrem imediatamente após o logout) |
| POST | `/api/v1/client-auth/logout` | ✓ | Logout: revogação do lado do servidor (`token_version +1`) → todos os tokens antigos do cliente são inválidos |
| POST | `/api/v1/client-auth/rotate-token` | ✓ | Gire o token de assinatura |
| POST | `/api/v1/client-auth/ticket` | ✓ | Emitir um **tíquete único de curta duração** (para fluxos `window.open`, como “download log bundle”, onde os cabeçalhos não podem ser definidos) |
| GET | `/api/v1/client-auth/oauth/:provider` | — | Iniciar OAuth: 307 → consentimento GitHub/Google (`state` anti-CSRF, 10 minutos); provedores com deficiência retornam 400 |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | — | Retorno de chamada OAuth: trocar código → criar/vincular cliente → emitir JWT versionado → 302 de volta ao portal (`/<portalPath>?access_token=...&refresh_token=...`) |

> O retorno de chamada OAuth é um **redirecionamento do navegador** (não uma chamada API): após um login bem-sucedido, o portal lê o token
> do URL em `onMounted` (ou via `postMessage` de mesma origem, consulte `handleOAuthCallback` no modelo de portal `web`).

## 4. Conta

| Método | Caminho | Descrição |
|---|---|---|
| GET | `/api/v1/client-auth/me` | Cliente atual: `{id, email, plan_id, plan_name, status, quota_bytes, used_bytes, upload_bytes, download_bytes, expires_at, ip_whitelist, device_limit, sub_token, sub_status, created_at, ...}` (envelope `data`) |
| POST | `/api/v1/client-auth/change-password` | Corpo: `{old_password, new_password}` |
| GET | `/api/v1/client-auth/checktime` | Hora do servidor (sincronização do relógio do cliente) |
| GET | `/api/v1/client-auth/connection-info` | Parâmetros de conexão do plano atual: `{concurrent, device_limit, protocol, speed_limit, tls, transport}` |

## 5. Assinaturas

| Método | Caminho | Descrição |
|---|---|---|
| POST | `/api/v1/client-auth/reset-subscription` | Redefinir a assinatura; isso gira `sub_token` |

> O link de assinatura é criado no frontend de `sub_token` retornado por `/me` (`/sub/:token`); não há
> endpoints `subscription-link` / `subscription-formats` separados.

## 6. Nós

| Método | Caminho | Descrição |
|---|---|---|
| GET | `/api/v1/client-auth/nodes` | Nós visíveis para o plano atual: `{items:[...]}` (exibir campos como nome/protocolo/transporte/região) |
| GET | `/api/v1/client-auth/nodes/:id/config` | Configuração completa do nó |
| GET | `/api/v1/client-auth/nodes/:id/link` | Link de compartilhamento (vless:// etc.) |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | Imagem do código QR |

## 7. Loja / planos / pedidos

| Método | Caminho | Descrição |
|---|---|---|
| GET | `/api/v1/client-auth/plans` | Lista de planos (matriz simples): `{id, name, description, category, sort_order, variants:[{id, name, billing_cycle, duration_days, price(cents), original_price, currency, quota_bytes, ...}]}` |
| GET | `/api/v1/client-auth/plans/:id` | Detalhe do plano |
| GET | `/api/v1/client-auth/traffic-packs` | Lista de pacotes de tráfego |
| POST | `/api/v1/client-auth/traffic-packs/buy` | Compre um pacote de tráfego |
| GET | `/api/v1/client-auth/orders` | Pedidos do meu plano (paginado) |
| POST | `/api/v1/client-auth/orders` | Criar um pedido (comprar/renovar uma variante do plano) |
| GET | `/api/v1/client-auth/orders/:id` | Detalhe do pedido |
| GET | `/api/v1/client-auth/invoices` | Faturas |
| GET | `/api/v1/client-auth/invoices/:id` | Detalhe da fatura |

## 8. Equilíbrio / recarga

| Método | Caminho | Descrição |
|---|---|---|
| GET | `/api/v1/client-auth/balance` | `{data:{client_id, balance(cents), frozen}}` |
| GET | `/api/v1/client-auth/balance/transactions` | Transações de saldo (`type`: cobrança/consumo/reembolso; paginado) |
| GET | `/api/v1/client-auth/charge/sites` | Locais de recarga disponíveis (`ListChargeSites`: somente habilitado) |
| POST | `/api/v1/client-auth/charge/create` | Crie um pedido de recarga (redireciona para pagamento de terceiros) |
| GET | `/api/v1/client-auth/charge/orders` | Lista de pedidos de recarga |
| GET | `/api/v1/client-auth/charge/orders/:id` | Detalhes do pedido de recarga |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | Pesquisar status de pagamento por trade_no |
| POST | `/api/v1/client-auth/charge/redeem-card` | Resgatar um cartão de recarga |

> A exclusão de um cliente **projeta** seu saldo e transações (consulte `DeleteClient` em `internal/service/client/service.go`).

## 9. Presentes / histórico de tráfego

| Método | Caminho | Descrição |
|---|---|---|
| GET | `/api/v1/client-auth/gifts` | Presentes exigíveis (matriz simples; `type`: dias/bytes) |
| POST | `/api/v1/client-auth/gifts/:id/claim` | Reivindicar |
| GET | `/api/v1/client-auth/traffic-history` | Histórico de tráfego mensal: `{data:{history:[{period_start,total_bytes,...}], months}}` |

## 10. Ingressos

| Método | Caminho | Descrição |
|---|---|---|
| GET | `/api/v1/client-auth/tickets` | Lista de ingressos: `{items,total}` |
| POST | `/api/v1/client-auth/tickets` | Crie um tíquete |
| GET | `/api/v1/client-auth/tickets/:id` | Detalhe do ticket (com respostas) |
| POST | `/api/v1/client-auth/tickets/:id/reply` | Responder |
| POST | `/api/v1/client-auth/tickets/:id/close` | Fechar |

## 11. Artigos/guias

| Método | Caminho | Descrição |
|---|---|---|
| GET | `/api/v1/client-auth/articles` | Lista de artigos (com filtro de categoria) |
| GET | `/api/v1/client-auth/articles/:slug` | Detalhe do artigo |

## 12. Referências (desabilitadas por padrão; podem ser habilitadas no editor do portal)

| Método | Caminho | Descrição |
|---|---|---|
| GET | `/api/v1/client-auth/referral/code` | Meu código de referência e link |
| GET | `/api/v1/client-auth/referral/stats` | Estatísticas de comissão (`commission_rate`, `available_commission`, `min_withdraw_amount`) |
| GET | `/api/v1/client-auth/referral/records` | Registros de referência |
| GET | `/api/v1/client-auth/referral/withdrawals` | Registros de retirada |
| POST | `/api/v1/client-auth/referral/withdraw` | Solicite um saque |

## 13. Contas compartilhadas

| Método | Caminho | Descrição |
|---|---|---|
| GET | `/api/v1/client-auth/shared-accounts` | Contas compartilhadas disponíveis (streaming, etc.) |
| GET | `/api/v1/client-auth/shared-accounts/:id` | Detalhe |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | Solicitar acesso |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | Reparo com um clique |

## 14. Pacotes de log (diagnóstico do assinante)

| Método | Caminho | Descrição |
|---|---|---|
| GET | `/api/v1/client-auth/logs/types` | Tipos de log que podem ser agrupados |
| GET | `/api/v1/client-auth/logs/packages` | Meus pacotes de registros |
| POST | `/api/v1/client-auth/logs/package` | Crie um trabalho de agrupamento de logs (somente nós de entrada) |
| GET | `/api/v1/client-auth/logs/packages/:id` | Status do trabalho (pronto/processando/com falha) |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | Baixe (prefira `POST /ticket` e anexe `?ticket=`) |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | Excluir um pacote de registros |

## 15. Configurações de segurança / lista de permissões de IP / cancelamento automático

| Método | Caminho | Descrição |
|---|---|---|
| GET/PUT | `/api/v1/client-auth/settings` | Configurações do cliente: `{ip_whitelist, device_limit}` (PUT) |
| GET | `/api/v1/client-auth/settings/whitelist` | Lista de permissões (matriz vazia) |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | Excluir uma entrada da lista de permissões |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | Adicione o IP atual à lista de permissões |
| POST | `/api/v1/client-auth/self-unban` | Cancelamento de banimento de autoatendimento (tentativas limitadas pela política) |

> A lista de permissões de IP **aplica-se apenas a URLs de assinatura** (`/sub/:token`); ele não bloqueia a entrada no portal/OAuth ou
> outros pontos de extremidade de autenticação do cliente. Se o usuário não tiver lista de permissões ou assinatura URL, o acesso será sempre permitido. O
> lista de permissões é uma configuração por usuário; não há mudança global.

**Fluxo de cancelamento automático**: disponível apenas para `auto_quota` (banimento automático em excesso de tráfego) quando o administrador
`self_unban.enabled` está ativado. O login (senha ou retorno de chamada OAuth) retorna `require_self_unban=true` + `self_unban_token`
(typ=self_unban, válido por 15 minutos, **apenas** utilizável em `POST /self-unban`; outros endpoints retornam 403/`SELF_UNBAN_TOKEN_SCOPE`).
O portal mostra então o cartão de cancelamento; após um cancelamento bem-sucedido, o usuário faz login novamente. Os retornos de chamada do OAuth entregam isso via
o fragmento URL `#require_self_unban=1&self_unban_token=..&self_unban_hint=..`.

## 16. Códigos de erro (trecho)

| código | Cenário |
|---|---|
| `INVALID_TOKEN` | Atualizar token inválido/expirado/incompatibilidade de versão |
| `INVALID_REQUEST` | Falha na validação do parâmetro |
| `NOT_FOUND` | Recurso ausente (entrada de ticket/pedido/lista de permissões, etc.) |
| `RELOAD_FAILED`/`SYNC_FAILED` | Falha na sincronização da configuração do proxy (lado do administrador) |
| 401+`errClientTokenRevoked` | incompatibilidade de token_version (desconectado/expulso) → os clientes devem atualizar; se isso falhar, limpe os tokens locais |

## 17. Documentos de conformidade (política de privacidade/termos de serviço)

Os links “Política de Privacidade · Termos de Serviço” na parte inferior do cartão de login (iguais aos sites comuns; clicar abre o conteúdo):

- **Configuração**: editor do portal → Configurações → Geral → Documentos de conformidade (**editar conteúdo por idioma**);
 você também pode `PUT /api/v1/portal/active` com campos estruturados `privacy_policy_url` / `privacy_policy_content` /
 `terms_of_service_url` / `terms_of_service_content` (nil = inalterado; o conteúdo é um mapa `{"zh-CN":"...","en":"..."}`).
- **Páginas acessíveis independentes** (renderizadas pelo servidor, fáceis para rastreadores de verificação Google; suporta `?lang=xx`):
  - Domínio do painel principal: `GET /p/legal/privacy`, `GET /p/legal/terms`
  - Domínio do portal dedicado (implantação raiz): `GET /legal/privacy`, `GET /legal/terms`
- **Lógica de exibição**: URL ou conteúdo, **URL vence** (abre externamente em uma nova aba); se o conteúdo for fornecido, clicando no
 o cartão de login renderiza Markdown em uma caixa de diálogo (`renderMd`) usando o idioma atual do portal.
- Entregue em tempo de execução via `/api/v1/public/portal-config`; o componente de login consome
 `ctx.privacyUrl / termsUrl / privacyContent / termsContent / openLegal / hasLegalLinks`.
- A verificação Google OAuth exige que esses links sejam acessíveis publicamente → use os links independentes acima no
 tela de consentimento; consulte `docs/oauth-google-verification.md` (contém modelos de política/termos de privacidade).

## 18. Exemplo de integração

```js
// Sign in
const { data } = await fetch('/api/v1/client-auth/login', {
  method: 'POST', headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ email, password })
}).then(r => r.json());
localStorage.setItem('client_token', data.access_token);
localStorage.setItem('client_refresh_token', data.refresh_token);

// Authenticated request
const me = await fetch('/api/v1/client-auth/me', {
  headers: { Authorization: 'Bearer ' + data.access_token }
}).then(r => r.json());
```

No 401, ligue para `POST /refresh` uma vez e tente novamente; se ainda falhar, limpe a sessão local
(o modelo do portal `api()` já implementa isso).
