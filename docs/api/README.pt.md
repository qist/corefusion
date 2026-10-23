<!-- source: README.md | locale: pt | updated: 2026-10-23 -->
# Documentação CoreFusion API

> Idiomas: [简体中文](README.md) · [English](README.en.md) · [繁體中文](README.zh-TW.md) · [Русский](README.ru.md) · [日本語](README.ja.md) · [한국어](README.ko.md) · [Deutsch](README.de.md) · [Français](README.fr.md) · [Español](README.es.md) · Português · [Türkçe](README.tr.md) · [Bahasa Indonesia](README.id.md) · [ไทย](README.th.md) · [Tiếng Việt](README.vi.md) · [العربية](README.ar.md) · [فارسی](README.fa.md)

Documentação API para **integradores externos, desenvolvimento secundário e clientes terceiros**. Todos os caminhos são relativos ao domínio de implantação.

| Documento | Público | Conteúdo |
|---|---|---|
| [subscrição.md](./subscription.pt.md) | Assinantes/desenvolvedores de clientes | Endpoint de assinatura (`/sub/:token`): formatos, cabeçalhos, códigos de status, exemplos |
| [portal-api.md](README.pt.md) | Frontend do portal/clientes terceiros | Portal do cliente API (`/api/v1/client-auth/*`, `/api/v1/public/*`): inscrição, login, planos, pedidos, saldo, tickets |
| [ retorno de chamada de cobrança.md](./charge-callback.pt.md) | Provedores de pagamento | Assinatura e verificação de retorno de chamada de pagamento |
| [admin-api.pt.md](./admin-api.pt.md) | Desenvolvimento/automação de painel | Tabela de rotas API de administração completa (`/api/v1/*`, agrupada por módulo) |

---

## 1. Portas e URLs base

O painel pode expor três mecanismos ao mesmo tempo, compartilhando um banco de dados:

| Motor | Acesso padrão | Objetivo |
|---|---|---|
| Painel principal | `https://<domain>/api/v1` | Back-end de administração + portal + assinaturas |
| Mecanismo de assinatura (porta dedicada) | `https://<domain>:<port>/:token` | Busca de assinatura de alta simultaneidade, sem prefixo `/sub` |
| Mecanismo de portal (porta dedicada) | `https://<portal-domain>[:port]/api/v1` | Portal somente cliente, páginas em `/` |

> O proxy reverso deve encaminhar ambos os caminhos de página e `/api/v1`; `X-Forwarded-Proto` / `X-Forwarded-Host` têm a honra de restaurar o esquema e o domínio reais.

## 2. Autenticação (três tipos, não intercambiáveis)

| Autenticação | Credencial | Aplica-se a |
|---|---|---|
| **Autorização de administrador** | `Authorization: Bearer <admin_token>` (de `/api/v1/auth/login`) | Administrador API |
| **Autenticação do cliente** | `Authorization: Bearer <client_token>` (de `/api/v1/client-auth/login`) | Portal API |
| **Token de assinatura** | Token no URL: `/sub/<token>` | Busca de assinatura, sem cabeçalho |

- Os tokens de acesso do cliente duram 24h, os tokens de atualização duram 7 dias; o JWT contém `tv` (token_version), portanto, o logout/saída forçada invalida todos os tokens antigos.
- Funções administrativas: `admin`/`operator`/`super_admin`; algumas operações de gravação requerem `RequireRole(RoleAdmin)` ou superior.

## 3. Formato de resposta

Sucesso (maioria dos endpoints):

```json
{ "code": 0, "data": { ... }, "message": "ok" }
```

Os pontos de extremidade da lista (alguns) retornam `{ "items": [...], "total": N }` ou uma matriz simples (indicada por ponto de extremidade abaixo).

Erros (HTTP 4xx/5xx):

```json
{ "code": "ERROR_CODE", "message": "human-readable description", "request_id": "…", "can_retry": false }
```

**Os valores são números inteiros em centavos**: `990` = ¥9,90 (`currency: "CNY"`).
**O tráfego está em bytes**: solicitações e respostas usam bytes.

## 4. Limites de taxa e modo de manutenção

- Os terminais públicos (`/api/v1/public/*`, `/sub/*`, inscrição/login) têm taxa limitada;
- O login também usa `LoginRateLimit`, a busca de assinatura usa `SubscribeRateLimit`;
- No modo de manutenção, os endpoints públicos e as assinaturas retornam uma mensagem de manutenção; o administrador API não é afetado.

## 5. Rotas de página de front-end (não API)

Essas rotas retornam ativos HTML/estáticos e estão fora dos documentos API; listados para que possam ser excluídos durante a integração:

| Método | Caminho | Descrição |
|---|---|---|
| GET | `/` | Painel de administração SPA (após login) |
| GET | `/p` | Portal HTML (`/` em um mecanismo de portal dedicado) |
| GET | `/:token` | Busca de assinatura em uma porta dedicada (sem prefixo `/sub`, mesma semântica de `/sub/:token`) |
| GET | `/static/:file` | Ativos estáticos de front-end |

## 6. Fonte da verdade

`internal/api/router/router.go` (746 rotas, 743 documentadas). A tabela de rotas em [admin-api.md](README.pt.md) é gerada a partir do código; se discordar do código, **o código vence**.
