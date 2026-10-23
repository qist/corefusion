<!-- source: portal-api.md | locale: es | updated: 2026-10-23 -->
# Referencia Portal API (Cliente/Portal API)

> Idiomas: [简体中文](portal-api.md) · [English](portal-api.en.md) · [繁體中文](portal-api.zh-TW.md) · [Русский](portal-api.ru.md) · [日本語](portal-api.ja.md) · [한국어](portal-api.ko.md) · [Deutsch](portal-api.de.md) · [Français](portal-api.fr.md) · Español · [Português](portal-api.pt.md) · [Türkçe](portal-api.tr.md) · [Bahasa Indonesia](portal-api.id.md) · [ไทย](portal-api.th.md) · [Tiếng Việt](portal-api.vi.md) · [العربية](portal-api.ar.md) · [فارسی](portal-api.fa.md)

> La referencia de interfaz **completa** para la interfaz del portal (`/p` o un puerto de portal dedicado) y clientes de terceros.
> Definiciones de ruta: `internal/api/router/router.go` (`clientAuthGroup` / `clientAuthProtected` / `/api/v1/public/*`).

## 1. Convenciones generales

### 1.1 Base URL

| Implementación | Base URL |
|---|---|
| motor del panel principal | `https://<panel-domain>/api/v1` (página del portal en `/p`) |
| Puerto de portal dedicado | `https://<portal-domain>[:port]/api/v1` (página del portal en `/`) |
| Dominio de portal dedicado + proxy inverso | Igual que el anterior; el proxy debe reenviar ambas rutas de página y `/api/v1` |

### 1.2 Autenticación

- Después de iniciar sesión, envíe `Authorization: Bearer <access_token>`;
- Los tokens de acceso duran 24 horas, los tokens de actualización duran 7 días;
- El JWT contiene `tv` (token_version): el cierre de sesión/cierre de sesión forzado lo incrementa e invalida todos los tokens antiguos.
 (`clients.token_version`, ver `internal/service/clientauth/service.go`);
- Cada punto final protegido pasa por `ClientAuthMiddleware` (verificación de versión + protección en modo de mantenimiento).

### 1.3 Sobre de respuesta

La mayoría de los puntos finales devuelven `{ "code": 0, "data": ..., "message": "..." }`;
algunos puntos finales de la lista devuelven `{ "items": [...], "total": N }` o una matriz simple (indicada por punto final a continuación).
Los errores devuelven `{ "code": "ERROR_CODE", "message": "...", "request_id": "...", "can_retry": ... }`
(HTTP 4xx/5xx).

### 1.4 Unidades de cantidad

Los campos de orden/saldo son **centavos enteros**: `990` = ¥9,90 (`currency: "CNY"`).

### 1.5 Ejemplo real (iniciar sesión → buscar perfil)

```bash
# Register (when registration_enabled=true)
curl -X POST https://<portal-domain>/api/v1/client-auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"plaintext-password","referral_code":""}'
# → 201 {"data":{"access_token":"<JWT>","refresh_token":"<JWT>","client":{...}}}

# Sign in
curl -X POST https://<portal-domain>/api/v1/client-auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"plaintext-password"}'
# → 200 same as above

# Fetch the current client profile (with token)
curl https://<portal-domain>/api/v1/client-auth/me \
  -H 'Authorization: Bearer <access_token>'
# → 200 {"data":{"id":27,"email":"user@example.com","nickname":"","status":"active","quota_bytes":10737418240,"used_bytes":0,...}}

# Fetch a subscription (no header, token in the path)
curl 'https://<portal-domain>/sub/<subscription_token>?format=mihomo_yaml'
# → 200 Mihomo YAML (or base64_uri / singbox_json / xray_json)

# Error example: expired token
# → 401 {"code":"errClientTokenRevoked","message":"...","request_id":"...","can_retry":false}
```

Al 401, llame a `POST /api/v1/client-auth/refresh` una vez y vuelva a intentarlo; si aún falla, borre la sesión local (la plantilla del portal ya hace esto).

---

## 2. Puntos finales públicos (sin inicio de sesión)

| Método | Camino | Descripción |
|---|---|---|
| GET | `/p` (`/` en un motor de portal dedicado) | Portal HTML (renderizado por servidor + tiempo de ejecución de Vue) |
| GET | `/api/v1/public/portal-config` | Configuración del tiempo de ejecución del portal: nombre del sitio, colores del tema, navegación, módulos, paquetes de idiomas, `registration_enabled`, `regions` (fuente de región de inicio de sesión), `privacy_policy_url` / `terms_of_service_url` / `privacy_policy_content` / `terms_of_service_content` (documentos de cumplimiento; contenido asignado por idioma; páginas independientes `/p/legal/*` ✓) |
| GET | `/api/v1/public/settings` | Configuraciones públicas (actualmente `registration_enabled`) |
| GET | `/api/v1/public/subscription-endpoints` | Lista de puntos finales de suscripción (matriz básica) |
| GET | `/api/v1/public/articles` | Artículos públicos (solo publicados) |
| GET | `/api/v1/public/articles/:slug` | Detalle del artículo |
| GET | `/api/v1/public/client-configs` | Configuración de descarga del cliente |
| GET | `/sub/:token` | Entrega de suscripción (`format` devuelve base64_uri / raw_uri / mihomo_yaml / singbox_json / xray_json, etc.; revocada/faltante → 404, suspendida/vencida/inactiva → 403. Consulte [subscription.md](portal-api.es.md)) |
| GET | `/api/v1/client-auth/oauth/providers` | Visibilidad del proveedor de OAuth: `{"data":{"providers":[{"provider":"github","enabled":true},...]}}` |
| GET | `/api/v1/system/maintenance` | Estado de mantenimiento (público). El portal muestra una página de mantenimiento cuando está habilitado |

> Todos estos incluyen el guardia de mantenimiento y la limitación de tarifa pública API.

## 3. Autenticación/OAuth

| Método | Camino | autenticación | Descripción |
|---|---|---|---|
| POST | `/api/v1/client-auth/register` | — | Regístrese. Carrocería: `{email, password, referral_code?}`. Devuelve `{access_token, refresh_token, client}`; tasa de registro limitada |
| POST | `/api/v1/client-auth/login` | — | Iniciar sesión. Cuerpo: `{email, password}`. Misma respuesta; emite JWT con `token_version` |
| POST | `/api/v1/client-auth/refresh` | — | Actualizar. Carrocería: `{refresh_token}`. Devuelve nuevos tokens de acceso/actualización (versión verificada; los tokens de actualización antiguos mueren inmediatamente después de cerrar sesión) |
| POST | `/api/v1/client-auth/logout` | ✓ | Cerrar sesión: revocación del lado del servidor (`token_version +1`) → todos los tokens antiguos para el cliente no son válidos |
| POST | `/api/v1/client-auth/rotate-token` | ✓ | Rotar el token de suscripción |
| POST | `/api/v1/client-auth/ticket` | ✓ | Emitir un **ticket único de corta duración** (para flujos `window.open` como "descargar paquete de registros" donde no se pueden configurar encabezados) |
| GET | `/api/v1/client-auth/oauth/:provider` | — | Iniciar OAuth: 307 → GitHub/Google consentimiento (`state` anti-CSRF, 10 minutos); proveedores discapacitados regresan 400 |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | — | Devolución de llamada de OAuth: intercambiar código → crear/vincular cliente → emitir JWT versionado → 302 de regreso al portal (`/<portalPath>?access_token=...&refresh_token=...`) |

> La devolución de llamada de OAuth es una **redirección del navegador** (no una llamada API): después de un inicio de sesión exitoso, el portal lee el token
> desde URL en `onMounted` (o mediante `postMessage` del mismo origen, consulte `handleOAuthCallback` en la plantilla del portal `web`).

## 4. cuenta

| Método | Camino | Descripción |
|---|---|---|
| GET | `/api/v1/client-auth/me` | Cliente actual: `{id, email, plan_id, plan_name, status, quota_bytes, used_bytes, upload_bytes, download_bytes, expires_at, ip_whitelist, device_limit, sub_token, sub_status, created_at, ...}` (sobre `data`) |
| POST | `/api/v1/client-auth/change-password` | Cuerpo: `{old_password, new_password}` |
| GET | `/api/v1/client-auth/checktime` | Hora del servidor (sincronización del reloj del cliente) |
| GET | `/api/v1/client-auth/connection-info` | Parámetros de conexión del plan actual: `{concurrent, device_limit, protocol, speed_limit, tls, transport}` |

## 5. Suscripciones

| Método | Camino | Descripción |
|---|---|---|
| POST | `/api/v1/client-auth/reset-subscription` | Restablecer la suscripción; esto gira `sub_token` |

> El enlace de suscripción está creado en la interfaz de `sub_token` devuelto por `/me` (`/sub/:token`); no hay
> separar los puntos finales `subscription-link` / `subscription-formats`.

## 6. Nodos

| Método | Camino | Descripción |
|---|---|---|
| GET | `/api/v1/client-auth/nodes` | Nodos visibles para el plan actual: `{items:[...]}` (campos de visualización como nombre/protocolo/transporte/región) |
| GET | `/api/v1/client-auth/nodes/:id/config` | Configuración completa del nodo |
| GET | `/api/v1/client-auth/nodes/:id/link` | Compartir enlace (vless://, etc.) |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | Imagen de código QR |

## 7. Tienda/planes/pedidos

| Método | Camino | Descripción |
|---|---|---|
| GET | `/api/v1/client-auth/plans` | Lista de planes (matriz desnuda): `{id, name, description, category, sort_order, variants:[{id, name, billing_cycle, duration_days, price(cents), original_price, currency, quota_bytes, ...}]}` |
| GET | `/api/v1/client-auth/plans/:id` | Detalle del plano |
| GET | `/api/v1/client-auth/traffic-packs` | Lista de paquetes de tráfico |
| POST | `/api/v1/client-auth/traffic-packs/buy` | Compra un paquete de tráfico |
| GET | `/api/v1/client-auth/orders` | Mis pedidos del plan (paginados) |
| POST | `/api/v1/client-auth/orders` | Crear un pedido (comprar/renovar una variante del plan) |
| GET | `/api/v1/client-auth/orders/:id` | Detalle del pedido |
| GET | `/api/v1/client-auth/invoices` | Facturas |
| GET | `/api/v1/client-auth/invoices/:id` | Detalle de factura |

## 8. Saldo/recarga

| Método | Camino | Descripción |
|---|---|---|
| GET | `/api/v1/client-auth/balance` | `{data:{client_id, balance(cents), frozen}}` |
| GET | `/api/v1/client-auth/balance/transactions` | Transacciones de saldo (`type`: cargo/consumo/reembolso; paginado) |
| GET | `/api/v1/client-auth/charge/sites` | Sitios de recarga disponibles (`ListChargeSites`: solo habilitado) |
| POST | `/api/v1/client-auth/charge/create` | Crear una orden de recarga (redirecciona al pago de terceros) |
| GET | `/api/v1/client-auth/charge/orders` | Lista de pedidos de recarga |
| GET | `/api/v1/client-auth/charge/orders/:id` | Detalle del pedido de recarga |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | Estado de pago de la encuesta por trade_no |
| POST | `/api/v1/client-auth/charge/redeem-card` | Canjear una tarjeta de recarga |

> La eliminación de un cliente **en cascada** en su saldo y transacciones (consulte `DeleteClient` en `internal/service/client/service.go`).

## 9. Regalos / historial de tráfico

| Método | Camino | Descripción |
|---|---|---|
| GET | `/api/v1/client-auth/gifts` | Obsequios reclamables (matriz básica; `type`: días/bytes) |
| POST | `/api/v1/client-auth/gifts/:id/claim` | Reclamar |
| GET | `/api/v1/client-auth/traffic-history` | Historial de tráfico mensual: `{data:{history:[{period_start,total_bytes,...}], months}}` |

## 10. Entradas

| Método | Camino | Descripción |
|---|---|---|
| GET | `/api/v1/client-auth/tickets` | Lista de entradas: `{items,total}` |
| POST | `/api/v1/client-auth/tickets` | Crear un billete |
| GET | `/api/v1/client-auth/tickets/:id` | Detalle del ticket (con respuestas) |
| POST | `/api/v1/client-auth/tickets/:id/reply` | Responder |
| POST | `/api/v1/client-auth/tickets/:id/close` | Cerrar |

## 11. Artículos / guías

| Método | Camino | Descripción |
|---|---|---|
| GET | `/api/v1/client-auth/articles` | Lista de artículos (con filtro de categorías) |
| GET | `/api/v1/client-auth/articles/:slug` | Detalle del artículo |

## 12. Referencias (deshabilitadas de forma predeterminada; se pueden habilitar en el editor del portal)

| Método | Camino | Descripción |
|---|---|---|
| GET | `/api/v1/client-auth/referral/code` | Mi código de referencia y enlace |
| GET | `/api/v1/client-auth/referral/stats` | Estadísticas de comisiones (`commission_rate`, `available_commission`, `min_withdraw_amount`) |
| GET | `/api/v1/client-auth/referral/records` | Registros de referencia |
| GET | `/api/v1/client-auth/referral/withdrawals` | Registros de retiro |
| POST | `/api/v1/client-auth/referral/withdraw` | Solicitar un retiro |

## 13. Cuentas compartidas

| Método | Camino | Descripción |
|---|---|---|
| GET | `/api/v1/client-auth/shared-accounts` | Cuentas compartidas disponibles (streaming, etc.) |
| GET | `/api/v1/client-auth/shared-accounts/:id` | Detalle |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | Solicitar acceso |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | Reparación con un clic |

## 14. Paquetes de registros (diagnóstico de abonado)

| Método | Camino | Descripción |
|---|---|---|
| GET | `/api/v1/client-auth/logs/types` | Tipos de registros que se pueden agrupar |
| GET | `/api/v1/client-auth/logs/packages` | Mis paquetes de registros |
| POST | `/api/v1/client-auth/logs/package` | Crear un trabajo de agrupación de registros (solo nodos de entrada) |
| GET | `/api/v1/client-auth/logs/packages/:id` | Estado del trabajo (listo/procesando/fallido) |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | Descargar (prefiera `POST /ticket` y agregue `?ticket=`) |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | Eliminar un paquete de registros |

## 15. Configuración de seguridad/lista de IP permitidas/autodesbloqueo

| Método | Camino | Descripción |
|---|---|---|
| GET/PUT | `/api/v1/client-auth/settings` | Configuración del cliente: `{ip_whitelist, device_limit}` (PUT) |
| GET | `/api/v1/client-auth/settings/whitelist` | Lista de permitidos (matriz básica) |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | Eliminar una entrada de la lista de permitidos |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | Agregue la IP actual a la lista de permitidos |
| POST | `/api/v1/client-auth/self-unban` | Autoservicio para desbloquear (intentos limitados por la política) |

> La lista de direcciones IP permitidas **solo se aplica a las URL de suscripción** (`/sub/:token`); no bloquea el inicio de sesión en el portal/OAuth o
> otros puntos finales de autenticación de cliente. Si el usuario no tiene lista de permitidos o no tiene suscripción URL, el acceso siempre está permitido. el
> la lista de permitidos es una configuración por usuario; no hay un cambio global.

**Flujo de autodesbloqueo**: disponible solo para `auto_quota` (prohibición automática de exceso de tráfico) cuando el lado administrador
`self_unban.enabled` está encendido. El inicio de sesión (contraseña o devolución de llamada de OAuth) devuelve `require_self_unban=true` + `self_unban_token`
(typ=self_unban, válido por 15 minutos, **solo** utilizable en `POST /self-unban`; otros puntos finales devuelven 403/`SELF_UNBAN_TOKEN_SCOPE`).
Luego, el portal muestra la tarjeta desbaneada; Después de un baneo exitoso, el usuario vuelve a iniciar sesión. Las devoluciones de llamada de OAuth entregan esto a través de
el fragmento URL `#require_self_unban=1&self_unban_token=..&self_unban_hint=..`.

## 16. Códigos de error (extracto)

| código | Escenario |
|---|---|
| `INVALID_TOKEN` | Actualizar token no válido/caducado/no coincide la versión |
| `INVALID_REQUEST` | La validación de parámetros falló |
| `NOT_FOUND` | Falta recurso (boleto/pedido/entrada en lista de permitidos, etc.) |
| `RELOAD_FAILED` / `SYNC_FAILED` | (Lado administrador) Error en la sincronización de la configuración del proxy |
| 401 + `errClientTokenRevoked` | token_version no coincide (desconectado/expulsado) → los clientes deben actualizar; si eso falla, borre los tokens locales |

## 17. Documentos de cumplimiento (política de privacidad/términos de servicio)

Los enlaces "Política de privacidad · Términos de servicio" en la parte inferior de la tarjeta de inicio de sesión (igual que los sitios normales; al hacer clic se abre el contenido):

- **Configuración**: editor del portal → Configuración → General → Documentos de cumplimiento (**editar contenido por idioma**);
 también puedes `PUT /api/v1/portal/active` con campos estructurados `privacy_policy_url` / `privacy_policy_content` /
 `terms_of_service_url` / `terms_of_service_content` (nulo = sin cambios; el contenido es un mapa `{"zh-CN":"...","en":"..."}`).
- **Páginas accesibles independientes** (renderizadas por el servidor, fáciles para los rastreadores de verificación Google; compatible con `?lang=xx`):
  - Dominio del panel principal: `GET /p/legal/privacy`, `GET /p/legal/terms`
  - Dominio de portal dedicado (implementación raíz): `GET /legal/privacy`, `GET /legal/terms`
- **Lógica de visualización**: URL o contenido, **URL gana** (se abre externamente en una nueva pestaña); si se proporciona contenido, haga clic en el
 La tarjeta de inicio de sesión muestra Markdown en un cuadro de diálogo (`renderMd`) utilizando el idioma actual del portal.
- Entregado en tiempo de ejecución a través de `/api/v1/public/portal-config`; el componente de inicio de sesión consume
 `ctx.privacyUrl / termsUrl / privacyContent / termsContent / openLegal / hasLegalLinks`.
- Google La verificación de OAuth requiere que estos enlaces sean de acceso público → utilice los enlaces independientes anteriores en el
 pantalla de consentimiento; consulte `docs/oauth-google-verification.md` (contiene políticas de privacidad/plantillas de términos).

## 18. Ejemplo de integración

```js
// Sign in
const { data } = await fetch('/api/v1/client-auth/login', {
  method: 'POST', headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ email, password })
}).then(r => r.json());
localStorage.setItem('client_token', data.access_token);
localStorage.setItem('client_refresh_token', data.refresh_token);

// Authenticated request
const me = await fetch('/api/v1/client-auth/me', {
  headers: { Authorization: 'Bearer ' + data.access_token }
}).then(r => r.json());
```

Al 401, llame a `POST /refresh` una vez y vuelva a intentarlo; Si aún falla, borre la sesión local.
(El `api()` de la plantilla del portal ya implementa esto).
