<!-- source: USER-GUIDE.md | locale: es | updated: 2026-11-23 -->
# Guía de operaciones CoreFusion

> Idiomas: [简体中文](USER-GUIDE.md) · [English](USER-GUIDE.en.md) · [繁體中文](USER-GUIDE.zh-TW.md) · [Русский](USER-GUIDE.ru.md) · [日本語](USER-GUIDE.ja.md) · [한국어](USER-GUIDE.ko.md) · [Deutsch](USER-GUIDE.de.md) · [Français](USER-GUIDE.fr.md) · Español · [Português](USER-GUIDE.pt.md) · [Türkçe](USER-GUIDE.tr.md) · [Bahasa Indonesia](USER-GUIDE.id.md) · [ไทย](USER-GUIDE.th.md) · [Tiếng Việt](USER-GUIDE.vi.md) · [العربية](USER-GUIDE.ar.md) · [فارسی](USER-GUIDE.fa.md)

> Esta guía es para usuarios de CoreFusion y cubre el flujo de trabajo completo desde la instalación hasta las operaciones diarias.

---

## Contenidos

1. [Descripción general del sistema](#1-descripción-general-del-sistema)
2. [Instalación](#2-instalación)
3. [Configuración inicial](#3-configuración-inicial)
4. [Panel de control](#4-panel-de-control)
5. [Gestión de nodos](#5-gestión-de-nodos)
6. [centro de protocolo](#6-centro-de-protocolo)
7. [Gestión de proxy](#7-gestión-de-proxy)
8. [Recursos de enrutamiento](#8-recursos-de-enrutamiento)
9. [Centro de suscripción](#9-centro-de-suscripción)
10. [Gestión de clientes](#10-gestión-de-clientes)
11. [Gestión de planos](#11-gestión-de-planos)
12. [Gestión de recargas](#12-gestión-de-recargas)
13. [Gestión de billetes](#13-gestión-de-billetes)
14. [Gestión de certificados](#14-gestión-de-certificados)
15. [Cuotas de tráfico](#15-cuotas-de-tráfico)
16. [proxies encadenados](#16-proxies-encadenados)
17. [Usuarios en línea](#17-usuarios-en-línea)
18. [Gestión de usuarios](#18-gestión-de-usuarios)
19. [sitio del portal](#19-sitio-del-portal)
20. [Configuración del sistema](#20-configuración-del-sistema)
21. [Consola de operaciones](#21-consola-de-operaciones)
22. [Monitoreo de base de datos](#22-monitoreo-de-base-de-datos)
23. [Registro de auditoría](#23-registro-de-auditoría)
24. [Gestión de registros](#24-gestión-de-registros)
25. [Extensiones de operaciones](#25-extensiones-de-operaciones)
26. [Preguntas frecuentes](#26-preguntas-frecuentes)
- [Apéndice](#apéndice)

---

## 1. Descripción general del sistema

CoreFusion es un panel de gestión de proxy multiprotocolo que admite VLESS, VMess, Trojan, Shadowsocks, Hysteria2, TUIC y más, con gestión de nodos, entrega de suscripciones, gestión de clientes, facturación, certificados y proxies encadenados.

### 1.1 Funciones principales

| Módulo | Descripción |
|--------|-------------|
| Multiprotocolo | VLESS, VMess, Trojan, Shadowsocks, Hysteria2, TUIC, WireGuard, etc. |
| multinúcleo | Xray-core y sing-box (Hysteria2/TUIC y otros protocolos QUIC se ejecutan en sing-box) |
| Multinodo | Cinco tipos de nodos: independiente, espejo, activo/en espera, entrada encadenada, salida encadenada |
| Entrega de suscripción | Múltiples formatos de suscripción, puntos finales y gestión de formatos |
| Facturación | Plantillas de planes, tarjetas de recarga, saldo, pedidos, comisión de revendedor |
| Certificados | Emisión de Let's Encrypt, validación de DNS-01/HTTP-01, distribución de certificados |
| proxy encadenado | Equilibrio de carga de múltiples salidas, registro de nodos externos, enrutamiento encadenado |
| Cuotas de tráfico | Monitoreo de tráfico mensual por nodo, suspensión automática y restablecimiento de registros |
| Portal | Editor de portal visual, multilingüe, plantillas personalizadas. |
| Notificaciones | Correo electrónico y Telegram, plantillas y difusiones. |

### 1.2 Arquitectura

- **Backend**: Ir, gRPC
- **Interfaz**: Vue 3 + Element Plus
- **Base de datos**: SQLite (predeterminado) / MySQL / PostgreSQL
- **Proxy**: proxy inverso Caddy / Nginx
- **Puerto del panel**: asignado por el asistente de configuración (puerto libre aleatorio de forma predeterminada; 8080 es una opción común)
- **Puerto gRPC del agente**: 18443 de forma predeterminada

---

## 2. Instalación

### 2.1 Entornos compatibles

| Artículo | Soporte |
|------|---------|
| Distribuciones | Debian/Ubuntu, RHEL/CentOS/Rocky/AlmaLinux/Fedora, Alpine, Arch, openSUSE |
| arquitecturas de CPU | Binarios oficiales: `amd64`, `arm64`; Las compilaciones de origen también son compatibles con `armv7`, `armv6`, `386`, `ppc64le`, `s390x`, `riscv64`, `loong64`. |
| Gerente de servicio | systemd (recomendado); recurre a `nohup` en contenedores u otros entornos sin systemd |
| Memoria/disco | ≥ 1 GB / ≥ 10 GB (la base de datos, los certificados, los registros y los archivos binarios centrales se encuentran en el directorio de datos) |
| Puerto del panel | elegido en el asistente de configuración (puerto libre aleatorio por defecto; `8080` y similares están bien) |
| Agente gRPC | `18443` por defecto |

El instalador instala automáticamente los comandos externos en tiempo de ejecución (`scripts/install.sh` y el script de instalación del Agente generado por el panel; omita con `--no-deps` / `CF_SKIP_DEPS=1`):

| Propósito | Paquete |
|---------|---------|
| Firewall/límite de velocidad/marcado de concurrencia | `nftables` (`nft`) |
| Conformación del tráfico, enlaces y sockets. | `iproute2` (`tc` / `ip` / `ss`) |
| Respaldo cuando nft no está disponible | `iptables` (`iptables-nft` en EL8+, `iptables` en EL7) |
| Seguimiento de conexión | `conntrack-tools` (`conntrack`) |
| Carga `nf_conntrack` / `ifb` | `kmod` (`modprobe` / `lsmod`) |
| Detección y limpieza de procesos. | `procps` (`pgrep` / `pkill`) |
| Comprobaciones de puertos/parámetros DH | `lsof` / `openssl` |
| Creación de usuario proxy | `shadow-utils` (Debian `passwd`, Alpine/Arco `shadow`) |
| Descargas y archivos | `curl` / `ca-certificates` / `tar` / `gzip` / `xz` |

> La falta de `nft` / `tc` solo afecta la limitación de velocidad y las funciones de firewall; el panel aún se inicia.
> Primero ejecute `--cpu-info` para ver cómo se detecta el entorno y qué paquetes faltan.

### 2.2 Instalación unifilar (recomendada)

```bash
curl -fsSL https://raw.githubusercontent.com/qist/corefusion/main/scripts/install.sh -o install.sh
sudo bash install.sh
```

O canalícelo directamente (sin interacción, sin indicadores adicionales):

```bash
curl -fsSL https://raw.githubusercontent.com/qist/corefusion/main/scripts/install.sh | sudo bash
```

El guión:

1. Detecte la distribución, la arquitectura de la CPU y la libc, luego instale los paquetes base y las dependencias de tiempo de ejecución;
2. Descargue `corefusion-linux-<arch>` de las versiones GitHub y verifíquelo con el `.sha256` correspondiente;
3. Instale en `/opt/corefusion` y cree `data/{sqlite,certs,logs,runtime}`;
4. Escriba solo un marcador de posición `.env` con `SAFE_PATHS=data` (sin configuración de base de datos) para que el primer inicio ingrese al asistente de configuración (el asistente genera el `.env` completo);
5. Instale e inicie un servicio systemd (o ejecútelo en `nohup` cuando systemd no esté disponible);
6. Espere hasta que esté listo, luego imprima la configuración URL, el token y las rutas del directorio.

Salida de ejemplo:

```text
==================================================================
 CoreFusion installation complete
------------------------------------------------------------------
 Install dir : /opt/corefusion
 Data dir    : /opt/corefusion/data
 Binary      : /opt/corefusion/corefusion
 Logs        : journalctl -u corefusion -f
------------------------------------------------------------------
 Setup wizard : http://<server-ip>:<random-port>/init?init_token=<token>
 Setup token  : <token>
 Valid for    : 2 hours (see startup log)
 Note         : the token is shown once; after setup this port closes and the panel switches to its normal port.
==================================================================
```

Si el panel solo escucha en `127.0.0.1` (por ejemplo, detrás de un proxy inverso), el script imprime un comando de túnel SSH en lugar de un URL inalcanzable.

> ¿Perdiste la ficha? Lea el registro de inicio nuevamente: `journalctl -u corefusion -n 50 --no-pager`
> (sin systemd: `tail -n 50 /opt/corefusion/data/logs/panel.log`).

### 2.2.1 Salida multilingüe

El instalador incluye cuatro idiomas de interfaz de usuario: chino simplificado, inglés, persa (فارسی) y ruso (Русский). Detecta automáticamente el sistema `LANG`, o puedes elegir explícitamente:

```bash
# English
sudo bash install.sh --lang en

# Persian via environment variable
sudo CF_LANG=fa bash install.sh

# Pass options through a pipe
curl -fsSL https://raw.githubusercontent.com/qist/corefusion/main/scripts/install.sh | sudo bash -s -- --lang ru
```

Valores admitidos: `zh` / `en` / `fa` / `ru`; También se acepta `--lang=ru`. Un valor explícito no admitido falla con la lista de idiomas disponibles.

### 2.3 Fijación de versiones/espejos/instalación sin conexión

```bash
# Pin a version
sudo bash install.sh --version v0.13.1

# GitHub download mirror (used as a URL prefix)
sudo bash install.sh --mirror https://ghproxy.net --version latest

# Use a local binary (offline / intranet / pre-built)
sudo bash install.sh --local ./dist/corefusion-linux-arm64

# Explicit target architecture
sudo bash install.sh --arch arm64 --local ./corefusion-linux-arm64
```

> Sin `--version`, el script utiliza la entrada más reciente en la lista de versiones de GitHub.
> La mayoría de las versiones son versiones preliminares, por lo que no se utiliza el punto final `/releases/latest`.
>
> Los lanzamientos oficiales solo proporcionan `amd64` / `arm64`; utilice `--source` o un binario compilado cruzado con `--local` para otras arquitecturas.

### 2.4 Construyendo desde la fuente

```bash
sudo bash install.sh --source --version v0.13.1
```

`--source` clona el repositorio (o usa el checkout existente cuando el script está dentro de él) y ejecuta `make panel`. Las versiones de la cadena de herramientas se eligen por entorno:

| dependencia | Selección |
|------------|-----------|
| ir | Leer desde `toolchain` / `go` en `go.mod` (actualmente **Go 1.27+**); si la versión del sistema es demasiado antigua, la cadena de herramientas correspondiente se descarga en `<install-dir>/.toolchain` |
| Nodo | **Nodo 22 LTS** de forma predeterminada (Vite requiere ≥ 18); se descarga una versión oficial coincidente cuando es necesario |
| Nodo (glibc < 2,28) | Utiliza la compilación `unofficial-builds` glibc-217 (por ejemplo, CentOS 7) |
| Nodo (alpino / musl) | Utiliza la distribución `nodejs` / `npm` |

Anulaciones:

```bash
# Force Go / Node versions
sudo CF_GO_VERSION=1.27.5 CF_NODE_MAJOR=22 bash install.sh --source

# Go toolchain mirror (default https://go.dev/dl)
sudo CF_GO_MIRROR=https://golang.google.cn/dl bash install.sh --source
```

Construcción manual (desarrollo/depuración):

```bash
git clone https://github.com/qist/corefusion.git
cd corefusion

# Frontend + panel for the current architecture; the frontend is embedded via go:embed
make panel

# Cross-compile
make panel-linux-amd64
make panel-linux-arm64
```

> `go build ./...` no produce un binario. Construye el panel con
> `go build -o corefusion ./cmd/corefusion/` o `make panel`.

### 2.5 Banderas del instalador

| Bandera | Descripción |
|------|-------------|
| `-d, --dir <PATH>` | Directorio de instalación, predeterminado `/opt/corefusion` |
| `-v, --version <TAG>` | Versión de panel, p. `v0.13.1`; predeterminado `latest` |
| `--binary` | Utilice el binario de versión GitHub (predeterminado) |
| `--source` | Compile desde el código fuente y prepare la coincidencia Go/Node |
| `--local <FILE>` | Utilice un binario local (instalación sin conexión) |
| `--arch <ARCH>` | Arquitectura de destino: `amd64` / `arm64` / `armv7` / `armv6` / `386` / `ppc64le` / `s390x` / `riscv64` / `loong64` |
| `--mirror <URL>` | Prefijo de descarga GitHub, p. `https://ghproxy.net` |
| `--repo <URL>` | Repositorio de origen, predeterminado `https://github.com/qist/corefusion.git` |
| `--ref <REF>` | Rama/etiqueta de origen; El valor predeterminado es `--version`; de lo contrario, `main`. |
| `--no-deps` | Omitir instalación de dependencia del sistema |
| `--no-start` | Instale y escriba el archivo de servicio sin iniciar |
| `--cpu-info` | Detección del entorno de impresión y dependencias faltantes, luego salga |
| `--lang <LANG>` | Idioma de salida: `zh` / `en` / `fa` / `ru` (predeterminado: sistema `LANG`) |
| `--uninstall` | Detenga el servicio y elimine el programa (conserva los datos y `.env`) |
| `--purge` | Con `--uninstall`, elimine también el directorio de datos y `.env` |

> Los lanzamientos oficiales solo incluyen archivos binarios `amd64` / `arm64`. Otras arquitecturas requieren `--source`,
> de lo contrario, la descarga devuelve 404.

Variables de entorno relacionadas:

| variable | Propósito |
|----------|---------|
| `CF_GO_VERSION` | Anular la versión de Go requerida para las compilaciones de código fuente |
| `CF_NODE_MAJOR` | Anular la versión principal del nodo (predeterminada 22) |
| `CF_GO_MIRROR` | Ir al espejo de descarga de la cadena de herramientas |
| `CF_SKIP_DEPS` | Establezca en `1` en el script de instalación del agente para omitir la instalación de dependencia. |
| `CF_LANG` | Anular el idioma de salida del instalador (igual que `--lang`) |

### 2.6 Diseño del directorio

```text
/opt/corefusion/
├── corefusion                 # panel binary
├── corefusion.bak             # previous version (when upgrading)
├── .env                       # environment file (0600, generated by the setup wizard)
├── .env.secret                # encryption key file (0600, generated on first start)
├── .toolchain/                # Go / Node downloaded for source builds
└── data/                      # data directory (SAFE_PATHS)
    ├── sqlite/app.db          # default SQLite database
    ├── certs/                 # certificates and keys
    ├── logs/                  # runtime logs (without systemd)
    ├── runtime/               # core runtime config and state
    ├── binaries/              # downloaded xray / sing-box cores
    └── proxy/                 # proxy (Caddy/Nginx) sites and config
```

> El instalador sólo crea `sqlite/`, `certs/`, `logs/` y `runtime/`; `binaries/`, `proxy/`, `rules/` y otros son creados por el panel o cuando se instalan los núcleos.
> El directorio de datos proviene de `SAFE_PATHS` en `.env` y puede ser una ruta absoluta. Cambiar el directorio de instalación no mueve los datos existentes.

### 2.7 Gestión de servicios

systemd (la mayoría de las distribuciones):

```bash
systemctl status corefusion --no-pager
systemctl restart corefusion
systemctl stop corefusion
journalctl -u corefusion -f          # follow logs
journalctl -u corefusion -n 100      # last 100 lines
```

Sin systemd (contenedores, etc.):

```bash
cd /opt/corefusion && ./corefusion            # foreground (debugging)
cd /opt/corefusion && setsid nohup ./corefusion >> data/logs/panel.log 2>&1 &
tail -f data/logs/panel.log
```

> Sin systemd el proceso no comienza al arrancar; reinícielo manualmente después de reiniciar (o vuelva a ejecutar el instalador).

### 2.8 Actualizar y desinstalar

Actualización (mantiene los datos y la configuración):

```bash
sudo bash install.sh --version <new-version>
```

El script detiene el servicio, realiza una copia de seguridad del binario antiguo como `corefusion.bak`, lo reemplaza y reinicia. Los modelos `.env` y `data/` existentes no se modifican.

Desinstalar:

```bash
# Remove the program only; keep data and .env
sudo bash install.sh --uninstall

# Also remove the data directory and .env (irreversible)
sudo bash install.sh --uninstall --purge
```

### 2.9 Archivo de entorno (.env)

El `.env` completo es **generado por el asistente de configuración** cuando se guarda la configuración (DSN de base de datos, directorio de datos, secretos, puertos, dominios).

En una instalación nueva, el instalador escribe solo este marcador de posición, de modo que el panel no confunda el `data/` existente con una instalación inicializada y omita el asistente:

```env
SAFE_PATHS=data
```

> Cuando ya existe una configuración completa (`.env` con `CF_DB_DSN`), el instalador nunca la sobrescribe y el asistente no vuelve a aparecer.
> Las ediciones manuales posteriores al `.env` entran en vigor después de reiniciar.

#### Directorio de datos

```env
SAFE_PATHS=data                       # relative to the install directory
# SAFE_PATHS=/srv/corefusion-data     # or an absolute path
```

#### Base de datos

`CF_DB_DSN` se detecta por prefijo:

| Prefijo | Base de datos |
|--------|----------|
| desarmado o un camino desnudo | SQLite (predeterminado) |
| `sqlite://` | SQLite (explícito) |
| `mysql://` | MySQL |
| `postgres://` / `postgresql://` | PostgreSQL |

`CF_DB_TYPE` es opcional y anula la detección (`sqlite` / `mysql` / `postgres`).

> **Contraseñas cifradas**: DSN de MySQL/PostgreSQL escritos por el panel (asistente de configuración/cambio de base de datos)
> se almacenan en `.env` como `enc:...` (AES-256-GCM, clave `CF_SECRET_ENCRYPTION_KEY`).
> Se descifran al leer; Los DSN en texto plano escritos a mano todavía funcionan.

**SQLite (predeterminado)**

```env
# Bare path (detected as SQLite)
CF_DB_DSN=data/sqlite/app.db

# Or explicit sqlite:// prefix
CF_DB_DSN=sqlite:///opt/corefusion/data/sqlite/app.db
```

**MySQL**

```env
# Format: mysql://user:password@tcp(host:port)/dbname?params
CF_DB_DSN=mysql://corefusion:password@tcp(127.0.0.1:3306)/corefusion?charset=utf8mb4&parseTime=True&loc=Local
```

**PostgreSQL**

```env
# Format: postgres://user:password@host:port/dbname?params
CF_DB_DSN=postgres://corefusion:password@127.0.0.1:5432/corefusion?sslmode=disable
```

#### Grupo de conexiones (opcional, MySQL/PostgreSQL)

```env
CF_DB_MAX_OPEN_CONNS=100
CF_DB_MAX_IDLE_CONNS=10
```

#### Otras variables comunes

| variable | Descripción |
|----------|-------------|
| `JWT_SECRET` | Clave de firma para tokens de inicio de sesión; generado en el primer inicio cuando falta |
| `CF_SECRET_ENCRYPTION_KEY` | Clave para cifrar configuraciones confidenciales (AES-256-GCM); generado cuando falta |
| `CF_INITIAL_ADMIN_PASSWORD` | Contraseña de administrador inicial (opcional; aleatoria e impresa una vez de forma predeterminada) |
| `CF_NETWORK_TRAFFIC_ACCOUNTING` | Establezca en `0` para deshabilitar la contabilidad de tráfico basada en iptables (habilitada de forma predeterminada) |
| `CF_NETWORK_TRAFFIC_TABLE` | Tabla de iptables utilizada para la contabilidad del tráfico, predeterminada `corefusion` |

> **La clave de cifrado reside en `.env.secret`**: en el primer inicio, `CF_SECRET_ENCRYPTION_KEY` se escribe en `.env.secret` (0600, el mismo directorio que `.env`) en lugar de `.env`; una clave heredada en `.env` se migra automáticamente. Haga una copia de seguridad de este archivo por separado; perderlo hace que todos los valores de `enc:` no se puedan descifrar.

> Los servidores proxy inversos confiables para resolver IP de clientes reales ya no se configuran mediante variables de entorno;
> se almacenan como `panel.trusted_proxies` en **Configuración → Red**.

> Tanto `.env` como `.env.secret` contienen secretos; el panel refuerza sus permisos a `0600` al inicio. Nunca los cometas.

### 2.10 Docker (opcional, crea tu propia imagen)

El repositorio **no** incluye un Dockerfile. Si lo coloca en un contenedor, escriba su propia imagen. Invocación de referencia:

```bash
docker run -d \
  --name corefusion \
  -p 8080:8080 \
  -p 18443:18443 \
  -v /opt/corefusion/data:/app/data \
  -v /opt/corefusion/.env:/app/.env \
  <your-image>
```

> Los contenedores generalmente no tienen privilegios systemd ni `nft`, por lo que las funciones de limitación de velocidad/firewall no funcionarán.
> El directorio de trabajo debe coincidir con la base del relativo `SAFE_PATHS`.

### 2.11 Comprobaciones posteriores a la instalación

```bash
# Environment and dependencies
sudo bash install.sh --cpu-info

# Service state
systemctl is-active corefusion

# Startup log (contains the setup URL and token)
journalctl -u corefusion -n 50 --no-pager

# Listening ports
ss -lntp | grep corefusion
```

---

## 3. Configuración inicial

En el primer inicio, si `.env` no tiene un `CF_DB_DSN` válido, el panel **no** ofrece la interfaz de usuario del administrador todavía. Comienza el
**asistente de configuración** en un puerto aleatorio entre 18000–18999 con un token único, luego se reinicia como el panel normal una vez guardado.

### 3.1 Abrir el asistente de configuración

1. Copie el “asistente de configuración” URL de la salida del instalador; o
2. Lea el registro en el servidor:

```bash
journalctl -u corefusion -n 50 --no-pager | grep -A2 'setup wizard'
```

```text
CoreFusion first start - setup wizard
  URL   : http://<ip>:<port>/init?init_token=<token>
  Token : <token>
  Valid : 2h0m0s
```

3. Abra el URL en un navegador (ya contiene el `init_token`, por lo que no es necesario realizar ninguna entrada manual).

> La sesión del asistente caduca después de **2 horas** de forma predeterminada. Si caduca o el panel se reinicia, obtenga uno nuevo.
> No compartas el token. Una vez que se completa la configuración, este puerto se cierra y solo queda el puerto normal del panel.

### 3.2 Pasos del asistente

El asistente tiene 8 pasos. Puedes regresar en cualquier momento; no se compromete nada hasta el guardado final.

| paso | Contenido |
|------|---------|
| 1. Base de datos | SQLite / MySQL / PostgreSQL |
| 2. Directorio de datos | Escrito en `SAFE_PATHS` en `.env` |
| 3. Claves de cifrado | `JWT_SECRET` y `CF_SECRET_ENCRYPTION_KEY` |
| 4. Núcleos | Qué núcleos de proxy instalar |
| 5. Apoderado | Ninguno / Nginx / Caddy |
| 6. Sitios y dominios | Puertos, ruta web, cuenta de administrador, moneda, zona horaria, dominios |
| 7. Certificado HTTPS | Deshabilitado / cargar / ACME |
| 8. Resumen | Revisar, descargar `.env`, guardar y reiniciar |

#### Paso 1: base de datos

- **SQLite (archivo local, recomendado)**: deje la ruta vacía para usar `<data-dir>/sqlite/app.db`.
- **MySQL / PostgreSQL**: host, puerto, usuario, contraseña, nombre de la base de datos; PostgreSQL también es compatible con `disable` / `require` / `verify-full`.
- Haga clic en "Probar conexión" para validar. Guardar también lo prueba. Una base de datos faltante se crea automáticamente (créela manualmente si la cuenta carece de `CREATE`).
- El DSN (incluida la contraseña) se cifra antes de escribirse en `.env`.

#### Paso 2: directorio de datos

Almacena la base de datos, certificados, registros, archivos binarios centrales y más; Este es `SAFE_PATHS` en `.env`. El valor predeterminado es `data`
(relativo al directorio de instalación); Se permite un camino absoluto. **No lo cambies casualmente más tarde**, o el panel
no encontrará la base de datos ni los certificados existentes.

#### Paso 3: claves de cifrado

- `JWT_SECRET`: firma tokens de inicio de sesión.
- `CF_SECRET_ENCRYPTION_KEY`: cifra configuraciones sensibles (contraseña de base de datos, credenciales de DNS, tarjetas de recarga, etc.).
- Ambos se generan aleatoriamente de forma predeterminada; utilice "Regenerar" para obtener nuevos valores. **Cambiar `CF_SECRET_ENCRYPTION_KEY` hace que los datos previamente cifrados sean ilegibles**: haga una copia de seguridad.

#### Paso 4: Núcleos

Seleccione los núcleos de proxy para instalar:

- **Xray**: VLESS / VMess / Trojan / Shadowsocks, etc.;
- **sing-box**: lleva protocolos Hysteria2 / TUIC y otros QUIC.

Después de guardar y reiniciar, el panel se descarga y los instala automáticamente. Verifique las versiones y el estado en "Protocolos". También puedes instalarlos más tarde.

#### Paso 5: proxy

- **Ninguno**: el panel atiende a los clientes directamente (maneje usted mismo TLS).
- **Nginx / Caddy**: el panel gestiona el proxy inverso y el alojamiento de certificados; elige uno.

Con un proxy seleccionado, el panel generalmente escucha solo en `127.0.0.1` y el proxy sirve al mundo exterior.

#### Paso 6: Sitios y dominios

| campo | Descripción |
|-------|-------------|
| Escuchar dirección | Derivado automáticamente: proxy → `127.0.0.1`; sin proxy con certificado → `0.0.0.0`; en caso contrario → `127.0.0.1` |
| Puerto del panel | Puerto libre aleatorio por defecto; cámbielo si lo desea (por ejemplo, 8080) |
| Puerto de suscripción | Opcional; `0` comparte el puerto del panel |
| Puerto del portal | Opcional; `0` comparte el puerto del panel |
| Ruta web | Ruta de la interfaz de usuario del administrador, predeterminada `/`; el API siempre es `/api/v1` |
| Nombre de usuario/contraseña de administrador | Aleatorio por defecto; **la contraseña se muestra solo una vez en el resumen** — guárdela |
| Modo de acceso | “Dominio” o “IP” |
| Moneda predeterminada/zona horaria | Moneda predeterminada para tarjetas de recarga y saldos; zona horaria del registro de proxy y del panel (por ejemplo, `Asia/Shanghai`) |
| Agregar al proxy inverso | Cuando se configuran un proxy y dominios, agregue dominios de panel/suscripción/portal al proxy de una sola vez |

Al ingresar dominios, se escribe la configuración correspondiente (dominio de devolución de llamada, punto final de suscripción, dominio compartido).

#### Paso 7: certificado HTTPS

| Fuente | Descripción |
|--------|-------------|
| Deshabilitar HTTPS | El panel/portal/suscripciones se brindan a través de HTTP (inseguro); con un proxy inverso puedes agregar TLS en el lado del proxy más tarde |
| Cargar certificado | Pegue el certificado PEM y la clave privada PEM (solo modo de acceso al dominio) |
| ACME | Emite el certificado cuando envía este paso; admite certificados de dominio e IP |

Opciones de ACME:

- **CA**: Ciframos, Ciframos (ensayo), ZeroSSL, CA pública Google;
- **Destino**: el dominio configurado o la IP pública (los certificados de IP solo son compatibles con Let's Encrypt);
- **Desafío**: `HTTP-01` (requiere puerto 80) o `DNS-01` (comodines, sin puerto 80; elija un proveedor DNS e ingrese las credenciales);
- **Tipo/tamaño de clave**: ECC 256/384 o RSA 2048/3072/4096;
- **Google CA pública**: requiere EAB; ingrese la ID de clave / HMAC manualmente o use una cuenta de servicio de GCP.

> Los certificados de IP detectan automáticamente el IPv4/IPv6 público del host. Detrás de un proxy, la dirección detectada es la del host, no la salida del proxy.
> Los certificados de dominio también se pueden emitir más adelante en "Certificados".

#### Paso 8: Resumen

Muestra la base de datos, el directorio de datos, los núcleos, el proxy, los puertos, la cuenta de administrador, los dominios y las claves `.env` (secretos enmascarados) y proporciona:

- **Descargar .env**: guarde una copia de la configuración;
- **URL de acceso**: URL finales para panel/suscripción/portal; HTTP está marcado como inseguro.

Haga clic en "Guardar y reiniciar" para escribir la base de datos y `.env`; el panel se reinicia en servicio normal.

### 3.3 Después de la configuración

- El servicio se reinicia y el registro de inicio imprime la dirección de escucha final;
- Inicie sesión con la cuenta de administrador, acceda a URL y a la ruta web desde el resumen;
- Si pierde la contraseña, restablezcala sin conexión con `./corefusion admin reset-password`.

### 3.4 Configuración del CLI no interactivo

Para servidores automatizados o sin cabeza, escriba la configuración y marque la configuración completada directamente:

```bash
cd /opt/corefusion
./corefusion init \
  --db-type sqlite \
  --data-dir /opt/corefusion/data \
  --admin-username admin \
  --admin-password 'change-me-strong-password' \
  --listen-port 8080 \
  --proxy-type caddy \
  --cores xray,singbox \
  --panel-domain panel.example.com \
  --tls-mode acme --tls-domain panel.example.com --tls-email you@example.com
```

Banderas comunes:

| Bandera | Descripción |
|------|-------------|
| `--db-type` / `--db-host` / `--db-port` / `--db-user` / `--db-password` / `--db-name` | Configuración de la base de datos |
| `--data-dir` | Directorio de datos; El valor predeterminado es `SAFE_PATHS` o `./data`. |
| `--listen-port` / `--subscription-port` / `--portal-port` / `--web-path` | Puertos y ruta web (puerto libre aleatorio por defecto) |
| `--admin-username` / `--admin-password` | Cuenta de administrador (contraseña aleatoria impresa una vez cuando se omite) |
| `--proxy-type` | `none` / `nginx` / `caddy` |
| `--cores` | Núcleos para instalar, separados por comas: `xray,singbox` |
| `--panel-domain` / `--subscription-domain` / `--portal-domain` | Dominios (escritos en la configuración y agregados al proxy inverso) |
| `--tls-mode` | `none` / `self_signed` / `acme` |
| `--tls-domain` / `--tls-email` / `--tls-challenge` / `--tls-key-type` / `--tls-key-bits` | Opciones de certificado (`--tls-challenge` por defecto es `http-01`; `dns-01` necesita `--tls-dns-channel` o `--tls-dns-provider`) |
| `--tls-ip-cert` | Emitir un certificado para la IP pública detectada (solo Let's Encrypt) |
| `--acme-provider` | `lets_encrypt` / `lets_encrypt_staging` / `zerossl` / `google` |
| `--currency` / `--timezone` | Moneda predeterminada/zona horaria |

Lista completa: `./corefusion init --help`. Luego inicie el servicio:

```bash
systemctl restart corefusion
```

### 3.5 Volver a ejecutar el asistente de configuración

Para empezar de nuevo (elimina `.env` y las marcas de configuración, **conserva los datos**):

```bash
cd /opt/corefusion
./corefusion reset-init      # type RESET to confirm
systemctl restart corefusion
```

Luego use el nuevo URL y el token del registro de inicio.

> La reinicialización restablece la cuenta de administrador y la ruta web. Los datos comerciales de la base de datos anterior permanecen, pero es posible que deba volver a crear la cuenta de inicio de sesión.
>
> `reset-init` elimina todo el `.env` (incluido el `SAFE_PATHS`). **Si su directorio de datos es una ruta absoluta personalizada**, restaure `SAFE_PATHS` antes de reiniciar; de lo contrario, el panel se verá en `./data`:
>
> ```bash
> printf 'SAFE_PATHS=/srv/corefusion-data\n' > /opt/corefusion/.env
> ```

### 3.6 Seguridad después del primer inicio de sesión

#### Cambiar la contraseña inicial

1. Haga clic en el nombre de usuario en la esquina superior derecha;
2. Elija "Cambiar contraseña";
3. Ingrese las contraseñas antiguas y nuevas;
4. Haga clic en "Guardar".

#### Habilitar inicio de sesión con código dinámico (2FA / TOTP)

> Una vez habilitada, la cuenta cambia a **inicio de sesión con código dinámico**: la contraseña estática ya no se acepta; solo el
> El código de 6 dígitos de tu aplicación de autenticación (o un código de recuperación único) funciona. Esto **reemplaza** el inicio de sesión con contraseña: es
> no verificación en dos pasos “contraseña + código”.

1. Haga clic en el nombre de usuario en la esquina superior derecha;
2. Elija "Habilitar 2FA";
3. Ingrese la contraseña actual (solo para confirmar este cambio);
4. Escanee el código QR con una aplicación de autenticación (Google Authenticator / 1Password, etc.) o ingrese la clave manualmente;
5. Ingresa el código de 6 dígitos para terminar de habilitar;
6. **Guarde los códigos de recuperación inmediatamente** (descargable); cada código funciona una vez.

Iniciar sesión después:

- Después de ingresar el nombre de usuario, el campo de contraseña se convierte en “código de 6 dígitos/código de recuperación” y el botón se convierte en “Iniciar sesión con código”;
- Ingrese el código actual; **la contraseña estática ya no se puede utilizar**;
- Los códigos de recuperación son de un solo uso; regenerarlos en “Inicio de sesión con código dinámico (2FA)” cuando se agoten (esto invalida todos los códigos antiguos).

Si se pierde el autenticador y se agotan los códigos de recuperación, desactive 2FA sin conexión en el servidor:

```bash
cd /opt/corefusion
./corefusion admin 2fa reset -u admin    # disables 2FA, clears the secret and recovery codes, revokes all sessions
```

> Un superadministrador único que no tenga el autenticador ni los códigos de recuperación no puede realizar la recuperación a través de la interfaz de usuario web y debe usar el comando anterior.

#### Registre un Passkey (opcional)

1. Haga clic en el nombre de usuario en la esquina superior derecha;
2. Elija “Gestión Passkey”;
3. Haga clic en "Registrar nuevo Passkey";
4. Complete el mensaje del dispositivo.

> **Nota**: Las claves de acceso requieren HTTPS o localhost, y el dominio debe estar en la lista de permitidos.

---

## 4. Panel de control

El panel muestra la descripción general del clúster de múltiples nodos y el estado de los nodos.

### 4.1 Qué muestra

- **Información del sistema**: nombre de host, versión del sistema operativo, tiempo de actividad, CPU, memoria
- **Tráfico**: ancho de banda en tiempo real, tráfico total, tendencia mensual
- **Estado del nodo**: recuentos en línea/fuera de línea/degradados, nodos de alta carga
- **Alertas**: nodos fuera de línea, nodos degradados, certificados que caducan pronto, advertencias de cuota
- **Recursos del clúster**: CPU, memoria, disco y rendimiento promedio

### 4.2 Acciones

- Haga clic en "Ver detalles" para abrir la página de detalles de un nodo.
- Haga clic en una tarjeta de nodo para ver su estado activo
- Utilice los filtros para limitar por estado, tipo o región

---

## 5. Gestión de nodos

La gestión de nodos es el núcleo de CoreFusion y admite varios tipos de nodos.

### 5.1 tipos de nodos

| Tipo | Descripción |
|------|-------------|
| Independiente (`standalone`) | Un nodo central independiente |
| Espejo (`mirror`) | Sincroniza la configuración desde un nodo de destino |
| Primario/copia de seguridad (`primary_backup`) | Nodos asociados que se conmutan entre sí |
| Entrada de cadena (`chain_entry`) | Nodo de entrada de un proxy encadenado |
| Salida de cadena (`chain_exit`) | Nodo de salida de un proxy encadenado |

### 5.2 Agregar un nodo

1. Abra "Gestión de nodos";
2. Haga clic en "Agregar servidor";
3. Complete lo básico:
   - Nombre, p.e. `HongKong-HK01`
   - Nombre de host: IP o dominio
   - Región, p.e. `HK`, `JP`, `US`
   - Tipo de nodo;
4. Configure el puerto del Agente (predeterminado `18443`);
5. Seleccione los núcleos compatibles;
6. Haga clic en "Guardar".

### 5.3 Implementación con un clic

1. Después de agregar el nodo, elija "Implementar comando" en la lista de nodos (la página de detalles también ofrece
 “Comando de instalación de Bootstrap”);
2. Copie el comando: contiene un token de registro único (válido por 24 horas, no lo reutilice);
3. Ejecútelo como root en el nodo remoto; parece:

   ```bash
   bash <(curl -fsSL "http://<panel-host>:<panel-port>/api/v1/agent/install?id=<node-id>&token=<one-time-token>&port=<panel-port>")
   ```

4. El Agente descarga el paquete, se registra en el panel y comienza.

### 5.4 Detalles del nodo

Haga clic en un nodo para abrir su página de detalles:

- **Descripción general**: información básica, estado del agente, estado del certificado; Las tarjetas de recursos muestran el uso de CPU/memoria/disco, tasa de subida/bajada de NIC, tasa de tráfico de usuario central, tráfico usado mensual, conexiones TCP, carga del sistema y tráfico acumulativo de salida de NIC (actualización silenciosa cada 10 segundos)
- **Centro de protocolo**: configuración del proceso central
- **Proxy**: configuración Caddy / Nginx
- **Procesos principales**: iniciar/detener/reiniciar
- **Tráfico principal**: tráfico de usuarios reenviado por el núcleo y su total mensual
- **WARP**: configuración WARP
- **Dominios directos**: gestión de dominios
- **Límite de tarifa**: modelado del tráfico
- **Certificados**: vinculaciones de certificados
- **Usuarios en línea**: usuarios en línea en tiempo real

### 5.5 Acciones de nodo

- **Configuración de sincronización**: envía la configuración del panel al nodo
- **Forzar reconexión**: abandone la conexión del Agente y vuelva a conectarse
- **Actualizar agente**: envíe una actualización del agente
- **Revocar certificado**: revocar el certificado de Agente del nodo

---

## 6. centro de protocolo

El centro de protocolo configura los parámetros de protocolo para los procesos centrales.

### 6.1 Núcleos soportados

| Núcleo | Descripción |
|------|-------------|
| Núcleo Xray | VLESS / VMess / Trojan / Shadowsocks y más |
| sing-box | Plataforma proxy universal modular con soporte TUN; lleva Hysteria2 / TUIC y otros protocolos QUIC |

### 6.2 Módulos de configuración

Los módulos difieren entre los dos núcleos; el panel muestra el conjunto que coincide con el núcleo seleccionado:

**Núcleo Xray**

- **Registro**: registro de acceso, registro de errores, modo de salida
- **API**: gRPC API dirección de escucha, lista de servicios
- **DNS**: servidores DNS, estrategia de consulta, hosts estáticos
- **Enrutamiento**: reglas de enrutamiento, equilibrio de carga, enrutamiento VLESS
- **Entradas/Salidas**: protocolo, transporte, TLS
- **Transporte**: WebSocket, gRPC, XHTTP, etc. (TUN es un protocolo de entrada, no un transporte)
- **TLS**: certificados, ALPN, huella digital uTLS, ECH
- **Reality**: gestión de objetivos, pares de claves, ID cortos
- **Comprobación de estado / FakeDNS / Métricas / Política / Datos geográficos / Límites de versión**: sondas, grupo de IP, Prometheus, tiempos de espera y buffers, archivos de recursos, rango de versiones principales

**sing-box**

- **Registro**: nivel, marca de tiempo, salida
- **NTP**: sincronización horaria
- **DNS**: servidores y reglas DNS
- **Enrutamiento**: salida predeterminada `final`, **conjuntos de reglas**, **reglas**, resolución de dominio predeterminada, estrategia de red y respaldo (consulte 6.6)
- **Entradas/Salidas**: protocolo, transporte, TLS
- **Puntos finales**: WireGuard (incluidas configuraciones WARP), clientes MASQUE y similares; varios pueden coexistir y se distinguen por `tag`

> Con "usar integración de red del sistema" habilitado, el **nombre** del punto final es el nombre de la interfaz del kernel:
> como máximo 15 caracteres, letras/dígitos/`._-` únicamente. Un nombre demasiado largo o no válido automáticamente retrocede
> al punto final `tag`. Los nuevos puntos finales tienen el prefijo por protocolo (`wg-` / `ts-` / `oc-` / `ovpn-c-` /
> `ovpn-s-` / `mq-c-` / `mq-s-`) y registros heredados como `WireGuard-xxxxxx` se muestran normalizados
> tan pronto como se abre el editor.
- **Servicios**: el array sing-box `services` de nivel superior, como `hysteria-realm`, DERP o `resolved`
- **Certificados / Clientes HTTP / Espacios de nombres de red**: secciones JSON que se expanden solo cuando es necesario
- **Experimental**: `clash_api` y similares; **no se escribe nada en la configuración generada a menos que esté habilitado**

> sing-box 1.12 eliminó los campos de enrutamiento `geosite` / `geoip`; El enrutamiento basado en geografía ahora pasa
> `rule_set`. Mantener los campos antiguos hace que el núcleo no pueda decodificar la configuración (`unknown field`).

### 6.3 Configuración entrante

1. Seleccione el núcleo y el protocolo;
2. Complete lo básico:
   - Etiqueta
   - Escuchar dirección y puerto
   - Tipo de protocolo
3. Establecer parámetros de protocolo (UUID, contraseña, etc.);
4. Elija el transporte (TCP, WebSocket, gRPC, etc.);
5. Configurar TLS (certificado, ALPN);
6. Guardar y aplicar al núcleo.

> Las nuevas entradas escuchan en `::` de forma predeterminada (pila dual en Linux, manejando IPv4 e IPv6).
> Cámbielo a `0.0.0.0` o una dirección específica solo cuando necesite restringir la interfaz.

Los modos de puerto "Usar puerto de entrada" también ofrecen **crear reglas de permiso de firewall automáticamente**: abrir el
El editor entrante prueba primero el firewall del nodo y cuando detecta firewalld / ufw / nftables / iptables.
puede permitir que el puerto se guarde de acuerdo con el protocolo de escucha (TCP/UDP).

- El salto de puerto hy2/histeria permite tanto el puerto de escucha como cada rango de puerto dinámico (por ejemplo, `20000-30000`); varios rangos están separados por comas;
- La asignación es idempotente: una regla de cobertura existente (rango de puertos/multipuerto) no se vuelve a crear;
- Cambiar el puerto revoca la regla anterior antes de permitir la nueva; eliminar el entrante, deshabilitar el salto de puerto o cambiar a Unix Socket / tun (sin puerto de escucha) lo limpia;
- Cuando no se detecta ningún firewall habilitado, solo se muestra una sugerencia y no se escribe ninguna regla.

**Transporte y flujo**:

- `xtls-rprx-vision` solo funciona sobre TCP sin formato: con gRPC / XHTTP / WebSocket el cliente falla por completo (sing-box: `initialize vision: not a valid supported TLS connection`) o no está de acuerdo con la verificación de flujo del servidor (`flow mismatch`). El panel elimina la visión de cualquier entrada no cruda/TCP al guardar y al generar la configuración de tiempo de ejecución;
- Los transportes sing-box son http / ws / quic / grpc / httpupgrade únicamente. **TCP sin formato significa que no hay ningún transporte**. La primera entrada (vacía) del menú desplegable de transporte es TCP sin formato; escribir `type: "tcp"` hace que el núcleo rechace la configuración.

### 6.3.1 Ruta del proxy inverso (Dominio de acceso → Frontal TLS → Ruta del proxy inverso)

Cuando la ruta de transporte propia del entrante es `/` pero una nginx / caddy externa la monta debajo de una subruta
(por ejemplo, `location /apps { proxy_pass http://127.0.0.1:8080/; }`), los clientes deben conectarse a `/apps` o al
la solicitud nunca llega a esa ubicación: la suscripción aún se descarga, pero el nodo no se conecta.
Complete **Ruta de proxy inverso** en “Dominio de acceso → Frontal TLS”:

- Ingrese `/apps` o `/apps?ed=2560` (con datos anteriores); las suscripciones y los enlaces compartidos **reemplazan** la ruta de transporte entrante con ellos;
- Se aplica a ws, httpupgrade, xhttp y h2/http; los transportes sin ruta (raw/tcp, kcp) se ignoran;
- Es independiente de dónde termina TLS: la ruta aún se aplica cuando Front TLS está apagado (siga el núcleo);
- gRPC no tiene el concepto de prefijo URL (la ruta del cliente siempre es `/{serviceName}/…`), por lo que la ruta se **fusiona en serviceName**
(`/apps` + `svc` → `apps/svc`); el proxy inverso debe eliminar ese prefijo en `grpc_pass`.

### 6.4 Configuración saliente

1. Agregue una salida;
2. Establecer la etiqueta y el protocolo;
3. Establezca la dirección y el puerto del servidor;
4. Configurar transporte y TLS;
5. Guardar y aplicar.

### 6.5 Compartir enlaces

1. Haga clic en "Compartir enlace" en la lista entrante;
2. Elige un formato:
   - Compartir URI
   - Mihomo YAML
   - sing-box JSON
   - Xray JSON
3. Copie el enlace o genere un código QR.

### 6.6 Conjuntos de reglas y enrutamiento sing-box

En “Centro de protocolos” → sing-box → “Enrutamiento” hay dos partes: **Conjuntos de reglas** declarar reutilizable
archivos de reglas y **Reglas** deciden las condiciones de coincidencia y la salida. Las reglas se evalúan desde
de arriba a abajo y el primer partido gana.

#### 6.6.1 Conjuntos de reglas

| campo | Descripción |
|-------|-------------|
| etiqueta | Identificador de conjunto de reglas al que hacen referencia las reglas; debe ser único |
| Tipo | `local` lee un archivo en el nodo; `remote` permite que el núcleo lo descargue en `update_interval` |
| Formato | `binary` para `.srs` (predeterminado); `source` para formato fuente `.json` |
| Camino | Las rutas `local` son relativas al directorio de datos del nodo; Los archivos en `data/rules/` usan `rules/xxx.srs`. |
| URL | Dirección de descarga para `remote`; `download_detour` selecciona la salida utilizada para la descarga (normalmente `direct`) |

Conjuntos de reglas locales (recomendado; descargue los archivos primero en “Recursos de servicio → Archivos de reglas”):

```jsonc
"route": {
  "rule_set": [
    { "type": "local", "tag": "geosite-cn", "format": "binary", "path": "rules/geosite-cn.srs" },
    { "type": "local", "tag": "geoip-cn", "format": "binary", "path": "rules/geoip-cn.srs" }
  ]
}
```

> Descargar el archivo en el panel no es suficiente: primero se debe enviar un conjunto de reglas `local` al
> nodes (“Enviar a todos los nodos”) para que el archivo exista en el `data/rules/` del nodo; de lo contrario el núcleo
> no se inicia porque no se puede encontrar el archivo.

Conjuntos de reglas remotas (el nodo debe poder llegar al URL):

```jsonc
{ "type": "remote", "tag": "geosite-openai", "format": "binary",
  "url": "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-openai.srs",
  "download_detour": "direct", "update_interval": "1d" }
```

Cuando elige un archivo local, la etiqueta se deriva del nombre del archivo (`rules/geosite-cn.srs` →
`geosite-cn`); para aparatos remotos proviene del final del URL. El formato también se completa desde
la extensión (`.srs` → `binary`, `.json` → `source`). Los valores derivados se pueden editar libremente y
Los valores que ya escribió nunca se sobrescriben.

> `download_detour` ha quedado obsoleto desde sing-box 1.14 y se eliminará en 1.16; todavía
> Funciona en 1.15. Las nuevas configuraciones deben utilizar `http_clients` de nivel superior con
> `route.default_http_client` en su lugar.

#### 6.6.2 Reglas

| campo | Partidos |
|-------|---------|
| `domain` | Dominio exacto |
| `domain_suffix` | Sufijo de dominio, incluidos los subdominios |
| `domain_keyword` | Coincidencia de subcadenas |
| `domain_regex` | expresión regular |
| `ip_cidr` / `ip_is_private` | IP de destino |
| `source_ip_cidr` | IP de origen |
| `port` / `port_range` | Puerto de destino |
| `network` | `tcp` / `udp` |
| `rule_set` | Etiquetas de los conjuntos de reglas a utilizar |
| `outbound` | Etiqueta de salida a utilizar (debe existir en "Salidas" o "Puntos finales") |
| `type: "logical"` | Condiciones combinadas con `mode` configurado en `and` / `or` |

Varios campos en una regla se combinan con AND (por ejemplo, `network` + `port`), por lo que no
Se necesita envoltorio `logical`. Utilice `type: "logical"` sólo para combinaciones O/NO. `rule_set`
debe colocarse dentro de las subreglas `rules`, no al lado de `rules`:

```jsonc
"rules": [
  { "domain_suffix": ["gstatic.com", "googleapis.com"], "outbound": "direct" },
  { "domain_keyword": ["ip.sb", "yandex.com"], "outbound": "warp" },
  { "rule_set": ["geosite-openai", "geosite-google"], "outbound": "warp" },
  { "rule_set": ["geoip-cn"], "outbound": "warp" },
  { "network": "tcp", "port": 443,
    "rule_set": ["geosite-google"], "outbound": "warp" },
  { "type": "logical", "mode": "or",
    "rules": [
      { "rule_set": ["geosite-google"] },
      { "rule_set": ["geosite-openai"] }
    ],
    "outbound": "warp" }
]
```

#### 6.6.3 Migración de reglas xray

| xray | sing-box |
|------|----------|
| Cadena desnuda (incluidos los valores desnudos en `domain`; coincidencia de subcadena) | `"domain_keyword": ["ip.sb"]` |
| `"domain": ["a.com"]` | `"domain_keyword": ["a.com"]`; use `"domain_suffix": ["a.com"]` para una coincidencia de sufijo real o `"domain": ["a.com"]` para una coincidencia exacta |
| `"domain:clstorage.net"` | `"domain_suffix": ["clstorage.net"]` |
| `"full:a.com"` | `"domain": ["a.com"]` |
| `"regexp:..."` | `"domain_regex": ["..."]` |
| `"ip": ["geoip:cn"]` | `"rule_set": ["geoip-cn"]` |
| `"domain": ["geosite:cn"]` | `"rule_set": ["geosite-cn"]` |
| `"outboundTag": "direct"` | `"outbound": "direct"` |

Notas:

- **Las etiquetas no se comparten entre núcleos**: una etiqueta saliente xray no se puede utilizar en una regla sing-box;
 utilice una etiqueta que realmente exista en sing-box "Salidas" o "Puntos finales".
- Una etiqueta de salida o de punto final incorrecta informa `outbound not found`; una etiqueta de conjunto de reglas incorrecta informa
 `rule-set not found`.
- Ponga primero las reglas directas/de lista permitida; una vez que una regla coincide, las reglas posteriores ya no se aplican.

---

## 7. Gestión de proxy

La administración de proxy configura el proxy inverso Caddy o Nginx.

### 7.1 Tipos de proxy

- **Caddy**: HTTPS, HTTP/1.1 y HTTP/2 automáticos (HTTP/3 / QUIC está deshabilitado para evitar un conflicto de UDP 443 con el enrutamiento de capa 4)
- **Nginx**: alto rendimiento, configuración flexible

### 7.2 Configuración del sitio

1. Abra "Gestión de proxy";
2. Elija el tipo de proxy (Caddy / Nginx);
3. Agregar un sitio:
   - Dominio
   - aguas arriba
   - Puerto local
4. Guardar.

### 7.3 Campos

- **upstream**: dirección backend, p.e. `127.0.0.1:8082`
- **local_port**: puerto de escucha local, p.e. `8443`
- **portal_forward**: regla de reenvío del portal
- **subscription_forward**: regla de reenvío de suscripción
- **Enrutamiento SNI**: cuando un entrante usa TLS / REALITY y los clientes siempre se conectan en 443, el panel deriva
 443 reglas de enrutamiento TCP de ClientHello SNI (tanto xray `streamSettings` como sing-box `tls.reality`);
 el proxy frontal solo reenvía TCP y TLS todavía termina dentro del núcleo

---

## 8. Recursos de enrutamiento

Los recursos de enrutamiento recopilan los activos compartidos utilizados por las suscripciones y el enrutamiento. La página tiene cinco pestañas:

| Pestaña | Propósito |
|-----|---------|
| Dominios preferidos directos y CF | Dominios directos de ingreso/IP y dominios preferidos Cloudflare, utilizados al reescribir direcciones de acceso en suscripciones y nodos externos |
| Grupo de IP de CF | Fuentes de IP, puertos y configuraciones de sonda preferidas, consumidas por el “dominio de acceso” |
| Reglas globales WAF | Fuentes de reglas y archivos de reglas para el proxy inverso WAF, enviados a los nodos |
| Archivos de reglas | Descarga, sincronización programada y envío de archivos de reglas (`.srs` / `.json` / `.dat`) |
| Nodos externos | Importar nodos externos desde v2ray URI / Base64 / suscripción URL (ver 16.3) |

### 8.1 Dominios preferidos directos y CF

- **Acceso directo**: dominio de entrada o IP, puerto 443 por defecto; vincular nodos o dejarlos vacíos para global;
- **Dominios preferidos CF**: dominio preferido Cloudflare más puerto, con verificación de conectividad;
- Ambos alimentan a los candidatos de “dominio de acceso” utilizados por las suscripciones y la reescritura de nodos externos (ver 16.3).

- **Deriva de entrada (primaria/en espera)**: una entrada directa puede vincular un canal DNS y un nombre de registro; después de una conmutación por error o un cambio de retorno, el panel señala el registro A en la IP pública principal actual (IP interna cuando está vacía). Solo se aceptan proveedores con un escritor de DNS implementado (`cloudflare` verificado; alidns / DNSPod / GoDaddy / Namecheap / Route53 / Google / Huawei Cloud implementado pero no verificado). La propagación de Cloudflare se midió aproximadamente entre 2 y 2,5 minutos con TTL fijado en 60; mantenga la nube naranja apagada para las entradas TCP/REALITY.

### 8.2 Grupo de IP de CF

- Mantenga las fuentes de IP preferidas (incluidas las fuentes de estilo `bestproxy`), el intervalo de recuperación y los puertos;
- La “configuración de la sonda” controla el intervalo de la sonda, el umbral de latencia y el nodo de entrada de la sonda; sondeo local
 solo filtra IP claramente inalcanzables, mientras que seleccionar un nodo de entrada simula la línea de ese nodo;
- El “marcador” podrá ser automático (núcleos instalados), `singbox`, `xray` o `mihomo`; núcleos que no son
 instalados no aparecen en la lista. El sondeo se ejecuta como un proceso temporal que finaliza cuando finaliza y nunca lee ni
 escribe la propia configuración del núcleo;
- Cuando el “dominio de acceso” utiliza el grupo de IP de CF sin un valor concreto, se elimina una entrada (con su puerto).
 elegido al azar.

### 8.3 Reglas globales WAF

- Configure la fuente de reglas WAF URL y los archivos de reglas para sincronizar;
- Después de la sincronización, se envían a los nodos y el proxy del nodo los carga (Nginx / Caddy);
- Desde ese URL se descarga un archivo con un URL completo; de lo contrario se utiliza “fuente URL + /nombre de archivo”;
- "Caché de coincidencias" (`match_cache` / `config_match_cache`) está habilitado de forma predeterminada: un proceso limitado
 el caché de resultados de coincidencias de reglas activas reduce el recorrido de reglas por solicitud. Es sólo global sin
 anulación por dominio; deshabilítelo temporalmente para la comparación A/B al investigar falsos positivos
 (Deshabilitarlo significa una evaluación completa de las reglas en cada solicitud y un mayor uso de la CPU).

### 8.4 Archivos de reglas (archivos de reglas de enrutamiento)

Esta pestaña solo descarga archivos de reglas en el panel y los envía a los nodos; nada surte efecto
automáticamente. Para usarlos en enrutamiento, consúltelos en “Centro de protocolos → sing-box → Enrutamiento”
en Conjuntos de reglas y Reglas (ver 6.6).

- **Repositorios de reglas**: ingrese `owner/repo` (GitHub, navegable) o una descarga completa de URL; programado
 la sincronización puede ejecutarse cada 6/12 horas, diariamente a las 03:00, diariamente a medianoche o semanalmente los domingos;
- **Agregar archivo**: para archivos individuales fuera de una versión del repositorio (por ejemplo, rama SagerNet `.srs`
 archivos); ingresa el URL completo y se descarga automáticamente;
- **Lista de archivos**: muestra el nombre, el tamaño, la última actualización y el interruptor de “Sincronización automática”, con “Descarga” por archivo;
 el botón "Enviar a todos los nodos" en la parte superior envía cada archivo de reglas a todos los nodos;
- Los archivos se descargan en el `data/rules/` del panel y, una vez presionados, se encuentran debajo de los datos del nodo.
 `rules/` del directorio; haga referencia a ellos en sing-box como `rules/<file name>`;
- Formatos:
  - `.srs`: conjunto de reglas binarias sing-box, recomendado y más pequeño;
  - `.json`: conjunto de reglas de formato fuente; `rule_set` debe configurar `format: "source"`;
  - `.dat`: xray `geosite.dat` / `geoip.dat`, que sing-box no puede utilizar directamente.

> El bloque plegable "Uso" en la parte superior de la página contiene ejemplos de referencia, el mapeo xray
> Tabla y errores comunes.

### 8.5 Nodos externos

La importación y gestión de nodos externos se trata en [16.3 Nodos externos](#163-external-nodes).

---

## 9. Centro de suscripción

El centro de suscripciones empaqueta nodos en suscripciones listas para el cliente: define **formatos** de suscripción (qué emitir, qué reglas, qué planes pueden verlos), luego **puntos finales** de suscripción (dominio público/ruta), luego entrega los enlaces.

### 9.1 Formatos de suscripción

“Centro de suscripción” → “Formatos de suscripción”; una fila por salida del cliente:

| campo | Descripción |
|---|---|
| Nombre | Único; `?config=<name>` coincide y el cuadro de diálogo del cliente lo muestra |
| Tipo de cliente | clash / shadowrocket / singbox / v2ray: solo visualización de agrupación |
| Formato de salida | `mihomo_yaml` (Clash/Mihomo), `singbox_json`, `xray_json`, `base64_uri`, `raw_uri` |
| Tipo de contenido/extensión | Encabezado de respuesta y nombre de archivo |
| Plataforma | ganar/mac/android/ios/universal; emparejado con `?platform=`, vacío = universal |
| Planes vinculados | Vacío = disponible para todos los planes; de lo contrario, solo esos planes pueden usarlo |
| Reglas | Reglas de enrutamiento por formato (editor estructurado para Mihomo, JSON sin formato por regla para sing-box / Xray). **Las reglas no se convierten entre formatos** |

Notas:

- Pueden existir varias configuraciones para el mismo formato de salida (por ejemplo, `out`, `home`); elija uno con `config=` en el enlace;
- Sin `config`, el orden es “configuración vinculada a la suscripción → coincidencia de `platform` → predeterminada”;
- Una plantilla sing-box puede llevar su propio `route.rule_set` para los clientes, normalmente como `remote` por lo que
 el cliente lo descarga; Los archivos locales enviados al `rules/` de un nodo en "Archivos de reglas" solo están disponibles
 al núcleo de ese nodo, no a las suscripciones de clientes;
- Al generar plantillas sing-box / Xray, el panel elimina las referencias colgantes (`final`, `dns.final`,
 `default_domain_resolver`, `download_detour`, `rule_set` local, definiciones de `rule_set` no válidas, vacías
 `balancer`, etc.) para que el cliente no falle al iniciar con un error GRAVE;
- Los alias salientes en las reglas se resuelven con etiquetas reales (`REJECT` / `blocked` → `block`, `DIRECT` →
 `direct`, no distingue entre mayúsculas y minúsculas); las salidas irresolubles recurren a la salida predeterminada y a la regla
 mantiene su posición;
- Siempre se genera un grupo con `tag = proxy`: un `selector` para sing-box y un equilibrador `random` para
 Xray y `outbound: proxy` en las reglas se reescriben en el `balancerTag` correspondiente. Se carga el editor de reglas
 candidatos salientes dinámicamente desde “grupos proxy + salidas personalizadas”, y las etiquetas ingresadas manualmente se
 todavía validado;
- Las configuraciones sing-box / Xray deben crearse por separado; si no existen, no aparecen en los enlaces de los clientes (ver 10.3).

- El editor de reglas estructuradas de Mihomo admite reglas lógicas `AND` / `OR` / `NOT` / `SUB-RULE`: escriba la condición como `((NETWORK,UDP),(DST-PORT,3478/19302))`; para `SUB-RULE`, la tercera columna es el nombre del grupo de subreglas. Las condiciones deben estar entre paréntesis;
- Los tipos de reglas están alineados con el último mihomo: `IP-SUFFIX`, `SRC-IP-CIDR/SRC-IP-SUFFIX/SRC-IP-ASN/SRC-GEOIP`, `IN-PORT/IN-TYPE/IN-USER/IN-NAME/REMATCH-NAME`, `PROCESS-NAME-WILDCARD/REGEX`, `PROCESS-PATH-WILDCARD/REGEX`, `UID`, `DSCP` están todos disponibles;
- `SUB-RULE` hace referencia al bloque `sub-rules:` de la configuración mihomo; el panel aún no edita ese bloque, así que defínelo primero mediante la importación YAML o el config_json compartido.

### 9.2 Puntos finales de suscripción

“Centro de suscripción” → “Puntos finales de suscripción”; una fila por dirección pública:

| campo | Descripción |
|---|---|
| Dominio | Dominio de suscripción pública |
| Camino | Prefijo bajo el dominio, predeterminado `/` |
| Habilitado | Los puntos finales deshabilitados se ignoran al crear enlaces |
| Descripción | Nota de texto libre |

- **Se admiten varias filas (multidominio)**; cuando se habilitan varios, el dominio del proveedor sigue al dominio utilizado por la solicitud actual (ver 9.4);
- Un puerto de suscripción dedicado sirve en la raíz (`/<token>`, no `/sub`); al compartir el puerto del panel es `/sub/<token>`;
- Los servidores proxy inversos deben reenviar `X-Forwarded-Proto` / `X-Forwarded-Host`; de lo contrario, se utiliza el primer punto final habilitado. Las plantillas de sitio Caddy / Nginx generadas por el panel incluyen ambos encabezados; las configuraciones de proxy mantenidas manualmente necesitan agregarse manualmente.

### 9.3 Enlaces de suscripción

Forma base: `https://<sub-domain>/<token>` (puerto de panel: `https://<panel-domain>/sub/<token>`)

| Parámetro | Descripción |
|---|---|
| `format` | Formato de salida; alias: `clash` → mihomo_yaml, `singbox`/`sing-box` → singbox_json, `xray` → xray_json, `raw` → raw_uri, `base64`/`v2ray` → base64_uri |
| `config` | Aplicar un formato de suscripción por nombre; debe coincidir con `format`; de lo contrario, se ignora |
| `platform` | Haga coincidir una configuración por plataforma (cuando se omite `config`) |
| `providers` | `1` / `true`: Clash Modo proveedor (`proxy-providers` + `use`); nodos en línea predeterminados |

Ejemplos:

```bash
# Clash dynamic subscription (config template + providers)
https://sub.example.com/<token>?format=clash&config=out&providers=1

# sing-box / Xray (only meaningful when such a config exists)
https://sub.example.com/<token>?format=singbox_json&config=sing-box-home
https://sub.example.com/<token>?format=xray_json&config=xray-home

# generic clients / plain text
https://sub.example.com/<token>?format=base64_uri
https://sub.example.com/<token>?format=raw_uri
```

### 9.4 Modo multidominio y proveedor

Con `providers=1`, cada entrada de `proxy-providers` apunta al punto final de la suscripción. Selección de dominio:

1. **El dominio que utilizó esta solicitud** (primer valor `X-Forwarded-Host`, puerto eliminado; de lo contrario, `Host`) si coincide con un punto final **habilitado**;
2. De lo contrario, el punto final habilitado con el **ID más bajo**;
3. El camino proviene de ese punto final: `https://<domain><path>/<token>?format=mihomo_proxies_yaml...`.

Por lo tanto, cambiar el dominio no requiere ningún cambio de panel: los clientes que obtienen a través del nuevo dominio obtienen el nuevo dominio dentro de los proveedores, y un dominio antiguo que se deja habilitado ya no está codificado. Si un cliente almacenó en caché el antiguo proveedor URL, actualice la suscripción (o borre la caché del proveedor/reinicie el cliente).

Notas adicionales:

- Se ignora `proxy-providers` dentro de un formato compartido (`config_json`) (evita fugas entre planes); los proveedores provienen de grupos de nodos del plan y fuentes externas;
- Las rutas de caché del proveedor interno y externo siempre se derivan de “valores predeterminados del proveedor interno → plantilla de ruta de caché” (predeterminado `./providers/{key}.yaml`); las fuentes externas no pueden fijar las suyas propias;
- El proveedor reservado heredado `subscription-nodes` ya no se emite: los grupos que utilizan "todos los nodos" se expanden a proveedores concretos o nodos en línea cuando el plan no tiene ninguno.

---

## 10. Gestión de clientes

La gestión de clientes se ocupa de los usuarios finales.

### 10.1 Agregar un cliente

1. Abra "Gestión de clientes";
2. Haga clic en "Agregar cliente";
3. Complete:
   - Correo electrónico
   - Contraseña
   - Planificar
   - Cuota de tráfico
   - tiempo de caducidad
4. Ahorrar.

### 10.2 Acciones del cliente

- **Editar**: cambiar los datos del cliente
- **Suspender**: suspender el servicio
- **Reanudar**: restaurar un cliente suspendido
- **Restablecer tráfico**: restablecer el tráfico utilizado
- **Restablecer contraseña**: restablecer la contraseña del cliente
- **Renovar**: ampliar la fecha de caducidad
- **Actualizar/bajar de categoría**: cambiar el plan
- **Eliminar**: eliminar el cliente

### 10.3 Detalles de la suscripción

Gestión de clientes → columna “Suscripción” → “Detalles”. El cuadro de diálogo enumera enlaces para cada formato disponible para el plan del cliente:

- **Clash / Mihomo**: dos filas: nodos en línea y `providers=1` (proveedores de proxy), ambos con `config=<name>`;
- **Sing-box / Xray**: se muestra solo cuando existe una configuración de ese formato y es visible para el plan; los enlaces llevan `config=<name>`;
- **Base64 URI / Raw URI**: siempre se muestra (no `config`);
- Cada fila tiene su propio botón de copiar; Al hacer clic en una fila, el código QR siguiente cambia a ese formato.

Entonces, con solo una configuración Clash, el cuadro de diálogo muestra Clash + Base64 + Raw; Los enlaces sing-box / Xray aparecen solo después de crear sus configuraciones.

El mismo cuadro de diálogo también muestra el uso del tráfico, el historial de pedidos y el historial de transacciones.

---

## 11. Gestión de planos

La gestión de planes configura los planes que vende.

### 11.1 Plantillas de planes

1. Abra “Gestión de planes” → “Plantillas de planes”;
2. Añade una plantilla:
   - Nombre de la plantilla
   - Categoría (estándar / premium / empresarial / prueba)
   - Renovación automática
   - Configuración de prueba
3. Guardar.

### 11.2 Variantes del plan

1. Agregue una variante debajo de la plantilla:
   - Nombre de la variante (por ejemplo, mensual, anual)
   - Precio (en centavos)
   - Duración (días)
   - Tráfico (GB, `0` = ilimitado)
   - Límite de concurrencia
   - Límite de velocidad (Mbps)
2. Guardar.

### 11.3 Complementos de tráfico

1. Abra "Complementos de tráfico";
2. Agregar un complemento:
   - Nombre
   - Precio
   - Tráfico
   - Modo de vencimiento (extender días/igual que el plan)
3. Guardar.

### 11.4 Cupones

1. Abra "Cupones";
2. Añadir un cupón:
   - Código
   - Tipo (porcentaje / monto fijo)
   - Valor de descuento
   - Límite de uso
   - Periodo de validez
   - Planes aplicables
3. Guardar.

---

## 12. Gestión de recargas

La gestión de recargas maneja sitios de recarga, pedidos y tarjetas de recarga.

> **Integración de pagos**
>
> El panel **no se integra directamente con instituciones de pago** (Alipay, WeChat Pay, Stripe, bancos, etc.). Solo se conecta a una **plataforma de pago intermedia**, que a su vez se integra con la institución de pago correspondiente, maneja la contratación, adquisición y liquidación, e informa los resultados del pago a través de la devolución de llamada de cargo [ API](api/charge-callback.md).
>
> La plataforma de pago intermedio **no forma parte de este proyecto; los operadores deben desarrollarlo ellos mismos (o utilizar su propio sistema)**. Una vez implementado, regístrelo en "Sitios de recarga" y complete la verificación del dominio para usarlo como punto de entrada de recarga; El sitio de recarga aquí es esa plataforma intermedia, no la institución de pago en sí. Las calificaciones de pago, las tarifas, la liquidación y el cumplimiento son acordados y a cargo del operador, la institución de pago y la plataforma intermedia.

### 12.1 Sitios de recarga

1. Abra “Gestión de recargas” → “Sitios de recarga”;
2. Agregar un sitio:
   - Nombre del sitio
   - Métodos de pago soportados por esa plataforma intermedia (Alipay, WeChat, USDT, Stripe, etc., proporcionados a través de su integración con instituciones de pago)
   - Moneda
   - tipo de cambio
3. Guardar;
4. Verifique el sitio (archivo HTTP o verificación DNS).

### 12.2 Órdenes de recarga

- Ver todos los pedidos de recarga
- Filtrar por estado (pendiente, pagado, fallido, caducado, reembolsado)
- Ver detalles del pedido
- Emitir reembolsos de administrador

### 12.3 Tarjetas de recarga

1. Abra “Tarjetas de recarga”;
2. Añadir una tarjeta:
   - Tipo (monto, plan, complemento de tráfico)
   - Valor nominal / plan
   - tiempo de caducidad
3. O generar de forma masiva:
   - Cantidad (hasta 1000)
   - Prefijo del número de tarjeta
   - Formato del número de tarjeta
4. Exportar como CSV.

### 12.4 Gestión del saldo

- Ver saldos de clientes
- Ajustar saldos (aumentar/disminuir/establecer)
- Ajustar cantidades congeladas
- Ver historial de transacciones

### 12.5 Revendedores

- Agregar revendedores
- Establecer tasas de comisión
- Ver comisión acumulada
- Retirar comisión

---

## 13. Gestión de billetes

La gestión de tickets maneja las solicitudes de atención al cliente.

### 13.1 Lista de entradas

- Ver todas las entradas
- Filtrar por estado (abierto, respondido, cerrado)
- Filtrar por categoría (técnica, facturación, cuenta, otra)
- Filtrar por prioridad (baja, normal, alta, urgente)

### 13.2 Acciones de tickets

- **Ver/responder**: abre un ticket y responde
- **Asignar**: asigna un ticket a un administrador
- **Cerrar**: cerrar un ticket resuelto

---

## 14. Gestión de certificados

La gestión de certificados maneja los certificados TLS.

### 14.1 Emisión de un certificado

1. Abra "Gestión de certificados";
2. Haga clic en "Emitir certificado";
3. Complete:
   - Lista de dominios (uno por línea)
   - Tipo de certificado (dominio/IP)
   - Tipo de clave (ECC/RSA)
   - Método de validación (DNS-01 / HTTP-01)
   - Proveedor DNS
   - Ubicación de emisión (host del panel/nodo)
4. Haga clic en "Problema".

### 14.2 Acciones de certificado

- **Renovar**: renovar manualmente
- **Implementar**: implementar en un nodo
- **Distribuir**: enviar a todos los nodos asociados
- **Migrar**: mueve referencias de un certificado antiguo a uno nuevo
- **Descargar**: descargar `cert` / `fullchain` / `privkey`

### 14.3 Carga de un certificado

1. Haga clic en "Cargar certificado";
2. Elige el método:
   - Pega el contenido
   - Subir archivos
3. Cargue la cadena de certificados y la clave privada;
4. Ahorrar.

### 14.4 Configuración EAB

- Administre las credenciales de EAB para la CA pública Google/ZeroSSL
- Obtenga EAB automáticamente
- Adjuntarlo automáticamente al emitir

---

## 15. Cuotas de tráfico

Las cuotas de tráfico monitorean el tráfico mensual de cada nodo.

### 15.1 Lista de cuotas

- Ver la configuración de cuotas para todos los nodos
- Mostrar el tamaño de la cuota, el tráfico utilizado y la asignación restante
- Mostrar porcentaje de uso

### 15.2 Uso del ciclo

- Ver detalles de uso del ciclo por nodo
- Mostrar tráfico entrante/saliente
- Mostrar hora de inicio/finalización del ciclo

### 15.3 Restablecer registro

- Ver registros de restablecimiento de tráfico
- Muestra el ciclo de reinicio, la fuente de activación y el operador.

---

## 16. proxies encadenados

Los proxies encadenados configuran enlaces de múltiples salidas.

### 16.1 Creando un enlace

1. Abra “proxies encadenados”;
2. Haga clic en "Nuevo enlace";
3. Seleccione el nodo de entrada;
4. Seleccione la entrada entrante;
5. Seleccionar nodos de salida (se permiten varios);
6. Seleccione una entrada para cada salida;
7. Establecer prioridad/peso;
8. Confirmar.

### 16.2 Gestión de enlaces

- Ver la descripción general de la topología
- Editar un enlace (prioridad, peso, etiqueta saliente)
- Eliminar un enlace
- comprobar el estado de salud

### 16.3 Nodos externos

- Importar nodos externos (v2ray URI, Base64 URI, suscripción URL)
- Registrar entradas externas
- Administrar nodos externos (editar/eliminar individualmente)
- Dominio de acceso: configurar aceleración CDN/IP preferida por nodo (dominio directo, preferido CF)
 dominio, grupo de IP de CF, personalizado). Sólo se reescribe el servidor/puerto entregado en las suscripciones;
 SNI/Estancia del anfitrión importada. Cuando está deshabilitado, se utiliza la dirección/puerto importados.
- Cuando se selecciona "Dominio preferido de CF/grupo de IP de CF" sin un valor específico, se elige una entrada
 aleatoriamente del pool (con su puerto); "directo/personalizado" requiere un valor explícito; de lo contrario
 vuelve a caer a la propia dirección del nodo

---

## 17. Usuarios en línea

Los usuarios en línea muestran el estado en línea en tiempo real.

### 17.1 Qué muestra

- Suscripciones en línea
- Dispositivos en línea/IP
- Nodo, IP y tiempo de conexión para cada suscripción
- Tasa de enlace ascendente/descendente en tiempo real
- Enlace ascendente total / enlace descendente total / tráfico total

> El enlace ascendente total, el enlace descendente total y el tráfico total se acumulan durante una **sesión en línea continua**:
> Las reconexiones de corta duración, los reinicios del núcleo y los reinicios de contadores no los borran. Sólo después de los 60
> segundos sin un nuevo informe, una nueva sesión comienza desde cero. Esta es una medida diferente.
> del propio contador de "tráfico desde este inicio" del núcleo y refleja mejor lo que un usuario realmente
> utilizado durante la sesión actual.

### 17.2 Filtrado

- Filtrar por nodo
- Búsqueda por correo electrónico/cliente/nodo/IP

---

## 18. Gestión de usuarios

La gestión de usuarios maneja los usuarios del panel.

### 18.1 Funciones

| Rol | Descripción |
|------|-------------|
| Superadministrador (`superAdmin`) | Privilegios más altos |
| Administrador (`admin`) | Privilegios de gestión |
| Operador (`operator`) | Privilegios operativos |
| Visor (`viewer`) | Sólo lectura |

### 18.2 Agregar un usuario

1. Abra "Gestión de usuarios";
2. Haga clic en "Agregar usuario";
3. Complete:
   - Nombre de usuario
   - Correo electrónico
   - Contraseña
   - Rol
4. Guardar.

### 18.3 Acciones del usuario

- **Editar**: cambiar los detalles del usuario
- **Eliminar**: eliminar el usuario

---

## 19. sitio del portal

El sitio del portal configura el portal del cliente.

### 19.1 Generalidades

- Nombre del sitio
- Descripción del sitio
- Inscripción abierta
- Camino del portal
- Puerto del portal

### 19.2 Apariencia

- Título del sitio
- Logotipo URL
- Colores primarios, de fondo, de texto y de acento
- fuente

### 19.3 Navegación

- Agregar páginas de navegación
- Configurar el nombre de la página, el icono y el enlace externo
- Administrar elementos de navegación

### 19.4 Módulos

- Agregar/editar módulos
- Configurar el título, la descripción y el icono del módulo
- Admite Markdown, texto enriquecido y HTML personalizado

### 19.5 idiomas

- Agregar idiomas
- Administrar copia
- Importar/exportar JSON

### 19.6 Plantillas

- Cambiar plantillas
- Personalizar HTML/CSS/JS
- Restablecer la plantilla predeterminada

---

## 20. Configuración del sistema

La configuración del sistema configura los parámetros del panel.

### Panel 20.1

- Escuchar dirección y puerto
- Directorio de datos
- Zona horaria
- Puerto de suscripción
- Límite de exportación
- Inscripción abierta
- Modo panel (completo/solo administración)

### 20.2 Seguridad

- Gestión de claves de cifrado
- Gestión de claves JWT
- lista negra de IP
- Passkey dominios incluidos en la lista permitida

### 20.3 OAuth

- GitHub OAuth
- Google OAuth
- Configuración de devolución de llamada URL

### 20.4 Notificaciones

- Notificaciones por correo electrónico (SMTP)
- Notificaciones Telegram
- Plantillas de notificación
- Configuración de monitoreo

### 20.5 Copia de seguridad y restauración

- Crear una copia de seguridad
- Lista de respaldo
- Descargar una copia de seguridad
- Restaurar una copia de seguridad
- Copias de seguridad programadas
- Almacenamiento remoto (FTP / S3 / WebDAV)

### 20.6 Desbloqueo automático

- Habilitar el desbloqueo automático
- Enfriamiento
- Estados aplicables

---

## 21. Consola de operaciones

La consola de operaciones administra instantáneas de configuración y aplica trabajos.

### 21.1 Instantáneas de configuración

- Crear una instantánea
- Listar instantáneas
- Retroceder a una instantánea
- Eliminar una instantánea

### 21.2 Aplicar trabajos

- Ver trabajos recientes
- Aplicar la configuración ahora (DB → tiempo de ejecución)
- Verificación previa (validar conflictos de puertos y configuración JSON)
- Cancelar un trabajo
- Reintentar un trabajo

### 21.3 Validación de nodo

- Seleccione un nodo y valide
- Ver el resultado de la validación
- Validar todos los nodos

### 21.4 Parada de emergencia

- Detenga de emergencia el proxy (escribe una configuración entrante vacía)
- Detenga inmediatamente todo el tráfico entrante

---

## 22. Monitoreo de base de datos

El monitoreo de la base de datos muestra el estado de la base de datos.

### 22.1 Qué muestra

- motor de base de datos
- Tamaño de la base de datos
- Conectividad
- Latencia
- Estado del grupo de conexiones
- Estadísticas de caché
- Estadísticas de consultas

### 22.2 Motores compatibles

- SQLite
- MySQL
- PostgreSQL

---

## 23. Registro de auditoría

El registro de auditoría registra las operaciones.

### 23.1 Qué muestra

- Categoría
- Tipo de evento
- Resultado
- IP de origen
- Usuario
- Detalles

### 23.2 Registro de operación del nodo

- Ver registros de operación de nodos
- Filtrar por nodo

---

## 24. Gestión de registros

La gestión de registros muestra los registros del sistema.

### 24.1 Tipos de registros

- Registros de paneles
- Registros de nodo
- Registros centrales
- Registros de proxy

### 24.2 Acciones

- Ver registros
- Descargar registros
- Registros limpios

---

## 25. Extensiones de operaciones

“Extensiones de operaciones” (`/client-ext`) configura marketing de referencia, comisiones y retiros;
“Documentos legales” (`/legal-documents`) mantiene los acuerdos del lado del portal.

### 25.1 Referencias y comisión

- **Configuración de reembolso**: tasa de reembolso (porcentaje del primer pago del invitado, `0` = deshabilitado), modo de comisión (solo la primera compra/todas las compras), monto mínimo de retiro (el saldo de comisión debe alcanzarlo antes de que se pueda solicitar un retiro)
- **Códigos de invitación**: cuando está habilitado, el registro requiere un código de invitación; establezca la longitud del código, los días de vencimiento y los usos máximos (`0` = ilimitado) y genere/vea la lista de códigos
- **Revisión de retiro**: filtrar por pendiente/aprobado/rechazado; agregue una nota de procesamiento y registre el método de pago y la dirección de la billetera
- **Registros de referencia**: invitante, invitado, monto de la comisión y estado de la comisión

### 25.2 Documentos legales

- Mantenga la “Política de privacidad” y los “Términos de servicio” por separado, un documento de Markdown por ubicación, con vista previa en vivo;
- Cada documento puede utilizar un URL externo en lugar de la página incorporada; cuando está vacía, se utiliza la página incorporada URL (que se puede copiar en la página);
- El portal muestra la versión de la configuración regional actual y recurre a la configuración regional predeterminada cuando falta.

---

## 26. Preguntas frecuentes

### 26.1 El panel no arranca

- Compruebe si el puerto ya está en uso
- Compruebe que `.env` sea válido
- Verifique los registros: `journalctl -u corefusion -f`; sin systemd, `tail -f <install-dir>/data/logs/panel.log`

### 26.2 Nodo fuera de línea

- Verifique que el Agente se esté ejecutando: `systemctl status corefusion-agent`
- Verificar la conectividad de la red
- Verifique las reglas del firewall (puerto `18443`)

### 26.3 Falla la emisión del certificado

- Verifique la resolución del DNS
- Compruebe que el puerto 80 sea accesible
- Verifique la configuración del proveedor DNS
- Verifique los registros de certificados

### 26.4 La suscripción no funciona

- Consulta el enlace de suscripción.
- Comprueba que el nodo está en línea.
- Compruebe que el proceso principal se esté ejecutando
- Verifique las reglas del firewall

### 26.5 La devolución de llamada de recarga falla

- Verifique la devolución de llamada URL
- Compruebe que la clave del sitio coincida
- Verifique la lista de IP permitidas
- Consulta los registros de pedidos

### 26.6 La página del portal es inaccesible

- Compruebe que el puerto del portal esté abierto
- Verifique la configuración de la ruta del portal
- **Importante**: asegúrese de que el proxy inverso reenvíe el dominio del portal al **puerto del motor del portal**, no al puerto del panel principal; el reenvío al puerto incorrecto muestra una página en blanco o el panel en lugar del portal. Marque `upstream` / `local_port` en el registro del sitio proxy.
- Abra `http://<portal-domain>:<portal-port>` directamente para verificar el motor del portal.
- Verifique la configuración del proxy inverso
- Consulta los registros del portal

---

## Apéndice

### A. Puertos predeterminados

| Puerto | Propósito |
|------|---------|
| Asignado por el asistente de configuración (puerto libre aleatorio por defecto) | Panel de administración (cambiable) |
| 18443 | Agente gRPC (cambiable) |
| 0 (deshabilitado por defecto) | Puerto de motor de suscripción dedicado (vacío o `0` comparte el puerto principal) |
| 0 (deshabilitado por defecto) | Puerto de motor de portal dedicado (vacío o `0` comparte el puerto principal) |
| 80 / 443 | Proxy inverso (Caddy / Nginx) HTTP / HTTPS |

### B. Rutas predeterminadas

| Camino | Propósito |
|------|---------|
| `/` | Panel de administración (el portal también reside en `/` cuando no se configura ningún motor de portal dedicado) |
| `/sub/:token` | Recuperación de suscripción (cuando no se configura ningún puerto de suscripción dedicado) |
| `/p` | Página del portal (cuando no se configura ningún puerto de portal dedicado) |
| `/:token` | Recuperación de suscripción (con un puerto de suscripción dedicado, sin el prefijo `/sub`) |

### C. Herramienta de línea de comandos

El binario del panel admite estos subcomandos (sin argumentos se inicia el servidor):

```bash
# Start the panel (same as ./corefusion server)
./corefusion

# Run database migrations
./corefusion migrate

# Non-interactive init / reset init state (see 3.4, 3.5)
./corefusion init --help
./corefusion reset-init

# Offline administration (when you cannot sign in)
./corefusion admin reset-password -u admin -p 'new-password'
./corefusion admin 2fa reset -u admin
./corefusion admin list

# Version / help
./corefusion version
./corefusion help
```

> Con systemd, utilice `systemctl restart corefusion` y `journalctl -u corefusion -f`; sin
> systemd (contenedores y similares), detenga el proceso anterior y reinícielo con `nohup`, los registros son
> en `data/logs/panel.log`.

---

## Cambios en v0.13.0-rc.1

- **Actualización en línea del panel**: «Configuración del sistema → Actualización del panel»:
  selección de versión (`pre` = preliminar), actualización, historial de tareas y reversión al
  binario anterior.
- **Actualización en línea del Agent**: en los detalles del nodo, «Actualizar Agent» desde el
  binario local del panel o desde GitHub Releases; los nodos actualizados desde versiones
  antiguas reciben `update.public_key` (0600) al conectar y validan firmas ed25519.
- **Sincronización WAF**: la última sincronización se guarda en
  `waf.github_source.last_sync`; reiniciar no reinicia el temporizador; `0 3 * * *` se ejecuta a
  las 03:00 en la zona `panel.timezone` (Asia/Shanghai por defecto); las ventanas perdidas se
  recuperan ~15 s después del arranque.
- **Lista blanca WAF**: el dominio del panel mantiene `whiteurl.rule` automáticamente; las
  comprobaciones de URL/parámetros/POST solo se omiten para la API de administración, así que
  guardar un sitio de pago con `http://127.0.0.1:18081` ya no se bloquea por SSRF.
- **Política de cuota**: acumular al renovar / borrar al caducar / restablecer al renovar (la
  variante puede heredar el plan); con «mensual» el panel revisa cada hora y pone el uso a cero.
- **Sitios de pago**: activar/desactivar ya no modifica claves ni métodos de pago; los pedidos
  de recarga se filtran por tipo y se exportan a CSV.
- **Portal**: «Recargar» admite pago en línea con un sitio activo y canje de tarjetas; sin
  sitios activos solo se muestra el canje.

---

> **Versión del documento**: v1.2
> **Última actualización**: 2026-11-20
> **Se aplica a**: CoreFusion v0.13.0-rc.1
