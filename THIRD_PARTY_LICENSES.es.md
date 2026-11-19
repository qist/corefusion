<!-- source: THIRD_PARTY_LICENSES.md | locale: es | updated: 2026-11-19 -->
# Componentes y licencias de terceros

> Idiomas: [简体中文](THIRD_PARTY_LICENSES.md) · [English](THIRD_PARTY_LICENSES.en.md) · [繁體中文](THIRD_PARTY_LICENSES.zh-TW.md) · [Русский](THIRD_PARTY_LICENSES.ru.md) · [日本語](THIRD_PARTY_LICENSES.ja.md) · [한국어](THIRD_PARTY_LICENSES.ko.md) · [Deutsch](THIRD_PARTY_LICENSES.de.md) · [Français](THIRD_PARTY_LICENSES.fr.md) · Español · [Português](THIRD_PARTY_LICENSES.pt.md) · [Türkçe](THIRD_PARTY_LICENSES.tr.md) · [Bahasa Indonesia](THIRD_PARTY_LICENSES.id.md) · [ไทย](THIRD_PARTY_LICENSES.th.md) · [Tiếng Việt](THIRD_PARTY_LICENSES.vi.md) · [العربية](THIRD_PARTY_LICENSES.ar.md) · [فارسی](THIRD_PARTY_LICENSES.fa.md)

CoreFusion utiliza los siguientes componentes de código abierto. Este archivo enumera los principales componentes y licencias; **la lista completa está definida por `go.mod` / `go.sum`, `web/admin/package.json` y el archivo de LICENCIA propio de cada módulo**.

Al distribuir archivos binarios de este proyecto, conserve este archivo, el proyecto `LICENSE` y los avisos de derechos de autor/licencia de todos los componentes de terceros.

## 1. Ir a las dependencias compiladas en el binario (principal)

| Componente | Licencia | Propósito |
|---|---|---|
| `github.com/xtls/xray-core` | MPL-2.0 | Análisis de configuración Xray, comandos gRPC, definiciones de protocolo |
| `github.com/xtls/reality` | MPL-2.0 | Soporte de protocolo REALITY |
| `github.com/sagernet/sing` | **GPL-3.0-or-later** | Vinculado indirectamente a través de xray-core |
| `github.com/sagernet/sing-shadowsocks` | **GPL-3.0-or-later** | Vinculado indirectamente a través de xray-core (Shadowsocks-2022) |
| `github.com/go-sql-driver/mysql` | MPL-2.0 | Controlador MySQL |
| `github.com/hashicorp/golang-lru/v2` | MPL-2.0 | caché LRU |
| `github.com/go-acme/lego/v5` | MIT | Emisión de certificado ACME |
| `github.com/gin-gonic/gin` | MIT | Marco HTTP |
| `gorm.io/gorm` | MIT | ORM |
| `gorm.io/driver/mysql` | MIT | Controlador MySQL para GORM |
| `gorm.io/driver/postgres` | MIT | Controlador PostgreSQL para GORM |
| `github.com/jackc/pgx/v5` | MIT | Controlador PostgreSQL |
| `github.com/qist/sqlite` | MIT | Controlador SQLite (base de datos predeterminada para el panel/Agente) |
| `github.com/aws/aws-sdk-go-v2` | Apache-2.0 | Copias de seguridad S3, Ruta 53 DNS-01 |
| `google.golang.org/grpc` | Apache-2.0 | Comunicación del agente gRPC |
| `google.golang.org/protobuf` | BSD-3-Clause | Tiempo de ejecución de Protobuf |
| `golang.org/x/crypto` | BSD-3-Clause | Primitivas criptográficas |
| `golang.org/x/oauth2` | BSD-3-Clause | OAuth2 / Google EAB |
| `golang.org/x/sys` | BSD-3-Clause | Llamadas al sistema (limitación de velocidad nft/tc, gestión de procesos) |
| `github.com/golang-jwt/jwt/v5` | MIT | JWT |
| `github.com/google/uuid` | BSD-3-Clause | UUID |
| `github.com/go-webauthn/webauthn` | BSD-3-Clause | Iniciar sesión en Passkey / WebAuthn |
| `github.com/pquerna/otp` | Apache-2.0 | Autenticación de dos factores TOTP |
| `github.com/jlaffaye/ftp` | ISC | Carga de copia de seguridad FTP |
| `gopkg.in/yaml.v3` | MIT / Apache-2.0 | Análisis de configuración YAML |

> ⚠ **Componentes GPL-3.0**: `sing` y `sing-shadowsocks` son GPL-3.0 o posterior. Están vinculados estáticamente al panel y a los binarios del Agente, por lo que **la distribución de binarios requiere proporcionar la fuente correspondiente** (consulte la sección 3 de [LEGAL.md](./LEGAL.md)).

## 2. Dependencias de frontend (compiladas en el binario del panel)

La interfaz de usuario de administración está construida con Vite y está integrada en el binario del panel a través de `go:embed`, por lo que los siguientes componentes también se distribuyen con el binario:

| Componente | Licencia | Propósito |
|---|---|---|
| Vue 3 / Vue Router / Pinia / Element Plus / axios / vue-i18n / marked / js-yaml / qrcode / Monaco Editor | MIT | Interfaz de usuario de administración, enrutamiento y estado, i18n, renderizado Markdown, códigos QR y editor de códigos |
| DOMPurify | Apache-2.0 / MPL-2.0 | Desinfección XSS antes de renderizar Markdown/HTML |

## 3. Programas independientes invocados en tiempo de ejecución (no distribuidos en este repositorio)

El panel/Agente puede descargar, instalar o invocar los siguientes programas independientes en tiempo de ejecución. Interactúan con este proyecto a través de procesos/interfaces, pero si los envías con tus entregables aún debes cumplir con sus licencias:

| Programa | Licencia | Notas |
|---|---|---|
| Xray-core | MPL-2.0 | Núcleo de proxy, proceso separado |
| sing-box | **GPL-3.0-or-later** | Núcleo de proxy, proceso separado; proporcionar la fuente correspondiente al distribuir |
| Caddy | Apache-2.0 | Proxy inverso / TLS |
| Nginx | BSD-2-Clause | Proxy inverso (instalado en el sistema, si se usa) |

> Hysteria2 y TUIC son **protocolos incluidos en sing-box**, no núcleos independientes ni binarios, por lo que no se enumeran por separado.

## 4. Archivos de datos

| Datos | Fuente / licencia |
|---|---|
| `geoip.dat` / `geosite.dat` | Según lo declarado por el repositorio de fuentes de datos (comúnmente GPL-3.0 o CC-BY-SA) |
| GeoLite2 (`GeoLite2-ASN.mmdb`, etc.) | CLUF MaxMind GeoLite2 / CC BY-SA 4.0; verificar la licencia antes del uso comercial |
| `country.mmdb` | Según lo declarado por su fuente de datos. |

## 5. Recomendaciones de cumplimiento

- Al entregar archivos binarios, incluya: `LICENSE`, este archivo y el texto de licencia o adquisición URL de cada componente GPL/MPL.
- Si modifica archivos fuente de componentes MPL-2.0 como `xray-core`, publique el código fuente de los archivos modificados.
- Si distribuye programas GPL como sing-box con su producto, proporcione a los destinatarios la fuente correspondiente completa de esa versión (o una oferta escrita efectiva).
- Marcas comerciales y nombres: no utilice nombres o logotipos de proyectos anteriores de manera que implique un respaldo oficial.
