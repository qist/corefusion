<!-- source: subscription.md | locale: es | updated: 2026-10-23 -->
# Suscripción API (pública)

> Idiomas: [简体中文](subscription.md) · [English](subscription.en.md) · [繁體中文](subscription.zh-TW.md) · [Русский](subscription.ru.md) · [日本語](subscription.ja.md) · [한국어](subscription.ko.md) · [Deutsch](subscription.de.md) · [Français](subscription.fr.md) · Español · [Português](subscription.pt.md) · [Türkçe](subscription.tr.md) · [Bahasa Indonesia](subscription.id.md) · [ไทย](subscription.th.md) · [Tiếng Việt](subscription.vi.md) · [العربية](subscription.ar.md) · [فارسی](subscription.fa.md)

Obtiene la lista de nodos de suscripción de un usuario. **Este es el punto final más utilizado por los usuarios finales**; no se requieren encabezados; el acceso se otorga mediante el token en el URL.

## 1. Punto final

| Implementación | URL |
|---|---|
| panel principal | `GET https://<panel-domain>/sub/<token>` |
| Puerto de suscripción dedicado | `GET https://<sub-domain>:<port>/<token>` (sin prefijo `/sub`) |

`<token>` es el token de acceso único de una suscripción, generado por el lado del administrador (visible en "Mis suscripciones" en el portal).

## 2. Parámetros de consulta

| Parámetro | Descripción |
|---|---|
| `format` | Formato de salida; anula el valor predeterminado de la suscripción. Ver abajo |
| `config` | Nombre de la plantilla de configuración (definido en "Formatos de suscripción" en la interfaz de usuario del administrador). Debe utilizar el mismo formato que el `format` de esta solicitud; de lo contrario, se ignora. |
| `platform` | Haga coincidir una plantilla de configuración por nombre de plataforma (se usa cuando se omite `config`); el formato también debe coincidir |

### Valores `format` admitidos

| Valor | Salida | Clientes |
|---|---|---|
| `base64_uri` (predeterminado) | Enlaces compartidos unidos a nueva línea, codificados en base64 en su conjunto | V2RayN, Shadowrocket, Surge y otros clientes genéricos |
| `raw_uri` | Enlaces compartidos unidos a nueva línea, texto sin formato | Clientes que necesitan enlaces simples/depuración |
| `mihomo_yaml` | Configuración completa de Clash/Mihomo YAML (con grupos de proxy, reglas) | Clash.Meta / mihomo / Choque Verge Rev |
| `mihomo_proxies_yaml` | YAML con solo la lista de nodos `proxies:` | fuente del proveedor de proxy mihomo (con actualización programada) |
| `singbox_json` | Configuración completa sing-box JSON | sing-box / SFI / FLClash (núcleo sing-box) |
| `xray_json` | Configuración completa de Xray JSON | Xray-núcleo |

> Se aplica la compatibilidad de protocolo/formato: por ejemplo, `anytls` solo aparece en formatos sing-box/Mihomo; Los nodos REALITY en un servidor sing-box no aparecen en las suscripciones Xray/sing-box JSON. Los nodos que el cliente no puede reconocer se eliminan automáticamente en lugar de fallar en toda la suscripción.

## 3. Encabezados de respuesta

| encabezado | Ejemplo | Descripción |
|---|---|---|
| `Subscription-Userinfo` | `upload=1024; download=2048; total=1073741824; expire=1760000000` | Uso y caducidad (bytes / segundos Unix). **También enviado al 403**, para que los clientes puedan mostrar la asignación restante |
| `Profile-Update-Interval` | `6` | Intervalo de actualización sugerido (horas) |
| `Profile-Title` | `CoreFusion` | Nombre del perfil (mostrado por Clash Verge Rev / V2RayN, etc.) |
| `Profile-Web-Page-Url` | `https://…/portal` | Enlace a la página del portal (“abrir página web” en clientes) |
| `Content-Type` | Por formato: `text/plain` / `text/yaml` / `application/json` |  |
| `Content-Disposition` | `attachment; filename="subscription.txt"` |  |

## 4. Códigos de estado

| HTTP | código | Significado |
|---|---|---|
| 200 | — | Normal (el cuerpo es la suscripción representada en `format`) |
| 400 | `MISSING_TOKEN` | Sin token en el URL |
| 403 | (texto sin formato) | Suscripción suspendida/caducada/inactiva. El mensaje está localizado (zh/en/ja/ko) a través de `Accept-Language` y es un motivo de una sola línea. |
| 404 | `SUBSCRIPTION_NOT_FOUND` | El token no existe |

> Las suscripciones revocadas se consideran inexistentes (404).

## 5. Ejemplos

```bash
# 1) Fetch with the default format
curl -L "https://panel.example.com/sub/abcd1234efgh5678"

# 2) Request Mihomo YAML with the "clash-global" config template
curl -L "https://panel.example.com/sub/abcd1234efgh5678?format=mihomo_yaml&config=clash-global"

# 3) Node list only (for a mihomo proxy-provider)
curl -L "https://panel.example.com/sub/abcd1234efgh5678?format=mihomo_proxies_yaml"

# 4) Check usage (headers only)
curl -sI "https://panel.example.com/sub/abcd1234efgh5678" | grep -i subscription-userinfo
```

Suscripción dinámica Clash/Mihomo (actualización periódica):

```yaml
proxy-providers:
  subscription-nodes:
    type: http
    url: "https://panel.example.com/sub/abcd1234efgh5678?format=mihomo_proxies_yaml"
    interval: 3600
    health-check:
      enable: true
      url: https://www.gstatic.com/generate_204
      interval: 300

proxy-groups:
  - name: PROXY
    type: select
    use:
      - subscription-nodes
```

## 6. Notas

- **El formato y la configuración están estrechamente relacionados**: la plantilla seleccionada por `config`/`platform` debe coincidir con la `format` de esta solicitud; las plantillas que no coinciden se ignoran; esto evita inyectar reglas de Mihomo en sing-box/Xray JSON.
- **Limitación de tasa**: el punto final de suscripción tiene su propio límite (`SubscribeRateLimit`); Los clientes abusivos son rechazados.
- **Registro de acceso**: cada recuperación registra información de IP/UA/CDN, visible en "Registro de acceso de suscripción" en la interfaz de usuario del administrador.
- Sin un dominio configurado en un puerto de suscripción dedicado, se utilizan los `X-Forwarded-Proto` / `X-Forwarded-Host` de la solicitud; asegúrese de que su proxy inverso reenvíe ambos encabezados.
