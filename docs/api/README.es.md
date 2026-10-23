<!-- source: README.md | locale: es | updated: 2026-10-23 -->
# Documentación CoreFusion API

> Idiomas: [简体中文](README.md) · [English](README.en.md) · [繁體中文](README.zh-TW.md) · [Русский](README.ru.md) · [日本語](README.ja.md) · [한국어](README.ko.md) · [Deutsch](README.de.md) · [Français](README.fr.md) · Español · [Português](README.pt.md) · [Türkçe](README.tr.md) · [Bahasa Indonesia](README.id.md) · [ไทย](README.th.md) · [Tiếng Việt](README.vi.md) · [العربية](README.ar.md) · [فارسی](README.fa.md)

Documentación API para **integradores externos, desarrollo secundario y clientes de terceros**. Todas las rutas son relativas al dominio de implementación.

| Documento | Audiencia | Contenido |
|---|---|---|
| [subscription.md](./subscription.es.md) | Suscriptores / desarrolladores de clientes | Punto final de suscripción (`/sub/:token`): formatos, encabezados, códigos de estado, ejemplos |
| [portal-api.md](README.es.md) | Frontend del portal/clientes de terceros | Portal de clientes API (`/api/v1/client-auth/*`, `/api/v1/public/*`): registro, inicio de sesión, planes, pedidos, saldo, tickets |
| [charge-callback.md](./charge-callback.es.md) | Proveedores de pago | Firma y verificación de devolución de llamada de pago |
| [admin-api.es.md](./admin-api.es.md) | Desarrollo de paneles / automatización | Tabla de rutas API de administración completa (`/api/v1/*`, agrupada por módulo) |

---

## 1. Puertos y URL base

El panel puede exponer tres motores al mismo tiempo, compartiendo una base de datos:

| motor | Acceso predeterminado | Propósito |
|---|---|---|
| panel principal | `https://<domain>/api/v1` | Backend de administración + portal + suscripciones |
| Motor de suscripción (puerto dedicado) | `https://<domain>:<port>/:token` | Recuperación de suscripción de alta concurrencia, sin prefijo `/sub` |
| Motor de portal (puerto dedicado) | `https://<portal-domain>[:port]/api/v1` | Portal exclusivo para clientes, páginas en `/` |

> El proxy inverso debe reenviar ambas rutas de página y `/api/v1`; Es un honor para `X-Forwarded-Proto` / `X-Forwarded-Host` restaurar el esquema y el dominio reales.

## 2. Autenticación (tres tipos, no intercambiables)

| autenticación | Credencial | Se aplica a |
|---|---|---|
| **Autenticación de administrador** | `Authorization: Bearer <admin_token>` (desde `/api/v1/auth/login`) | Administrador API |
| **Autenticación de cliente** | `Authorization: Bearer <client_token>` (desde `/api/v1/client-auth/login`) | Portal API |
| **Ficha de suscripción** | Ficha en el URL: `/sub/<token>` | Recuperación de suscripción, sin encabezado |

- Los tokens de acceso al cliente duran 24 horas, los tokens de actualización duran 7 días; el JWT contiene `tv` (token_version), por lo que el cierre de sesión/cierre de sesión forzado invalida todos los tokens antiguos.
- Funciones de administrador: `admin` / `operator` / `super_admin`; algunas operaciones de escritura requieren `RequireRole(RoleAdmin)` o superior.

## 3. Formato de respuesta

Éxito (la mayoría de los puntos finales):

```json
{ "code": 0, "data": { ... }, "message": "ok" }
```

Los puntos finales de la lista (algunos) devuelven `{ "items": [...], "total": N }` o una matriz básica (indicada por punto final a continuación).

Errores (HTTP 4xx/5xx):

```json
{ "code": "ERROR_CODE", "message": "human-readable description", "request_id": "…", "can_retry": false }
```

**Las cantidades son números enteros en centavos**: `990` = ¥9,90 (`currency: "CNY"`).
**El tráfico está en bytes**: tanto las solicitudes como las respuestas utilizan bytes.

## 4. Límites de tarifa y modo de mantenimiento

- Los puntos finales públicos (`/api/v1/public/*`, `/sub/*`, registro/inicio de sesión) tienen una tarifa limitada;
- El inicio de sesión también usa `LoginRateLimit`, la recuperación de suscripción usa `SubscribeRateLimit`;
- En el modo de mantenimiento, los puntos finales públicos y las suscripciones devuelven un mensaje de mantenimiento; el administrador API no se ve afectado.

## 5. Rutas de la página frontal (no API)

Estas rutas devuelven recursos HTML/estáticos y están fuera de los documentos API; enumerados para que puedan excluirse durante la integración:

| Método | Camino | Descripción |
|---|---|---|
| GET | `/` | Panel de administración SPA (después de iniciar sesión) |
| GET | `/p` | Portal HTML (`/` en un motor de portal dedicado) |
| GET | `/:token` | Recuperación de suscripción en un puerto dedicado (sin prefijo `/sub`, misma semántica que `/sub/:token`) |
| GET | `/static/:file` | Activos estáticos de front-end |

## 6. fuente de verdad

`internal/api/router/router.go` (746 rutas, 743 documentadas). La tabla de rutas en [admin-api.md](README.es.md) se genera a partir de código; si no está de acuerdo con el código, **el código gana**.
