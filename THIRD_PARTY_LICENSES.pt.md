<!-- source: THIRD_PARTY_LICENSES.md | locale: pt | updated: 2026-10-23 -->
# Componentes e licenças de terceiros

> Idiomas: [简体中文](THIRD_PARTY_LICENSES.md) · [English](THIRD_PARTY_LICENSES.en.md) · [繁體中文](THIRD_PARTY_LICENSES.zh-TW.md) · [Русский](THIRD_PARTY_LICENSES.ru.md) · [日本語](THIRD_PARTY_LICENSES.ja.md) · [한국어](THIRD_PARTY_LICENSES.ko.md) · [Deutsch](THIRD_PARTY_LICENSES.de.md) · [Français](THIRD_PARTY_LICENSES.fr.md) · [Español](THIRD_PARTY_LICENSES.es.md) · Português · [Türkçe](THIRD_PARTY_LICENSES.tr.md) · [Bahasa Indonesia](THIRD_PARTY_LICENSES.id.md) · [ไทย](THIRD_PARTY_LICENSES.th.md) · [Tiếng Việt](THIRD_PARTY_LICENSES.vi.md) · [العربية](THIRD_PARTY_LICENSES.ar.md) · [فارسی](THIRD_PARTY_LICENSES.fa.md)

CoreFusion usa os seguintes componentes de código aberto. Este arquivo lista os principais componentes e licenças; **a lista completa é definida por `go.mod` / `go.sum` e pelo próprio arquivo LICENSE de cada módulo**.

Ao distribuir binários deste projeto, mantenha este arquivo, o projeto `LICENSE` e os avisos de direitos autorais/licença de todos os componentes de terceiros.

## 1. Dependências Go compiladas no binário (principal)

| Componente | Licença | Objetivo |
|---|---|---|
| `github.com/xtls/xray-core` | MPL-2.0 | Análise de configuração Xray, comandos gRPC, definições de protocolo |
| `github.com/xtls/reality` | MPL-2.0 | Suporte ao protocolo REALITY |
| `github.com/sagernet/sing` | **GPL-3.0 ou posterior** | Vinculado indiretamente via xray-core |
| `github.com/sagernet/sing-shadowsocks` | **GPL-3.0 ou posterior** | Vinculado indiretamente via núcleo xray (Shadowsocks-2022) |
| `github.com/go-sql-driver/mysql` | MPL-2.0 | Driver MySQL |
| `github.com/hashicorp/golang-lru/v2` | MPL-2.0 | Cache LRU |
| `github.com/go-acme/lego/v4` | MIT | Emissão do certificado ACME |
| `github.com/gin-gonic/gin` | MIT | Estrutura HTTP |
| `gorm.io/gorm` | MIT | ORM |
| `github.com/aws/aws-sdk-go-v2` | Apache-2.0 | Backups S3, Route53 DNS-01 |
| `google.golang.org/grpc` | Apache-2.0 | Comunicação do agente gRPC |
| `google.golang.org/protobuf` | Cláusula BSD-3 | Tempo de execução do protobuf |
| `golang.org/x/crypto` | Cláusula BSD-3 | Primitivas criptográficas |
| `golang.org/x/oauth2` | Cláusula BSD-3 | OAuth2/Google EAB |
| `github.com/golang-jwt/jwt/v5` | MIT | JWT |
| `github.com/google/uuid` | Cláusula BSD-3 | UUID |

> ⚠ **Componentes GPL-3.0**: `sing` e `sing-shadowsocks` são GPL-3.0 ou posterior. Eles estão estaticamente vinculados aos binários do painel e do Agente, portanto, **a distribuição de binários requer o fornecimento da Fonte Correspondente** (consulte a seção 3 de [LEGAL.md](THIRD_PARTY_LICENSES.pt.md)).

## 2. Programas autônomos invocados em tempo de execução (não distribuídos neste repositório)

O painel/Agente pode baixar, instalar ou invocar os seguintes programas independentes em tempo de execução. Eles interagem com este projeto através de processos/interfaces, mas se você os enviar com suas entregas, ainda deverá cumprir suas licenças:

| Programa | Licença | Notas |
|---|---|---|
| Núcleo Xray | MPL-2.0 | Núcleo de proxy, processo separado |
| sing-box | **GPL-3.0 ou posterior** | Núcleo proxy, processo separado; forneça a fonte correspondente ao distribuir |
| Caddie | Apache-2.0 | Proxy reverso/TLS |
| Nginx | Cláusula BSD-2 | Proxy reverso (instalado pelo sistema, se usado) |

> Hysteria2 e TUIC são **protocolos transportados por sing-box**, e não núcleos ou binários independentes, portanto, não são listados separadamente.

## 3. Arquivos de dados

| Dados | Fonte/licença |
|---|---|
| `geoip.dat`/`geosite.dat` | Conforme declarado pelo repositório da fonte de dados (geralmente GPL-3.0 ou CC-BY-SA) |
| GeoLite2 (`GeoLite2-ASN.mmdb`, etc.) | MaxMind GeoLite2 EULA/CC BY-SA 4.0; verificar o licenciamento antes do uso comercial |
| `country.mmdb` | Conforme declarado por sua fonte de dados |

## 4. Recomendações de conformidade

- Ao entregar binários, inclua: `LICENSE`, este arquivo e o texto da licença ou aquisição URL de cada componente GPL/MPL.
- Se você modificar os arquivos de origem dos componentes MPL-2.0, como `xray-core`, publique a origem dos arquivos modificados.
- Se você distribuir programas GPL como sing-box com seu produto, forneça aos destinatários a fonte correspondente completa dessa versão (ou uma oferta efetiva por escrito).
- Marcas registradas e nomes: não use nomes ou logotipos de projetos anteriores de forma que implique endosso oficial.
