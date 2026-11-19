<!-- source: THIRD_PARTY_LICENSES.md | locale: pt | updated: 2026-11-19 -->
# Componentes e licenças de terceiros

> Idiomas: [简体中文](THIRD_PARTY_LICENSES.md) · [English](THIRD_PARTY_LICENSES.en.md) · [繁體中文](THIRD_PARTY_LICENSES.zh-TW.md) · [Русский](THIRD_PARTY_LICENSES.ru.md) · [日本語](THIRD_PARTY_LICENSES.ja.md) · [한국어](THIRD_PARTY_LICENSES.ko.md) · [Deutsch](THIRD_PARTY_LICENSES.de.md) · [Français](THIRD_PARTY_LICENSES.fr.md) · [Español](THIRD_PARTY_LICENSES.es.md) · Português · [Türkçe](THIRD_PARTY_LICENSES.tr.md) · [Bahasa Indonesia](THIRD_PARTY_LICENSES.id.md) · [ไทย](THIRD_PARTY_LICENSES.th.md) · [Tiếng Việt](THIRD_PARTY_LICENSES.vi.md) · [العربية](THIRD_PARTY_LICENSES.ar.md) · [فارسی](THIRD_PARTY_LICENSES.fa.md)

CoreFusion usa os seguintes componentes de código aberto. Este arquivo lista os principais componentes e licenças; **a lista completa é definida por `go.mod` / `go.sum`, `web/admin/package.json` e arquivo de LICENÇA próprio de cada módulo**.

Ao distribuir binários deste projeto, mantenha este arquivo, o projeto `LICENSE` e os avisos de direitos autorais/licença de todos os componentes de terceiros.

## 1. Dependências Go compiladas no binário (principal)

| Componente | Licença | Objetivo |
|---|---|---|
| `github.com/xtls/xray-core` | MPL-2.0 | Análise de configuração Xray, comandos gRPC, definições de protocolo |
| `github.com/xtls/reality` | MPL-2.0 | Suporte ao protocolo REALITY |
| `github.com/sagernet/sing` | **GPL-3.0-or-later** | Vinculado indiretamente via xray-core |
| `github.com/sagernet/sing-shadowsocks` | **GPL-3.0-or-later** | Vinculado indiretamente via núcleo xray (Shadowsocks-2022) |
| `github.com/go-sql-driver/mysql` | MPL-2.0 | Driver MySQL |
| `github.com/hashicorp/golang-lru/v2` | MPL-2.0 | Cache LRU |
| `github.com/go-acme/lego/v5` | MIT | Emissão do certificado ACME |
| `github.com/gin-gonic/gin` | MIT | Estrutura HTTP |
| `gorm.io/gorm` | MIT | ORM |
| `gorm.io/driver/mysql` | MIT | Driver MySQL para GORM |
| `gorm.io/driver/postgres` | MIT | Driver PostgreSQL para GORM |
| `github.com/jackc/pgx/v5` | MIT | Driver PostgreSQL |
| `github.com/qist/sqlite` | MIT | Driver SQLite (banco de dados padrão para painel/Agente) |
| `github.com/aws/aws-sdk-go-v2` | Apache-2.0 | Backups S3, Route53 DNS-01 |
| `google.golang.org/grpc` | Apache-2.0 | Comunicação do agente gRPC |
| `google.golang.org/protobuf` | BSD-3-Clause | Tempo de execução do protobuf |
| `golang.org/x/crypto` | BSD-3-Clause | Primitivas criptográficas |
| `golang.org/x/oauth2` | BSD-3-Clause | OAuth2/Google EAB |
| `golang.org/x/sys` | BSD-3-Clause | Chamadas de sistema (limitação de taxa NFT/TC, gerenciamento de processos) |
| `github.com/golang-jwt/jwt/v5` | MIT | JWT |
| `github.com/google/uuid` | BSD-3-Clause | UUID |
| `github.com/go-webauthn/webauthn` | BSD-3-Clause | Login Passkey/WebAuthn |
| `github.com/pquerna/otp` | Apache-2.0 | Autenticação de dois fatores TOTP |
| `github.com/jlaffaye/ftp` | ISC | Carregamento de backup FTP |
| `gopkg.in/yaml.v3` | MIT / Apache-2.0 | Análise de configuração YAML |

> ⚠ **Componentes GPL-3.0**: `sing` e `sing-shadowsocks` são GPL-3.0 ou posterior. Eles estão estaticamente vinculados aos binários do painel e do agente, portanto, **a distribuição de binários requer o fornecimento da fonte correspondente** (consulte a seção 3 de [LEGAL.md](./LEGAL.md)).

## 2. Dependências de frontend (compiladas no binário do painel)

A UI administrativa é construída com Vite e incorporada ao binário do painel via `go:embed`, portanto, os seguintes componentes também são distribuídos com o binário:

| Componente | Licença | Objetivo |
|---|---|---|
| Vue 3 / Vue Router / Pinia / Element Plus / axios / vue-i18n / marked / js-yaml / qrcode / Monaco Editor | MIT | Admin UI, roteamento e estado, i18n, renderização Markdown, códigos QR e editor de código |
| DOMPurify | Apache-2.0 / MPL-2.0 | Sanitização XSS antes de renderizar Markdown/HTML |

## 3. Programas autônomos baixados/invocados pelo painel em tempo de execução

Ao instalar ou atualizar núcleos, o painel/agente baixa os seguintes binários de GitHub e os executa como processos separados. **Artefatos marcados como "construídos por este projeto" são compilados e publicados por esta organização a partir de fonte upstream** (`qist/*-binaries`) e são distribuídos junto com o produto, portanto, as obrigações de licença correspondentes (incluindo disponibilidade de fonte para GPL) ficam com este projeto; os outros são URLs de download de versões oficiais do upstream.

| Programa | Licença | Notas |
|---|---|---|
| Xray-core | MPL-2.0 | Núcleo proxy, processo separado; lançamento oficial upstream (`XTLS/Xray-core`) |
| sing-box | **GPL-3.0-or-later** | Núcleo proxy, processo separado; **construído por este projeto** (`qist/singbox-binaries`; versões oficiais não possuem `with_v2ray_api`, que é necessário para contabilização de tráfego por usuário); forneça a fonte correspondente ao distribuir |
| Caddy | Apache-2.0 | Proxy reverso/TLS; **construído por este projeto** (`qist/caddy-binaries`) |
| Nginx | BSD-2-Clause | Procuração reversa; **construído por este projeto** (`qist/nginx-binaries`, dividido por glibc/musl) |

> Hysteria2 e TUIC são **protocolos transportados por sing-box**, e não núcleos ou binários independentes, portanto, não são listados separadamente.

## 4. Arquivos de dados

| Dados | Fonte/licença |
|---|---|
| `geoip.dat`/`geosite.dat` | Conforme declarado pelo repositório da fonte de dados (geralmente GPL-3.0 ou CC-BY-SA) |
| GeoLite2 (`GeoLite2-ASN.mmdb`, etc.) | MaxMind GeoLite2 EULA/CC BY-SA 4.0; verificar o licenciamento antes do uso comercial |
| `country.mmdb` | Conforme declarado por sua fonte de dados |

## 5. Recomendações de conformidade

- Ao entregar binários, inclua: `LICENSE`, este arquivo e o texto da licença ou aquisição URL de cada componente GPL/MPL.
- Os binários construídos por este projeto (sing-box / Caddy / Nginx) são fornecidos com o painel: como são compilados a partir da fonte upstream, publique como foram construídos (versão, tags de construção, sinalizadores de configuração); Os componentes GPL (sing-box) também exigem a fonte correspondente completa dessa versão ou uma rota de aquisição eficaz.
- Se você modificar os arquivos de origem dos componentes MPL-2.0, como `xray-core`, publique a origem dos arquivos modificados.
- Se você distribuir programas GPL como sing-box com seu produto, forneça aos destinatários a fonte correspondente completa dessa versão (ou uma oferta efetiva por escrito).
- Marcas registradas e nomes: não use nomes ou logotipos de projetos anteriores de forma que implique endosso oficial.
