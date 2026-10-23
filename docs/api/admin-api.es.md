<!-- source: admin-api.md | locale: es | updated: 2026-10-23 -->
# Administrador API

> Idiomas: [简体中文](admin-api.md) · [English](admin-api.en.md) · [繁體中文](admin-api.zh-TW.md) · [Русский](admin-api.ru.md) · [日本語](admin-api.ja.md) · [한국어](admin-api.ko.md) · [Deutsch](admin-api.de.md) · [Français](admin-api.fr.md) · Español · [Português](admin-api.pt.md) · [Türkçe](admin-api.tr.md) · [Bahasa Indonesia](admin-api.id.md) · [ไทย](admin-api.th.md) · [Tiếng Việt](admin-api.vi.md) · [العربية](admin-api.ar.md) · [فارسی](admin-api.fa.md)

La tabla de rutas de administración completa API para **desarrollo de paneles/automatización/integradores**.

- Base URL: `https://<panel-domain>/api/v1`
- Autenticación: `Authorization: Bearer <admin_token>` (de `POST /api/v1/auth/login`)
- Formatos de respuesta/error comunes y convenciones de límite de velocidad: consulte [README.md](./README.es.md)
- Fuente de la verdad: `internal/api/router/router.go` (739 rutas de administración); si esta tabla no está de acuerdo, **el código gana**

## Marcadores de permiso

| marcador | Significado |
|---|---|
| Público | No es necesario iniciar sesión (es posible que aún se apliquen guardias de mantenimiento o limitaciones de tarifas) |
| Público · tarifa limitada | Sin inicio de sesión, con un límite de tarifa dedicado |
| Iniciado sesión | Requiere un token de administrador registrado (`AuthRequired`) |
| administrador | Requiere rol ≥ `admin` (`RequireRole(RoleAdmin)`) |
| superadministrador | Requiere rol = `super_admin` |

Niveles de rol: `viewer(10) < operator(20) < admin(30) < super_admin(40)`; Los roles insuficientes devuelven 403 `INSUFFICIENT_ROLE`.

---

## Inicio rápido (ejemplos reales)

Estos ejemplos provienen de respuestas locales reales; los tokens y los datos están redactados.

### 1. Inicia sesión y obtén un token

```bash
curl -X POST https://<panel-domain>/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"your-password"}'
```

Respuesta:

```json
{
  "data": {
    "access_token": "<JWT, valid 24h>",
    "refresh_token": "<JWT, valid 7 days>",
    "user": { "id": 1, "role": "super_admin", "username": "admin" }
  }
}
```

### 2. Llame a un punto final empresarial con el token

```bash
curl https://<panel-domain>/api/v1/clients?limit=1 \
  -H 'Authorization: Bearer <access_token>'
```

Respuesta (las listas utilizan la estructura `items` + `total`):

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

### 3. Ejemplos de respuestas de error

**401 no autenticado:**

```json
{
  "code": "MISSING_TOKEN",
  "message": "missing authorization header",
  "request_id": "a02cfdf9-6be0-46be-8584-824caab34dc7",
  "can_retry": false,
  "requires_action": false
}
```

**Recurso 404 no encontrado:**

```json
{
  "code": "CLIENT_NOT_FOUND",
  "message": "client not found",
  "request_id": "099949bd-4ded-468d-a6f8-f8e798af2273",
  "can_retry": false,
  "requires_action": false
}
```

### 4. Paginación

Todos los puntos finales de la lista usan `limit` (tamaño de página, predeterminado 20) y `offset` (basado en 0) y devuelven `{items, total, limit, offset}`. Algunos puntos finales heredados devuelven una matriz básica (anotada por tabla).

---

## Contenidos

- [Autenticación y seguridad del administrador](#autenticación-y-seguridad-del-administrador)
- [cuentas de administrador](#cuentas-de-administrador)
- [Gestión de clientes](#gestión-de-clientes)
- [Plantillas de planos](#plantillas-de-planos)
- [Variantes de planes](#variantes-de-planes)
- [Paquetes de tráfico](#paquetes-de-tráfico)
- [Cupones](#cupones)
- [Grupos de nodos](#grupos-de-nodos)
- [Servidores de nodo](#servidores-de-nodo)
- [Registro de auditoría de nodo](#registro-de-auditoría-de-nodo)
- [Nodos (modelo heredado, retirado)](#nodos-modelo-heredado-retirado)
- [Descripción general de la región](#descripción-general-de-la-región)
- [Suscripciones](#suscripciones)
- [Formatos de suscripción](#formatos-de-suscripción)
- [Puntos finales de suscripción](#puntos-finales-de-suscripción)
- [Compartir dominios](#compartir-dominios)
- [Cloudflare IP](#cloudflare-ip)
- [Configuración de suscripción (Mihomo/Clash)](#configuración-de-suscripción-mihomoclash)
- [Sitios de recarga](#sitios-de-recarga)
- [Órdenes de recarga](#órdenes-de-recarga)
- [Tarjetas de recarga](#tarjetas-de-recarga)
- [Cobrar eventos de devolución de llamada](#cobrar-eventos-de-devolución-de-llamada)
- [Cargar reintentos de devolución de llamada](#cargar-reintentos-de-devolución-de-llamada)
- [Saldo del cliente](#saldo-del-cliente)
- [Regalos](#regalos)
- [Configuración de referencia](#configuración-de-referencia)
- [Retiros de referencias](#retiros-de-referencias)
- [Listas de IP permitidas (resumen)](#listas-de-ip-permitidas-resumen)
- [Enlaces de OAuth del cliente](#enlaces-de-oauth-del-cliente)
- [Entradas (administrador)](#entradas-administrador)
- [Artículos](#artículos)
- [Cuentas compartidas](#cuentas-compartidas)
- [Certificados](#certificados)
- [Canales de certificación](#canales-de-certificación)
- [Configuración EAB](#configuración-eab)
- [Cuentas ACME](#cuentas-acme)
- [CA](#built-in-ca) incorporado
- [CA incorporada](#ca-incorporada)
- [Reglas geográficas](#reglas-geográficas)
- [Biblioteca de reglas WAF](#biblioteca-de-reglas-waf)
- [Gestión de registros](#gestión-de-registros)
- [Copias de seguridad](#copias-de-seguridad)
- [Configuración del sistema](#configuración-del-sistema)
- [Auditoría de seguridad](#auditoría-de-seguridad)
- [Métricas](#métricas)
- [Plantillas de portales](#plantillas-de-portales)
- [Instalación del agente](#instalación-del-agente)
- [Entrada de devolución de llamada de cargo](#entrada-de-devolución-de-llamada-de-cargo)
- [Entradas (con nodos)](#entradas-con-nodos)
- [Configuración pública](#configuración-pública)
- [Autenticación de cliente y negocio (la misma configuración en el motor de administración)](#autenticación-de-cliente-y-negocio-la-misma-configuración-en-el-motor-de-administración)
- [Entrega de suscripción](#entrega-de-suscripción)
- [Flujo de trabajo de configuración en tiempo de ejecución](#flujo-de-trabajo-de-configuración-en-tiempo-de-ejecución)
- [Políticas de acceso al servicio (retiradas)](#políticas-de-acceso-al-servicio-retiradas)
- [Monitoreo de cuota de tráfico](#monitoreo-de-cuota-de-tráfico)
- [Controles de salud](#controles-de-salud)
- [página del portal](#página-del-portal)

---

### Autenticación y seguridad del administrador

Inicio de sesión, cierre de sesión, cambio de contraseña, 2FA (TOTP), Passkey (WebAuthn) y tickets únicos para el panel de administración.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| POST | `/api/v1/auth/2fa/disable` | Iniciado sesión | Desactivar |
| POST | `/api/v1/auth/2fa/enable` | Iniciado sesión | Habilitar |
| POST | `/api/v1/auth/2fa/recovery-codes` | Iniciado sesión | Regenerar códigos de recuperación |
| POST | `/api/v1/auth/2fa/setup` | Iniciado sesión | Configuración |
| GET | `/api/v1/auth/2fa/status` | Iniciado sesión | Estado |
| POST | `/api/v1/auth/change-password` | Iniciado sesión | Cambiar contraseña |
| POST | `/api/v1/auth/login` | Público | Iniciar sesión |
| POST | `/api/v1/auth/login/2fa` | Público | Inicio de sesión con código dinámico (2FA) |
| GET | `/api/v1/auth/login/2fa-status` | Público | Compruebe si la cuenta tiene 2FA habilitado |
| POST | `/api/v1/auth/logout` | Iniciado sesión | Cerrar sesión |
| GET | `/api/v1/auth/me` | Iniciado sesión | Usuario registrado actualmente |
| GET | `/api/v1/auth/passkey/credentials` | Iniciado sesión | Lista de credenciales Passkey |
| DELETE | `/api/v1/auth/passkey/credentials/:id` | Iniciado sesión | Eliminar credencial Passkey |
| POST | `/api/v1/auth/passkey/login/begin` | Público | Iniciar sesión |
| POST | `/api/v1/auth/passkey/login/finish` | Público | Iniciar sesión completo |
| POST | `/api/v1/auth/passkey/register/begin` | Iniciado sesión | Iniciar registro |
| POST | `/api/v1/auth/passkey/register/finish` | Iniciado sesión | Registro completo |
| GET | `/api/v1/auth/passkey/rpids` | Iniciado sesión | Lista de permitidos de ID de RP |
| POST | `/api/v1/auth/passkey/rpids` | Iniciado sesión | Actualizar la lista de permitidos de ID de RP |
| GET | `/api/v1/auth/passkey/status` | Público | Estado |
| POST | `/api/v1/auth/refresh` | Público | Actualizar |
| POST | `/api/v1/auth/ticket` | Iniciado sesión | Emitir un billete único de corta duración |

### cuentas de administrador

Cuentas de usuario del panel de administración. El listado requiere administrador; crear/actualizar/eliminar requiere superadministrador (evita la escalada de privilegios).

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/users` | administrador | Lista |
| POST | `/api/v1/users` | superadministrador | crear |
| DELETE | `/api/v1/users/:id` | superadministrador | Eliminar |
| PUT | `/api/v1/users/:id` | superadministrador | Actualizar |

### Gestión de clientes

CRUD, suspender/reanudar, actualizar/bajar de categoría, renovación y restablecimiento de uso para suscriptores (cuentas del portal).

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/clients` | Iniciado sesión | Listar clientes |
| POST | `/api/v1/clients` | Iniciado sesión | Crear cliente. cuerpo: `{"email":"user@example.com","password":"plaintext-password","plan_id":1,"quota_bytes":10737418240,"expires_at":"2026-12-31T00:00:00Z","remark":""}`. Se requiere `plan_id`; `password` puede estar vacío (los clientes pueden configurarlo más adelante en el portal) |
| DELETE | `/api/v1/clients/:id` | Iniciado sesión | Eliminar cliente |
| GET | `/api/v1/clients/:id` | Iniciado sesión | conseguir cliente |
| PUT | `/api/v1/clients/:id` | Iniciado sesión | Actualizar cliente |
| POST | `/api/v1/clients/:id/downgrade` | Iniciado sesión | Bajar de categoría al cliente |
| GET | `/api/v1/clients/:id/orders` | Iniciado sesión | Lista de pedidos de clientes |
| POST | `/api/v1/clients/:id/renew` | Iniciado sesión | Renovar cliente |
| POST | `/api/v1/clients/:id/reset-usage` | Iniciado sesión | Restablecer uso |
| POST | `/api/v1/clients/:id/restore` | Iniciado sesión | Reanudar cliente |
| POST | `/api/v1/clients/:id/suspend` | Iniciado sesión | Suspender cliente |
| POST | `/api/v1/clients/:id/upgrade` | Iniciado sesión | Actualizar cliente |

### Plantillas de planos

Planes (variantes, ciclos de facturación, precios, cuotas de tráfico) y autorización de nodos a nivel de plantilla.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/plan-templates` | Iniciado sesión | Lista de plantillas |
| POST | `/api/v1/plan-templates` | Iniciado sesión | Crear plantilla |
| DELETE | `/api/v1/plan-templates/:id` | Iniciado sesión | Eliminar plantilla |
| GET | `/api/v1/plan-templates/:id` | Iniciado sesión | Detalles de la plantilla |
| PUT | `/api/v1/plan-templates/:id` | Iniciado sesión | Actualizar plantilla |
| GET | `/api/v1/plan-templates/:id/accessible-nodes` | Iniciado sesión | Obtener nodos disponibles |
| GET | `/api/v1/plan-templates/:id/billing-node-options` | Iniciado sesión | Obtenga opciones de nodos facturables |
| PUT | `/api/v1/plan-templates/:id/node-groups` | Iniciado sesión | Establecer grupos de nodos de plantilla |
| POST | `/api/v1/plan-templates/:id/sync-accounts` | Iniciado sesión | Sincronizar cuentas con la plantilla |
| GET | `/api/v1/plan-templates/:id/variants` | Iniciado sesión | Lista de variantes |
| POST | `/api/v1/plan-templates/:id/variants` | Iniciado sesión | Crear variante |
| POST | `/api/v1/plan-templates/sync-all-accounts` | Iniciado sesión | Sincronizar todas las cuentas |

### Variantes de planes

Actualice y elimine variantes del plan (la creación está anidada en plantillas de plan).

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| DELETE | `/api/v1/plan-variants/:id` | Iniciado sesión | Eliminar variante |
| PUT | `/api/v1/plan-variants/:id` | Iniciado sesión | Variante de actualización |

### Paquetes de tráfico

Packs de tráfico que se pueden adquirir por separado.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/traffic-packs` | Iniciado sesión | Lista de paquetes de tráfico |
| POST | `/api/v1/traffic-packs` | Iniciado sesión | Crear paquete de tráfico |
| DELETE | `/api/v1/traffic-packs/:id` | Iniciado sesión | Eliminar paquete de tráfico |
| PUT | `/api/v1/traffic-packs/:id` | Iniciado sesión | Actualizar paquete de tráfico |

### Cupones

Crea, valida y canjea cupones.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/promotions` | Iniciado sesión | Listar cupones |
| POST | `/api/v1/promotions` | Iniciado sesión | Crear cupón |
| DELETE | `/api/v1/promotions/:id` | Iniciado sesión | Eliminar cupón |
| PUT | `/api/v1/promotions/:id` | Iniciado sesión | Actualizar cupón |
| POST | `/api/v1/promotions/validate` | Iniciado sesión | Validar cupón |

### Grupos de nodos

Organice los nodos en grupos a los que hacen referencia los planes.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/node-groups` | Iniciado sesión | Listar grupos de nodos |
| POST | `/api/v1/node-groups` | Iniciado sesión | Crear grupo de nodos |
| DELETE | `/api/v1/node-groups/:id` | Iniciado sesión | Eliminar grupo de nodos |
| PUT | `/api/v1/node-groups/:id` | Iniciado sesión | Actualizar grupo de nodos |
| GET | `/api/v1/node-groups/:id/members` | Iniciado sesión | Listar miembros del grupo |
| POST | `/api/v1/node-groups/:id/members` | Iniciado sesión | Agregar miembro del grupo |
| DELETE | `/api/v1/node-groups/:id/members/:mid` | Iniciado sesión | Eliminar miembro del grupo |

### Servidores de nodo

Recursos principales: nodo CRUD, entrega de configuración central (xray/sing-box), configuración de entrada/salida/punto final, control de procesos, certificados, WAF, límites de velocidad, salto de puerto, proxies encadenados, espejo/copia de seguridad primaria, instalación/actualización del agente, control de tiempo de ejecución de xray. Muchas subrutas, agrupadas por familia de características.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/node-servers` | administrador | Lista |
| POST | `/api/v1/node-servers` | administrador | crear |
| DELETE | `/api/v1/node-servers/:id` | administrador | Eliminar |
| GET | `/api/v1/node-servers/:id` | administrador | Detalles |
| PUT | `/api/v1/node-servers/:id` | administrador | Actualizar |
| GET | `/api/v1/node-servers/:id/binaries` | administrador | Lista de núcleos instalados |
| POST | `/api/v1/node-servers/:id/binaries/ech-generate` | administrador | Generar par de claves ECH |
| POST | `/api/v1/node-servers/:id/binaries/install` | administrador | Instalar/actualizar núcleo |
| GET | `/api/v1/node-servers/:id/binaries/interfaces` | administrador | Lista de interfaces de red |
| POST | `/api/v1/node-servers/:id/binaries/mldsa65-key` | administrador | Generar clave MLDSA65 |
| POST | `/api/v1/node-servers/:id/binaries/reality-key` | administrador | Generar par de claves REALITY |
| POST | `/api/v1/node-servers/:id/binaries/reality-public-key` | administrador | Derivar la clave pública de la clave privada REALITY |
| GET | `/api/v1/node-servers/:id/binaries/releases` | administrador | Versiones disponibles |
| POST | `/api/v1/node-servers/:id/binaries/vlessenc` | administrador | Generar parámetros de cifrado VLESS |
| POST | `/api/v1/node-servers/:id/binaries/wg-key` | administrador | Generar clave WireGuard |
| POST | `/api/v1/node-servers/:id/binaries/x25519-key` | administrador | Generar par de claves X25519 |
| GET | `/api/v1/node-servers/:id/bootstrap-cmd` | administrador | Obtener el comando de arranque del agente |
| POST | `/api/v1/node-servers/:id/bootstrap-token` | administrador | Generar token de arranque |
| POST | `/api/v1/node-servers/:id/caddy/batch-update-log-enabled` | administrador | Cambios de registro de actualización masiva |
| POST | `/api/v1/node-servers/:id/caddy/batch-update-ports` | administrador | Puertos de actualización masiva |
| POST | `/api/v1/node-servers/:id/caddy/generate-config` | administrador | Generar configuración |
| DELETE | `/api/v1/node-servers/:id/caddy/l4-rules` | administrador | Eliminar todas las reglas L4 |
| POST | `/api/v1/node-servers/:id/caddy/save-and-apply` | administrador | Guardar y aplicar |
| GET | `/api/v1/node-servers/:id/caddy/sites` | administrador | Lista de sitios |
| POST | `/api/v1/node-servers/:id/caddy/sites` | administrador | Crear sitio |
| DELETE | `/api/v1/node-servers/:id/caddy/sites/:siteId` | administrador | Eliminar sitio |
| PUT | `/api/v1/node-servers/:id/caddy/sites/:siteId` | administrador | Sitio de actualización |
| DELETE | `/api/v1/node-servers/:id/caddy/sni-rules` | administrador | Eliminar todas las reglas SNI |
| POST | `/api/v1/node-servers/:id/caddy/sni-rules/toggle` | administrador | Alternar reglas SNI |
| GET | `/api/v1/node-servers/:id/caddy/sni-settings` | administrador | Configuración de SNI |
| POST | `/api/v1/node-servers/:id/caddy/sni-settings` | administrador | Guardar la configuración de SNI |
| GET | `/api/v1/node-servers/:id/certificates` | administrador | Lista de certificados de nodo |
| DELETE | `/api/v1/node-servers/:id/certificates/:cid` | administrador | Certificado de desvinculación |
| POST | `/api/v1/node-servers/:id/certificates/:cid` | administrador | Certificado de vinculación |
| GET | `/api/v1/node-servers/:id/chain-config` | administrador | Generar configuración encadenada |
| GET | `/api/v1/node-servers/:id/chain-health` | administrador | Estado de salud de la cadena |
| POST | `/api/v1/node-servers/:id/control` | superadministrador | Núcleo del nodo de control. cuerpo: `{"core_type":"xray","action":"start"}` (acción = iniciar/detener/reiniciar). `core_type` puede ser `xray`/`singbox` |
| GET | `/api/v1/node-servers/:id/degradation-status` | administrador | Estado degradado |
| POST | `/api/v1/node-servers/:id/distribute-cert` | administrador | Distribuir certificado |
| GET | `/api/v1/node-servers/:id/endpoint-configs` | administrador | Lista |
| POST | `/api/v1/node-servers/:id/endpoint-configs` | administrador | crear |
| DELETE | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | administrador | Eliminar |
| GET | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | administrador | Detalles |
| PUT | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | administrador | Actualizar |
| POST | `/api/v1/node-servers/:id/failover` | administrador | Activar conmutación por error para este nodo |
| GET | `/api/v1/node-servers/:id/failover-status` | administrador | Estado de conmutación por error |
| POST | `/api/v1/node-servers/:id/force-reconnect` | administrador | Forzar reconexión |
| GET | `/api/v1/node-servers/:id/free-port` | administrador | Solicitar un puerto libre |
| POST | `/api/v1/node-servers/:id/geo-rules/push` | administrador | Empujar a este nodo |
| GET | `/api/v1/node-servers/:id/inbound-configs` | administrador | Lista |
| POST | `/api/v1/node-servers/:id/inbound-configs` | administrador | crear |
| DELETE | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | administrador | Eliminar |
| GET | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | administrador | Detalles |
| PUT | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | administrador | Actualizar |
| GET | `/api/v1/node-servers/:id/inbound-configs/:cfgId/share-links` | administrador | Compartir enlaces y fragmentos |
| POST | `/api/v1/node-servers/:id/inbound-configs/:cfgId/users/:email/pause` | administrador | Suspender a este usuario |
| POST | `/api/v1/node-servers/:id/inbound-configs/:cfgId/users/:email/resume` | administrador | Restaurar este usuario |
| POST | `/api/v1/node-servers/:id/inbound-configs/reset-all-traffic` | administrador | Restablecer todo el tráfico entrante |
| GET | `/api/v1/node-servers/:id/install-script` | administrador | Obtener script de instalación del agente |
| GET | `/api/v1/node-servers/:id/metrics-history` | administrador | Obtener historial de métricas |
| GET | `/api/v1/node-servers/:id/mirror-status` | administrador | Estado del espejo |
| POST | `/api/v1/node-servers/:id/nginx/batch-update-ports` | administrador | Puertos de actualización masiva |
| POST | `/api/v1/node-servers/:id/nginx/generate-config` | administrador | Generar configuración |
| DELETE | `/api/v1/node-servers/:id/nginx/l4-rules` | administrador | Eliminar todas las reglas L4 |
| POST | `/api/v1/node-servers/:id/nginx/save-and-apply` | administrador | Guardar y aplicar |
| GET | `/api/v1/node-servers/:id/nginx/settings` | administrador | Leer configuración |
| POST | `/api/v1/node-servers/:id/nginx/settings` | administrador | Guardar configuración |
| GET | `/api/v1/node-servers/:id/nginx/sites` | administrador | Lista de sitios |
| POST | `/api/v1/node-servers/:id/nginx/sites` | administrador | Crear sitio |
| DELETE | `/api/v1/node-servers/:id/nginx/sites/:siteId` | administrador | Eliminar sitio |
| PUT | `/api/v1/node-servers/:id/nginx/sites/:siteId` | administrador | Sitio de actualización |
| DELETE | `/api/v1/node-servers/:id/nginx/sni-rules` | administrador | Eliminar todas las reglas SNI |
| POST | `/api/v1/node-servers/:id/nginx/sni-rules/toggle` | administrador | Alternar reglas SNI |
| PUT | `/api/v1/node-servers/:id/node-type` | administrador | Actualizar tipo de nodo |
| GET | `/api/v1/node-servers/:id/online-users` | administrador | Usuarios en línea |
| GET | `/api/v1/node-servers/:id/outbound-configs` | administrador | Lista |
| POST | `/api/v1/node-servers/:id/outbound-configs` | administrador | crear |
| DELETE | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | administrador | Eliminar |
| GET | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | administrador | Detalles |
| PUT | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | administrador | Actualizar |
| PUT | `/api/v1/node-servers/:id/outbound-configs/reorder` | administrador | ordenar |
| GET | `/api/v1/node-servers/:id/port-hop/rules` | administrador | Vista previa de la regla de salto de puerto |
| GET | `/api/v1/node-servers/:id/protocols` | administrador | Lista de protocolos |
| GET | `/api/v1/node-servers/:id/protocols/:coreType` | administrador | Configuración de protocolo |
| PUT | `/api/v1/node-servers/:id/protocols/:coreType` | administrador | Guardar configuración de protocolo |
| GET | `/api/v1/node-servers/:id/protocols/:coreType/config-file` | administrador | Leer el archivo de configuración principal |
| GET | `/api/v1/node-servers/:id/protocols/:coreType/section/:section` | administrador | Leer sección de protocolo |
| PUT | `/api/v1/node-servers/:id/protocols/:coreType/section/:section` | administrador | Guardar sección de protocolo |
| POST | `/api/v1/node-servers/:id/protocols/:coreType/validate` | administrador | Validar configuración (ejecución en seco del núcleo real) |
| GET | `/api/v1/node-servers/:id/proxy-status` | administrador | Estado de proxy remoto |
| POST | `/api/v1/node-servers/:id/proxy/batch-update-ports` | administrador | Puertos de actualización masiva |
| GET | `/api/v1/node-servers/:id/proxy/config` | administrador | Leer configuración |
| PUT | `/api/v1/node-servers/:id/proxy/config` | administrador | Actualizar configuración |
| POST | `/api/v1/node-servers/:id/proxy/control` | administrador | Control de registro de proxy encadenado |
| POST | `/api/v1/node-servers/:id/proxy/download` | administrador | Descargar la versión principal del proxy |
| POST | `/api/v1/node-servers/:id/proxy/generate-config` | administrador | Generar configuración |
| POST | `/api/v1/node-servers/:id/proxy/guardian` | administrador | Actualizar configuración del supervisor |
| POST | `/api/v1/node-servers/:id/proxy/install` | administrador | Instalar proxy |
| DELETE | `/api/v1/node-servers/:id/proxy/l4-rules` | administrador | Eliminar todas las reglas L4 |
| GET | `/api/v1/node-servers/:id/proxy/l4-settings` | administrador | configuración L4 |
| POST | `/api/v1/node-servers/:id/proxy/l4-settings` | administrador | Guardar configuración L4 |
| GET | `/api/v1/node-servers/:id/proxy/log-settings` | administrador | Configuración de registro |
| POST | `/api/v1/node-servers/:id/proxy/log-settings` | administrador | Guardar configuración de registro |
| GET | `/api/v1/node-servers/:id/proxy/nginx-settings` | administrador | Configuración de nginx |
| POST | `/api/v1/node-servers/:id/proxy/nginx-settings` | administrador | Guardar la configuración de nginx |
| GET | `/api/v1/node-servers/:id/proxy/releases` | administrador | Versiones disponibles |
| POST | `/api/v1/node-servers/:id/proxy/reload` | administrador | Recargar configuración |
| POST | `/api/v1/node-servers/:id/proxy/restart` | administrador | Reiniciar |
| GET | `/api/v1/node-servers/:id/proxy/sites` | administrador | Lista de sitios |
| POST | `/api/v1/node-servers/:id/proxy/sites` | administrador | Crear sitio |
| DELETE | `/api/v1/node-servers/:id/proxy/sites/:siteId` | administrador | Eliminar sitio |
| PUT | `/api/v1/node-servers/:id/proxy/sites/:siteId` | administrador | Sitio de actualización |
| DELETE | `/api/v1/node-servers/:id/proxy/sni-rules` | administrador | Eliminar todas las reglas SNI |
| POST | `/api/v1/node-servers/:id/proxy/sni-rules/toggle` | administrador | Alternar reglas SNI |
| GET | `/api/v1/node-servers/:id/proxy/sni-settings` | administrador | Configuración de SNI |
| POST | `/api/v1/node-servers/:id/proxy/sni-settings` | administrador | Guardar la configuración de SNI |
| POST | `/api/v1/node-servers/:id/proxy/start` | administrador | Empezar |
| GET | `/api/v1/node-servers/:id/proxy/status` | administrador | Estado del proceso de proxy |
| POST | `/api/v1/node-servers/:id/proxy/stop` | administrador | Detener |
| POST | `/api/v1/node-servers/:id/proxy/sync` | administrador | Configuración de proxy de sincronización |
| GET | `/api/v1/node-servers/:id/rate-limit` | administrador | Leer configuración |
| PUT | `/api/v1/node-servers/:id/rate-limit` | administrador | Actualizar configuración |
| GET | `/api/v1/node-servers/:id/rate-limit/status` | administrador | Estado |
| GET | `/api/v1/node-servers/:id/recover-mode` | administrador | Modo de recuperación de nodo de lectura |
| PUT | `/api/v1/node-servers/:id/recover-mode` | administrador | Establecer el modo de recuperación del nodo |
| POST | `/api/v1/node-servers/:id/revoke-cert` | administrador | Revocar certificado de agente |
| GET | `/api/v1/node-servers/:id/services` | administrador | Lista |
| POST | `/api/v1/node-servers/:id/services` | administrador | crear |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId` | administrador | Eliminar |
| GET | `/api/v1/node-servers/:id/services/:serviceId` | administrador | Detalles |
| PUT | `/api/v1/node-servers/:id/services/:serviceId` | administrador | Actualizar |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId/crash-logs` | administrador | Borrar registros de fallos |
| GET | `/api/v1/node-servers/:id/services/:serviceId/crash-logs` | administrador | Lista de registros de fallos |
| GET | `/api/v1/node-servers/:id/services/:serviceId/crash-logs/:logId` | administrador | Detalles del registro de fallos |
| GET | `/api/v1/node-servers/:id/services/:serviceId/endpoints` | administrador | Lista de puntos finales |
| POST | `/api/v1/node-servers/:id/services/:serviceId/endpoints` | administrador | Crear punto final |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId/endpoints/:endpointId` | administrador | Eliminar punto final |
| PUT | `/api/v1/node-servers/:id/services/:serviceId/endpoints/:endpointId` | administrador | Actualizar punto final |
| PUT | `/api/v1/node-servers/:id/services/:serviceId/guardian` | administrador | Actualizar configuración del supervisor |
| GET | `/api/v1/node-servers/:id/services/:serviceId/process-status` | administrador | Obtener estado de procesamiento |
| POST | `/api/v1/node-servers/:id/services/:serviceId/restart` | administrador | Reiniciar |
| POST | `/api/v1/node-servers/:id/services/:serviceId/start` | administrador | Empezar |
| POST | `/api/v1/node-servers/:id/services/:serviceId/stop` | administrador | Detener |
| GET | `/api/v1/node-servers/:id/services/process-status` | administrador | Todos los estados del proceso |
| GET | `/api/v1/node-servers/:id/share-domains` | administrador | Compartir dominios para este nodo |
| POST | `/api/v1/node-servers/:id/share-domains` | administrador | crear |
| DELETE | `/api/v1/node-servers/:id/share-domains/:domainId` | administrador | Eliminar |
| PUT | `/api/v1/node-servers/:id/share-domains/:domainId` | administrador | Actualizar |
| DELETE | `/api/v1/node-servers/:id/share-domains/all` | administrador | Borrar todo |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/apply` | administrador | Aplicar reglas de firewall de salto de puerto |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/remove` | administrador | Eliminar reglas de firewall de salto de puerto |
| POST | `/api/v1/node-servers/:id/speed-test` | administrador | Prueba de velocidad del nodo |
| GET | `/api/v1/node-servers/:id/status` | administrador | Estado del nodo |
| POST | `/api/v1/node-servers/:id/sync` | administrador | Configuración del nodo de sincronización (generar y enviar) |
| POST | `/api/v1/node-servers/:id/sync-data` | superadministrador | Enviar datos del panel a los nodos (sincronización completa de usuarios/suscripciones/planes) |
| POST | `/api/v1/node-servers/:id/sync-mirror` | administrador | Sincronizar nodo espejo |
| GET | `/api/v1/node-servers/:id/tags` | administrador | Lista de etiquetas |
| POST | `/api/v1/node-servers/:id/tags` | administrador | Agregar etiqueta |
| DELETE | `/api/v1/node-servers/:id/tags/:tagKey` | administrador | Eliminar etiqueta |
| GET | `/api/v1/node-servers/:id/traffic-history` | administrador | Historial de tráfico |
| POST | `/api/v1/node-servers/:id/traffic-resume` | administrador | Restaurar el tráfico suspendido por exceso |
| GET | `/api/v1/node-servers/:id/traffic-usage` | administrador | Uso del tráfico |
| POST | `/api/v1/node-servers/:id/update-agent` | administrador | Agente de actualización |
| GET | `/api/v1/node-servers/:id/used-server-names` | administrador | Lista de REALITY SNI en uso |
| GET | `/api/v1/node-servers/:id/waf/config` | administrador | Leer configuración |
| PUT | `/api/v1/node-servers/:id/waf/config` | administrador | Guardar configuración |
| POST | `/api/v1/node-servers/:id/waf/disable-domain` | administrador | Deshabilitar la protección del dominio |
| GET | `/api/v1/node-servers/:id/waf/domain-config` | administrador | Lista de configuración de dominio |
| PUT | `/api/v1/node-servers/:id/waf/domain-config` | administrador | Guardar configuraciones de dominio de forma masiva |
| DELETE | `/api/v1/node-servers/:id/waf/domain-config/:domain` | administrador | Eliminar configuración de dominio |
| GET | `/api/v1/node-servers/:id/waf/domain-config/:domain` | administrador | Detalles de configuración del dominio |
| PUT | `/api/v1/node-servers/:id/waf/domain-config/:domain` | administrador | Guardar configuración de dominio |
| GET | `/api/v1/node-servers/:id/waf/domain-rules` | administrador | Directorio de reglas de dominio |
| GET | `/api/v1/node-servers/:id/waf/domain-rules/:domain/:name` | administrador | Leer archivo de reglas de dominio |
| PUT | `/api/v1/node-servers/:id/waf/domain-rules/:domain/:name` | administrador | Guardar archivo de reglas de dominio |
| GET | `/api/v1/node-servers/:id/waf/domain-status` | administrador | Estado de protección del dominio |
| POST | `/api/v1/node-servers/:id/waf/enable-domain` | administrador | Habilitar la protección del dominio |
| POST | `/api/v1/node-servers/:id/waf/push` | administrador | Empujar a este nodo |
| GET | `/api/v1/node-servers/:id/waf/rules` | administrador | Lista de reglas del nodo WAF |
| DELETE | `/api/v1/node-servers/:id/waf/rules/:name` | administrador | Eliminar reglas del nodo WAF |
| GET | `/api/v1/node-servers/:id/waf/rules/:name` | administrador | Detalles de la regla del nodo WAF |
| PUT | `/api/v1/node-servers/:id/waf/rules/:name` | administrador | Guardar reglas del nodo WAF |
| GET | `/api/v1/node-servers/:id/waf/status` | administrador | Estado del nodo WAF |
| PUT | `/api/v1/node-servers/:id/waf/status` | administrador | Nodo de conmutación WAF |
| POST | `/api/v1/node-servers/:id/waf/sync` | administrador | Sincronizar con este nodo |
| GET | `/api/v1/node-servers/:id/warp` | administrador | Lista |
| DELETE | `/api/v1/node-servers/:id/warp/:warpId` | administrador | Eliminar |
| GET | `/api/v1/node-servers/:id/warp/config` | administrador | Leer configuración |
| POST | `/api/v1/node-servers/:id/warp/generate-config` | administrador | Generar configuración |
| POST | `/api/v1/node-servers/:id/warp/register` | administrador | Registrarse |
| GET | `/api/v1/node-servers/:id/warp/status` | administrador | Estado |
| GET | `/api/v1/node-servers/:id/xray/control/balancer/:tag` | administrador | Obtener información del balanceador |
| POST | `/api/v1/node-servers/:id/xray/control/balancer/:tag` | administrador | Establecer objetivo de equilibrio |
| POST | `/api/v1/node-servers/:id/xray/control/firewall/apply` | administrador | Aplicar reglas de firewall de salto de puerto |
| POST | `/api/v1/node-servers/:id/xray/control/firewall/remove` | administrador | Eliminar reglas de firewall de salto de puerto |
| POST | `/api/v1/node-servers/:id/xray/control/inbound` | administrador | Agregar entrante |
| DELETE | `/api/v1/node-servers/:id/xray/control/inbound/:tag` | administrador | Eliminar entrante |
| POST | `/api/v1/node-servers/:id/xray/control/inbound/:tag/user` | administrador | Agregar usuario |
| DELETE | `/api/v1/node-servers/:id/xray/control/inbound/:tag/user/:email` | administrador | Eliminar usuario |
| POST | `/api/v1/node-servers/:id/xray/control/outbound` | administrador | Agregar salida |
| DELETE | `/api/v1/node-servers/:id/xray/control/outbound/:tag` | administrador | Quitar salientes |
| POST | `/api/v1/node-servers/:id/xray/control/routing` | administrador | Solicitar ruta |
| POST | `/api/v1/node-servers/:id/xray/control/routing/rule` | administrador | Agregar regla de enrutamiento |
| DELETE | `/api/v1/node-servers/:id/xray/control/routing/rule/:tag` | administrador | Eliminar regla de enrutamiento |
| GET | `/api/v1/node-servers/:id/xray/control/routing/rules` | administrador | Listar reglas de enrutamiento |
| POST | `/api/v1/node-servers/:id/xray/control/sync` | administrador | Activar manualmente la sincronización de configuración xray |
| GET | `/api/v1/node-servers/:id/xray/stats` | administrador | Obtener estadísticas |
| POST | `/api/v1/node-servers/:id/xray/stats/reset/:email` | administrador | Restablecer estadísticas de usuario |
| GET | `/api/v1/node-servers/:id/xray/stats/users` | administrador | Obtener estadísticas de usuario |
| GET | `/api/v1/node-servers/billable` | administrador | Lista de nodos facturables |
| GET | `/api/v1/node-servers/ca-cert` | administrador | certificado de CA |
| GET | `/api/v1/node-servers/ca-status` | administrador | Estado de CA |
| GET | `/api/v1/node-servers/dashboard` | administrador | Estado del panel |
| POST | `/api/v1/node-servers/distribute-cert` | administrador | Distribuir certificado a todos los nodos. |
| POST | `/api/v1/node-servers/failover-check` | administrador | Activar verificación de conmutación por error |
| GET | `/api/v1/node-servers/metrics/aggregated` | administrador | Métricas agregadas |
| POST | `/api/v1/node-servers/migrate-cert` | administrador | Migrar certificado |
| GET | `/api/v1/node-servers/online-summary` | administrador | Resumen en línea |
| GET | `/api/v1/node-servers/online-users` | administrador | Todos los usuarios en línea |
| GET | `/api/v1/node-servers/pending-agent-updates` | administrador | Agentes pendientes de actualización |
| POST | `/api/v1/node-servers/recommend-region` | administrador | Regiones recomendadas |
| POST | `/api/v1/node-servers/select-nodes` | administrador | Seleccionar nodos para suscripción |
| POST | `/api/v1/node-servers/speed-test-all` | administrador | Prueba de velocidad todos los nodos |
| GET | `/api/v1/node-servers/speed-test-results` | administrador | Resultados de la prueba de velocidad |
| GET | `/api/v1/node-servers/traffic/stats` | administrador | Estadísticas de tráfico |
| POST | `/api/v1/node-servers/update-all-agents` | administrador | Actualizar todos los agentes |
| DELETE | `/api/v1/node-servers/users/:client_id` | administrador | Sincronizar-eliminar un cliente |
| GET | `/api/v1/node-servers/users/retry-queue` | administrador | Estado de la cola de reintento |
| POST | `/api/v1/node-servers/users/retry-queue/process` | administrador | Cola de reintento de proceso |
| POST | `/api/v1/node-servers/users/sync` | administrador | Sincronizar un solo cliente |
| POST | `/api/v1/node-servers/validate-chain-config` | administrador | Validar configuración encadenada |

### Registro de auditoría de nodo

Auditoría y exportación de operaciones del lado del nodo.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/node-audit-logs` | Iniciado sesión | Registro de auditoría de nodo |
| GET | `/api/v1/node-audit-logs/export` | Iniciado sesión | Exportar registros de auditoría de nodos |

### Nodos (modelo heredado, retirado)

> `/api/v1/nodes` CRUD se retiró el 20/10/2026: los nodos se administran de manera uniforme a través de `/api/v1/node-servers`.
> Las consultas de los usuarios en línea permanecen en `/api/v1/node-servers/{online-users,online-summary,:id/online-users}`.

### Descripción general de la región

Vista de nodo agregada por región.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/regions/:region/servers` | Iniciado sesión | Lista de nodos para esta región |
| GET | `/api/v1/regions/overview` | Iniciado sesión | Descripción general de la región |

### Suscripciones

Gestión de registros de suscripción: revocar, rotar token, registros de acceso, restablecer uso.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/subscriptions` | Iniciado sesión | Lista |
| POST | `/api/v1/subscriptions` | Iniciado sesión | crear |
| DELETE | `/api/v1/subscriptions/:id` | Iniciado sesión | Eliminar |
| GET | `/api/v1/subscriptions/:id` | Iniciado sesión | Detalles |
| PUT | `/api/v1/subscriptions/:id` | Iniciado sesión | Actualizar |
| GET | `/api/v1/subscriptions/:id/access-logs` | Iniciado sesión | Registro de acceso |
| POST | `/api/v1/subscriptions/:id/enable` | Iniciado sesión | Habilitar |
| POST | `/api/v1/subscriptions/:id/reset-usage` | Iniciado sesión | Restablecer uso |
| POST | `/api/v1/subscriptions/:id/restore` | Iniciado sesión | Currículum |
| POST | `/api/v1/subscriptions/:id/revoke` | Iniciado sesión | Revocar |
| POST | `/api/v1/subscriptions/:id/rotate` | Iniciado sesión | rotar |
| GET | `/api/v1/subscriptions/:id/rotations` | Iniciado sesión | Historial de rotación |

### Formatos de suscripción

> El CRUD `/api/v1/subscription-formats` del lado administrativo se retiró el 20/10/2026: el punto de entrada real es
> `/api/v1/mihomo/configs` (centro de formato de suscripción). La tabla `subscription_formats` permanece,
> Se utiliza únicamente como datos iniciales para el punto final público `/api/v1/public/client-configs`.

### Puntos finales de suscripción

Dominios de suscripción dedicados (proxy inverso a la ruta `/sub` del panel).

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/subscription-endpoints` | Iniciado sesión | Lista |
| POST | `/api/v1/subscription-endpoints` | Iniciado sesión | crear |
| DELETE | `/api/v1/subscription-endpoints/:id` | Iniciado sesión | Eliminar |
| GET | `/api/v1/subscription-endpoints/:id` | Iniciado sesión | Detalles |
| PUT | `/api/v1/subscription-endpoints/:id` | Iniciado sesión | Actualizar |

### Compartir dominios

Acceda al dominio/grupo de IP utilizado en los enlaces del cliente.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/share-domains` | Iniciado sesión | Lista |
| POST | `/api/v1/share-domains` | Iniciado sesión | crear |
| DELETE | `/api/v1/share-domains/:id` | Iniciado sesión | Eliminar |
| PUT | `/api/v1/share-domains/:id` | Iniciado sesión | Actualizar |
| DELETE | `/api/v1/share-domains/all` | Iniciado sesión | Borrar todo |
| POST | `/api/v1/share-domains/seed` | Iniciado sesión | Inicializar datos predeterminados |

### Cloudflare IP

Cloudflare CDN Gestión de fuentes y grupos de IP.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| DELETE | `/api/v1/cfip/entries` | Iniciado sesión | Borrar entradas |
| GET | `/api/v1/cfip/entries` | Iniciado sesión | Lista de entrada |
| POST | `/api/v1/cfip/entries` | Iniciado sesión | Agregar entrada |
| DELETE | `/api/v1/cfip/entries/:id` | Iniciado sesión | Eliminar entrada |
| POST | `/api/v1/cfip/fetch-all` | Iniciado sesión | Obtener todas las fuentes |
| POST | `/api/v1/cfip/seed` | Iniciado sesión | Inicializar datos predeterminados |
| GET | `/api/v1/cfip/sources` | Iniciado sesión | Lista de fuentes de datos |
| POST | `/api/v1/cfip/sources` | Iniciado sesión | Crear fuente de datos |
| DELETE | `/api/v1/cfip/sources/:id` | Iniciado sesión | Eliminar fuente de datos |
| PUT | `/api/v1/cfip/sources/:id` | Iniciado sesión | Actualizar fuente de datos |
| POST | `/api/v1/cfip/sources/:id/fetch` | Iniciado sesión | Obtener esta fuente |

### Configuración de suscripción (Mihomo/Clash)

Configuración de plantilla de suscripción: grupos, reglas, proveedores de reglas, vista previa de YAML.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/mihomo/configs` | Iniciado sesión | Lista |
| POST | `/api/v1/mihomo/configs` | Iniciado sesión | crear |
| DELETE | `/api/v1/mihomo/configs/:id` | Iniciado sesión | Eliminar |
| GET | `/api/v1/mihomo/configs/:id` | Iniciado sesión | Detalles |
| PUT | `/api/v1/mihomo/configs/:id` | Iniciado sesión | Actualizar |
| GET | `/api/v1/mihomo/configs/:id/proxy-groups` | Iniciado sesión | lista de grupos |
| POST | `/api/v1/mihomo/configs/:id/proxy-groups` | Iniciado sesión | Crear grupo |
| GET | `/api/v1/mihomo/configs/:id/rule-providers` | Iniciado sesión | Lista de proveedores de reglas |
| POST | `/api/v1/mihomo/configs/:id/rule-providers` | Iniciado sesión | Crear proveedor de reglas |
| GET | `/api/v1/mihomo/configs/:id/rules` | Iniciado sesión | Lista de reglas |
| POST | `/api/v1/mihomo/configs/:id/rules` | Iniciado sesión | Crear regla |
| GET | `/api/v1/mihomo/configs/:id/yaml` | Iniciado sesión | Vista previa generada YAML |
| DELETE | `/api/v1/mihomo/proxy-groups/:id` | Iniciado sesión | Eliminar grupo |
| DELETE | `/api/v1/mihomo/rule-providers/:id` | Iniciado sesión | Eliminar proveedor de reglas |
| DELETE | `/api/v1/mihomo/rules/:id` | Iniciado sesión | Eliminar regla |

### Sitios de recarga

Integración de sitios de pago de terceros: claves de firma, archivos de verificación, verificación de dominio, documentos de integración.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/charge-sites` | Iniciado sesión | Lista de sitios |
| POST | `/api/v1/charge-sites` | Iniciado sesión | Crear sitio |
| DELETE | `/api/v1/charge-sites/:id` | Iniciado sesión | Eliminar sitio |
| GET | `/api/v1/charge-sites/:id` | Iniciado sesión | Detalles del sitio |
| PUT | `/api/v1/charge-sites/:id` | Iniciado sesión | Sitio de actualización |
| POST | `/api/v1/charge-sites/:id/clear-prev-secret` | Iniciado sesión | Borrar llave antigua |
| POST | `/api/v1/charge-sites/:id/generate-verify` | Iniciado sesión | Generar archivo de verificación |
| POST | `/api/v1/charge-sites/:id/generate-verify-file` | Iniciado sesión | Generar archivo de verificación |
| GET | `/api/v1/charge-sites/:id/integration-doc` | Iniciado sesión | Obtener documento de integración |
| POST | `/api/v1/charge-sites/:id/rotate-secret` | Iniciado sesión | Girar clave de firma |
| POST | `/api/v1/charge-sites/:id/verify-dns` | Iniciado sesión | Verificar dominio DNS |
| POST | `/api/v1/charge-sites/:id/verify-domain` | Iniciado sesión | Verificar desafío de dominio HTTP |
| POST | `/api/v1/charge-sites/:id/verify-domain-dns` | Iniciado sesión | Verificar dominio DNS |
| POST | `/api/v1/charge-sites/:id/verify-domain-http` | Iniciado sesión | Verificar desafío de dominio HTTP |

### Órdenes de recarga

Consultas de pedidos de recarga, devoluciones, exportación CSV.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/charge-orders` | Iniciado sesión | lista de pedidos |
| GET | `/api/v1/charge-orders/:id` | Iniciado sesión | Detalles del pedido |
| POST | `/api/v1/charge-orders/:id/refund` | Iniciado sesión | Reembolso |
| GET | `/api/v1/charge-orders/export-csv` | Iniciado sesión | Órdenes de recarga de exportación CSV |

### Tarjetas de recarga

Generación masiva de tarjetas, registros de uso, exportación CSV.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/charge-cards` | Iniciado sesión | Lista de tarjetas |
| POST | `/api/v1/charge-cards` | Iniciado sesión | Crear tarjeta |
| DELETE | `/api/v1/charge-cards/:id` | Iniciado sesión | Eliminar tarjeta |
| GET | `/api/v1/charge-cards/:id/usages` | Iniciado sesión | Registros de uso de tarjetas |
| POST | `/api/v1/charge-cards/batch` | Iniciado sesión | Generar tarjetas de forma masiva |
| GET | `/api/v1/charge-cards/export-csv` | Iniciado sesión | Tarjetas de exportación CSV |

### Cobrar eventos de devolución de llamada

Auditoría de eventos de devolución de llamada.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/charge-callback-events` | Iniciado sesión | Lista de eventos de devolución de llamada |

### Cargar reintentos de devolución de llamada

Reintento manual de devoluciones de llamada fallidas.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/charge-callback-retries` | Iniciado sesión | Lista de devolución de llamadas fallidas |
| POST | `/api/v1/charge-callback-retries/:id/retry` | administrador | Reintentar devolución de llamada |

### Saldo del cliente

Vista de administrador de saldos y transacciones.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/client-balances` | Iniciado sesión | Lista de saldos de clientes |
| GET | `/api/v1/client-balances/transactions` | Iniciado sesión | Transacciones de saldo (admin) |
| GET | `/api/v1/client-balances/transactions/export-csv` | Iniciado sesión | Transacciones de saldo de exportación CSV |

### Regalos

Emitir obsequios reclamables (días/tráfico).

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/client-gifts` | Iniciado sesión | Lista de regalos |
| POST | `/api/v1/client-gifts` | Iniciado sesión | crear regalo |
| POST | `/api/v1/client-gifts/batch` | Iniciado sesión | Crear regalos a granel |
| GET | `/api/v1/client-gifts/search-clients` | Iniciado sesión | Buscar clientes |

### Configuración de referencia

Configuración y estadísticas de comisiones de referidos.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/referral/config` | Iniciado sesión | Configuración de referencia |
| PUT | `/api/v1/referral/config` | administrador | Guardar configuración de referencia |
| GET | `/api/v1/referral/records` | Iniciado sesión | Lista de registros de referencia |
| GET | `/api/v1/referral/stats` | Iniciado sesión | Estadísticas de referencias |

### Retiros de referencias

Revisar las solicitudes de retiro.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/referral-withdrawals` | Iniciado sesión | Lista de solicitudes de retiro |
| POST | `/api/v1/referral-withdrawals/:id/approve` | Iniciado sesión | Aprobar retiro |
| POST | `/api/v1/referral-withdrawals/:id/reject` | Iniciado sesión | Rechazar retiro |

### Listas de IP permitidas (resumen)

Vista de lista de IP permitidas en todos los clientes.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/ip-whitelists` | Iniciado sesión | Todas las listas de IP permitidas de clientes |

### Enlaces de OAuth del cliente

Consultar vinculaciones de cuentas de terceros del cliente.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/client-oauth-bindings` | Iniciado sesión | Lista vinculante de OAuth del cliente |

### Entradas (administrador)

Contestación, asignación y cierre de tickets.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/tickets` | Iniciado sesión | Lista de entradas |
| GET | `/api/v1/tickets/:id` | Iniciado sesión | Detalles del billete |
| POST | `/api/v1/tickets/:id/assign` | Iniciado sesión | Asignar ticket |
| POST | `/api/v1/tickets/:id/close` | Iniciado sesión | Cerrar ticket |
| POST | `/api/v1/tickets/:id/reply` | Iniciado sesión | Responder al ticket |

### Artículos

CRUD para artículos del portal.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/articles` | Iniciado sesión | Lista de artículos |
| POST | `/api/v1/articles` | Iniciado sesión | Crear artículo |
| DELETE | `/api/v1/articles/:id` | Iniciado sesión | Eliminar artículo |
| GET | `/api/v1/articles/:id` | Iniciado sesión | Detalles del artículo |
| PUT | `/api/v1/articles/:id` | Iniciado sesión | Actualizar artículo |

### Cuentas compartidas

Registros de emisión y reparación de cuentas compartidas (streaming, etc.).

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/shared-accounts` | Iniciado sesión | Lista de cuentas compartidas |
| POST | `/api/v1/shared-accounts` | Iniciado sesión | Crear cuenta compartida |
| DELETE | `/api/v1/shared-accounts/:id` | Iniciado sesión | Eliminar cuenta compartida |
| GET | `/api/v1/shared-accounts/:id` | Iniciado sesión | Detalles de la cuenta compartida |
| PUT | `/api/v1/shared-accounts/:id` | Iniciado sesión | Actualizar cuenta compartida |
| GET | `/api/v1/shared-accounts/:id/repair-logs` | Iniciado sesión | Registros de reparación |

### Certificados

Emisión, carga, renovación y exportación de certificado público (ACME); Se admite la emisión en un nodo específico.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/certificates` | Iniciado sesión | Lista |
| POST | `/api/v1/certificates` | Iniciado sesión | crear |
| DELETE | `/api/v1/certificates/:id` | Iniciado sesión | Eliminar |
| GET | `/api/v1/certificates/:id` | Iniciado sesión | Detalles |
| PUT | `/api/v1/certificates/:id` | Iniciado sesión | Actualizar |
| GET | `/api/v1/certificates/:id/download` | Iniciado sesión | Descargar |
| POST | `/api/v1/certificates/:id/renew` | Iniciado sesión | Renovar cliente |
| GET | `/api/v1/certificates/check-port` | Iniciado sesión | comprobar puerto |
| GET | `/api/v1/certificates/detect-ip` | Iniciado sesión | IP de la sonda |
| GET | `/api/v1/certificates/export` | Iniciado sesión | Exportar |
| POST | `/api/v1/certificates/issue` | Iniciado sesión | Problema. cuerpo: `{"domains":["example.com","*.example.com"],"challenge_type":"dns-01","dns_channel_id":2,"key_type":"rsa","key_bits":4096}`. `challenge_type` por defecto es http-01; `dns_channel_id` selecciona un canal DNS (un proveedor DNS configurado + credenciales) |
| POST | `/api/v1/certificates/issue-on-node` | Iniciado sesión | Emitir al nodo |
| POST | `/api/v1/certificates/upload` | Iniciado sesión | Subir |

### Canales de certificación

Configuración del canal (CA) ACME.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/cert-channels` | Iniciado sesión | Lista |
| POST | `/api/v1/cert-channels` | Iniciado sesión | crear |
| DELETE | `/api/v1/cert-channels/:id` | Iniciado sesión | Eliminar |
| PUT | `/api/v1/cert-channels/:id` | Iniciado sesión | Actualizar |
| GET | `/api/v1/cert-channels/providers` | Iniciado sesión | Proveedor |

### Configuración EAB

Credenciales que vinculan cuentas externas como ZeroSSL / Google.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/eab-configs` | Iniciado sesión | Lista |
| POST | `/api/v1/eab-configs` | Iniciado sesión | crear |
| DELETE | `/api/v1/eab-configs/:id` | Iniciado sesión | Eliminar |
| PUT | `/api/v1/eab-configs/:id` | Iniciado sesión | Actualizar |
| POST | `/api/v1/eab-configs/fetch-google` | Iniciado sesión | Obtener Google EAB de GCP |
| POST | `/api/v1/eab-configs/fetch-zerossl` | Iniciado sesión | Obtenga ZeroSSL EAB con un correo electrónico en un clic |

### Cuentas ACME

Gestión de cuenta ACME (correo electrónico de contacto) desde el panel.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/acme-accounts` | Iniciado sesión | Lista |
| POST | `/api/v1/acme-accounts` | Iniciado sesión | crear |
| DELETE | `/api/v1/acme-accounts/:id` | Iniciado sesión | Eliminar |
| PUT | `/api/v1/acme-accounts/:id` | Iniciado sesión | Actualizar |
| GET | `/api/v1/acme-accounts/default` | Iniciado sesión | Obtener valores predeterminados |

### CA incorporada

Panel interno CA: Emisión/renovación/revocación de certificados de agente, exportación de CRL, cola de distribución.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/ca/agent-certs` | Iniciado sesión | Lista de certificados de agente |
| POST | `/api/v1/ca/agent-certs/:server_id/renew` | Iniciado sesión | Renovar certificado de agente |
| GET | `/api/v1/ca/distribution-queue` | Iniciado sesión | Cola de distribución de certificados |
| DELETE | `/api/v1/ca/distribution-queue/:id` | Iniciado sesión | Eliminar entrada de la cola de distribución |
| POST | `/api/v1/ca/distribution-queue/:id/retry` | Iniciado sesión | Reintentar la distribución del certificado |
| GET | `/api/v1/ca/revocations` | Iniciado sesión | Lista de revocación |
| GET | `/api/v1/ca/revocations/export` | Iniciado sesión | Exportar CRL |
| GET | `/api/v1/ca/status` | Iniciado sesión | Estado de CA |

### Reglas geográficas

Sincronización y entrega de fuentes y archivos de datos geoip/geosite.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/geo-rules/files` | Iniciado sesión | Lista de archivos |
| DELETE | `/api/v1/geo-rules/files/:fileId` | Iniciado sesión | Eliminar archivo |
| PUT | `/api/v1/geo-rules/files/:fileId/auto-sync` | Iniciado sesión | Alternar sincronización automática |
| POST | `/api/v1/geo-rules/files/:fileId/download` | Iniciado sesión | Descargar |
| GET | `/api/v1/geo-rules/files/:fileId/download-file` | Iniciado sesión | Descargar archivo |
| GET | `/api/v1/geo-rules/github-assets` | Iniciado sesión | Obtener activos del repositorio GitHub |
| POST | `/api/v1/geo-rules/push-all` | Iniciado sesión | Empujar a todos los nodos |
| POST | `/api/v1/geo-rules/seed` | Iniciado sesión | Inicializar datos predeterminados |
| GET | `/api/v1/geo-rules/sources` | Iniciado sesión | Lista de fuentes de datos |
| POST | `/api/v1/geo-rules/sources` | Iniciado sesión | Crear fuente de datos |
| DELETE | `/api/v1/geo-rules/sources/:sourceId` | Iniciado sesión | Eliminar fuente de datos |
| PUT | `/api/v1/geo-rules/sources/:sourceId` | Iniciado sesión | Actualizar fuente de datos |
| GET | `/api/v1/geo-rules/sources/:sourceId/assets` | Iniciado sesión | Obtener activos de lanzamiento |
| POST | `/api/v1/geo-rules/sources/:sourceId/files` | Iniciado sesión | Agregar archivo |
| PUT | `/api/v1/geo-rules/sources/:sourceId/schedule` | Iniciado sesión | Alternar sincronización programada |
| POST | `/api/v1/geo-rules/sources/:sourceId/select` | Iniciado sesión | Seleccionar archivo |
| POST | `/api/v1/geo-rules/sources/:sourceId/sync` | Iniciado sesión | Sincronizar esta fuente de datos |
| GET | `/api/v1/geo-rules/status` | Iniciado sesión | Estado |
| POST | `/api/v1/geo-rules/sync-all` | Iniciado sesión | Sincronizar todo |
| GET | `/api/v1/geo-rules/sync-status/:key` | Iniciado sesión | Progreso de sincronización |

### Biblioteca de reglas WAF

Reglas globales de WAF: origen, sincronización y entrega de GitHub a todos los nodos.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/waf/github-source` | Iniciado sesión | fuente de datos GitHub |
| PUT | `/api/v1/waf/github-source` | Iniciado sesión | Guarde la fuente de datos GitHub |
| POST | `/api/v1/waf/push-all` | Iniciado sesión | Empujar a todos los nodos |
| GET | `/api/v1/waf/rules` | Iniciado sesión | Lista de reglas |
| DELETE | `/api/v1/waf/rules/:name` | Iniciado sesión | Eliminar regla |
| GET | `/api/v1/waf/rules/:name` | Iniciado sesión | obtener regla |
| PUT | `/api/v1/waf/rules/:name` | Iniciado sesión | Guardar regla |
| POST | `/api/v1/waf/sync-all` | Iniciado sesión | Sincronizar todo |
| POST | `/api/v1/waf/sync-github` | Iniciado sesión | Sincronización desde GitHub |

### Gestión de registros

Lectura y transmisión de registros de nodo/núcleo, políticas de limpieza, control de escritura de registros.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| POST | `/api/v1/log-manage/cleanup-all` | Iniciado sesión | Limpia todo ahora |
| GET | `/api/v1/log-manage/cleanup-rules` | Iniciado sesión | Lista de reglas de limpieza |
| PUT | `/api/v1/log-manage/cleanup-rules/:id` | Iniciado sesión | Actualizar regla de limpieza |
| POST | `/api/v1/log-manage/cleanup-table/:table` | Iniciado sesión | Limpiar la tabla especificada ahora |
| GET | `/api/v1/log-manage/core-log-output` | Iniciado sesión | Configuración de salida del registro principal |
| PUT | `/api/v1/log-manage/core-log-output` | Iniciado sesión | Guardar la configuración de salida del registro principal |
| GET | `/api/v1/log-manage/core/:type/stdout-log` | Iniciado sesión | Leer los registros de salida estándar principales |
| GET | `/api/v1/log-manage/core/:type/stdout-stream` | Iniciado sesión | Salida estándar del núcleo de transmisión |
| POST | `/api/v1/log-manage/discover` | Iniciado sesión | Descubrir archivos de registro de nodos |
| GET | `/api/v1/log-manage/log-output` | Iniciado sesión | Configuración de salida de registro |
| PUT | `/api/v1/log-manage/log-output` | Iniciado sesión | Guardar configuración de salida del registro |
| POST | `/api/v1/log-manage/log-output/release/:type` | Iniciado sesión | Lanzamiento de adquisición de salida estándar del registro |
| POST | `/api/v1/log-manage/log-output/takeover/:type` | Iniciado sesión | Tomar el control de la salida estándar del registro |
| GET | `/api/v1/log-manage/nodes/:serverId/logs` | Iniciado sesión | Lista de archivos de registro de nodos |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type` | Iniciado sesión | Leer registros de nodos |
| POST | `/api/v1/log-manage/nodes/:serverId/logs/:type/rotate` | Iniciado sesión | Rotar registros de nodos |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type/stream` | Iniciado sesión | Registros de nodos de transmisión |
| GET | `/api/v1/log-manage/policies` | Iniciado sesión | Lista de políticas |
| POST | `/api/v1/log-manage/policies` | Iniciado sesión | Crear política |
| DELETE | `/api/v1/log-manage/policies/:id` | Iniciado sesión | Eliminar política |
| GET | `/api/v1/log-manage/policies/:id` | Iniciado sesión | Detalles de la póliza |
| PUT | `/api/v1/log-manage/policies/:id` | Iniciado sesión | Actualizar política |
| GET | `/api/v1/log-manage/policies/:id/jobs` | Iniciado sesión | lista de trabajos de limpieza |
| GET | `/api/v1/log-manage/policies/:id/log` | Iniciado sesión | Leer registros |
| GET | `/api/v1/log-manage/policies/:id/log-stream` | Iniciado sesión | Registros de transmisión |
| POST | `/api/v1/log-manage/policies/:id/rotate` | Iniciado sesión | rotar |
| GET | `/api/v1/log-manage/proxy/:type/stdout-log` | Iniciado sesión | Leer los registros de salida estándar principales |
| GET | `/api/v1/log-manage/proxy/:type/stdout-stream` | Iniciado sesión | Salida estándar del núcleo de transmisión |
| GET | `/api/v1/log-manage/scheduler-config` | Iniciado sesión | Configuración de programación |
| PUT | `/api/v1/log-manage/scheduler-config` | Iniciado sesión | Actualizar configuración de programación |
| GET | `/api/v1/log-manage/status` | Iniciado sesión | Estado |
| POST | `/api/v1/log-manage/sync` | Iniciado sesión | Sincronizar políticas de limpieza |
| GET | `/api/v1/log-manage/writers` | Iniciado sesión | Estado del escritor |
| PUT | `/api/v1/log-manage/writers/:type/config` | Iniciado sesión | Actualizar configuración del escritor |
| GET | `/api/v1/log-manage/writers/:type/files` | Iniciado sesión | Lista de archivos de registro del escritor |
| POST | `/api/v1/log-manage/writers/:type/start` | Iniciado sesión | Empezar escritor |
| POST | `/api/v1/log-manage/writers/:type/stop` | Iniciado sesión | dejar de escribir |

### Copias de seguridad

Base de datos/copias de seguridad completas, restauración, trabajos programados, almacenamiento remoto FTP/S3/WebDAV.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| DELETE | `/api/v1/backups/:id` | administrador | Eliminar copia de seguridad |
| GET | `/api/v1/backups/:id/download` | administrador | Descargar copia de seguridad |
| POST | `/api/v1/backups/:id/restore` | administrador | Restaurar desde copia de seguridad |
| POST | `/api/v1/backups/:id/upload-ftp` | administrador | Subir a FTP |
| POST | `/api/v1/backups/:id/upload-s3` | administrador | Subir a S3 |
| POST | `/api/v1/backups/:id/upload-webdav` | administrador | Subir a WebDAV |
| POST | `/api/v1/backups/create` | administrador | Crear copia de seguridad completa (con volcado de base de datos) |
| GET | `/api/v1/backups/list` | administrador | Lista completa de copias de seguridad |
| POST | `/api/v1/backups/remote-restore` | administrador | Restaurar desde remoto |
| GET | `/api/v1/backups/restore-status` | administrador | Restaurar el progreso |
| GET | `/api/v1/backups/schedules` | administrador | Lista de trabajos programados |
| POST | `/api/v1/backups/schedules` | administrador | Crear trabajo programado |
| DELETE | `/api/v1/backups/schedules/:id` | administrador | Eliminar trabajo programado |
| PUT | `/api/v1/backups/schedules/:id` | administrador | Actualizar trabajo programado |
| GET | `/api/v1/backups/storages` | administrador | Lista de configuración de almacenamiento |
| POST | `/api/v1/backups/storages` | administrador | Crear configuración de almacenamiento |
| DELETE | `/api/v1/backups/storages/:id` | administrador | Eliminar configuración de almacenamiento |
| PUT | `/api/v1/backups/storages/:id` | administrador | Actualizar configuración de almacenamiento |
| GET | `/api/v1/backups/storages/:id/files` | administrador | Lista de archivos de copia de seguridad remota |
| POST | `/api/v1/backups/upload-restore` | administrador | Cargar copia de seguridad y restaurar |

### Configuración del sistema

Configuración y migración de bases de datos, configuraciones del panel, claves JWT, pruebas SMTP/Telegram, plantillas de notificación, actualizaciones en línea (principalmente administrativas).

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| PUT | `/api/v1/system/bootstrap-finish` | administrador | Complete la configuración del primer arranque |
| GET | `/api/v1/system/bootstrap-state` | administrador | Estado del primer arranque |
| GET | `/api/v1/system/config-file/:coreType` | administrador | Leer el archivo de configuración principal |
| GET | `/api/v1/system/db-config` | administrador | Configuración de base de datos |
| PUT | `/api/v1/system/db-config` | administrador | Actualizar la configuración de la base de datos |
| POST | `/api/v1/system/db-config/migrate` | administrador | Migrar datos de base de datos |
| POST | `/api/v1/system/db-config/switch` | administrador | Cambiar de base de datos (con migración en un solo paso) |
| POST | `/api/v1/system/db-config/test` | administrador | Probar la conexión de la base de datos |
| GET | `/api/v1/system/db-pool-stats` | administrador | Estadísticas del grupo de conexiones |
| GET | `/api/v1/system/db-stats` | administrador | Estadísticas de la base de datos |
| GET | `/api/v1/system/db-stats/ping` | administrador | Sonda de base de datos |
| POST | `/api/v1/system/db-stats/reset` | administrador | Restablecer estadísticas de la base de datos |
| GET | `/api/v1/system/encryption-key-status` | administrador | Estado de la clave de cifrado |
| GET | `/api/v1/system/env-config` | administrador | configuración .env |
| PUT | `/api/v1/system/env-config` | administrador | Actualizar la configuración .env |
| POST | `/api/v1/system/generate-encryption-key` | administrador | Generar clave de cifrado |
| GET | `/api/v1/system/github-config` | administrador | Configuración GitHub |
| PUT | `/api/v1/system/github-config` | administrador | Actualizar la configuración de GitHub |
| GET | `/api/v1/system/jwt-secret-status` | administrador | Estado de la clave JWT |
| GET | `/api/v1/system/maintenance` | Público | Estado del modo de mantenimiento (público) |
| PUT | `/api/v1/system/maintenance` | administrador | Alternar modo de mantenimiento |
| POST | `/api/v1/system/migrate-data` | administrador | Migrar datos |
| GET | `/api/v1/system/notification-config` | administrador | Configuración de notificación |
| PUT | `/api/v1/system/notification-config` | administrador | Actualizar configuración de notificación |
| GET | `/api/v1/system/notification-templates` | administrador | Lista de plantillas |
| POST | `/api/v1/system/notification-templates` | administrador | Crear plantilla |
| DELETE | `/api/v1/system/notification-templates/:id` | administrador | Eliminar plantilla |
| PUT | `/api/v1/system/notification-templates/:id` | administrador | Actualizar plantilla |
| POST | `/api/v1/system/notification-templates/:id/broadcast` | administrador | Notificación de difusión por plantilla |
| POST | `/api/v1/system/notification-templates/:id/send` | administrador | Enviar notificación de prueba |
| GET | `/api/v1/system/panel-config` | administrador | Configuración del panel |
| PUT | `/api/v1/system/panel-config` | administrador | Actualizar configuración del panel |
| GET | `/api/v1/system/reality-targets` | administrador | Lista de objetivos REALITY |
| PUT | `/api/v1/system/reality-targets` | administrador | Actualizar objetivo REALITY |
| POST | `/api/v1/system/reality-targets/reset` | administrador | Restablecer el objetivo REALITY a sus valores predeterminados |
| POST | `/api/v1/system/regenerate-jwt-secret` | administrador | Regenerar clave JWT |
| GET | `/api/v1/system/releases` | administrador | Versiones disponibles |
| POST | `/api/v1/system/restart` | administrador | Reiniciar |
| POST | `/api/v1/system/rollback` | administrador | Reversión del sistema |
| GET | `/api/v1/system/settings` | administrador | Leer configuración |
| PUT | `/api/v1/system/settings/batch` | administrador | Configuración de actualización masiva |
| GET | `/api/v1/system/stats` | administrador | Estado del sistema |
| GET | `/api/v1/system/subscription-billing-templates` | administrador | Plantilla de facturación de suscripción |
| PUT | `/api/v1/system/subscription-billing-templates` | administrador | Actualizar plantilla de facturación de suscripción |
| POST | `/api/v1/system/telegram-chat-id` | administrador | Obtener ID de chat Telegram |
| POST | `/api/v1/system/test-smtp` | administrador | Prueba SMTP |
| POST | `/api/v1/system/test-telegram` | administrador | Prueba Telegram |
| POST | `/api/v1/system/update` | administrador | Actualización del sistema |
| GET | `/api/v1/system/update-jobs` | administrador | Actualizar lista de trabajos |
| POST | `/api/v1/system/upload-mtls-ca` | administrador | Cargar certificado mTLS CA |

### Auditoría de seguridad

Auditar consultas de eventos y exportar.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/security/audit-events` | Iniciado sesión | Lista de eventos de auditoría |
| GET | `/api/v1/security/audit-events/export` | Iniciado sesión | Exportar eventos de auditoría |

### Métricas

Métricas agregadas del panel (nodos/certificados/servicios).

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/metrics/certificates` | Iniciado sesión | Métricas de certificado |
| GET | `/api/v1/metrics/nodes` | Iniciado sesión | Métricas de nodo |
| GET | `/api/v1/metrics/overview` | Iniciado sesión | Descripción general |

### Plantillas de portales

Plantillas de portal, configuración de activación y gestión de caché.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/portal/active` | Iniciado sesión | Plantilla activa |
| PUT | `/api/v1/portal/active` | Iniciado sesión | Actualizar plantilla activa |
| DELETE | `/api/v1/portal/cache` | Iniciado sesión | Borrar caché del portal |
| GET | `/api/v1/portal/cache` | Iniciado sesión | Estado de caché |
| PUT | `/api/v1/portal/cache/dev-mode` | Iniciado sesión | Alternar modo de desarrollo de caché |
| POST | `/api/v1/portal/cache/rebuild` | Iniciado sesión | Reconstruir la caché del portal |
| POST | `/api/v1/portal/reset` | Iniciado sesión | Restaurar plantilla predeterminada |
| GET | `/api/v1/portal/templates` | Iniciado sesión | Lista |
| POST | `/api/v1/portal/templates` | Iniciado sesión | crear |
| DELETE | `/api/v1/portal/templates/:id` | Iniciado sesión | Eliminar |
| GET | `/api/v1/portal/templates/:id` | Iniciado sesión | Detalles |
| PUT | `/api/v1/portal/templates/:id` | Iniciado sesión | Actualizar |
| POST | `/api/v1/portal/templates/:id/activate` | Iniciado sesión | Activar plantilla |

> `/portal/templates` CRUD es un **API avanzado**: la interfaz de usuario del portal utiliza principalmente `PUT /api/v1/portal/active`
> (configuración de sitio completo de una sola vez); administrar/cambiar múltiples plantillas con este grupo. Ambos son compatibles y no entran en conflicto.

### Instalación del agente

Script de instalación y descarga del Agente de Nodo (público).

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| POST | `/api/v1/agent/bootstrap` | Público | Registro del primer inicio del agente (el agente informa con un token de inicio; público) |
| GET | `/api/v1/agent/releases` | administrador | Versiones de agente disponibles (para la página de actualización) |
| GET | `/api/v1/agent/version` | administrador | Versión actual del Agente en el panel |
| GET | `/api/v1/agent/download` | Público | Descargar Agente |
| GET | `/api/v1/agent/install` | Público | Obtener script de instalación |

### Entrada de devolución de llamada de cargo

Devolución de llamada de plataforma de pago de terceros (pública, firma verificada).

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| POST | `/api/v1/charge/callback/:site_id` | Público | Manejar devolución de llamada de cargo |

### Entradas (con nodos)

Lista entrante utilizada para la selección de nodos del plan.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/inbounds-with-server` | Iniciado sesión | Todos los entrantes (incluidos sus nodos) |

### Configuración pública

Configuración del tiempo de ejecución del portal sin inicio de sesión (guardia de mantenimiento + límite de tarifa pública).

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/public/articles` | Público | Lista de artículos públicos |
| GET | `/api/v1/public/articles/:slug` | Público | Detalles del artículo público |
| GET | `/api/v1/public/client-configs` | Público | Configuración de descarga del cliente (pública) |
| GET | `/api/v1/public/portal-config` | Público | Configuración del tiempo de ejecución del portal (nombre del sitio, tema, idiomas, documentos de cumplimiento; público) |
| GET | `/api/v1/public/settings` | Público | Configuraciones públicas (por ejemplo, registro_enabled; público) |
| GET | `/api/v1/public/subscription-endpoints` | Público | Lista de puntos finales de suscripción habilitados (públicos) |

### Autenticación de cliente y negocio (la misma configuración en el motor de administración)

Puntos finales del cliente idénticos al portal; consulte `portal-api.en.md` para obtener más detalles.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/api/v1/client-auth/articles` | Público | Lista de artículos |
| GET | `/api/v1/client-auth/articles/:slug` | Público | Obtener artículo |
| GET | `/api/v1/client-auth/balance` | Público | obtener saldo |
| GET | `/api/v1/client-auth/balance/transactions` | Público | Transacciones de saldo |
| GET | `/api/v1/client-auth/bills` | Público | lista de pedidos |
| POST | `/api/v1/client-auth/change-password` | Público | Cambiar contraseña |
| POST | `/api/v1/client-auth/charge/create` | Público | Crear orden de recarga |
| GET | `/api/v1/client-auth/charge/orders` | Público | Listar pedidos de recarga |
| GET | `/api/v1/client-auth/charge/orders/:id` | Público | Obtener detalles del pedido de recarga |
| POST | `/api/v1/client-auth/charge/redeem-card` | Público | Canjear tarjeta |
| GET | `/api/v1/client-auth/charge/sites` | Público | Listar sitios de recarga |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | Público | Consultar estado de recarga |
| GET | `/api/v1/client-auth/checktime` | Público | Obtener la hora del servidor (sincronización del reloj) |
| GET | `/api/v1/client-auth/connection-info` | Público | Obtener parámetros de conexión del plan |
| GET | `/api/v1/client-auth/gifts` | Público | Lista de regalos |
| POST | `/api/v1/client-auth/gifts/:id/claim` | Público | Reclamar regalo |
| GET | `/api/v1/client-auth/invoices` | Público | lista de pedidos |
| GET | `/api/v1/client-auth/invoices/:id` | Público | Detalles del pedido |
| GET | `/api/v1/client-auth/ip-whitelist` | Público | Lista de IP permitidas |
| DELETE | `/api/v1/client-auth/ip-whitelist/:id` | Público | Eliminar de la lista de IP permitidas |
| POST | `/api/v1/client-auth/ip-whitelist/auto-add` | Público | Agregar IP automáticamente |
| POST | `/api/v1/client-auth/login` | Público | Iniciar sesión |
| POST | `/api/v1/client-auth/logout` | Público | Cerrar sesión |
| POST | `/api/v1/client-auth/logs/package` | Público | Crear paquete de registros |
| GET | `/api/v1/client-auth/logs/packages` | Público | Listar paquetes de registros |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | Público | Eliminar paquete de registros |
| GET | `/api/v1/client-auth/logs/packages/:id` | Público | Obtener paquete de registros |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | Público | Descargar paquete de registros |
| GET | `/api/v1/client-auth/logs/types` | Público | Comprobación del estado del paquete de registros |
| GET | `/api/v1/client-auth/me` | Público | Obtener el usuario registrado actualmente |
| GET | `/api/v1/client-auth/nodes` | Público | Mi lista de nodos |
| GET | `/api/v1/client-auth/nodes/:id/config` | Público | Configuración de nodo |
| GET | `/api/v1/client-auth/nodes/:id/link` | Público | Enlace para compartir nodo |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | Público | Código QR del nodo |
| GET | `/api/v1/client-auth/oauth/:provider` | Público | Autorizar |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | Público | Devolución de llamada |
| GET | `/api/v1/client-auth/oauth/providers` | Público | Estado del proveedor |
| GET | `/api/v1/client-auth/orders` | Público | lista de pedidos |
| POST | `/api/v1/client-auth/orders` | Público | Crear pedido |
| GET | `/api/v1/client-auth/orders/:id` | Público | Detalles del pedido |
| GET | `/api/v1/client-auth/plans` | Público | Lista de planes |
| GET | `/api/v1/client-auth/plans/:id` | Público | Detalles del plano |
| GET | `/api/v1/client-auth/profile` | Público | Obtener perfil |
| PUT | `/api/v1/client-auth/profile` | Público | Actualizar perfil |
| POST | `/api/v1/client-auth/profile/bind-oauth` | Público | Vincular cuenta de terceros |
| DELETE | `/api/v1/client-auth/profile/unbind-oauth/:provider` | Público | Desvincular cuenta de terceros |
| GET | `/api/v1/client-auth/referral/code` | Público | Obtener código de verificación de referencia |
| GET | `/api/v1/client-auth/referral/records` | Público | Listar registros de referencia |
| GET | `/api/v1/client-auth/referral/stats` | Público | Obtener estadísticas de referencias |
| POST | `/api/v1/client-auth/referral/withdraw` | Público | Solicitar retiro |
| GET | `/api/v1/client-auth/referral/withdrawals` | Público | Listar retiros de referencias |
| POST | `/api/v1/client-auth/refresh` | Público | Actualizar |
| POST | `/api/v1/client-auth/register` | Público | Registrarse |
| POST | `/api/v1/client-auth/reset-subscription` | Público | Restablecer suscripción |
| POST | `/api/v1/client-auth/rotate-token` | Público | Rotar token de suscripción |
| POST | `/api/v1/client-auth/self-unban` | Público | Desbanificación de autoservicio |
| PUT | `/api/v1/client-auth/settings` | Público | Actualizar configuración |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | Público | Agregar IP automáticamente |
| GET | `/api/v1/client-auth/settings/whitelist` | Público | Lista de IP permitidas |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | Público | Eliminar de la lista de IP permitidas |
| GET | `/api/v1/client-auth/shared-accounts` | Público | Listar cuentas compartidas |
| GET | `/api/v1/client-auth/shared-accounts/:id` | Público | Obtener cuenta compartida |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | Público | Solicitar cuenta compartida |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | Público | Solicitud de reparación |
| POST | `/api/v1/client-auth/ticket` | Público | Emitir un billete único de corta duración |
| GET | `/api/v1/client-auth/tickets` | Público | Listar entradas |
| POST | `/api/v1/client-auth/tickets` | Público | Crear ticket |
| GET | `/api/v1/client-auth/tickets/:id` | Público | Obtener boleto |
| POST | `/api/v1/client-auth/tickets/:id/close` | Público | Cerrar ticket |
| POST | `/api/v1/client-auth/tickets/:id/reply` | Público | Responder al ticket |
| GET | `/api/v1/client-auth/traffic-history` | Público | Historial de tráfico |
| GET | `/api/v1/client-auth/traffic-packs` | Público | Lista de paquetes de tráfico |
| POST | `/api/v1/client-auth/traffic-packs/buy` | Público | Comprar paquete de tráfico |

### Entrega de suscripción

Punto de entrada público para que los clientes obtengan suscripciones; ver `subscription.en.md`.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/sub/:token` | Público | Servir suscripción |

### Flujo de trabajo de configuración en tiempo de ejecución

Instantáneas de configuración/ejecución en seco/aplicar/revertir (`apply`, `rollback` y `disable-proxy` requieren administrador).

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| POST | `/runtime/apply` | Público | Aplicar |
| GET | `/runtime/apply-jobs` | Iniciado sesión | Listar trabajos |
| GET | `/runtime/apply-jobs/:id` | Iniciado sesión | conseguir trabajo |
| POST | `/runtime/apply-jobs/:id/cancel` | Público | Cancelar trabajo |
| POST | `/runtime/apply-jobs/:id/retry` | Público | Reintentar trabajo |
| POST | `/runtime/disable-proxy` | Público | Deshabilitar proxy |
| POST | `/runtime/dry-run` | Iniciado sesión | Ejecución en seco |
| GET | `/runtime/nodes` | Iniciado sesión | Listar nodos |
| GET | `/runtime/overview` | Iniciado sesión | Descripción general |
| POST | `/runtime/rollback` | Público | retroceder |
| GET | `/runtime/snapshots` | Iniciado sesión | Listar instantáneas |
| POST | `/runtime/snapshots` | Iniciado sesión | Crear instantánea |
| DELETE | `/runtime/snapshots/:id` | Público | Eliminar instantánea |
| GET | `/runtime/snapshots/:id` | Iniciado sesión | Obtener instantánea |
| POST | `/runtime/verify` | Iniciado sesión | verificar |

### Políticas de acceso al servicio (retiradas)

> `/services/:id/access-policies` y `/services/:id/ip-bans` se retiraron el 20/10/2026:
> no hay llamadas de administrador y ninguna ruta de conexión consume estas políticas.

### Monitoreo de cuota de tráfico

Vista de monitoreo de solo lectura de cuotas y uso.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/traffic/quotas` | Iniciado sesión | lista de cuotas |
| GET | `/traffic/quotas/:id/usage` | Iniciado sesión | Uso de cuota |
| GET | `/traffic/reset-logs` | Iniciado sesión | Registros de restablecimiento de cuotas |
| GET | `/traffic/usage-history` | Iniciado sesión | Historial de uso |

### Controles de salud

Sonda de vida.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/health` | Público | Sonda de vida; 200 significa saludable |

### página del portal

Página del Portal SPA.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/p` | Público | Portal HTML (renderizado por servidor + tiempo de ejecución de Vue) |

### Desafíos ACME

Devolución de llamada de desafío HTTP-01.

| Método | Camino | Permiso | Descripción |
|---|---|---|---|
| GET | `/.well-known/acme-challenge/:token` | Público | ACME HTTP-01 devolución de llamada de desafío (verifica el dominio durante la emisión) |
