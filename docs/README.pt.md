<!-- source: README.md | locale: pt | updated: 2026-10-23 -->
# Documentação CoreFusion

> Idiomas: [简体中文](README.md) · [English](README.en.md) · [繁體中文](README.zh-TW.md) · [Русский](README.ru.md) · [日本語](README.ja.md) · [한국어](README.ko.md) · [Deutsch](README.de.md) · [Français](README.fr.md) · [Español](README.es.md) · Português · [Türkçe](README.tr.md) · [Bahasa Indonesia](README.id.md) · [ไทย](README.th.md) · [Tiếng Việt](README.vi.md) · [العربية](README.ar.md) · [فارسی](README.fa.md)

## [USER-GUIDE.pt.md](./USER-GUIDE.pt.md) — Guia do usuário

Um guia de operações completo para **usuários** (operações/administradores/operadores): instalação, configuração inicial, módulos de recursos, perguntas frequentes e apêndice.

## [api/](./api/) — Referência pública API

Para integradores, desenvolvimento secundário e clientes terceiros:

- [api/README.pt.md](./api/README.pt.md) — visão geral: base URL, autenticação, formato de resposta, limites de taxa
- [api/subscription.pt.md](./api/subscription.pt.md) — endpoint de assinatura (`/sub/:token`), formatos, cabeçalhos de resposta, exemplos
- [api/portal-api.pt.md](./api/portal-api.pt.md) — portal do cliente API (cadastro/login, planos, pedidos, saldo, ingressos)
- [api/charge-callback.pt.md](./api/charge-callback.pt.md) — assinatura e verificação de retorno de chamada de pagamento
- [api/admin-api.pt.md](./api/admin-api.pt.md) — tabela de rotas admin API agrupada por módulo

## Documentos de projeto

Os documentos de design são materiais internos (propriedade intelectual principal) e **não são distribuídos** com este repositório. Entre em contato com o mantenedor se precisar deles.

> **Escopo**: o aplicativo cliente (IU local, controle de TUN, bandeja, mesclagem de múltiplas assinaturas) é um produto separado e está fora do escopo deste repositório. A documentação aqui cobre apenas o lado do servidor (painel/nós/entrega de assinatura).

## Outros guias de operações

- `cert-google-eab.md` — Google Emissão de certificado EAB *(chinês)*
- `oauth-google-verification.md` — Google Verificação OAuth *(chinês)*
- `notification-templates.md` — variáveis do modelo de notificação *(chinês)*

## Cobertura de tradução

| Documento | zh-CN | en | zh-TW | Outras localidades |
|----------|-------|----|-------|---------------|
| README.md | ✅ | ✅ | ✅ | ru ✅ |
| USER-GUIDE.md | ✅ | ✅ | ✅ | ru ✅ |
| I18N.md | ✅ | — | — | — |
| api/README.md | ✅ | ✅ | ✅ | ru ✅ |
| api/subscription.md | ✅ | ✅ | ✅ | ru ✅ |
| api/portal-api.md | ✅ | ✅ | ✅ | ru ✅ |
| api/charge-callback.md | ✅ | ✅ | ✅ | ru ✅ |
| api/admin-api.md | ✅ | ✅ | ✅ | ru ✅ |
| cert-google-eab.md | ✅ | ✅ | ✅ | ru ✅ |
| oauth-google-verification.md | ✅ | ✅ | ✅ | ru ✅ |
| notification-templates.md | ✅ | ✅ | ✅ | ru ✅ |
| LEGAL.md | ✅ | ✅ | ✅ | ru ✅ |
| THIRD_PARTY_LICENSES.md | ✅ | ✅ | ✅ | ru ✅ |

Consulte [I18N.md](README.pt.md) para nomenclatura, navegação de idioma e regras de revisão *(chinês)*.
