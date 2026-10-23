<!-- source: notification-templates.md | locale: es | updated: 2026-10-23 -->
# Variables de plantilla de notificación

> Idiomas: [简体中文](notification-templates.md) · [English](notification-templates.en.md) · [繁體中文](notification-templates.zh-TW.md) · [Русский](notification-templates.ru.md) · [日本語](notification-templates.ja.md) · [한국어](notification-templates.ko.md) · [Deutsch](notification-templates.de.md) · [Français](notification-templates.fr.md) · Español · [Português](notification-templates.pt.md) · [Türkçe](notification-templates.tr.md) · [Bahasa Indonesia](notification-templates.id.md) · [ไทย](notification-templates.th.md) · [Tiếng Việt](notification-templates.vi.md) · [العربية](notification-templates.ar.md) · [فارسی](notification-templates.fa.md)

Las plantillas de notificación utilizan marcadores de posición `{variable}` que se reemplazan con valores reales al enviar. Este documento enumera todas las variables admitidas, las variables disponibles por evento y las reglas de activación automática.

## 1. Tipos de plantillas y audiencias

| Audiencia | canal | Destinatario | Uso típico |
|---|---|---|---|
| **Usuarios del panel** (administrador) | Telegram / Correo electrónico | Canales y destinatarios configurados por el administrador | Alertas de nodo, informes de respaldo, anuncios del sistema. |
| **Suscriptores** | Correo electrónico | El propio correo electrónico del cliente (rellenado automáticamente) | Saldo bajo, recordatorios de vencimiento de suscripción |

## 2. Variables comunes

Disponible en todas las plantillas:

| variable | Descripción | Ejemplo |
|---|---|---|
| `{time}` | enviar hora | `2026-09-28 14:30:00` |
| `{user}` | Correo electrónico del cliente (plantillas de suscriptor) / identificador de usuario | `user@example.com` |
| `{message}` | Organismo de notificación | Texto del anuncio de mantenimiento. |

## 3. Variables de eventos de suscriptor

### Recordatorio de vencimiento de la suscripción (event_key = `subscription_expire`)

Escaneado diariamente y enviado a suscriptores activos que vencen dentro de los 7 días:

| variable | Descripción | Ejemplo |
|---|---|---|
| `{user}` | Correo electrónico del cliente | `user@example.com` |
| `{expire_date}` | Vencimiento de la suscripción | `2026-10-05 09:00` |
| `{days}` | Días restantes | `7` |
| `{time}` | enviar hora | `2026-09-28 09:05:00` |

### Recordatorio de saldo bajo (event_key = `balance_low`)

Escaneado diariamente y enviado a clientes cuyo saldo está por debajo del umbral:

| variable | Descripción | Ejemplo |
|---|---|---|
| `{user}` | Correo electrónico del cliente | `user@example.com` |
| `{balance}` | Saldo actual (unidades monetarias, dos decimales) | `0.85` |
| `{time}` | enviar hora | `2026-09-28 09:05:00` |

## 4. Variables de ejemplo de plantilla de operaciones

Cuando hace clic en "Enviar" para una prueba de conectividad, las plantillas de usuario del panel llenan marcadores de posición con estos ejemplos:

| variable | Descripción | Ejemplo |
|---|---|---|
| `{node}` | Nombre de nodo | `node-01` |
| `{backup_file}` | Nombre del archivo de copia de seguridad | `corefusion-20260101-0900.tar.gz` |
| `{size}` | Tamaño del archivo | `128MB` |
| `{domain}` | Dominio | `example.com` |
| `{expire_date}` | Fecha de caducidad | `2026-10-12` |
| `{days}` | Días restantes | `14` |
| `{count}` | contar | `3` |

## 5. Reglas de activación automática

- **Horario**: el primer escaneo se ejecuta 30 segundos después de que se inicia el panel, luego diariamente a las **09:05**
- **Caducidad de la suscripción**: analiza las suscripciones activas (`status=active`) que caducan en 7 días; el cliente también debe estar activo
- **Saldo bajo**: escanea clientes activos con saldo ≤ umbral; el umbral es `notify.balance_threshold`, **en centavos** (predeterminado `100` = 1,00)
- **Deduplicación**: como máximo un mensaje por cliente por evento **por día** (`notification_send_logs` deduplicación por día)

## 6. Monitoreo de la salud del nodo

Los latidos del corazón de los nodos se controlan continuamente; **las notificaciones se envían solo cuando cambia el estado; las rondas sin cambios son silenciosas**:

| Tecla de configuración | Descripción | Predeterminado |
|---|---|---|
| `monitor.node_interval_seconds` | Intervalo de comprobación (segundos) | `60` (mínimo 30) |
| `monitor.node_offline_timeout_seconds` | Tiempo de espera del latido (segundos) | `120` |

- **Alerta fuera de línea**: cuando un nodo pasa de estar en línea a estar fuera de línea, envía la plantilla de administración “**Alerta de nodo fuera de línea**” con `{node}` (nombre) y `{time}`.
- **Aviso de recuperación**: cuando un nodo se recupera, envía “**Aviso de recuperación de nodo**” con las mismas variables
- **Rondas silenciosas**: no hay mensaje cuando el estado no cambia; después de que un panel reinicia la primera ronda solo establece una línea de base
- **Monitorizar sin enviar**: deshabilitar la plantilla correspondiente (Habilitado = APAGADO)

## 7. Alertas mensuales de cuota de tráfico de nodos (automáticas)

- **Activador**: cuando el tráfico mensual de un nodo alcanza el umbral de cuota, envía “**Alerta de cuota de tráfico de nodo**”; cuando supera el 100% y se suspende, envía una alerta de exceso de límite (claves de evento `traffic_quota_threshold` / `traffic_quota_exceeded`)
- **Umbral**: `monitor.traffic_quota_threshold_percent` (porcentaje, `80` predeterminado; configure `0` para deshabilitar las alertas de umbral)
- **Deduplicación**: como máximo una por nodo por ciclo de facturación (el ciclo sigue el `traffic_quota_reset_day` del nodo)
- Variables disponibles: `{node}` (nombre), `{used}` (GB utilizados), `{quota}` (cuota de GB), `{percent}` (porcentaje), `{time}`

## 8. Notificaciones de resultados de respaldo (automáticas)

- **Activador**: se envía inmediatamente después de que se ejecuta una copia de seguridad programada (`BackupScheduler`): el éxito envía el “**Informe de copia de seguridad diario**”, el error (creación de la copia de seguridad/lectura del almacenamiento/carga remota) envía “**Alerta de error de copia de seguridad**”
- Variables disponibles: `{backup_file}` (nombre), `{size}` (tamaño, solo informe de éxito), `{message}` (motivo de falla, solo alerta de falla), `{time}`
- **Monitorizar sin enviar**: se omite silenciosamente cuando la plantilla está deshabilitada (Habilitado = APAGADO)

## 9. Recordatorios de vencimiento de certificados (automáticos)

- **Activador**: diariamente a las 09:05, escanea los certificados `active` que están a punto de caducar (**certificados IP con 3 días restantes**, **certificados de dominio con 7 días restantes**) y envía la plantilla de administración “**Recordatorio de vencimiento del certificado**” por certificado
- **Deduplicación**: como máximo un recordatorio por certificado por día; después de la renovación, el vencimiento sale de la ventana (IP 3 días / dominio 7 días) y los recordatorios se detienen
- Variables disponibles: `{domain}` (dominio del certificado), `{expire_date}` (fecha de vencimiento), `{days}` (días restantes), `{time}`

## 10. Alertas de fallas del proceso central (automáticas)

- **Activador**: cuando un proceso central (xray/singbox) sale de forma anormal o el proceso proxy (caddy/nginx) falla, envía la plantilla de administración “**Alerta de falla del proceso principal**” inmediatamente; el supervisor luego lo reinicia con un retroceso exponencial
- Variables disponibles: `{core}` (tipo de proceso), `{restarts}` / `{max}` (recuento/límite de reinicio), `{message}` (mensaje de salida, solo procesos principales), `{time}`
- **Monitorizar sin enviar**: se omite silenciosamente cuando la plantilla está deshabilitada (Habilitado = APAGADO)

## 11. Avisos de alta de nuevos usuarios (automáticos)

- **Activador**: se envía inmediatamente después de que un cliente se registra correctamente, utilizando la plantilla de administración "**Aviso de registro de nuevo usuario**"
- Variables disponibles: `{user}` (correo electrónico del cliente), `{time}`

## 12. Uso

1. **Agregar una plantilla**: Configuración de notificaciones → Plantillas de notificaciones → Agregar plantilla → “Rellenar desde preestablecido” y ajustar según sea necesario
2. **Habilitar**: marque Habilitado y guarde; activadores automáticos solo procesan plantillas habilitadas
3. **Prueba**: haga clic en "Enviar" en la lista para enviar un mensaje real con variables de ejemplo (los correos electrónicos de prueba de la plantilla de suscriptor van a la dirección del remitente del administrador)
4. **Notificaciones inmediatas** (código comercial): llame a `NotifyClientEvent(ctx, db, settingSvc, eventKey, clientEmail, vars)` en el sitio del evento (recarga, deducción, renovación) para notificar a un solo cliente de inmediato, fuera del límite de escaneo diario
