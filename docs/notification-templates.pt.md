<!-- source: notification-templates.md | locale: pt | updated: 2026-10-23 -->
# Variáveis do modelo de notificação

> Idiomas: [简体中文](notification-templates.md) · [English](notification-templates.en.md) · [繁體中文](notification-templates.zh-TW.md) · [Русский](notification-templates.ru.md) · [日本語](notification-templates.ja.md) · [한국어](notification-templates.ko.md) · [Deutsch](notification-templates.de.md) · [Français](notification-templates.fr.md) · [Español](notification-templates.es.md) · Português · [Türkçe](notification-templates.tr.md) · [Bahasa Indonesia](notification-templates.id.md) · [ไทย](notification-templates.th.md) · [Tiếng Việt](notification-templates.vi.md) · [العربية](notification-templates.ar.md) · [فارسی](notification-templates.fa.md)

Os modelos de notificação usam espaços reservados `{variable}` que são substituídos por valores reais durante o envio. Este documento lista todas as variáveis ​​suportadas, as variáveis ​​disponíveis por evento e as regras de acionamento automático.

## 1. Tipos de modelos e públicos

| Público | Canal | Destinatário | Uso típico |
|---|---|---|---|
| **Usuários do painel** (administrador) | Telegram / E-mail | Canais e destinatários configurados pelo administrador | Alertas de nós, relatórios de backup, anúncios do sistema |
| **Assinantes** | E-mail | O e-mail do próprio cliente (preenchido automaticamente) | Saldo baixo, lembretes de vencimento de assinatura |

## 2. Variáveis comuns

Disponível em todos os modelos:

| Variável | Descrição | Exemplo |
|---|---|---|
| `{time}` | Hora de envio | `2026-09-28 14:30:00` |
| `{user}` | E-mail do cliente (modelos de assinante)/identificador de usuário | `user@example.com` |
| `{message}` | Corpo de notificação | Texto do anúncio de manutenção |

## 3. Variáveis de evento do assinante

### Lembrete de expiração de assinatura (event_key = `subscription_expire`)

Digitalizado diariamente e enviado para assinantes ativos com expiração em 7 dias:

| Variável | Descrição | Exemplo |
|---|---|---|
| `{user}` | E-mail do cliente | `user@example.com` |
| `{expire_date}` | Expiração da assinatura | `2026-10-05 09:00` |
| `{days}` | Dias restantes | `7` |
| `{time}` | Hora de envio | `2026-09-28 09:05:00` |

### Lembrete de saldo baixo (event_key = `balance_low`)

Digitalizado diariamente e enviado aos clientes cujo saldo está abaixo do limite:

| Variável | Descrição | Exemplo |
|---|---|---|
| `{user}` | E-mail do cliente | `user@example.com` |
| `{balance}` | Saldo atual (unidades monetárias, duas casas decimais) | `0.85` |
| `{time}` | Hora de envio | `2026-09-28 09:05:00` |

## 4. Variáveis de exemplo do modelo de operações

Quando você clica em “Enviar” para um teste de conectividade, os modelos de usuário do painel preenchem os espaços reservados com estes exemplos:

| Variável | Descrição | Exemplo |
|---|---|---|
| `{node}` | Nome do nó | `node-01` |
| `{backup_file}` | Nome do arquivo de backup | `corefusion-20260101-0900.tar.gz` |
| `{size}` | Tamanho do arquivo | `128MB` |
| `{domain}` | Domínio | `example.com` |
| `{expire_date}` | Data de validade | `2026-10-12` |
| `{days}` | Dias restantes | `14` |
| `{count}` | Contar | `3` |

## 5. Regras de acionamento automático

- **Programação**: a primeira varredura é executada 30 segundos após o início do painel e, em seguida, diariamente às **09h05**
- **Expiração da assinatura**: verifica assinaturas ativas (`status=active`) que expiram em 7 dias; o cliente também deve estar ativo
- **Saldo baixo**: verifica clientes ativos com saldo ≤ limite; o limite é `notify.balance_threshold`, **em centavos** (padrão `100` = 1,00)
- **Desduplicação**: no máximo uma mensagem por cliente por evento **por dia** (desduplicações `notification_send_logs` por dia)

## 6. Monitoramento da integridade do nó

As pulsações do nó são verificadas continuamente; **as notificações são enviadas somente quando o estado muda — rodadas sem alteração são silenciosas**:

| Chave de configuração | Descrição | Padrão |
|---|---|---|
| `monitor.node_interval_seconds` | Intervalo de verificação (segundos) | `60` (mínimo 30) |
| `monitor.node_offline_timeout_seconds` | Tempo limite de pulsação (segundos) | `120` |

- **Alerta offline**: quando um nó passa de online para offline, envia o modelo de administração “**Alerta offline de nó**” com `{node}` (nome) e `{time}`
- **Aviso de recuperação**: quando um nó se recupera, envia “**Aviso de recuperação de nó**” com as mesmas variáveis
- **Rodadas silenciosas**: nenhuma mensagem quando o estado permanece inalterado; após o reinício do painel, a primeira rodada apenas estabelece uma linha de base
- **Monitorar sem enviar**: desabilitar o template correspondente (Ativado = DESLIGADO)

## 7. Alertas mensais de cota de tráfego de nós (automáticos)

- **Trigger**: quando o tráfego mensal de um nó atinge o limite de cota, envia “**Alerta de cota de tráfego do nó**”; quando ultrapassa 100% e é suspenso, envia alerta de excesso de limite (chaves de evento `traffic_quota_threshold` / `traffic_quota_exceeded`)
- **Limite**: `monitor.traffic_quota_threshold_percent` (porcentagem, padrão `80`; defina `0` para desativar alertas de limite)
- **Desduplicação**: no máximo uma por nó por ciclo de faturamento (o ciclo segue o `traffic_quota_reset_day` do nó)
- Variáveis disponíveis: `{node}` (nome), `{used}` (GB usado), `{quota}` (cota GB), `{percent}` (porcentagem), `{time}`

## 8. Notificações de resultados de backup (automático)

- **Trigger**: enviado imediatamente após a execução de um backup agendado (`BackupScheduler`) — sucesso envia “**Relatório de backup diário**”, falha (criação do backup/leitura de armazenamento/upload para remoto) envia “**Alerta de falha de backup**”
- Variáveis disponíveis: `{backup_file}` (nome), `{size}` (tamanho, somente relatório de sucesso), `{message}` (motivo da falha, somente alerta de falha), `{time}`
- **Monitorar sem enviar**: ignorado silenciosamente quando o modelo está desabilitado (Ativado = DESLIGADO)

## 9. Lembretes de expiração de certificado (automático)

- **Acionador**: diariamente às 09h05, verifica certificados `active` prestes a expirar (**certificados IP com 3 dias restantes**, **certificados de domínio com 7 dias restantes**) e envia o modelo de administração “**Lembrete de expiração de certificado**” por certificado
- **Desduplicação**: no máximo um lembrete por certificado por dia; após a renovação, a expiração sai da janela (IP 3 dias / domínio 7 dias) e os lembretes param
- Variáveis disponíveis: `{domain}` (domínio do certificado), `{expire_date}` (data de validade), `{days}` (dias restantes), `{time}`

## 10. Alertas de falha do processo principal (automático)

- **Acionador**: quando um processo principal (xray/singbox) é encerrado de forma anormal ou o processo proxy (caddy/nginx) falha, envia o modelo de administração “**Alerta de falha do processo principal**” imediatamente; o supervisor então o reinicia com espera exponencial
- Variáveis disponíveis: `{core}` (tipo de processo), `{restarts}` / `{max}` (contagem/limite de reinicialização), `{message}` (mensagem de saída, somente processos principais), `{time}`
- **Monitorar sem enviar**: ignorado silenciosamente quando o modelo está desabilitado (Ativado = DESLIGADO)

## 11. Avisos de registro de novos usuários (automático)

- **Acionador**: enviado imediatamente após o registro de um cliente com sucesso, usando o modelo de administração “**Aviso de registro de novo usuário**”
- Variáveis disponíveis: `{user}` (e-mail do cliente), `{time}`

## 12. Uso

1. **Adicione um modelo**: Configurações de notificação → Modelos de notificação → Adicionar modelo → “Preencher a partir da predefinição” e ajuste conforme necessário
2. **Ativar**: marque Ativado e salve; gatilhos automáticos apenas processam modelos habilitados
3. **Teste**: clique em “Enviar” na lista para enviar uma mensagem real com variáveis de exemplo (e-mails de teste de modelo de assinante vão para o endereço remetente do administrador)
4. **Notificações imediatas** (código comercial): ligue para `NotifyClientEvent(ctx, db, settingSvc, eventKey, clientEmail, vars)` no local do evento (recarga, desconto, renovação) para notificar um único cliente imediatamente, fora do limite diário de varredura
