<!-- source: THIRD_PARTY_LICENSES.md | locale: es | updated: 2026-10-23 -->
# Componentes y licencias de terceros

> Idiomas: [简体中文](THIRD_PARTY_LICENSES.md) · [English](THIRD_PARTY_LICENSES.en.md) · [繁體中文](THIRD_PARTY_LICENSES.zh-TW.md) · [Русский](THIRD_PARTY_LICENSES.ru.md) · [日本語](THIRD_PARTY_LICENSES.ja.md) · [한국어](THIRD_PARTY_LICENSES.ko.md) · [Deutsch](THIRD_PARTY_LICENSES.de.md) · [Français](THIRD_PARTY_LICENSES.fr.md) · Español · [Português](THIRD_PARTY_LICENSES.pt.md) · [Türkçe](THIRD_PARTY_LICENSES.tr.md) · [Bahasa Indonesia](THIRD_PARTY_LICENSES.id.md) · [ไทย](THIRD_PARTY_LICENSES.th.md) · [Tiếng Việt](THIRD_PARTY_LICENSES.vi.md) · [العربية](THIRD_PARTY_LICENSES.ar.md) · [فارسی](THIRD_PARTY_LICENSES.fa.md)

CoreFusion utiliza los siguientes componentes de código abierto. Este archivo enumera los principales componentes y licencias; **la lista completa está definida por `go.mod` / `go.sum` y el archivo de LICENCIA propio de cada módulo**.

Al distribuir archivos binarios de este proyecto, conserve este archivo, el proyecto `LICENSE` y los avisos de derechos de autor/licencia de todos los componentes de terceros.

## 1. Ir a las dependencias compiladas en el binario (principal)

| Componente | Licencia | Propósito |
|---|---|---|
| `github.com/xtls/xray-core` | MPL-2.0 | Análisis de configuración Xray, comandos gRPC, definiciones de protocolo |
| `github.com/xtls/reality` | MPL-2.0 | Soporte de protocolo REALITY |
| `github.com/sagernet/sing` | **GPL-3.0 o posterior** | Vinculado indirectamente a través de xray-core |
| `github.com/sagernet/sing-shadowsocks` | **GPL-3.0 o posterior** | Vinculado indirectamente a través de xray-core (Shadowsocks-2022) |
| `github.com/go-sql-driver/mysql` | MPL-2.0 | Controlador MySQL |
| `github.com/hashicorp/golang-lru/v2` | MPL-2.0 | caché LRU |
| `github.com/go-acme/lego/v4` | MIT | Emisión de certificado ACME |
| `github.com/gin-gonic/gin` | MIT | Marco HTTP |
| `gorm.io/gorm` | MIT | ORM |
| `github.com/aws/aws-sdk-go-v2` | Apache-2.0 | Copias de seguridad S3, Ruta 53 DNS-01 |
| `google.golang.org/grpc` | Apache-2.0 | Comunicación del agente gRPC |
| `google.golang.org/protobuf` | Cláusula BSD-3 | Tiempo de ejecución de Protobuf |
| `golang.org/x/crypto` | Cláusula BSD-3 | Primitivas criptográficas |
| `golang.org/x/oauth2` | Cláusula BSD-3 | OAuth2 / Google EAB |
| `github.com/golang-jwt/jwt/v5` | MIT | JWT |
| `github.com/google/uuid` | Cláusula BSD-3 | UUID |

> ⚠ **Componentes GPL-3.0**: `sing` y `sing-shadowsocks` son GPL-3.0 o posterior. Están vinculados estáticamente al panel y a los binarios del Agente, por lo que **la distribución de binarios requiere proporcionar la fuente correspondiente** (consulte la sección 3 de [LEGAL.md](THIRD_PARTY_LICENSES.es.md)).

## 2. Programas independientes invocados en tiempo de ejecución (no distribuidos en este repositorio)

El panel/Agente puede descargar, instalar o invocar los siguientes programas independientes en tiempo de ejecución. Interactúan con este proyecto a través de procesos/interfaces, pero si los envías con tus entregables aún debes cumplir con sus licencias:

| Programa | Licencia | Notas |
|---|---|---|
| Xray-núcleo | MPL-2.0 | Núcleo de proxy, proceso separado |
| sing-box | **GPL-3.0 o posterior** | Núcleo de proxy, proceso separado; proporcionar la fuente correspondiente al distribuir |
| carrito | Apache-2.0 | Proxy inverso / TLS |
| Nginx | Cláusula BSD-2 | Proxy inverso (instalado en el sistema, si se usa) |

> Hysteria2 y TUIC son **protocolos incluidos en sing-box**, no núcleos independientes ni binarios, por lo que no se enumeran por separado.

## 3. Archivos de datos

| Datos | Fuente / licencia |
|---|---|
| `geoip.dat` / `geosite.dat` | Según lo declarado por el repositorio de fuentes de datos (comúnmente GPL-3.0 o CC-BY-SA) |
| GeoLite2 (`GeoLite2-ASN.mmdb`, etc.) | CLUF MaxMind GeoLite2 / CC BY-SA 4.0; verificar la licencia antes del uso comercial |
| `country.mmdb` | Según lo declarado por su fuente de datos. |

## 4. Recomendaciones de cumplimiento

- Al entregar archivos binarios, incluya: `LICENSE`, este archivo y el texto de licencia o adquisición URL de cada componente GPL/MPL.
- Si modifica archivos fuente de componentes MPL-2.0 como `xray-core`, publique el código fuente de los archivos modificados.
- Si distribuye programas GPL como sing-box con su producto, proporcione a los destinatarios la fuente correspondiente completa de esa versión (o una oferta escrita efectiva).
- Marcas comerciales y nombres: no utilice nombres o logotipos de proyectos anteriores de manera que implique un respaldo oficial.
