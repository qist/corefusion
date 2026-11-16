<!-- source: admin-api.md | locale: pt | updated: 2026-11-16 -->
# Administrador API

> Idiomas: [简体中文](admin-api.md) · [English](admin-api.en.md) · [繁體中文](admin-api.zh-TW.md) · [Русский](admin-api.ru.md) · [日本語](admin-api.ja.md) · [한국어](admin-api.ko.md) · [Deutsch](admin-api.de.md) · [Français](admin-api.fr.md) · [Español](admin-api.es.md) · Português · [Türkçe](admin-api.tr.md) · [Bahasa Indonesia](admin-api.id.md) · [ไทย](admin-api.th.md) · [Tiếng Việt](admin-api.vi.md) · [العربية](admin-api.ar.md) · [فارسی](admin-api.fa.md)

A tabela de rotas administrativa API completa para **desenvolvimento/automação/integradores de painel**.

- Base URL: `https://<panel-domain>/api/v1`
- Autenticação: `Authorization: Bearer <admin_token>` (de `POST /api/v1/auth/login`)
- Formatos comuns de resposta/erro e convenções de limite de taxa: consulte [README.md](./README.pt.md)
- Fonte da verdade: `internal/api/router/router.go` (a contagem varia de acordo com a versão; o código é oficial); se esta tabela discordar, **o código vence**

## Marcadores de permissão

| Marcador | Significado |
|---|---|
| Público | Não é necessário fazer login (proteção de manutenção/limitação de taxa ainda pode ser aplicada) |
| Público · taxa limitada | Sem login, com limite de taxa dedicado |
| Conectado | Requer um token de administrador conectado (`AuthRequired`) |
| Operador | Requer função ≥ `operator` (`RequireRole(RoleOperator)`) |
| Administrador | Requer função ≥ `admin` (`RequireRole(RoleAdmin)`) |
| Superadministrador | Requer função = `super_admin` |
| Cliente (público) | Não é necessário token de cliente (login/registro, etc.) |
| Cliente (logado) | Requer `Authorization: Bearer <client_token>` (um token de cliente do portal, não um token de administrador) |

Níveis de função: `viewer(10) < operator(20) < admin(30) < super_admin(40)`; funções insuficientes retornam 403 `INSUFFICIENT_ROLE`.

---

## Início rápido (exemplos reais)

Estes exemplos provêm de respostas locais reais; tokens e dados são editados.

### 1. Faça login e receba um token

```bash
curl -X POST https://<panel-domain>/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"your-password"}'
```

Resposta:

```json
{
  "data": {
    "access_token": "<JWT, valid 24h>",
    "refresh_token": "<JWT, valid 7 days>",
    "user": { "id": 1, "role": "super_admin", "username": "admin" }
  }
}
```

### 2. Chame um endpoint de negócios com o token

```bash
curl https://<panel-domain>/api/v1/clients?limit=1 \
  -H 'Authorization: Bearer <access_token>'
```

Resposta (as listas usam a estrutura `items` + `total`):

```json
{
  "data": {
    "items": [
      {
        "id": 27,
        "email": "user@example.com",
        "status": "active",
        "quota_bytes": 10737418240,
        "used_bytes": 0,
        "plan_id": 1,
        "expires_at": null,
        "created_at": "2026-09-30T15:54:32+08:00"
      }
    ],
    "total": 22,
    "limit": 1,
    "offset": 0
  }
}
```

### 3. Exemplos de resposta de erro

**401 não autenticado:**

```json
{
  "code": "MISSING_TOKEN",
  "message": "missing authorization header",
  "request_id": "a02cfdf9-6be0-46be-8584-824caab34dc7",
  "can_retry": false,
  "requires_action": false
}
```

**Recurso 404 não encontrado:**

```json
{
  "code": "CLIENT_NOT_FOUND",
  "message": "client not found",
  "request_id": "099949bd-4ded-468d-a6f8-f8e798af2273",
  "can_retry": false,
  "requires_action": false
}
```

### 4. Paginação

Todos os terminais da lista usam `limit` (tamanho de página, padrão 20) e `offset` (baseado em 0) e retornam `{items, total, limit, offset}`. Alguns endpoints herdados retornam uma matriz simples (anotada por tabela).

---

## Conteúdo

- [Autenticação e segurança de administrador](#autenticação-e-segurança-de-administrador)
- [Contas de administrador](#contas-de-administrador)
- [Gestão de clientes](#gestão-de-clientes)
- [Modelos de plano](#modelos-de-plano)
- [Variantes do plano](#variantes-do-plano)
- [Pacotes de trânsito](#pacotes-de-trânsito)
- Cupons [](#coupons)
- [Cupons](#cupons)
- [Grupos de nós](#grupos-de-nós)
- [Servidores de nó](#servidores-de-nó)
- [Log de auditoria do nó](#log-de-auditoria-do-nó)
- [Nós (modelo legado, retirado)](#nós-modelo-legado-retirado)
- [Visão geral da região](#visão-geral-da-região)
- [Assinaturas](#assinaturas)
- [Formatos de assinatura](#formatos-de-assinatura)
- [Pontos de extremidade de assinatura](#pontos-de-extremidade-de-assinatura)
- [Compartilhar domínios](#compartilhar-domínios)
- [IP Cloudflare](#ip-cloudflare)
- [Configuração de assinatura (Mihomo/Clash)](#configuração-de-assinatura-mihomoclash)
- [Sites de recarga](#sites-de-recarga)
- [Recarregar pedidos](#recarregar-pedidos)
- [Cartões de recarga](#cartões-de-recarga)
- [Cobrar eventos de retorno de chamada](#cobrar-eventos-de-retorno-de-chamada)
- [Cobrar novas tentativas de retorno de chamada](#cobrar-novas-tentativas-de-retorno-de-chamada)
- [Saldo do cliente](#saldo-do-cliente)
- [Presentes](#presentes)
- [Configuração de referência](#configuração-de-referência)
- Listas de permissões [IP (resumo)](#ip-allow-lists-summary)
- [Retiradas de referência](#retiradas-de-referência)
- [Listas de permissões de IP (resumo)](#listas-de-permissões-de-ip-resumo)
- [Vinculações OAuth do cliente](#vinculações-oauth-do-cliente)
- [Ingressos (administrador)](#ingressos-administrador)
- [Artigos](#artigos)
- [Contas compartilhadas](#contas-compartilhadas)
- Configuração [EAB](#eab-configuration)
- Contas [ACME](#acme-accounts)
- [CA](#built-in-ca) integrado
- [Certificados](#certificados)
- Biblioteca de regras [WAF](#waf-rule-library)
- [Canais de certificado](#canais-de-certificado)
- [Configuração EAB](#configuração-eab)
- [Contas ACME](#contas-acme)
- [CA integrada](#ca-integrada)
- [Regras geográficas](#regras-geográficas)
- [Biblioteca de regras WAF](#biblioteca-de-regras-waf)
- [Gerenciamento de registros](#gerenciamento-de-registros)
- [Cópias de segurança](#cópias-de-segurança)
- [Configurações do sistema](#configurações-do-sistema)
- [Auditoria de segurança](#auditoria-de-segurança)
- [Métricas](#métricas)
- [Modelos de portal](#modelos-de-portal)
- [Instalação do agente](#instalação-do-agente)
- [Cobrar entrada de retorno de chamada](#cobrar-entrada-de-retorno-de-chamada)
- [Entradas (com nós)](#entradas-com-nós)
- [Configuração pública](#configuração-pública)
- [Autenticação de cliente e negócios (mesmo definido no mecanismo de administração)](#autenticação-de-cliente-e-negócios-mesmo-definido-no-mecanismo-de-administração)
- Desafios [ACME](#acme-challenges)

---

### Autenticação e segurança de administrador

Login, logout, alteração de senha, 2FA (TOTP), Passkey (WebAuthn) e tickets únicos para o painel de administração.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| POST | `/api/v1/auth/2fa/disable` | Conectado | Desativar |
| POST | `/api/v1/auth/2fa/enable` | Conectado | Habilitar |
| POST | `/api/v1/auth/2fa/recovery-codes` | Conectado | Gerar novamente códigos de recuperação |
| POST | `/api/v1/auth/2fa/setup` | Conectado | Configurações |
| GET | `/api/v1/auth/2fa/status` | Conectado | Estado |
| POST | `/api/v1/auth/change-password` | Conectado | Alterar senha |
| POST | `/api/v1/auth/login` | Público | Faça login |
| POST | `/api/v1/auth/login/2fa` | Público | Login de código dinâmico (2FA) |
| GET | `/api/v1/auth/login/2fa-status` | Público | Verifique se a conta tem 2FA ativado |
| POST | `/api/v1/auth/logout` | Conectado | Sair |
| GET | `/api/v1/auth/me` | Conectado | Usuário conectado atual |
| GET | `/api/v1/auth/passkey/credentials` | Conectado | Lista de credenciais Passkey |
| DELETE | `/api/v1/auth/passkey/credentials/:id` | Conectado | Excluir credencial Passkey |
| POST | `/api/v1/auth/passkey/login/begin` | Público | Iniciar login |
| POST | `/api/v1/auth/passkey/login/finish` | Público | Login completo |
| POST | `/api/v1/auth/passkey/register/begin` | Conectado | Iniciar registro |
| POST | `/api/v1/auth/passkey/register/finish` | Conectado | Concluir registro |
| GET | `/api/v1/auth/passkey/rpids` | Administrador | Lista de permissões de ID de RP |
| POST | `/api/v1/auth/passkey/rpids` | Administrador | Atualizar lista de permissões de ID de RP |
| GET | `/api/v1/auth/passkey/status` | Público | Estado |
| POST | `/api/v1/auth/refresh` | Público | Atualizar |
| POST | `/api/v1/auth/ticket` | Conectado | Emitir ticket único de curta duração |

### Contas de administrador

Contas de usuário do painel de administração. A listagem requer administrador; criar/atualizar/excluir requer superadministrador (evita escalonamento de privilégios).

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/users` | Administrador | Lista |
| POST | `/api/v1/users` | Superadministrador | Criar |
| DELETE | `/api/v1/users/:id` | Superadministrador | Excluir |
| PUT | `/api/v1/users/:id` | Superadministrador | Atualizar |

### Gestão de clientes

CRUD, suspensão/retomada, upgrade/downgrade, renovação e redefinição de uso para assinantes (contas do portal).

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/clients` | Conectado | Listar clientes |
| POST | `/api/v1/clients` | Operador | Criar cliente. corpo: `{"email":"user@example.com","password":"plaintext-password","plan_id":1,"quota_bytes":10737418240,"expires_at":"2026-12-31T00:00:00Z","remark":""}`. `plan_id` é necessário; `password` pode estar vazio (os clientes podem configurá-lo posteriormente no portal) |
| DELETE | `/api/v1/clients/:id` | Operador | Excluir cliente |
| GET | `/api/v1/clients/:id` | Conectado | Obter cliente |
| PUT | `/api/v1/clients/:id` | Operador | Atualizar cliente |
| POST | `/api/v1/clients/:id/downgrade` | Operador | Fazer downgrade do cliente |
| GET | `/api/v1/clients/:id/orders` | Conectado | Lista de pedidos do cliente |
| POST | `/api/v1/clients/:id/renew` | Operador | Renovar cliente |
| POST | `/api/v1/clients/:id/reset-usage` | Operador | Redefinir uso |
| POST | `/api/v1/clients/:id/restore` | Operador | Retomar cliente |
| POST | `/api/v1/clients/:id/suspend` | Operador | Suspender cliente |
| POST | `/api/v1/clients/:id/upgrade` | Operador | Atualizar cliente |

### Modelos de plano

Planos (variantes, ciclos de cobrança, preços, cotas de tráfego) e autorização de nó em nível de modelo.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/plan-templates` | Conectado | Lista de modelos |
| POST | `/api/v1/plan-templates` | Administrador | Criar modelo |
| DELETE | `/api/v1/plan-templates/:id` | Administrador | Excluir modelo |
| GET | `/api/v1/plan-templates/:id` | Conectado | Detalhes do modelo |
| PUT | `/api/v1/plan-templates/:id` | Administrador | Atualizar modelo |
| GET | `/api/v1/plan-templates/:id/accessible-nodes` | Conectado | Obtenha nós disponíveis |
| GET | `/api/v1/plan-templates/:id/billing-node-options` | Conectado | Obtenha opções de nós faturáveis |
| PUT | `/api/v1/plan-templates/:id/node-groups` | Administrador | Definir grupos de nós de modelo |
| POST | `/api/v1/plan-templates/:id/sync-accounts` | Administrador | Sincronizar contas com modelo |
| GET | `/api/v1/plan-templates/:id/variants` | Conectado | Lista de variantes |
| POST | `/api/v1/plan-templates/:id/variants` | Administrador | Criar variante |
| POST | `/api/v1/plan-templates/sync-all-accounts` | Administrador | Sincronize todas as contas |

### Variantes do plano

Atualizar e excluir variantes de plano (a criação está aninhada em modelos de plano).

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| DELETE | `/api/v1/plan-variants/:id` | Administrador | Excluir variante |
| PUT | `/api/v1/plan-variants/:id` | Administrador | Variante de atualização |

### Pacotes de trânsito

Pacotes de tráfego que podem ser adquiridos separadamente.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/traffic-packs` | Conectado | Lista de pacotes de tráfego |
| POST | `/api/v1/traffic-packs` | Administrador | Criar pacote de tráfego |
| DELETE | `/api/v1/traffic-packs/:id` | Administrador | Excluir pacote de tráfego |
| PUT | `/api/v1/traffic-packs/:id` | Administrador | Atualizar pacote de tráfego |

### Cupons

Crie, valide e resgate cupons.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/promotions` | Conectado | Listar cupons |
| POST | `/api/v1/promotions` | Administrador | Criar cupom |
| DELETE | `/api/v1/promotions/:id` | Administrador | Excluir cupom |
| PUT | `/api/v1/promotions/:id` | Administrador | Atualizar cupom |
| POST | `/api/v1/promotions/validate` | Conectado | Validar cupom |

### Grupos de nós

Organize nós em grupos referenciados por planos.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/node-groups` | Conectado | Listar grupos de nós |
| POST | `/api/v1/node-groups` | Administrador | Criar grupo de nós |
| DELETE | `/api/v1/node-groups/:id` | Administrador | Excluir grupo de nós |
| PUT | `/api/v1/node-groups/:id` | Administrador | Atualizar grupo de nós |
| GET | `/api/v1/node-groups/:id/members` | Conectado | Listar membros do grupo |
| POST | `/api/v1/node-groups/:id/members` | Administrador | Adicionar membro do grupo |
| DELETE | `/api/v1/node-groups/:id/members/:mid` | Administrador | Remover membro do grupo |

### Servidores de nó

Recursos principais: nó CRUD, entrega de configuração principal (xray/sing-box), configuração de entrada/saída/endpoint, controle de processo, certificados, WAF, limites de taxa, salto de porta, proxies encadeados, backup espelhado/primário, instalação/atualização de agente, controle de tempo de execução xray. Muitos subcaminhos, agrupados por família de recursos.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/node-servers` | Administrador | Lista |
| POST | `/api/v1/node-servers` | Administrador | Criar |
| DELETE | `/api/v1/node-servers/:id` | Administrador | Excluir |
| GET | `/api/v1/node-servers/:id` | Administrador | Detalhes |
| PUT | `/api/v1/node-servers/:id` | Administrador | Atualizar |
| GET | `/api/v1/node-servers/:id/binaries` | Administrador | Lista principal instalada |
| POST | `/api/v1/node-servers/:id/binaries/ech-generate` | Administrador | Gerar par de chaves ECH |
| POST | `/api/v1/node-servers/:id/binaries/install` | Administrador | Instalar/atualizar núcleo |
| GET | `/api/v1/node-servers/:id/binaries/interfaces` | Administrador | Lista de interfaces de rede |
| POST | `/api/v1/node-servers/:id/binaries/mldsa65-key` | Administrador | Gerar chave MLDSA65 |
| POST | `/api/v1/node-servers/:id/binaries/reality-key` | Administrador | Gerar par de chaves REALITY |
| POST | `/api/v1/node-servers/:id/binaries/reality-public-key` | Administrador | Derive a chave pública da chave privada REALITY |
| GET | `/api/v1/node-servers/:id/binaries/releases` | Administrador | Versões disponíveis |
| POST | `/api/v1/node-servers/:id/binaries/vlessenc` | Administrador | Gere parâmetros de criptografia VLESS |
| POST | `/api/v1/node-servers/:id/binaries/wg-key` | Administrador | Gerar chave WireGuard |
| POST | `/api/v1/node-servers/:id/binaries/x25519-key` | Administrador | Gerar par de chaves X25519 |
| GET | `/api/v1/node-servers/:id/bootstrap-cmd` | Administrador | Obter comando de inicialização do agente |
| POST | `/api/v1/node-servers/:id/bootstrap-token` | Administrador | Gerar token de inicialização |
| POST | `/api/v1/node-servers/:id/caddy/batch-update-log-enabled` | Administrador | Alterações de log de atualização em massa |
| POST | `/api/v1/node-servers/:id/caddy/batch-update-ports` | Administrador | Portas de atualização em massa |
| POST | `/api/v1/node-servers/:id/caddy/generate-config` | Administrador | Gerar configuração |
| DELETE | `/api/v1/node-servers/:id/caddy/l4-rules` | Administrador | Excluir todas as regras L4 |
| POST | `/api/v1/node-servers/:id/caddy/save-and-apply` | Administrador | Salvar e aplicar |
| GET | `/api/v1/node-servers/:id/caddy/sites` | Administrador | Lista de sites |
| POST | `/api/v1/node-servers/:id/caddy/sites` | Administrador | Criar site |
| POST | `/api/v1/node-servers/:id/caddy/import` | Administrador | Importar sites |
| POST | `/api/v1/node-servers/:id/caddy/preview-site` | Administrador | Pré-visualização do site Caddy renderizado |
| DELETE | `/api/v1/node-servers/:id/caddy/sites/:siteId` | Administrador | Excluir site |
| PUT | `/api/v1/node-servers/:id/caddy/sites/:siteId` | Administrador | Atualizar site |
| DELETE | `/api/v1/node-servers/:id/caddy/sni-rules` | Administrador | Exclua todas as regras SNI |
| POST | `/api/v1/node-servers/:id/caddy/sni-rules/toggle` | Administrador | Alternar regras SNI |
| GET | `/api/v1/node-servers/:id/caddy/sni-settings` | Administrador | Configurações SNI |
| POST | `/api/v1/node-servers/:id/caddy/sni-settings` | Administrador | Salvar configurações SNI |
| GET | `/api/v1/node-servers/:id/certificates` | Administrador | Lista de certificados de nós |
| DELETE | `/api/v1/node-servers/:id/certificates/:cid` | Administrador | Desvincular certificado |
| POST | `/api/v1/node-servers/:id/certificates/:cid` | Administrador | Vincular certificado |
| GET | `/api/v1/node-servers/:id/chain-config` | Administrador | Gerar configuração encadeada |
| GET | `/api/v1/node-servers/:id/chain-health` | Administrador | Estado de saúde da cadeia |
| POST | `/api/v1/node-servers/:id/control` | Superadministrador | Núcleo do nó de controle. corpo: `{"core_type":"xray","action":"start"}` (ação = iniciar/parar/reiniciar). `core_type` pode ser `xray`/`singbox` |
| GET | `/api/v1/node-servers/:id/degradation-status` | Administrador | Status degradado |
| POST | `/api/v1/node-servers/:id/distribute-cert` | Administrador | Distribuir certificado |
| GET | `/api/v1/node-servers/:id/endpoint-configs` | Administrador | Lista |
| POST | `/api/v1/node-servers/:id/endpoint-configs` | Administrador | Criar |
| DELETE | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | Administrador | Excluir |
| GET | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | Administrador | Detalhes |
| PUT | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | Administrador | Atualizar |
| POST | `/api/v1/node-servers/:id/failover` | Administrador | Acionar failover para este nó |
| GET | `/api/v1/node-servers/:id/failover-status` | Administrador | Status de failover |
| POST | `/api/v1/node-servers/:id/force-reconnect` | Administrador | Forçar reconexão |
| GET | `/api/v1/node-servers/:id/free-port` | Administrador | Solicite uma porta gratuita |
| POST | `/api/v1/node-servers/:id/geo-rules/push` | Administrador | Envie para este nó |
| GET | `/api/v1/node-servers/:id/firewall/status` | Administrador | Detecção de firewall (firewalld/ufw/nft/iptables) |
| POST | `/api/v1/node-servers/:id/firewall/allow-port` | Administrador | Permitir porta de escuta de entrada (com suporte para vários intervalos) |
| POST | `/api/v1/node-servers/:id/firewall/remove-port` | Administrador | Revogar permissão de porta de entrada (incluindo duplicatas) |
| GET | `/api/v1/node-servers/:id/inbound-configs` | Administrador | Lista |
| POST | `/api/v1/node-servers/:id/inbound-configs` | Administrador | Criar |
| DELETE | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | Administrador | Excluir |
| GET | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | Administrador | Detalhes |
| PUT | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | Administrador | Atualizar |
| GET | `/api/v1/node-servers/:id/inbound-configs/:cfgId/share-links` | Administrador | Compartilhe links e snippets |
| POST | `/api/v1/node-servers/:id/inbound-configs/:cfgId/users/:email/pause` | Administrador | Suspender este usuário |
| POST | `/api/v1/node-servers/:id/inbound-configs/:cfgId/users/:email/resume` | Administrador | Restaurar este usuário |
| POST | `/api/v1/node-servers/:id/inbound-configs/reset-all-traffic` | Administrador | Redefinir todo o tráfego de entrada |
| GET | `/api/v1/node-servers/:id/install-script` | Administrador | Obtenha o script de instalação do agente |
| GET | `/api/v1/node-servers/:id/metrics-history` | Administrador | Obtenha histórico de métricas |
| GET | `/api/v1/node-servers/:id/mirror-status` | Administrador | Status do espelho |
| POST | `/api/v1/node-servers/:id/nginx/batch-update-ports` | Administrador | Portas de atualização em massa |
| POST | `/api/v1/node-servers/:id/nginx/generate-config` | Administrador | Gerar configuração |
| DELETE | `/api/v1/node-servers/:id/nginx/l4-rules` | Administrador | Excluir todas as regras L4 |
| POST | `/api/v1/node-servers/:id/nginx/save-and-apply` | Administrador | Salvar e aplicar |
| GET | `/api/v1/node-servers/:id/nginx/settings` | Administrador | Ler configurações |
| POST | `/api/v1/node-servers/:id/nginx/settings` | Administrador | Salvar configurações |
| GET | `/api/v1/node-servers/:id/nginx/sites` | Administrador | Lista de sites |
| POST | `/api/v1/node-servers/:id/nginx/sites` | Administrador | Criar site |
| POST | `/api/v1/node-servers/:id/nginx/import` | Administrador | Importar sites |
| POST | `/api/v1/node-servers/:id/nginx/preview-site` | Administrador | Visualização do site Nginx renderizado |
| POST | `/api/v1/node-servers/:id/nginx/ssl/generate` | Administrador | Gerar arquivos de certificado SSL |
| DELETE | `/api/v1/node-servers/:id/nginx/sites/:siteId` | Administrador | Excluir site |
| PUT | `/api/v1/node-servers/:id/nginx/sites/:siteId` | Administrador | Atualizar site |
| DELETE | `/api/v1/node-servers/:id/nginx/sni-rules` | Administrador | Exclua todas as regras SNI |
| POST | `/api/v1/node-servers/:id/nginx/sni-rules/toggle` | Administrador | Alternar regras SNI |
| PUT | `/api/v1/node-servers/:id/node-type` | Administrador | Atualizar tipo de nó |
| GET | `/api/v1/node-servers/:id/online-users` | Administrador | Usuários on-line |
| GET | `/api/v1/node-servers/:id/outbound-configs` | Administrador | Lista |
| POST | `/api/v1/node-servers/:id/outbound-configs` | Administrador | Criar |
| DELETE | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | Administrador | Excluir |
| GET | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | Administrador | Detalhes |
| PUT | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | Administrador | Atualizar |
| PUT | `/api/v1/node-servers/:id/outbound-configs/reorder` | Administrador | Classificar |
| GET | `/api/v1/node-servers/:id/port-hop/rules` | Administrador | Visualização da regra de salto de porta |
| GET | `/api/v1/node-servers/:id/protocols` | Administrador | Lista de protocolos |
| GET | `/api/v1/node-servers/:id/protocols/:coreType` | Administrador | Configuração do protocolo |
| PUT | `/api/v1/node-servers/:id/protocols/:coreType` | Administrador | Salvar configuração do protocolo |
| GET | `/api/v1/node-servers/:id/protocols/:coreType/config-file` | Administrador | Leia o arquivo de configuração principal |
| GET | `/api/v1/node-servers/:id/protocols/:coreType/section/:section` | Administrador | Leia a seção do protocolo |
| PUT | `/api/v1/node-servers/:id/protocols/:coreType/section/:section` | Administrador | Salvar seção de protocolo |
| POST | `/api/v1/node-servers/:id/protocols/:coreType/validate` | Administrador | Validar configuração (teste de núcleo real) |
| GET | `/api/v1/node-servers/:id/proxy-status` | Administrador | Status do proxy remoto |
| POST | `/api/v1/node-servers/:id/proxy/batch-update-ports` | Administrador | Portas de atualização em massa |
| GET | `/api/v1/node-servers/:id/proxy/config` | Administrador | Ler configuração |
| PUT | `/api/v1/node-servers/:id/proxy/config` | Administrador | Atualizar configuração |
| POST | `/api/v1/node-servers/:id/proxy/control` | Administrador | Controle de registro de proxy encadeado |
| POST | `/api/v1/node-servers/:id/proxy/download` | Administrador | Baixe a versão principal do proxy |
| POST | `/api/v1/node-servers/:id/proxy/generate-config` | Administrador | Gerar configuração |
| POST | `/api/v1/node-servers/:id/proxy/guardian` | Administrador | Atualizar configuração do supervisor |
| POST | `/api/v1/node-servers/:id/proxy/install` | Administrador | Instalar proxy |
| DELETE | `/api/v1/node-servers/:id/proxy/l4-rules` | Administrador | Excluir todas as regras L4 |
| GET | `/api/v1/node-servers/:id/proxy/l4-settings` | Administrador | Configurações L4 |
| POST | `/api/v1/node-servers/:id/proxy/l4-settings` | Administrador | Salvar configurações L4 |
| GET | `/api/v1/node-servers/:id/proxy/log-settings` | Administrador | Configurações de registro |
| POST | `/api/v1/node-servers/:id/proxy/log-settings` | Administrador | Salvar configurações de registro |
| GET | `/api/v1/node-servers/:id/proxy/nginx-settings` | Administrador | Configurações nginx |
| POST | `/api/v1/node-servers/:id/proxy/nginx-settings` | Administrador | Salvar configurações nginx |
| GET | `/api/v1/node-servers/:id/proxy/releases` | Administrador | Versões disponíveis |
| POST | `/api/v1/node-servers/:id/proxy/reload` | Administrador | Recarregar configuração |
| POST | `/api/v1/node-servers/:id/proxy/restart` | Administrador | Reiniciar |
| GET | `/api/v1/node-servers/:id/proxy/sites` | Administrador | Lista de sites |
| POST | `/api/v1/node-servers/:id/proxy/sites` | Administrador | Criar site |
| DELETE | `/api/v1/node-servers/:id/proxy/sites/:siteId` | Administrador | Excluir site |
| PUT | `/api/v1/node-servers/:id/proxy/sites/:siteId` | Administrador | Atualizar site |
| DELETE | `/api/v1/node-servers/:id/proxy/sni-rules` | Administrador | Exclua todas as regras SNI |
| POST | `/api/v1/node-servers/:id/proxy/sni-rules/toggle` | Administrador | Alternar regras SNI |
| GET | `/api/v1/node-servers/:id/proxy/sni-settings` | Administrador | Configurações SNI |
| POST | `/api/v1/node-servers/:id/proxy/sni-settings` | Administrador | Salvar configurações SNI |
| POST | `/api/v1/node-servers/:id/proxy/start` | Administrador | Começar |
| GET | `/api/v1/node-servers/:id/proxy/status` | Administrador | Status do processo de proxy |
| POST | `/api/v1/node-servers/:id/proxy/stop` | Administrador | Pare |
| POST | `/api/v1/node-servers/:id/proxy/sync` | Administrador | Sincronizar configuração de proxy |
| GET | `/api/v1/node-servers/:id/rate-limit` | Administrador | Ler configuração |
| PUT | `/api/v1/node-servers/:id/rate-limit` | Administrador | Atualizar configuração |
| GET | `/api/v1/node-servers/:id/rate-limit/status` | Administrador | Estado |
| GET | `/api/v1/node-servers/:id/recover-mode` | Administrador | Leia o modo de recuperação do nó |
| PUT | `/api/v1/node-servers/:id/recover-mode` | Administrador | Definir modo de recuperação de nó |
| POST | `/api/v1/node-servers/:id/revoke-cert` | Administrador | Revogar certificado do Agente |
| GET | `/api/v1/node-servers/:id/services` | Administrador | Lista |
| POST | `/api/v1/node-servers/:id/services` | Administrador | Criar |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId` | Administrador | Excluir |
| GET | `/api/v1/node-servers/:id/services/:serviceId` | Administrador | Detalhes |
| PUT | `/api/v1/node-servers/:id/services/:serviceId` | Administrador | Atualizar |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId/crash-logs` | Administrador | Limpar registros de falhas |
| GET | `/api/v1/node-servers/:id/services/:serviceId/crash-logs` | Administrador | Lista de registros de falhas |
| GET | `/api/v1/node-servers/:id/services/:serviceId/crash-logs/:logId` | Administrador | Detalhes do registro de falhas |
| GET | `/api/v1/node-servers/:id/services/:serviceId/endpoints` | Administrador | Lista de endpoints |
| POST | `/api/v1/node-servers/:id/services/:serviceId/endpoints` | Administrador | Criar ponto de extremidade |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId/endpoints/:endpointId` | Administrador | Excluir ponto de extremidade |
| PUT | `/api/v1/node-servers/:id/services/:serviceId/endpoints/:endpointId` | Administrador | Atualizar ponto de extremidade |
| PUT | `/api/v1/node-servers/:id/services/:serviceId/guardian` | Administrador | Atualizar configuração do supervisor |
| GET | `/api/v1/node-servers/:id/services/:serviceId/process-status` | Administrador | Obtenha o status do processamento |
| POST | `/api/v1/node-servers/:id/services/:serviceId/restart` | Administrador | Reiniciar |
| POST | `/api/v1/node-servers/:id/services/:serviceId/start` | Administrador | Começar |
| POST | `/api/v1/node-servers/:id/services/:serviceId/stop` | Administrador | Pare |
| GET | `/api/v1/node-servers/:id/services/process-status` | Administrador | Todos os status do processo |
| GET | `/api/v1/node-servers/:id/share-domains` | Administrador | Compartilhe domínios para este nó |
| POST | `/api/v1/node-servers/:id/share-domains` | Administrador | Criar |
| DELETE | `/api/v1/node-servers/:id/share-domains/:domainId` | Administrador | Excluir |
| PUT | `/api/v1/node-servers/:id/share-domains/:domainId` | Administrador | Atualizar |
| DELETE | `/api/v1/node-servers/:id/share-domains/all` | Administrador | Limpar tudo |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/apply` | Administrador | Aplicar regras de firewall de salto de porta |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/remove` | Administrador | Remover regras de firewall de salto de porta |
| GET | `/api/v1/node-servers/:id/singbox/connections` | Administrador | Conexões sing-box ativas (clash_api, somente nó local) |
| POST | `/api/v1/node-servers/:id/speed-test` | Administrador | Teste de velocidade do nó |
| GET | `/api/v1/node-servers/:id/status` | Administrador | Status do nó |
| POST | `/api/v1/node-servers/:id/sync` | Administrador | Configuração do nó de sincronização (gerar e enviar por push) |
| POST | `/api/v1/node-servers/:id/sync-data` | Superadministrador | Enviar dados do painel para nós (sincronização completa de usuários/assinaturas/planos) |
| POST | `/api/v1/node-servers/:id/sync-mirror` | Administrador | Sincronizar nó de espelho |
| GET | `/api/v1/node-servers/:id/tags` | Administrador | Lista de tags |
| POST | `/api/v1/node-servers/:id/tags` | Administrador | Adicionar etiqueta |
| DELETE | `/api/v1/node-servers/:id/tags/:tagKey` | Administrador | Excluir etiqueta |
| GET | `/api/v1/node-servers/:id/traffic-history` | Administrador | Histórico de trânsito |
| POST | `/api/v1/node-servers/:id/traffic-resume` | Administrador | Restaurar o tráfego suspenso por excesso |
| GET | `/api/v1/node-servers/:id/traffic-usage` | Administrador | Uso de tráfego |
| POST | `/api/v1/node-servers/:id/update-agent` | Administrador | Agente de atualização |
| GET | `/api/v1/node-servers/:id/used-server-names` | Administrador | Lista REALITY SNI em uso |
| GET | `/api/v1/node-servers/:id/waf/config` | Administrador | Ler configuração |
| PUT | `/api/v1/node-servers/:id/waf/config` | Administrador | Salvar configuração |
| POST | `/api/v1/node-servers/:id/waf/disable-domain` | Administrador | Desativar proteção de domínio |
| GET | `/api/v1/node-servers/:id/waf/domain-config` | Administrador | Lista de configurações de domínio |
| PUT | `/api/v1/node-servers/:id/waf/domain-config` | Administrador | Salvar configurações de domínio em massa |
| DELETE | `/api/v1/node-servers/:id/waf/domain-config/:domain` | Administrador | Excluir configuração de domínio |
| GET | `/api/v1/node-servers/:id/waf/domain-config/:domain` | Administrador | Detalhes de configuração do domínio |
| PUT | `/api/v1/node-servers/:id/waf/domain-config/:domain` | Administrador | Salvar configuração do domínio |
| GET | `/api/v1/node-servers/:id/waf/domain-rules` | Administrador | Diretório de regras de domínio |
| GET | `/api/v1/node-servers/:id/waf/domain-rules/:domain/:name` | Administrador | Ler arquivo de regras de domínio |
| PUT | `/api/v1/node-servers/:id/waf/domain-rules/:domain/:name` | Administrador | Salvar arquivo de regras de domínio |
| GET | `/api/v1/node-servers/:id/waf/domain-status` | Administrador | Status de proteção de domínio |
| POST | `/api/v1/node-servers/:id/waf/enable-domain` | Administrador | Habilitar proteção de domínio |
| POST | `/api/v1/node-servers/:id/waf/push` | Administrador | Envie para este nó |
| GET | `/api/v1/node-servers/:id/waf/rules` | Administrador | Lista de regras do nó WAF |
| DELETE | `/api/v1/node-servers/:id/waf/rules/:name` | Administrador | Excluir regras do nó WAF |
| GET | `/api/v1/node-servers/:id/waf/rules/:name` | Administrador | Detalhes da regra do nó WAF |
| PUT | `/api/v1/node-servers/:id/waf/rules/:name` | Administrador | Salvar regras do nó WAF |
| GET | `/api/v1/node-servers/:id/waf/status` | Administrador | Status do nó WAF |
| PUT | `/api/v1/node-servers/:id/waf/status` | Administrador | Alternar nó WAF |
| POST | `/api/v1/node-servers/:id/waf/sync` | Administrador | Sincronizar com este nó |
| GET | `/api/v1/node-servers/:id/warp` | Administrador | Lista |
| DELETE | `/api/v1/node-servers/:id/warp/:warpId` | Administrador | Excluir |
| GET | `/api/v1/node-servers/:id/warp/config` | Administrador | Ler configuração |
| POST | `/api/v1/node-servers/:id/warp/generate-config` | Administrador | Gerar configuração |
| POST | `/api/v1/node-servers/:id/warp/register` | Administrador | Cadastre-se |
| POST | `/api/v1/node-servers/:id/warp/masque/register` | Administrador | Registre o perfil WARP MASQUE (etapa 1 de 2) |
| POST | `/api/v1/node-servers/:id/warp/:warpId/masque-outbound` | Administrador | Crie uma saída sing-box MASQUE a partir de um perfil WARP |
| GET | `/api/v1/node-servers/:id/warp/status` | Administrador | Estado |
| GET | `/api/v1/node-servers/:id/xray/control/balancer/:tag` | Administrador | Obtenha informações do balanceador |
| POST | `/api/v1/node-servers/:id/xray/control/balancer/:tag` | Administrador | Definir meta do balanceador |
| POST | `/api/v1/node-servers/:id/xray/control/firewall/apply` | Administrador | Aplicar regras de firewall de salto de porta |
| POST | `/api/v1/node-servers/:id/xray/control/firewall/remove` | Administrador | Remover regras de firewall de salto de porta |
| POST | `/api/v1/node-servers/:id/xray/control/inbound` | Administrador | Adicionar entrada |
| DELETE | `/api/v1/node-servers/:id/xray/control/inbound/:tag` | Administrador | Remover entrada |
| POST | `/api/v1/node-servers/:id/xray/control/inbound/:tag/user` | Administrador | Adicionar usuário |
| DELETE | `/api/v1/node-servers/:id/xray/control/inbound/:tag/user/:email` | Administrador | Remover usuário |
| POST | `/api/v1/node-servers/:id/xray/control/outbound` | Administrador | Adicionar saída |
| DELETE | `/api/v1/node-servers/:id/xray/control/outbound/:tag` | Administrador | Remover saída |
| POST | `/api/v1/node-servers/:id/xray/control/routing` | Administrador | Solicitar rota |
| POST | `/api/v1/node-servers/:id/xray/control/routing/rule` | Administrador | Adicionar regra de roteamento |
| DELETE | `/api/v1/node-servers/:id/xray/control/routing/rule/:tag` | Administrador | Remover regra de roteamento |
| GET | `/api/v1/node-servers/:id/xray/control/routing/rules` | Administrador | Listar regras de roteamento |
| POST | `/api/v1/node-servers/:id/xray/control/sync` | Administrador | Acionar manualmente a sincronização de configuração xray |
| GET | `/api/v1/node-servers/:id/xray/stats` | Administrador | Obtenha estatísticas |
| POST | `/api/v1/node-servers/:id/xray/stats/reset/:email` | Administrador | Redefinir estatísticas do usuário |
| GET | `/api/v1/node-servers/:id/xray/stats/users` | Administrador | Obtenha estatísticas do usuário |
| GET | `/api/v1/node-servers/billable` | Administrador | Lista de nós faturáveis |
| GET | `/api/v1/node-servers/ca-cert` | Administrador | Certificado CA |
| GET | `/api/v1/node-servers/ca-status` | Administrador | Status da autoridade de certificação |
| GET | `/api/v1/node-servers/dashboard` | Administrador | Status do painel |
| POST | `/api/v1/node-servers/distribute-cert` | Administrador | Distribuir certificado para todos os nós |
| POST | `/api/v1/node-servers/failover-check` | Administrador | Acionar verificação de failover |
| GET | `/api/v1/node-servers/metrics/aggregated` | Administrador | Métricas agregadas |
| POST | `/api/v1/node-servers/migrate-cert` | Administrador | Migrar certificado |
| GET | `/api/v1/node-servers/online-summary` | Administrador | Resumo on-line |
| GET | `/api/v1/node-servers/online-users` | Administrador | Todos os usuários on-line |
| GET | `/api/v1/node-servers/pending-agent-updates` | Administrador | Agentes com atualização pendente |
| POST | `/api/v1/node-servers/recommend-region` | Administrador | Regiões recomendadas |
| POST | `/api/v1/node-servers/select-nodes` | Administrador | Selecione nós para assinatura |
| POST | `/api/v1/node-servers/speed-test-all` | Administrador | Teste de velocidade em todos os nós |
| GET | `/api/v1/node-servers/speed-test-results` | Administrador | Resultados do teste de velocidade |
| GET | `/api/v1/node-servers/traffic/stats` | Administrador | Estatísticas de tráfego |
| POST | `/api/v1/node-servers/update-all-agents` | Administrador | Atualizar todos os agentes |
| DELETE | `/api/v1/node-servers/users/:client_id` | Administrador | Sincronizar-excluir um cliente |
| GET | `/api/v1/node-servers/users/retry-queue` | Administrador | Tentar novamente o status da fila |
| POST | `/api/v1/node-servers/users/retry-queue/process` | Administrador | Fila de novas tentativas de processo |
| POST | `/api/v1/node-servers/users/sync` | Administrador | Sincronize um único cliente |
| POST | `/api/v1/node-servers/validate-chain-config` | Administrador | Validar configuração encadeada |

### Log de auditoria do nó

Auditoria e exportação de operação do lado do nó.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/node-audit-logs` | Administrador | Log de auditoria do nó |
| GET | `/api/v1/node-audit-logs/export` | Administrador | Exportar logs de auditoria de nós |

### Nós (modelo legado, retirado)

> `/api/v1/nodes` CRUD foi descontinuado em 20/10/2026: os nós são gerenciados uniformemente por meio de `/api/v1/node-servers`.
> As consultas dos usuários online permanecem em `/api/v1/node-servers/{online-users,online-summary,:id/online-users}`.

### Visão geral da região

Visualização de nós agregada por região.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/regions/:region/servers` | Conectado | Lista de nós para esta região |
| GET | `/api/v1/regions/overview` | Conectado | Visão geral da região |

### Assinaturas

Gerenciamento de registros de assinatura: revogação, rotação de token, logs de acesso, redefinição de uso.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/subscriptions` | Conectado | Lista |
| POST | `/api/v1/subscriptions` | Operador | Criar |
| DELETE | `/api/v1/subscriptions/:id` | Operador | Excluir |
| GET | `/api/v1/subscriptions/:id` | Conectado | Detalhes |
| PUT | `/api/v1/subscriptions/:id` | Operador | Atualizar |
| GET | `/api/v1/subscriptions/:id/access-logs` | Conectado | Registro de acesso |
| POST | `/api/v1/subscriptions/:id/enable` | Operador | Habilitar |
| POST | `/api/v1/subscriptions/:id/reset-usage` | Operador | Redefinir uso |
| POST | `/api/v1/subscriptions/:id/restore` | Operador | Currículo |
| POST | `/api/v1/subscriptions/:id/revoke` | Operador | Revogar |
| POST | `/api/v1/subscriptions/:id/rotate` | Operador | Girar |
| GET | `/api/v1/subscriptions/:id/rotations` | Conectado | Histórico de rotação |

### Formatos de assinatura

> O `/api/v1/subscription-formats` CRUD do lado do administrador foi desativado em 20/10/2026: o ponto de entrada real é
> `/api/v1/mihomo/configs` (centro de formato de assinatura). A tabela `subscription_formats` permanece,
> usados apenas como dados iniciais para o endpoint público `/api/v1/public/client-configs`.

### Pontos de extremidade de assinatura

Domínios de assinatura dedicados (com proxy reverso para o caminho `/sub` do painel).

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/subscription-endpoints` | Conectado | Lista |
| POST | `/api/v1/subscription-endpoints` | Administrador | Criar |
| DELETE | `/api/v1/subscription-endpoints/:id` | Administrador | Excluir |
| GET | `/api/v1/subscription-endpoints/:id` | Conectado | Detalhes |
| PUT | `/api/v1/subscription-endpoints/:id` | Administrador | Atualizar |

### Compartilhar domínios

Acesse o pool de domínio/IP usado em links de clientes.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/share-domains` | Conectado | Lista |
| POST | `/api/v1/share-domains` | Administrador | Criar |
| DELETE | `/api/v1/share-domains/:id` | Administrador | Excluir |
| PUT | `/api/v1/share-domains/:id` | Administrador | Atualizar |
| DELETE | `/api/v1/share-domains/all` | Administrador | Limpar tudo |
| POST | `/api/v1/share-domains/seed` | Administrador | Inicializar dados padrão |

### IP Cloudflare

Cloudflare CDN Pool de IP e gerenciamento de origem.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| DELETE | `/api/v1/cfip/entries` | Administrador | Limpar entradas |
| GET | `/api/v1/cfip/entries` | Conectado | Lista de inscrições |
| POST | `/api/v1/cfip/entries` | Administrador | Adicionar entrada |
| DELETE | `/api/v1/cfip/entries/:id` | Administrador | Excluir entrada |
| POST | `/api/v1/cfip/fetch-all` | Administrador | Buscar todas as fontes |
| GET | `/api/v1/cfip/probe-config` | Conectado | Configuração do probe IP preferencial CF (nós de entrada + discadores instalados) |
| PUT | `/api/v1/cfip/probe-config` | Administrador | Salvar intervalo de sondagem/limite de latência/nó de entrada/discador |
| POST | `/api/v1/cfip/check` | Administrador | Teste todos os domínios/IPs preferenciais agora |
| POST | `/api/v1/cfip/seed` | Administrador | Inicializar dados padrão |
| GET | `/api/v1/cfip/sources` | Conectado | Lista de fontes de dados |
| POST | `/api/v1/cfip/sources` | Administrador | Criar fonte de dados |
| DELETE | `/api/v1/cfip/sources/:id` | Administrador | Excluir fonte de dados |
| PUT | `/api/v1/cfip/sources/:id` | Administrador | Atualizar fonte de dados |
| POST | `/api/v1/cfip/sources/:id/fetch` | Administrador | Busque esta fonte |

### Configuração de assinatura (Mihomo/Clash)

Configuração do modelo de assinatura: grupos, regras, provedores de regras, visualização YAML.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/mihomo/configs` | Conectado | Lista |
| POST | `/api/v1/mihomo/configs` | Administrador | Criar |
| DELETE | `/api/v1/mihomo/configs/:id` | Administrador | Excluir |
| GET | `/api/v1/mihomo/configs/:id` | Conectado | Detalhes |
| PUT | `/api/v1/mihomo/configs/:id` | Administrador | Atualizar |
| GET | `/api/v1/mihomo/configs/:id/rules` | Conectado | Lista de regras |
| POST | `/api/v1/mihomo/configs/:id/rules` | Administrador | Criar regra |
| PUT | `/api/v1/mihomo/configs/:id/rules/order` | Administrador | Reordenar regras |
| PUT | `/api/v1/mihomo/rules/:id` | Administrador | Atualizar regra |
| DELETE | `/api/v1/mihomo/rules/:id` | Administrador | Excluir regra |

### Sites de recarga

Integração de sites de pagamento de terceiros: chaves de assinatura, arquivos de verificação, verificação de domínio, documentos de integração.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/charge-sites` | Administrador | Lista de sites |
| POST | `/api/v1/charge-sites` | Administrador | Criar site |
| DELETE | `/api/v1/charge-sites/:id` | Administrador | Excluir site |
| GET | `/api/v1/charge-sites/:id` | Administrador | Detalhes do site |
| PUT | `/api/v1/charge-sites/:id` | Administrador | Atualizar site |
| POST | `/api/v1/charge-sites/:id/clear-prev-secret` | Administrador | Limpar chave antiga |
| POST | `/api/v1/charge-sites/:id/generate-verify` | Administrador | Gerar arquivo de verificação |
| POST | `/api/v1/charge-sites/:id/generate-verify-file` | Administrador | Gerar arquivo de verificação |
| GET | `/api/v1/charge-sites/:id/integration-doc` | Administrador | Obtenha documento de integração |
| POST | `/api/v1/charge-sites/:id/rotate-secret` | Administrador | Girar chave de assinatura |
| POST | `/api/v1/charge-sites/:id/verify-dns` | Administrador | Verifique o domínio DNS |
| POST | `/api/v1/charge-sites/:id/verify-domain` | Administrador | Verifique o desafio do domínio HTTP |
| POST | `/api/v1/charge-sites/:id/verify-domain-dns` | Administrador | Verifique o domínio DNS |
| POST | `/api/v1/charge-sites/:id/verify-domain-http` | Administrador | Verifique o desafio do domínio HTTP |

### Recarregar pedidos

Consultas de pedidos de recarga, reembolsos, exportação CSV.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/charge-orders` | Administrador | Lista de pedidos |
| GET | `/api/v1/charge-orders/:id` | Administrador | Detalhes do pedido |
| POST | `/api/v1/charge-orders/:id/refund` | Administrador | Reembolso |
| GET | `/api/v1/charge-orders/export-csv` | Administrador | Pedidos de recarga de exportação CSV |

### Cartões de recarga

Geração de cartões em massa, registros de uso, exportação CSV.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/charge-cards` | Administrador | Lista de cartas |
| POST | `/api/v1/charge-cards` | Administrador | Criar cartão |
| DELETE | `/api/v1/charge-cards/:id` | Administrador | Excluir cartão |
| GET | `/api/v1/charge-cards/:id/usages` | Administrador | Registros de uso do cartão |
| POST | `/api/v1/charge-cards/batch` | Administrador | Gerar cartões em massa |
| GET | `/api/v1/charge-cards/export-csv` | Administrador | Cartões de exportação CSV |

### Cobrar eventos de retorno de chamada

Auditoria de evento de retorno de chamada.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/charge-callback-events` | Conectado | Lista de eventos de retorno de chamada |

### Cobrar novas tentativas de retorno de chamada

Nova tentativa manual de retornos de chamada com falha.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/charge-callback-retries` | Conectado | Lista de retorno de chamada com falha |
| POST | `/api/v1/charge-callback-retries/:id/retry` | Administrador | Tentar novamente o retorno de chamada |

### Saldo do cliente

Visualização administrativa de saldos e transações.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/client-balances` | Conectado | Lista de saldo do cliente |
| GET | `/api/v1/client-balances/transactions` | Conectado | Transações de saldo (admin) |
| GET | `/api/v1/client-balances/transactions/export-csv` | Conectado | Transações de saldo de exportação CSV |

### Presentes

Emitir presentes exigíveis (dias/tráfego).

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/client-gifts` | Conectado | Lista de presentes |
| POST | `/api/v1/client-gifts` | Administrador | Criar presente |
| POST | `/api/v1/client-gifts/batch` | Administrador | Crie presentes em massa |
| GET | `/api/v1/client-gifts/search-clients` | Conectado | Pesquisar clientes |

### Configuração de referência

Configuração e estatísticas da comissão de referência.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/referral/config` | Conectado | Configuração de referência |
| PUT | `/api/v1/referral/config` | Administrador | Salvar configuração de referência |
| GET | `/api/v1/referral/records` | Conectado | Lista de registros de referência |
| GET | `/api/v1/referral/stats` | Conectado | Estatísticas de referência |

### Retiradas de referência

Revise as solicitações de retirada.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/referral-withdrawals` | Administrador | Lista de solicitações de saque |
| POST | `/api/v1/referral-withdrawals/:id/approve` | Administrador | Aprovar retirada |
| POST | `/api/v1/referral-withdrawals/:id/reject` | Administrador | Rejeitar retirada |

### Listas de permissões de IP (resumo)

Visualização da lista de permissões de IP em todos os clientes.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/ip-whitelists` | Conectado | Todas as listas de permissões de IP do cliente |

### Vinculações OAuth do cliente

Consulte vinculações de contas de terceiros do cliente.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/client-oauth-bindings` | Conectado | Lista de vinculação OAuth do cliente |

### Ingressos (administrador)

Resposta, atribuição e encerramento de tickets.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/tickets` | Conectado | Lista de ingressos |
| GET | `/api/v1/tickets/:id` | Conectado | Detalhes do ingresso |
| POST | `/api/v1/tickets/:id/assign` | Operador | Atribuir tíquete |
| POST | `/api/v1/tickets/:id/close` | Operador | Fechar tíquete |
| POST | `/api/v1/tickets/:id/reply` | Operador | Responder ao ticket |

### Artigos

CRUD para artigos do portal.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/articles` | Conectado | Lista de artigos |
| POST | `/api/v1/articles` | Administrador | Criar artigo |
| DELETE | `/api/v1/articles/:id` | Administrador | Excluir artigo |
| GET | `/api/v1/articles/:id` | Conectado | Detalhes do artigo |
| PUT | `/api/v1/articles/:id` | Administrador | Artigo de atualização |

### Contas compartilhadas

Emissão e reparo de registros de contas compartilhadas (streaming, etc.).

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/shared-accounts` | Conectado | Lista de contas compartilhadas |
| POST | `/api/v1/shared-accounts` | Administrador | Criar conta compartilhada |
| DELETE | `/api/v1/shared-accounts/:id` | Administrador | Excluir conta compartilhada |
| GET | `/api/v1/shared-accounts/:id` | Conectado | Detalhes da conta compartilhada |
| PUT | `/api/v1/shared-accounts/:id` | Administrador | Atualizar conta compartilhada |
| GET | `/api/v1/shared-accounts/:id/repair-logs` | Conectado | Registros de reparo |

### Certificados

Emissão, upload, renovação e exportação de certificado público (ACME); a emissão em um nó específico é suportada.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/certificates` | Administrador | Lista |
| POST | `/api/v1/certificates` | Administrador | Criar |
| DELETE | `/api/v1/certificates/:id` | Administrador | Excluir |
| GET | `/api/v1/certificates/:id` | Administrador | Detalhes |
| PUT | `/api/v1/certificates/:id` | Administrador | Atualizar |
| GET | `/api/v1/certificates/:id/download` | Administrador | Baixar |
| POST | `/api/v1/certificates/:id/renew` | Administrador | Renovar cliente |
| GET | `/api/v1/certificates/check-port` | Administrador | Verifique a porta |
| GET | `/api/v1/certificates/detect-ip` | Administrador | Sondar IP |
| GET | `/api/v1/certificates/export` | Administrador | Exportar |
| POST | `/api/v1/certificates/issue` | Administrador | Problema. corpo: `{"domains":["example.com","*.example.com"],"challenge_type":"dns-01","dns_channel_id":2,"key_type":"rsa","key_bits":4096}`. O padrão `challenge_type` é http-01; `dns_channel_id` seleciona um canal DNS (um provedor DNS configurado + credenciais) |
| POST | `/api/v1/certificates/issue-on-node` | Administrador | Emitir para o nó |
| POST | `/api/v1/certificates/upload` | Administrador | Carregar |
| GET | `/api/v1/certificate-tasks/:id` | Administrador | Consultar o status da tarefa em segundo plano |

### Canais de certificado

Configuração do canal (CA) ACME.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/cert-channels` | Administrador | Lista |
| POST | `/api/v1/cert-channels` | Administrador | Criar |
| DELETE | `/api/v1/cert-channels/:id` | Administrador | Excluir |
| PUT | `/api/v1/cert-channels/:id` | Administrador | Atualizar |
| GET | `/api/v1/cert-channels/providers` | Administrador | Provedor |

### Configuração EAB

Credenciais vinculando contas externas, como ZeroSSL/Google.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/eab-configs` | Administrador | Lista |
| POST | `/api/v1/eab-configs` | Administrador | Criar |
| DELETE | `/api/v1/eab-configs/:id` | Administrador | Excluir |
| PUT | `/api/v1/eab-configs/:id` | Administrador | Atualizar |
| POST | `/api/v1/eab-configs/fetch-google` | Administrador | Buscar Google EAB do GCP |
| POST | `/api/v1/eab-configs/fetch-zerossl` | Administrador | Obtenha ZeroSSL EAB com um e-mail com um clique |

### Contas ACME

Gerenciamento de conta ACME no painel (e-mail de contato).

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/acme-accounts` | Administrador | Lista |
| POST | `/api/v1/acme-accounts` | Administrador | Criar |
| DELETE | `/api/v1/acme-accounts/:id` | Administrador | Excluir |
| PUT | `/api/v1/acme-accounts/:id` | Administrador | Atualizar |
| GET | `/api/v1/acme-accounts/default` | Administrador | Obter padrões |

### CA integrada

CA do painel interno: emissão/renovação/revogação de certificado do agente, exportação de CRL, fila de distribuição.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/ca/agent-certs` | Administrador | Lista de certificados de agente |
| POST | `/api/v1/ca/agent-certs/:server_id/renew` | Administrador | Renovar certificado de Agente |
| GET | `/api/v1/ca/distribution-queue` | Administrador | Fila de distribuição de certificados |
| DELETE | `/api/v1/ca/distribution-queue/:id` | Administrador | Excluir entrada da fila de distribuição |
| POST | `/api/v1/ca/distribution-queue/:id/retry` | Administrador | Tentar novamente a distribuição do certificado |
| GET | `/api/v1/ca/revocations` | Administrador | Lista de revogação |
| GET | `/api/v1/ca/revocations/export` | Administrador | Exportar CRL |
| GET | `/api/v1/ca/status` | Administrador | Status da autoridade de certificação |

### Regras geográficas

Sincronização e entrega de fontes e arquivos de dados geoip/geosite.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/geo-rules/files` | Conectado | Lista de arquivos |
| DELETE | `/api/v1/geo-rules/files/:fileId` | Administrador | Excluir arquivo |
| PUT | `/api/v1/geo-rules/files/:fileId/auto-sync` | Administrador | Alternar sincronização automática |
| POST | `/api/v1/geo-rules/files/:fileId/download` | Administrador | Baixar |
| GET | `/api/v1/geo-rules/files/:fileId/download-file` | Conectado | Baixar arquivo |
| GET | `/api/v1/geo-rules/github-assets` | Conectado | Buscar ativos do repositório GitHub |
| POST | `/api/v1/geo-rules/push-all` | Administrador | Enviar para todos os nós |
| POST | `/api/v1/geo-rules/seed` | Administrador | Inicializar dados padrão |
| GET | `/api/v1/geo-rules/sources` | Conectado | Lista de fontes de dados |
| POST | `/api/v1/geo-rules/sources` | Administrador | Criar fonte de dados |
| DELETE | `/api/v1/geo-rules/sources/:sourceId` | Administrador | Excluir fonte de dados |
| PUT | `/api/v1/geo-rules/sources/:sourceId` | Administrador | Atualizar fonte de dados |
| GET | `/api/v1/geo-rules/sources/:sourceId/assets` | Conectado | Buscar ativos de liberação |
| POST | `/api/v1/geo-rules/sources/:sourceId/files` | Administrador | Adicionar arquivo |
| PUT | `/api/v1/geo-rules/sources/:sourceId/schedule` | Administrador | Alternar sincronização agendada |
| POST | `/api/v1/geo-rules/sources/:sourceId/select` | Administrador | Selecione o arquivo |
| POST | `/api/v1/geo-rules/sources/:sourceId/sync` | Administrador | Sincronizar esta fonte de dados |
| GET | `/api/v1/geo-rules/status` | Conectado | Estado |
| POST | `/api/v1/geo-rules/sync-all` | Administrador | Sincronizar tudo |
| GET | `/api/v1/geo-rules/sync-status/:key` | Conectado | Sincronizar progresso |

### Biblioteca de regras WAF

Regras globais WAF: origem GitHub, sincronização, entrega para todos os nós.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/waf/github-source` | Conectado | Fonte de dados GitHub |
| PUT | `/api/v1/waf/github-source` | Administrador | Salvar fonte de dados GitHub |
| POST | `/api/v1/waf/push-all` | Administrador | Enviar para todos os nós |
| GET | `/api/v1/waf/rules` | Conectado | Lista de regras |
| DELETE | `/api/v1/waf/rules/:name` | Administrador | Excluir regra |
| GET | `/api/v1/waf/rules/:name` | Conectado | Obter regra |
| PUT | `/api/v1/waf/rules/:name` | Administrador | Salvar regra |
| POST | `/api/v1/waf/sync-all` | Administrador | Sincronizar tudo |
| POST | `/api/v1/waf/sync-github` | Administrador | Sincronizar de GitHub |

### Gerenciamento de registros

Leitura e streaming de log de nó/núcleo, políticas de limpeza, controle de gravador de log.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| POST | `/api/v1/log-manage/cleanup-all` | Administrador | Limpe tudo agora |
| GET | `/api/v1/log-manage/cleanup-rules` | Administrador | Lista de regras de limpeza |
| PUT | `/api/v1/log-manage/cleanup-rules/:id` | Administrador | Atualizar regra de limpeza |
| POST | `/api/v1/log-manage/cleanup-table/:table` | Administrador | Limpe a tabela especificada agora |
| GET | `/api/v1/log-manage/core-log-output` | Administrador | Configuração de saída de log principal |
| PUT | `/api/v1/log-manage/core-log-output` | Administrador | Salvar configuração de saída de log principal |
| GET | `/api/v1/log-manage/core/:type/stdout-log` | Administrador | Leia os principais logs stdout |
| GET | `/api/v1/log-manage/core/:type/stdout-stream` | Administrador | Stream principal stdout |
| POST | `/api/v1/log-manage/discover` | Administrador | Descubra arquivos de log do nó |
| GET | `/api/v1/log-manage/log-output` | Administrador | Configuração de saída de registro |
| PUT | `/api/v1/log-manage/log-output` | Administrador | Salvar configuração de saída de log |
| GET | `/api/v1/log-manage/nodes/:serverId/logs` | Administrador | Lista de arquivos de log do nó |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type` | Administrador | Ler logs de nós |
| POST | `/api/v1/log-manage/nodes/:serverId/logs/:type/rotate` | Administrador | Girar logs de nós |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type/stream` | Administrador | Logs de nós de streaming |
| GET | `/api/v1/log-manage/policies` | Administrador | Lista de políticas |
| POST | `/api/v1/log-manage/policies` | Administrador | Criar política |
| DELETE | `/api/v1/log-manage/policies/:id` | Administrador | Excluir política |
| GET | `/api/v1/log-manage/policies/:id` | Administrador | Detalhes da política |
| PUT | `/api/v1/log-manage/policies/:id` | Administrador | Atualizar política |
| GET | `/api/v1/log-manage/policies/:id/jobs` | Administrador | Lista de trabalhos de limpeza |
| GET | `/api/v1/log-manage/policies/:id/log` | Administrador | Ler registros |
| GET | `/api/v1/log-manage/policies/:id/log-stream` | Administrador | Registros de fluxo |
| POST | `/api/v1/log-manage/policies/:id/rotate` | Administrador | Girar |
| GET | `/api/v1/log-manage/proxy/:type/stdout-log` | Administrador | Leia os principais logs stdout |
| GET | `/api/v1/log-manage/proxy/:type/stdout-stream` | Administrador | Stream principal stdout |
| GET | `/api/v1/log-manage/scheduler-config` | Administrador | Configuração de agendamento |
| PUT | `/api/v1/log-manage/scheduler-config` | Administrador | Atualizar configuração de agendamento |
| GET | `/api/v1/log-manage/status` | Administrador | Estado |
| POST | `/api/v1/log-manage/sync` | Administrador | Políticas de limpeza de sincronização |
| GET | `/api/v1/log-manage/writers` | Administrador | Status de escritor |

### Cópias de segurança

Backups completos/de banco de dados, restauração, tarefas agendadas, armazenamento remoto FTP/S3/WebDAV.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| DELETE | `/api/v1/backups/:id` | Superadministrador | Excluir backup |
| GET | `/api/v1/backups/:id/download` | Administrador | Baixar cópia de segurança |
| POST | `/api/v1/backups/:id/restore` | Superadministrador | Restaurar do backup |
| POST | `/api/v1/backups/:id/upload-ftp` | Superadministrador | Carregar para FTP |
| POST | `/api/v1/backups/:id/upload-s3` | Superadministrador | Carregar para S3 |
| POST | `/api/v1/backups/:id/upload-webdav` | Superadministrador | Carregar para WebDAV |
| POST | `/api/v1/backups/create` | Administrador | Crie backup completo (com dump de banco de dados) |
| GET | `/api/v1/backups/list` | Administrador | Lista completa de backups |
| POST | `/api/v1/backups/remote-restore` | Superadministrador | Restaurar remotamente |
| GET | `/api/v1/backups/restore-status` | Superadministrador | Restaurar o progresso |
| GET | `/api/v1/backups/schedules` | Administrador | Lista de trabalhos agendados |
| POST | `/api/v1/backups/schedules` | Administrador | Criar trabalho agendado |
| DELETE | `/api/v1/backups/schedules/:id` | Administrador | Excluir trabalho agendado |
| PUT | `/api/v1/backups/schedules/:id` | Administrador | Atualizar trabalho agendado |
| GET | `/api/v1/backups/storages` | Administrador | Lista de configurações de armazenamento |
| POST | `/api/v1/backups/storages` | Administrador | Criar configuração de armazenamento |
| DELETE | `/api/v1/backups/storages/:id` | Administrador | Excluir configuração de armazenamento |
| PUT | `/api/v1/backups/storages/:id` | Administrador | Atualizar configuração de armazenamento |
| GET | `/api/v1/backups/storages/:id/files` | Administrador | Lista de arquivos de backup remoto |
| POST | `/api/v1/backups/upload-restore` | Superadministrador | Carregar backup e restauração |

### Configurações do sistema

Configuração e migração de banco de dados, configurações de painel, chaves JWT, testes SMTP/Telegram, modelos de notificação, atualizações online (principalmente admin).

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| PUT | `/api/v1/system/bootstrap-finish` | Administrador | Conclua a configuração da primeira inicialização |
| GET | `/api/v1/system/bootstrap-state` | Administrador | Status da primeira inicialização |
| GET | `/api/v1/system/config-file/:coreType` | Administrador | Leia o arquivo de configuração principal |
| GET | `/api/v1/system/db-config` | Administrador | Configuração do banco de dados |
| PUT | `/api/v1/system/db-config` | Administrador | Atualizar configuração do banco de dados |
| POST | `/api/v1/system/db-config/migrate` | Superadministrador | Migrar dados do banco de dados |
| POST | `/api/v1/system/db-config/switch` | Superadministrador | Alternar banco de dados (com migração em uma etapa) |
| POST | `/api/v1/system/db-config/test` | Administrador | Testar conexão com o banco de dados |
| GET | `/api/v1/system/db-pool-stats` | Administrador | Estatísticas do pool de conexões |
| GET | `/api/v1/system/db-stats` | Administrador | Estatísticas de banco de dados |
| GET | `/api/v1/system/db-stats/ping` | Administrador | Sonda de banco de dados |
| POST | `/api/v1/system/db-stats/reset` | Administrador | Redefinir estatísticas do banco de dados |
| GET | `/api/v1/system/encryption-key-status` | Administrador | Status da chave de criptografia |
| GET | `/api/v1/system/env-config` | Administrador | configuração .env |
| PUT | `/api/v1/system/env-config` | Administrador | Atualizar configuração .env |
| POST | `/api/v1/system/generate-encryption-key` | Superadministrador | Gerar chave de criptografia |
| GET | `/api/v1/system/github-config` | Administrador | Configuração GitHub |
| PUT | `/api/v1/system/github-config` | Administrador | Atualizar configuração GitHub |
| GET | `/api/v1/system/jwt-secret-status` | Administrador | Status da chave JWT |
| GET | `/api/v1/system/maintenance` | Público | Status do modo de manutenção (público) |
| PUT | `/api/v1/system/maintenance` | Administrador | Alternar modo de manutenção |
| POST | `/api/v1/system/migrate-data` | Superadministrador | Migrar dados |
| GET | `/api/v1/system/notification-config` | Administrador | Configuração de notificação |
| PUT | `/api/v1/system/notification-config` | Administrador | Atualizar configuração de notificação |
| GET | `/api/v1/system/notification-templates` | Administrador | Lista de modelos |
| POST | `/api/v1/system/notification-templates` | Administrador | Criar modelo |
| DELETE | `/api/v1/system/notification-templates/:id` | Administrador | Excluir modelo |
| PUT | `/api/v1/system/notification-templates/:id` | Administrador | Atualizar modelo |
| POST | `/api/v1/system/notification-templates/:id/broadcast` | Administrador | Notificação de transmissão por modelo |
| POST | `/api/v1/system/notification-templates/:id/send` | Administrador | Enviar notificação de teste |
| GET | `/api/v1/system/panel-config` | Administrador | Configuração do painel |
| PUT | `/api/v1/system/panel-config` | Administrador | Atualizar configuração do painel |
| GET | `/api/v1/system/reality-targets` | Administrador | Lista de alvos REALITY |
| PUT | `/api/v1/system/reality-targets` | Administrador | Atualizar destino REALITY |
| POST | `/api/v1/system/reality-targets/reset` | Superadministrador | Redefinir o destino REALITY para o padrão |
| GET | `/api/v1/system/reality-local-sni` | Administrador | Leia a configuração local do Reality SNI |
| PUT | `/api/v1/system/reality-local-sni` | Administrador | Atualizar configuração local do Reality SNI |
| POST | `/api/v1/system/regenerate-jwt-secret` | Superadministrador | Regenerar chave JWT |
| GET | `/api/v1/system/releases` | Administrador | Versões disponíveis |
| POST | `/api/v1/system/restart` | Superadministrador | Reiniciar |
| POST | `/api/v1/system/rollback` | Superadministrador | Reversão do sistema |
| GET | `/api/v1/system/settings` | Administrador | Ler configurações |
| PUT | `/api/v1/system/settings/batch` | Administrador | Configurações de atualização em massa |
| GET | `/api/v1/system/stats` | Administrador | Status do sistema |
| GET | `/api/v1/system/subscription-billing-templates` | Administrador | Modelo de cobrança de assinatura |
| PUT | `/api/v1/system/subscription-billing-templates` | Administrador | Atualizar modelo de cobrança de assinatura |
| POST | `/api/v1/system/telegram-chat-id` | Administrador | Obtenha o ID de bate-papo Telegram |
| POST | `/api/v1/system/test-smtp` | Administrador | Teste SMTP |
| POST | `/api/v1/system/test-telegram` | Administrador | Teste Telegram |
| POST | `/api/v1/system/update` | Superadministrador | Atualização do sistema |
| GET | `/api/v1/system/update-jobs` | Administrador | Atualizar lista de trabalhos |
| POST | `/api/v1/system/upload-mtls-ca` | Administrador | Carregar certificado mTLS CA |

### Auditoria de segurança

Consultas de eventos de auditoria e exportação.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/security/audit-events` | Administrador | Lista de eventos de auditoria |
| GET | `/api/v1/security/audit-events/export` | Administrador | Exportar eventos de auditoria |
| GET | `/api/v1/security/subscription-access-logs` | Administrador | Logs de busca de assinatura (filtrar por ID de assinatura/IP/e-mail do cliente) |

### Métricas

Métricas agregadas do painel (nós/certificados/serviços).

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/metrics/certificates` | Administrador | Métricas de certificado |
| GET | `/api/v1/metrics/nodes` | Administrador | Métricas de nó |
| GET | `/api/v1/metrics/overview` | Administrador | Visão geral |

### Modelos de portal

Modelos de portal, configuração de ativação e gerenciamento de cache.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/portal/active` | Administrador | Modelo ativo |
| PUT | `/api/v1/portal/active` | Administrador | Atualizar modelo ativo |
| DELETE | `/api/v1/portal/cache` | Administrador | Limpar cache do portal |
| GET | `/api/v1/portal/cache` | Administrador | Estado do cache |
| PUT | `/api/v1/portal/cache/dev-mode` | Administrador | Alternar modo de desenvolvimento de cache |
| POST | `/api/v1/portal/cache/rebuild` | Administrador | Reconstruir cache do portal |
| GET | `/api/v1/portal/lang/builtin` | Administrador | Pacotes de idiomas integrados |
| POST | `/api/v1/portal/reset` | Administrador | Restaurar modelo padrão |
| GET | `/api/v1/portal/templates` | Administrador | Lista |
| POST | `/api/v1/portal/templates` | Administrador | Criar |
| DELETE | `/api/v1/portal/templates/:id` | Administrador | Excluir |
| GET | `/api/v1/portal/templates/:id` | Administrador | Detalhes |
| PUT | `/api/v1/portal/templates/:id` | Administrador | Atualizar |
| POST | `/api/v1/portal/templates/:id/activate` | Administrador | Ativar modelo |

> `/portal/templates` CRUD é um **API avançado**: a UI do portal usa principalmente `PUT /api/v1/portal/active`
> (configuração única do site completo); gerenciar/alternar vários modelos com este grupo. Ambos são suportados e não entram em conflito.

### Instalação do agente

Script de instalação e download do Node Agent (público).

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| POST | `/api/v1/agent/bootstrap` | Público | Registro de primeira inicialização do agente (relatórios do agente com um token de inicialização; público) |
| GET | `/api/v1/agent/releases` | Administrador | Versões disponíveis do agente (para a página de atualização) |
| GET | `/api/v1/agent/version` | Administrador | Versão atual do agente no painel |
| GET | `/api/v1/agent/download` | Público | Agente de download |
| GET | `/api/v1/agent/install` | Público | Obtenha o script de instalação |
| GET | `/api/v1/agent/origin` | Público | Endereço de origem do painel (público) |

### Cobrar entrada de retorno de chamada

Retorno de chamada de plataforma de pagamento de terceiros (pública, assinatura verificada).

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| POST | `/api/v1/charge/callback/:site_id` | Público | Lidar com retorno de chamada de cobrança |

### Entradas (com nós)

Lista de entrada usada para seleção do nó do plano.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/inbounds-with-server` | Conectado | Todas as entradas (incluindo seus nós) |

### Configuração pública

Configuração de tempo de execução do portal sem login (guarda de manutenção + limite de taxa pública).

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/public/articles` | Público | Lista pública de artigos |
| GET | `/api/v1/public/articles/:slug` | Público | Detalhes do artigo público |
| GET | `/api/v1/public/client-configs` | Público | Configuração de download do cliente (pública) |
| GET | `/api/v1/public/portal-config` | Público | Configuração do tempo de execução do portal (nome do site, tema, idiomas, documentos de conformidade; público) |
| GET | `/api/v1/public/settings` | Público | Configurações públicas (por exemplo, Registration_enabled; public) |
| GET | `/api/v1/public/subscription-endpoints` | Público | Lista de endpoints de assinatura habilitada (pública) |

### Autenticação de cliente e negócios (mesmo definido no mecanismo de administração)

Pontos de extremidade do cliente idênticos ao portal. Eles usam um **token de cliente** (`Authorization: Bearer <client_token>`), não um token de administrador; consulte `portal-api.en.md` para obter detalhes.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/client-auth/articles` | Cliente (logado) | Listar artigos |
| GET | `/api/v1/client-auth/articles/:slug` | Cliente (logado) | Obter artigo |
| GET | `/api/v1/client-auth/balance` | Cliente (logado) | Obtenha equilíbrio |
| GET | `/api/v1/client-auth/balance/transactions` | Cliente (logado) | Transações de saldo |
| POST | `/api/v1/client-auth/change-password` | Cliente (logado) | Alterar senha |
| POST | `/api/v1/client-auth/charge/create` | Cliente (logado) | Criar pedido de recarga |
| GET | `/api/v1/client-auth/charge/orders` | Cliente (logado) | Listar pedidos de recarga |
| GET | `/api/v1/client-auth/charge/orders/:id` | Cliente (logado) | Obtenha detalhes do pedido de recarga |
| POST | `/api/v1/client-auth/charge/redeem-card` | Cliente (logado) | Resgatar cartão |
| GET | `/api/v1/client-auth/charge/sites` | Cliente (logado) | Listar sites de recarga |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | Cliente (logado) | Consultar status de recarga |
| GET | `/api/v1/client-auth/checktime` | Cliente (logado) | Obtenha a hora do servidor (sincronização do relógio) |
| GET | `/api/v1/client-auth/connection-info` | Cliente (logado) | Obtenha parâmetros de conexão do plano |
| GET | `/api/v1/client-auth/gifts` | Cliente (logado) | Listar presentes |
| POST | `/api/v1/client-auth/gifts/:id/claim` | Cliente (logado) | Reivindicar presente |
| GET | `/api/v1/client-auth/invoices` | Cliente (logado) | Lista de pedidos |
| GET | `/api/v1/client-auth/invoices/:id` | Cliente (logado) | Detalhes do pedido |
| POST | `/api/v1/client-auth/login` | Cliente (público) | Faça login |
| POST | `/api/v1/client-auth/logout` | Cliente (logado) | Sair |
| POST | `/api/v1/client-auth/logs/package` | Cliente (logado) | Criar pacote de registros |
| GET | `/api/v1/client-auth/logs/packages` | Cliente (logado) | Listar pacotes de registros |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | Cliente (logado) | Excluir pacote de registros |
| GET | `/api/v1/client-auth/logs/packages/:id` | Cliente (logado) | Obter pacote de registros |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | Cliente (logado) | Baixar pacote de registros |
| GET | `/api/v1/client-auth/logs/types` | Cliente (logado) | Verificação de integridade do pacote de registros |
| GET | `/api/v1/client-auth/me` | Cliente (logado) | Obtenha o usuário conectado atual |
| GET | `/api/v1/client-auth/nodes` | Cliente (logado) | Minha lista de nós |
| GET | `/api/v1/client-auth/nodes/:id/config` | Cliente (logado) | Configuração do nó |
| GET | `/api/v1/client-auth/nodes/:id/link` | Cliente (logado) | Link de compartilhamento de nó |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | Cliente (logado) | Código QR do nó |
| GET | `/api/v1/client-auth/oauth/:provider` | Cliente (público) | Autorizar |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | Cliente (público) | Retorno de chamada |
| GET | `/api/v1/client-auth/oauth/providers` | Cliente (público) | Status do provedor |
| GET | `/api/v1/client-auth/orders` | Cliente (logado) | Lista de pedidos |
| POST | `/api/v1/client-auth/orders` | Cliente (logado) | Criar pedido |
| GET | `/api/v1/client-auth/orders/:id` | Cliente (logado) | Detalhes do pedido |
| GET | `/api/v1/client-auth/plans` | Cliente (logado) | Lista de planos |
| GET | `/api/v1/client-auth/plans/:id` | Cliente (logado) | Detalhes do plano |
| GET | `/api/v1/client-auth/referral/code` | Cliente (logado) | Obtenha o código de verificação de referência |
| GET | `/api/v1/client-auth/referral/records` | Cliente (logado) | Listar registros de referência |
| GET | `/api/v1/client-auth/referral/stats` | Cliente (logado) | Obtenha estatísticas de referência |
| POST | `/api/v1/client-auth/referral/withdraw` | Cliente (logado) | Solicitar retirada |
| GET | `/api/v1/client-auth/referral/withdrawals` | Cliente (logado) | Listar retiradas de referência |
| POST | `/api/v1/client-auth/refresh` | Cliente (público) | Atualizar |
| POST | `/api/v1/client-auth/register` | Cliente (público) | Cadastre-se |
| POST | `/api/v1/client-auth/reset-subscription` | Cliente (logado) | Redefinir assinatura |
| POST | `/api/v1/client-auth/rotate-token` | Cliente (logado) | Alternar token de assinatura |
| POST | `/api/v1/client-auth/self-unban` | Cliente (logado) | Desbanimento de autoatendimento |
| PUT | `/api/v1/client-auth/settings` | Cliente (logado) | Atualizar configurações |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | Cliente (logado) | Adicionar IP automaticamente |
| GET | `/api/v1/client-auth/settings/whitelist` | Cliente (logado) | Listar lista de permissões de IP |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | Cliente (logado) | Remover da lista de permissões de IP |
| GET | `/api/v1/client-auth/shared-accounts` | Cliente (logado) | Listar contas compartilhadas |
| GET | `/api/v1/client-auth/shared-accounts/:id` | Cliente (logado) | Obtenha uma conta compartilhada |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | Cliente (logado) | Solicitar conta compartilhada |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | Cliente (logado) | Solicitação de reparo |
| POST | `/api/v1/client-auth/ticket` | Cliente (logado) | Emitir ticket único de curta duração |
| GET | `/api/v1/client-auth/tickets` | Cliente (logado) | Listar ingressos |
| POST | `/api/v1/client-auth/tickets` | Cliente (logado) | Criar ingresso |
| GET | `/api/v1/client-auth/tickets/:id` | Cliente (logado) | Obter ingresso |
| POST | `/api/v1/client-auth/tickets/:id/close` | Cliente (logado) | Fechar tíquete |
| POST | `/api/v1/client-auth/tickets/:id/reply` | Cliente (logado) | Responder ao ticket |
| GET | `/api/v1/client-auth/traffic-history` | Cliente (logado) | Histórico de trânsito |
| GET | `/api/v1/client-auth/traffic-packs` | Cliente (logado) | Lista de pacotes de tráfego |
| POST | `/api/v1/client-auth/traffic-packs/buy` | Cliente (logado) | Comprar pacote de tráfego |

### Entrega de assinatura

Ponto de entrada público para clientes buscarem assinaturas; consulte `subscription.en.md`.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/sub/:token` | Público | Servir assinatura |

### Fluxo de trabalho de configuração de tempo de execução

Instantâneos de configuração / simulação / aplicação / reversão (`apply`, `rollback` e `disable-proxy` requerem administrador).

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| POST | `/api/v1/runtime/apply` | Administrador | Aplicar |
| GET | `/api/v1/runtime/apply-jobs` | Conectado | Listar empregos |
| GET | `/api/v1/runtime/apply-jobs/:id` | Conectado | Conseguir emprego |
| POST | `/api/v1/runtime/apply-jobs/:id/cancel` | Administrador | Cancelar trabalho |
| POST | `/api/v1/runtime/apply-jobs/:id/retry` | Administrador | Tentar novamente o trabalho |
| POST | `/api/v1/runtime/disable-proxy` | Administrador | Desativar proxy |
| POST | `/api/v1/runtime/dry-run` | Conectado | Execução de simulação |
| GET | `/api/v1/runtime/nodes` | Conectado | Listar nós |
| GET | `/api/v1/runtime/overview` | Conectado | Visão geral |
| POST | `/api/v1/runtime/rollback` | Administrador | Reverter |
| GET | `/api/v1/runtime/snapshots` | Conectado | Listar instantâneos |
| POST | `/api/v1/runtime/snapshots` | Conectado | Criar instantâneo |
| DELETE | `/api/v1/runtime/snapshots/:id` | Administrador | Excluir instantâneo |
| GET | `/api/v1/runtime/snapshots/:id` | Conectado | Obter instantâneo |
| POST | `/api/v1/runtime/verify` | Conectado | Verifique |

### Políticas de acesso ao serviço (descontinuadas)

> `/services/:id/access-policies` e `/services/:id/ip-bans` foram desativados em 20/10/2026:
> não há chamadas administrativas e nenhum caminho de conexão consome essas políticas.

### Monitoramento de cota de tráfego

Visualização de monitoramento somente leitura de cotas e uso.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/api/v1/traffic/quotas` | Conectado | Lista de cotas |
| GET | `/api/v1/traffic/quotas/:id/usage` | Conectado | Uso de cota |
| GET | `/api/v1/traffic/reset-logs` | Conectado | Registros de redefinição de cota |
| GET | `/api/v1/traffic/usage-history` | Conectado | Histórico de uso |

### Verificações de saúde

Sonda de vivacidade.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/health` | Público | Sonda de vivacidade; 200 significa saudável |

### Página do portal

Página SPA do Portal.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/p` | Público | HTML do portal (renderizado pelo servidor + tempo de execução Vue) |

### Desafios ACME

Retorno de chamada de desafio HTTP-01.

| Método | Caminho | Permissão | Descrição |
|---|---|---|---|
| GET | `/.well-known/acme-challenge/:token` | Público | Retorno de chamada de desafio ACME HTTP-01 (verifica o domínio durante a emissão) |
