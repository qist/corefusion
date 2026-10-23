<!-- source: cert-google-eab.md | locale: es | updated: 2026-10-23 -->
# Guía de CA pública Google (emisión automática de EAB)

> Idiomas: [简体中文](cert-google-eab.md) · [English](cert-google-eab.en.md) · [繁體中文](cert-google-eab.zh-TW.md) · [Русский](cert-google-eab.ru.md) · [日本語](cert-google-eab.ja.md) · [한국어](cert-google-eab.ko.md) · [Deutsch](cert-google-eab.de.md) · [Français](cert-google-eab.fr.md) · Español · [Português](cert-google-eab.pt.md) · [Türkçe](cert-google-eab.tr.md) · [Bahasa Indonesia](cert-google-eab.id.md) · [ไทย](cert-google-eab.th.md) · [Tiếng Việt](cert-google-eab.vi.md) · [العربية](cert-google-eab.ar.md) · [فارسی](cert-google-eab.fa.md)

> Se aplica a: panel v0.12.101+ | Página: **Certificados → Configuración EAB**
> Código: `internal/service/certificate/eab_google.go`, `internal/api/handlers/certificates/eab_config.go`

## 1. Por qué Google requiere EAB

| CA | ¿EAB es obligatorio para ACME? | Cómo conseguir EAB |
|---|---|---|
| Vamos a cifrar | ❌No | — |
| ZeroSSL | ✅ Sí | **Cambie un correo electrónico por uno** (`api.zerossl.com/acme/eab-credentials-email`, igual que acme.sh); el panel puede recuperarlo automáticamente |
| **Google CA pública** | ✅ **Sí** | **Debe crearse con autorización de GCP** (un correo electrónico no es suficiente) → el enfoque de esta página |

En otras palabras, Google **no puede evitar la "autorización"**: ya sea un archivo JSON de cuenta de servicio o un token de acceso temporal.

## 2. Las reglas estrictas de Google (lea primero)

| regla | Descripción |
|---|---|
| **Usar dentro de los 7 días** | Una clave EAB **caduca** si no se utiliza dentro de los 7 días y se debe volver a crear |
| **Solo una cuenta** | Una clave EAB registra exactamente una cuenta ACME; después del registro la clave **queda invalidada** y no se puede reutilizar |
| **Las cuentas ACME no caducan** | La cuenta ACME creada con ella es **de larga duración** → **las renovaciones no necesitan EAB** |
| ⚠ **Eliminar el proyecto GCP elimina todas las cuentas** | Eliminar el proyecto invalida inmediatamente **todas las cuentas ACME** bajo él y **la renovación se vuelve imposible** |

> Conclusión: **EAB es una "credencial de incorporación" única; la cuenta ACME es lo que dura.**
> El panel almacena cuentas en `data/acme/accounts/<ca>.json` — **haga una copia de seguridad de este directorio**.

## 3. Tres rutas (comience con la ruta A, sin búsqueda de menú)

> Los puntos de entrada de Cloud Console han cambiado varias veces, por lo que los enlaces directos y Cloud Shell son lo primero; Las rutas del menú son alternativas.

### Ruta A (la más fácil, recomendada primero): un comando en GCP Cloud Shell

Cloud Shell es una terminal de navegador con **gcloud preinstalado y ya iniciado**; sin configuración local ni menús.

1. Abra <https://console.cloud.google.com/cloudshell> (o el ícono `>_` en la parte superior derecha de la consola)
2. Seleccione el proyecto (el ID del proyecto es visible en el selector de proyectos en la parte superior):
   ```bash
   gcloud config set project <your-project-id>
   ```
3. Habilite la CA pública API (**equivalente** a habilitarla en la biblioteca API):
   ```bash
   gcloud services enable publicca.googleapis.com
   ```
4. Crea el EAB (**estas son las dos cadenas que necesitas**):
   ```bash
   gcloud publicca external-account-keys create
   ```
 En la salida: `keyId` → **EAB-KID** del panel; `b64MacKey` → **EAB-HMAC-Key** del panel
5. De vuelta en el panel: **Certificados → Configuración EAB → Agregar configuración EAB → CA: Google CA pública** →
 ingrese los dos valores en `EAB-KID` / `EAB-HMAC-Key` → guardar ✓

> ⚠ Costo: Google Las claves EAB son **7 días, únicas** ✗ → después de 7 días (o al cambiar de cuenta) **repita el paso 4**.
> Para evitar eso, use la ruta B (almacene una cuenta de servicio una vez y deje que el panel vuelva a solicitarla automáticamente).

### Ruta B (configuración única, luego automática): clave de cuenta de servicio JSON

**① Habilite la CA pública API** (el enlace directo es el más fácil)
<https://console.cloud.google.com/apis/library/publicca.googleapis.com?project=YOUR_PROJECT_ID> → haga clic en “Activar”
Alternativa: ☰ → **API y servicios → Biblioteca** → buscar `Public CA` → abrir → “Habilitar”

**② Crear una cuenta de servicio**
<https://console.cloud.google.com/iam-admin/serviceaccounts?project=YOUR_PROJECT_ID>
→ “**+ Crear cuenta de servicio**” → cualquier nombre (por ejemplo, `corefusion-eab`) → “Crear y continuar” → puedes **omitir** el paso del rol → “Listo”
Alternativa: ☰ → **IAM y administrador → Cuentas de servicio**

**③ Otorga el rol mínimo** (más fácil en la misma página)
Haga clic en la cuenta recién creada → **pestaña “Permisos”** → “Conceder acceso” → nuevo director: el **correo electrónico de la cuenta de servicio**
→ busque el rol para `publicca` → elija **“Creador de claves de cuenta externa de CA pública”** → guarde
Alternativa: ☰ → **IAM y administrador → IAM** → “Conceder acceso” → mismo correo electrónico + función
⚠ **No otorgar propietario/editor** ✗

**④ Descargue la clave JSON**
Cuentas de servicio → abra la cuenta → **pestaña “Claves”** → “**Agregar clave**” → “**Crear nueva clave**” → **JSON** → crear
(el navegador descarga un `xxx.json`; o fila `⋮` → “Administrar claves” → mismo flujo)

**⑤ De vuelta en el panel**: ingrese el **ID del proyecto** y pegue el **contenido completo** del JSON en “Archivo de clave de cuenta de servicio” → haga clic en “Solicitar EAB automáticamente”
→ luego el panel **vuelve a solicitar EAB automáticamente** cuando caduca ✓, no más visitas a GCP ✓

### Ruta C (solo prueba única): pegue un token de acceso

En Cloud Shell:
```bash
gcloud auth print-access-token
```
Pegue el resultado en el “Token de acceso temporal” del panel más el ID del proyecto → haga clic en “Solicitar EAB automáticamente”
⚠ Válido por aproximadamente 1 hora y **no almacenado** ✓ → solo verificación, no se puede volver a solicitar automáticamente ✗

### Hoja de trucos cuando no puedes encontrar un punto de entrada (los menús de la consola cambian)

| Tarea | Enlace directo (el más fiable) | Alternativa: menú |
|---|---|---|
| Habilitar la CA pública API | `https://console.cloud.google.com/apis/library/publicca.googleapis.com?project=<PROJECT_ID>` | ☰ → API y servicios → Biblioteca → buscar `Public CA` |
| cuentas de servicio | `https://console.cloud.google.com/iam-admin/serviceaccounts?project=<PROJECT_ID>` | ☰ → IAM y administrador → Cuentas de servicio |
| Conceder rol | `https://console.cloud.google.com/iam-admin/iam?project=<PROJECT_ID>` | ☰ → IAM y administrador → IAM → Conceder acceso |
| Concha de nube | `https://console.cloud.google.com/cloudshell` | Icono `>_` en la parte superior derecha |

> Consejo: reemplace `<PROJECT_ID>` / `YOUR_PROJECT_ID` con el ID del proyecto real (como `my-project-123456`) para ir directamente a la página.

## 4. Configurando el panel (dos maneras)

Vaya a **Certificados → Configuración EAB → Agregar configuración EAB** y elija **CA pública Google**:

### Opción A (recomendada): pegue la cuenta de servicio JSON

| campo | Valor |
|---|---|
| ID del proyecto de GCP | El ID del proyecto de la sección 3, p. `my-project-123456` |
| Cuenta de servicio JSON | Pegue el **contenido completo** de `corefusion-eab.json` |
| Token de acceso temporal | dejar vacío |
| Nombre | Generado automáticamente como `Google Public CA - <project-id>`; editable |

Haga clic en **Solicitar EAB automáticamente** → en caso de éxito, la clave KID/HMAC se completa y guarda; luego simplemente elija `CA = Google Public CA` al emitir (EAB está asociado por CA; no hay selección manual).

> La cuenta de servicio JSON está **cifrada en la configuración EAB de la base de datos del panel** (`json:"-"`, **nunca devuelta a la interfaz**).
> El objetivo es **volver a solicitar una clave automáticamente después del vencimiento de 7 días** en lugar de visitas manuales a GCP.

### Opción B: token de acceso temporal (único)

```bash
gcloud auth print-access-token        # valid ~1 hour
```

Péguelo en "Token de acceso temporal", complete el ID del proyecto, haga clic en **Solicitar EAB automáticamente**.
⚠ Este token **no se almacena** y se debe volver a obtener después de su vencimiento → está bien para una prueba única, no para uso a largo plazo.

### Equivalente de línea de comando (opcional, para comparación)

```bash
gcloud publicca external-account-keys create
# returns keyId / b64MacKey, corresponding to ACME's --eab-kid / --eab-hmac-key
```

## 4.5 ¿Por qué JSON en lugar de "una sola llave"?

Esta es la pregunta más común. Separe los dos tipos de “clave”:

| Lo que podrías esperar | ¿Qué se necesita realmente? |
|---|---|
| **Clave API** (la cadena `AIza…`) | ❌ **Inútil**: una clave API es solo un identificador público y **no conlleva identidad**; La creación de EAB necesita **permisos IAM**, por lo que se requieren credenciales de afirmación de identidad. |
| — | ✅ **Clave de cuenta de servicio**: el **archivo** descargado de GCP “IAM → Cuentas de servicio → Claves → Crear nueva clave → **JSON**” |

Punto clave: **el archivo JSON en sí es "una clave"**: GCP empaqueta la identidad de la cuenta (`client_email`) más la clave privada (`private_key`) en un solo archivo. El panel firma un JWT con él → lo intercambia por un token de acceso OAuth2 → llama a la CA pública API.
Entonces “pegue JSON” ≠ llenando muchos campos; **pegue el archivo completo** (el panel también completa campos opcionales como `type` / `token_uri`).

**El EAB real son las dos cadenas** (`EAB-KID` + `EAB-HMAC-Key`), que son el **resultado** de la solicitud; el panel los completa automáticamente.

### ¿No quieres entregar el expediente? Dos alternativas

| Método | como | Costo |
|---|---|---|
| **Token de acceso temporal** | `gcloud auth print-access-token` → pegar en “Token de acceso temporal” | ~1 hora de validez, **no almacenado** → solo una vez, sin nueva solicitud automática |
| **NIÑO manual/HMAC** | Ejecute localmente `gcloud publicca external-account-keys create` → ingrese `keyId` / `b64MacKey` en “EAB-KID / EAB-HMAC-Key” | No se entregaron credenciales ✓; pero Google EAB es **7 días, una sola vez** → repetir después de la expiración |

> El panel recomienda la cuenta de servicio JSON por una razón: **EAB vence después de 7 días**, y solo una cuenta de servicio almacenada permite **volver a solicitar automáticamente** sin trabajo manual.

## 4.6 Preguntas frecuentes: ¿puedo utilizar una clave API?

**No.** Esta no es una opción de implementación: **Google la rechaza explícitamente** (verificado en ambos sentidos):

```bash
# 1) Query parameter
curl -X POST "https://publicca.googleapis.com/v1/projects/<ID>/locations/global/externalAccountKeys?key=AIza..." -d '{}'
# 2) x-goog-api-key header recommended by the official docs
curl -X POST "https://publicca.googleapis.com/v1/projects/<ID>/locations/global/externalAccountKeys" \
     -H "x-goog-api-key: AIza..." -d '{}'
```

Ambos regresan:

```json
{"error":{"code":401,"status":"UNAUTHENTICATED",
  "message":"API keys are not supported by this API. Expected OAuth2 access token
             or other authentication credentials that assert a principal."}}
```

A modo de comparación, devuelve un **portador de OAuth (falso)**
`Request had invalid authentication credentials. Expected OAuth 2 access token...`—
por lo que se reconoce OAuth mientras que **las claves API se rechazan directamente** ✗.

**Por qué**: crear EAB es una **operación administrativa** que requiere saber “quién es usted y si tiene `roles/publicca.externalAccountKeyCreator`” ✗
— una clave API solo identifica un proyecto/aplicación ✓ y **no expresa una identidad principal** ✓.
(Las "Prácticas recomendadas para administrar claves API" de Google recomiendan **IAM + credenciales de cuenta de servicio de corta duración** para escenarios administrativos ✓: el mismo enfoque utilizado aquí ✓.)

**Entonces solo hay tres caminos** ✓ (ver §3): cuenta de servicio JSON ✓ / token de acceso temporal ✓ /
o **sin ninguna credencial**: ejecute `gcloud publicca external-account-keys create` en Cloud Shell ✓
e ingresa `keyId` / `b64MacKey` en el panel ✓ (**usa esto si GCP IAM te bloquea** ✓).

## 5. Altas y renovaciones automáticas (importante)

```
Panel: CA = Google Public CA
  ├─ usable EAB present in the config → use it
  └─ missing/expired → request a new one on the spot with the stored service-account JSON (auto-registration)
        │
First issuance on the node/panel ── registers the ACME account with EAB (one-time!)
        │
Account persisted: data/acme/accounts/google.json (panel) / <Agent data_dir>/acme/accounts/google.json (node)
        │
Every later renewal ── uses the account key only; **EAB is never needed again**
```

Por lo tanto:

- **La primera emisión se registra automáticamente**; no se necesita ningún paso manual de GCP;
- Las renovaciones **no** consumen EAB (siempre que el archivo de la cuenta exista y el directorio de datos se mantenga estable);
- ⚠ El directorio de datos ACME del lado del agente es una ruta absoluta fija (predeterminado `/var/lib/corefusion-agent/acme`);
 **no lo cambies casualmente**, o será tratada como una cuenta nueva → se requiere otra clave EAB.

## 6. Solución de problemas

| Síntoma/error | causa | Arreglar |
|---|---|---|
| `PERMISSION_DENIED` (403) | La cuenta de servicio carece de `roles/publicca.externalAccountKeyCreator` | Volver a la sección 3 ④ |
| `SERVICE_DISABLED` (403) | `publicca.googleapis.com` no habilitado para el proyecto | Sección 3 ② |
| `NOT_FOUND` (404) | ID de proyecto incorrecto/proyecto eliminado | Verificar el ID del proyecto |
| Informes de emisión `externalAccountRequired` | Esta CA no tiene EAB | Solicite/configure EAB en el panel primero |
| Informes de emisión `Accounts must have at least one contact.` | No se proporcionó ningún correo electrónico durante el registro. | Configure el correo electrónico de la cuenta ACME en la página del certificado (el panel usa la cuenta predeterminada) |
| La renovación solicita nuevamente EAB / el registro falla | Archivo de cuenta ACME perdido (directorio de datos cambiado/limpiado) | **Restaurar la copia de seguridad**; sin uno, solicite un nuevo EAB usando la cuenta de servicio |
| El emisor cambia de Google a Let's Encrypt | Al comando de renovación le falta la CA (error antiguo, solucionado en v0.12.101) | Actualización y reedición |

## 7. Recomendaciones de seguridad y operaciones.

1. **Mínimo privilegio**: otorga la cuenta de servicio solo `roles/publicca.externalAccountKeyCreator`, nunca Propietario/Editor.
2. **No eliminar el proyecto**: eliminar el proyecto de GCP invalida todas sus cuentas ACME → los certificados no se pueden renovar.
3. **Siempre haga una copia de seguridad** `data/acme/accounts/` (la identidad de la cuenta en sí) y la configuración EAB de la base de datos del panel.
4. **Que no cunda el pánico por la caducidad del EAB**: mientras exista el archivo de cuenta ACME, las renovaciones no se ven afectadas; Solo se necesita un nuevo EAB para registrar una nueva cuenta.
5. Si es posible que se haya filtrado la clave de la cuenta de servicio: elimine esa clave en GCP y pegue un nuevo JSON en el panel.

## 8. Puntos finales relacionados (para secuencias de comandos)

| Punto final | Descripción |
|---|---|
| `POST /api/v1/eab-configs/fetch-google` | Cree un EAB usando la autorización de GCP. Cuerpo: `{project_id, service_account_json?, access_token?}` |
| `POST /api/v1/eab-configs/fetch-zerossl` | Obtenga un ZeroSSL EAB con un correo electrónico. Cuerpo: `{email}` |
| `GET  /api/v1/eab-configs` | Enumere las configuraciones de EAB (**sin** la clave HMAC o la cuenta de servicio JSON; solo `has_mac_key` / `has_google_sa`) |
