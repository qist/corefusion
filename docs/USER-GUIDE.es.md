<!-- source: USER-GUIDE.md | locale: es | updated: 2026-10-26 -->
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

CoreFusion es un panel de administración de proxy multiprotocolo que admite VLESS, VMess, Trojan, Shadowsocks, Hysteria2, TUIC y más, con administración de nodos, entrega de suscripciones, administración de clientes, facturación, certificados y proxies encadenados.

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
- **Proxy**: Proxy inverso Caddy / Nginx
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

El instalador incluye cuatro idiomas de interfaz: chino simplificado, inglés, persa (فارسی) y ruso (Русский). Detecta automáticamente `LANG` del sistema o puedes elegir explícitamente:

```bash
# Inglés
sudo bash install.sh --lang en

# Persa mediante variable de entorno
sudo CF_LANG=fa bash install.sh

# Pasar opciones en una instalación por tubería
curl -fsSL https://raw.githubusercontent.com/qist/corefusion/main/scripts/install.sh | sudo bash -s -- --lang ru
```

Valores admitidos: `zh` / `en` / `fa` / `ru`; también se acepta `--lang=ru`. Un valor explícito no admitido falla mostrando la lista de idiomas disponibles.

### 2.3 Fijación de versiones/espejos/instalación sin conexión

```bash
# Pin a version
sudo bash install.sh --version v0.13.0-alpha.2

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
sudo bash install.sh --source --version v0.13.0-alpha.2
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
| `-v, --version <TAG>` | Versión de panel, p. `v0.13.0-alpha.2`; predeterminado `latest` |
| `--binary` | Utilice el binario de versión GitHub (predeterminado) |
| `--source` | Compile desde el código fuente y prepare la coincidencia Go/Node |
| `--local <FILE>` | Utilice un binario local (instalación sin conexión) |
| `--arch <ARCH>` | Arquitectura de destino: `amd64` / `arm64` / `armv7` / `armv6` / `386` / `ppc64le` / `s390x` / `riscv64` / `loong64` |
| `--mirror <URL>` | Prefijo de descarga GitHub, p.e. `https://ghproxy.net` |
| `--repo <URL>` | Repositorio de origen, predeterminado `https://github.com/qist/corefusion.git` |
| `--ref <REF>` | Rama/etiqueta de origen; El valor predeterminado es `--version`; de lo contrario, `main`. |
| `--no-deps` | Omitir instalación de dependencia del sistema |
| `--no-start` | Instale y escriba el archivo de servicio sin iniciar |
| `--cpu-info` | Detección del entorno de impresión y dependencias faltantes, luego salga |
| `--lang <LANG>` | Idioma de salida: `zh` / `en` / `fa` / `ru` (por defecto: `LANG` del sistema) |
| `--uninstall` | Detenga el servicio y elimine el programa (conserva los datos y `.env`) |
| `--purge` | Con `--uninstall`, elimine también el directorio de datos y `.env` |

Variables de entorno relacionadas:

| variable | Propósito |
|----------|---------|
| `CF_GO_VERSION` | Anular la versión de Go requerida para las compilaciones de código fuente |
| `CF_NODE_MAJOR` | Anular la versión principal del nodo (predeterminada 22) |
| `CF_GO_MIRROR` | Ir al espejo de descarga de la cadena de herramientas |
| `CF_SKIP_DEPS` | Establezca en `1` en el script de instalación del agente para omitir la instalación de dependencia. |
| `CF_LANG` | Sobrescribir el idioma de salida del instalador (igual que `--lang`) |

### 2.6 Diseño del directorio

```text
/opt/corefusion/
├── corefusion                 # panel binary
├── corefusion.bak             # previous version (when upgrading)
├── .env                       # environment file (0600, generated by the setup wizard)
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

En una instalación nueva, el instalador escribe solo este marcador de posición, para que el panel no confunda el `data/` existente con una instalación inicializada y omita el asistente:

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

> Los servidores proxy inversos confiables para resolver IP de clientes reales ya no se configuran mediante variables de entorno;
> se almacenan como `panel.trusted_proxies` en **Configuración → Red**.

> `.env` contiene secretos; el panel refuerza sus permisos a `0600` al inicio. Nunca lo cometas.

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
| Ruta web | Ruta de la interfaz de usuario del administrador, predeterminada `/`; el API es siempre `/api/v1` |
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
- **Desafío**: `HTTP-01` (requiere puerto 80) o `DNS-01` (comodines, sin puerto 80; elija un proveedor de DNS e ingrese las credenciales);
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
- Inicie sesión con la cuenta de administrador, acceda a URL y la ruta web desde el resumen;
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

1. Después de agregar el nodo, haga clic en "Comando Bootstrap";
2. Copie el comando de instalación;
3. Ejecútelo como root en el nodo remoto:

   ```bash
   curl -fsSL https://your-panel/install.sh | bash
   ```

4. El Agente descarga, se registra y se inicia automáticamente.

### 5.4 Detalles del nodo

Haga clic en un nodo para abrir su página de detalles:

- **Descripción general**: información básica, estado del agente, estado del certificado
- **Centro de protocolo**: configuración del proceso central
- **Proxy**: configuración Caddy/Nginx
- **Procesos principales**: iniciar/detener/reiniciar
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
| Xray-núcleo | VLESS / VMess / Trojan / Shadowsocks y más |
| sing-box | Plataforma proxy universal modular con soporte TUN; lleva Hysteria2 / TUIC y otros protocolos QUIC |

### 6.2 Módulos de configuración

- **Registro**: registro de acceso, registro de errores, modo de salida
- **API**: dirección de escucha de gRPC API, lista de servicios
- **DNS**: servidores DNS, estrategia de consulta, hosts estáticos
- **Enrutamiento**: reglas de enrutamiento, equilibrio de carga, enrutamiento VLESS
- **Entrante**: lista de entrada, configuración de protocolo, transporte, TLS
- **Salidas**: lista de salida, configuración de protocolo, transporte
- **Transporte**: WebSocket, gRPC, XHTTP, TUN, etc.
- **TLS**: certificados, ALPN, huella digital uTLS, ECH
- **Reality**: gestión de objetivos, pares de claves, ID cortos
- **Comprobación de estado**: sonda URL, intervalo, simultaneidad
- **FakeDNS**: grupo de IP, tamaño del grupo
- **Métricas**: oyente de métricas de Prometheus
- **Política**: tiempos de espera de conexión, buffers, estadísticas
- **Datos geográficos**: actualización automática, archivos de recursos
- **Límites de versión**: versión mínima/máxima

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

---

## 7. Gestión de proxy

La gestión de proxy configura el proxy inverso Caddy o Nginx.

### 7.1 Tipos de proxy

- **Caddy**: automático HTTPS, HTTP/2, HTTP/3
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

---

## 8. Recursos de enrutamiento

Los recursos de enrutamiento administran las reglas de enrutamiento de suscripción.

### 8.1 Características

- Administrar reglas de enrutamiento
- Configurar políticas de enrutamiento
- Admite múltiples formatos de enrutamiento

### 8.2 Uso

1. Abra "Recursos de enrutamiento";
2. Agregue una regla de enrutamiento;
3. Establezca el nombre, el tipo y las condiciones de la regla;
4. Guardar y aplicar.

---

## 9. Centro de suscripción

El centro de suscripciones gestiona enlaces de suscripción y puntos finales.

### 9.1 Formatos de suscripción

1. Abra “Centro de suscripciones” → “Formatos de suscripción”;
2. Añade un formato:
   - Nombre del formato
   - Formato de salida (Base64, YAML, JSON, etc.)
   - Protocolos incluidos
3. Guardar.

### 9.2 Puntos finales de suscripción

1. Abra “Centro de suscripciones” → “Puntos finales de suscripción”;
2. Agregue un punto final:
   - Nombre del punto final
   - Escuchar dirección
   - Camino
   - Formato asociado
3. Guardar.

### 9.3 Enlaces de suscripción

- Obtenga una suscripción en `http://your-domain/sub/:token`
- Utilice parámetros como `?format=yaml` para seleccionar un formato

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
4. Guardar.

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

Haga clic en un cliente para ver:

- Enlace de suscripción
- Uso del tráfico
- Historial de pedidos
- Historial de transacciones

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
2. Ahorrar.

### 11.3 Complementos de tráfico

1. Open “Traffic add-ons”;
2. Agregar un complemento:
   - Nombre
   - Precio
   - Tráfico
   - Expiry mode (extend days / same as plan)
3. Guardar.

### 11.4 Cupones

1. Abra "Cupones";
2. Añadir un cupón:
   - Código
   - Type (percentage / fixed amount)
   - Valor de descuento
   - Límite de uso
   - Periodo de validez
   - Planes aplicables
3. Guardar.

---

## 12. Gestión de recargas

La gestión de recargas maneja sitios de recarga, pedidos y tarjetas de recarga.

### 12.1 Sitios de recarga

1. Abra “Gestión de recargas” → “Sitios de recarga”;
2. Agregar un sitio:
   - Nombre del sitio
   - Métodos de pago admitidos (Alipay, WeChat, USDT, Stripe, etc.)
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
- Obtener EAB automáticamente
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
- Administrar nodos externos

---

## 17. Usuarios en línea

Los usuarios en línea muestran el estado en línea en tiempo real.

### 17.1 Qué muestra

- Suscripciones en línea
- Dispositivos en línea/IP
- Nodos en línea
- Tarifa en tiempo real
- Tráfico total

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
- Configurar el título, la descripción y el ícono del módulo
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

Las extensiones de operaciones configuran extensiones opcionales.

### 25.1 Características

- Extensiones personalizadas
- Configuración de extensión
- Gestión de extensiones

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

# Offline administration (when you cannot sign in)
./corefusion admin reset-password -u admin -p 'new-password'
./corefusion admin 2fa reset -u admin
./corefusion admin list

# Version / help
./corefusion version
./corefusion help
```

> Sin una unidad systemd, reinicie deteniendo el proceso anterior e iniciándolo nuevamente con `nohup`; Los registros están en `<install-dir>/data/logs/panel.log`.

---

> **Versión del documento**: v1.0
> **Última actualización**: 2026-10-23
> **Se aplica a**: CoreFusion v1.0+
