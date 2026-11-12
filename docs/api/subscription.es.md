<!-- source: subscription.md | locale: es | updated: 2026-11-12 -->
# Suscripción API (pública)

> Idiomas: [简体中文](subscription.md) · [English](subscription.en.md) · [繁體中文](subscription.zh-TW.md) · [Русский](subscription.ru.md) · [日本語](subscription.ja.md) · [한국어](subscription.ko.md) · [Deutsch](subscription.de.md) · [Français](subscription.fr.md) · Español · [Português](subscription.pt.md) · [Türkçe](subscription.tr.md) · [Bahasa Indonesia](subscription.id.md) · [ไทย](subscription.th.md) · [Tiếng Việt](subscription.vi.md) · [العربية](subscription.ar.md) · [فارسی](subscription.fa.md)

Obtiene la lista de nodos de suscripción de un usuario. **Este es el punto final más utilizado por los usuarios finales**; no se requieren encabezados; el acceso se otorga mediante el token en el URL.

## 1. Punto final

| Implementación | URL |
|---|---|
| Panel principal | `GET https://<panel-domain>/sub/<token>` |
| Puerto de suscripción dedicado | `GET https://<sub-domain>:<port>/<token>` (sin prefijo `/sub`) |

`<token>` es el token de acceso único de una suscripción, generado por el lado del administrador (visible en "Mis suscripciones" en el portal).

## 2. Parámetros de consulta

| Parámetro | Descripción |
|---|---|
| `format` | Formato de salida; anula el valor predeterminado de la suscripción. Ver abajo |
| `config` | Nombre de la plantilla de configuración (definido en "Formatos de suscripción" en la interfaz de usuario del administrador). Debe utilizar el mismo formato que el `format` de esta solicitud; de lo contrario, se ignora. |
| `platform` | Haga coincidir una plantilla de configuración por nombre de plataforma (se usa cuando se omite `config`); el formato también debe coincidir |
| `providers` | Solo `mihomo_yaml`: `1` / `true` habilita el **modo Proveedor** (emite `proxy-providers` y hace referencia a ellos a través de `use`); El valor predeterminado o cualquier otro valor mantiene los nodos alineados en `proxies`. Ver §6 |

### Valores `format` admitidos

| Valor | Salida | Clientes |
|---|---|---|
| `base64_uri` (predeterminado) | Enlaces compartidos unidos a nueva línea, codificados en base64 en su conjunto | V2RayN, Shadowrocket, Surge y otros clientes genéricos |
| `raw_uri` | Enlaces compartidos unidos a nueva línea, texto sin formato | Clientes que necesitan enlaces simples/depuración |
| `mihomo_yaml` | Configuración completa de Clash/Mihomo YAML (con grupos de proxy, reglas) | Clash.Meta / mihomo / Choque Verge Rev |
| `mihomo_proxies_yaml` | YAML con solo la lista de nodos `proxies:` | fuente del proveedor de proxy mihomo (con actualización programada) |
| `singbox_json` | Configuración completa sing-box JSON | sing-box / SFI / FLClash (núcleo sing-box) |
| `xray_json` | Configuración completa Xray JSON | Xray-núcleo |

> Se aplica la compatibilidad de protocolo/formato: por ejemplo, `anytls` solo aparece en formatos sing-box/Mihomo; Los nodos REALITY en un servidor sing-box no aparecen en las suscripciones Xray/sing-box JSON. Los nodos que el cliente no puede reconocer se eliminan automáticamente en lugar de fallar en toda la suscripción.

## 3. Encabezados de respuesta

| encabezado | Ejemplo | Descripción |
|---|---|---|
| `Subscription-Userinfo` | `upload=1024; download=2048; total=1073741824; expire=1760000000` | Uso y caducidad (bytes / segundos Unix). **También enviado al 403**, para que los clientes puedan mostrar la asignación restante |
| `Profile-Update-Interval` | `6` | Intervalo de actualización sugerido (horas) |
| `Profile-Title` | `CoreFusion` | Nombre del perfil (mostrado por Clash Verge Rev / V2RayN, etc.) |
| `Profile-Web-Page-Url` | `https://portal.example.com` | Portal URL (“abrir página web” en clientes; ruta raíz cuando el portal tiene su propio puerto, en caso contrario /p) |
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

# 5) Clash dynamic subscription: config template + Provider mode
curl -L "https://sub.example.com/abcd1234efgh5678?format=clash&config=out&providers=1"

# 6) sing-box / Xray with their own config templates (rules come from the template)
curl -L "https://sub.example.com/abcd1234efgh5678?format=singbox_json&config=sing-box-home"
curl -L "https://sub.example.com/abcd1234efgh5678?format=xray_json&config=xray-home"
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

## 6. Modo proveedor y múltiples dominios de suscripción

### 6.1 Qué emite `providers=1`

Con `format=mihomo_yaml&providers=1` (o `format=clash&providers=1`) el YAML:

- ya no incluye nodos en el nivel superior: emite `proxy-providers:`;
- los grupos proxy hacen referencia a fuentes de nodos a través de `use: [<provider-key>...]`;
- cualquier `proxy-providers` escrito en un formato de suscripción compartida (`mihomo_configs.config_json`) se **ignora** (una configuración compartida sirve para varios planes; los proveedores codificados se filtrarían entre los planes). Grupo → asignaciones de fuentes se encuentran en “Gestión de planes → grupo → asignación de fuentes”.

Tipos de proveedores:

| amable | clave | Notas |
|---|---|---|
| Interno | Nombre del grupo de nodos desinfectado | Generado por grupo de nodos del plan; contiene solo los nodos de ese grupo |
| Reservado | `subscription-nodes` | Todos los nodos del plan; su fuente URL **omite** el parámetro `provider` |
| Externo | Nombre configurado en el plan | Fuente externa (`http` YAML o `file`) |

Campos comunes por proveedor:

- `url`: apunta a esta suscripción con `format=mihomo_proxies_yaml`, heredando `config` / `platform` de la solicitud principal; los proveedores internos añaden `provider=<key>`;
- `path`: **siempre derivado de “valores predeterminados del proveedor interno → plantilla de ruta de caché”** (el valor predeterminado es `./providers/{key}.yaml`, se sustituye por `{key}`). Los proveedores externos tampoco pueden marcar su propio camino;
- `health-check` — `enable / url / interval`; los proveedores internos heredan los valores predeterminados, los proveedores externos los configuran en el plan;
- `interval`: intervalo de actualización en segundos.

### 6.2 Qué dominio entra en el proveedor URL (multidominio)

“Centro de suscripciones → Puntos finales de suscripción” puede contener **varios** registros (dominio + ruta + habilitado). El proveedor `url` elige el dominio en este orden:

1. **El dominio que realmente usó esta solicitud**: el primer valor de `X-Forwarded-Host` (puerto eliminado); de lo contrario, `Host`; debe coincidir con un punto final **habilitado**;
2. Si nada coincide, recurra al punto final habilitado con el **ID más bajo**;
3. La ruta proviene del punto final coincidente (`path`, predeterminado `/`), lo que produce `https://<domain><path>/<token>?format=mihomo_proxies_yaml...`.

Consecuencias:

- Un cliente que busca un nuevo dominio obtiene ese dominio dentro del proveedor - **no es necesario cambiar el panel**;
- Un dominio antiguo que todavía está habilitado en la lista ya no está codificado;
- Si un cliente almacenó en caché el antiguo proveedor URL, actualizar la suscripción (o borrar el caché del proveedor/reiniciar el cliente) lo cambia.

Los servidores proxy inversos deben reenviar `X-Forwarded-Proto` y `X-Forwarded-Host`; de lo contrario, se utiliza el punto final alternativo.

## 7. Notas

- **El formato y la configuración están estrechamente relacionados**: la plantilla seleccionada por `config`/`platform` debe coincidir con la `format` de esta solicitud; las plantillas que no coinciden se ignoran; esto evita inyectar reglas de Mihomo en sing-box/Xray JSON.
- **`providers=1` solo afecta a Mihomo YAML**: sing-box / Xray / base64 / raw ignórelo (los nodos permanecen en línea).
- **Limitación de tasa**: el punto final de suscripción tiene su propio límite (`SubscribeRateLimit`); Los clientes abusivos son rechazados.
- **Registro de acceso**: cada recuperación registra información de IP/UA/CDN, visible en "Registro de acceso de suscripción" en la interfaz de usuario del administrador.
- Sin un dominio configurado en un puerto de suscripción dedicado, se utilizan los `X-Forwarded-Proto` / `X-Forwarded-Host` de la solicitud; asegúrese de que su proxy inverso reenvíe ambos encabezados.
- En implementaciones de múltiples dominios, mantenga la lista de puntos finales alineada con los dominios que los clientes realmente usan; ver §6.2.
