<!-- source: oauth-google-verification.md | locale: es | updated: 2026-10-23 -->
# Google Guía de verificación de OAuth

> Idiomas: [简体中文](oauth-google-verification.md) · [English](oauth-google-verification.en.md) · [繁體中文](oauth-google-verification.zh-TW.md) · [Русский](oauth-google-verification.ru.md) · [日本語](oauth-google-verification.ja.md) · [한국어](oauth-google-verification.ko.md) · [Deutsch](oauth-google-verification.de.md) · [Français](oauth-google-verification.fr.md) · Español · [Português](oauth-google-verification.pt.md) · [Türkçe](oauth-google-verification.tr.md) · [Bahasa Indonesia](oauth-google-verification.id.md) · [ไทย](oauth-google-verification.th.md) · [Tiếng Việt](oauth-google-verification.vi.md) · [العربية](oauth-google-verification.ar.md) · [فارسی](oauth-google-verification.fa.md)

> Alcance: inicio de sesión Google para el portal CoreFusion (`internal/service/oauth`, alcances: `openid`, `email`, `profile`).
> Comparación de GitHub al final (GitHub no tiene flujo de verificación).

## 1. Modo de prueba versus modo de producción

La pantalla de consentimiento de OAuth Google tiene dos estados de publicación:

| Estado | ¿Quién puede iniciar sesión? | Se requiere verificación |
|---|---|---|
| **Pruebas** | Solo cuentas Google en la lista de "usuarios de prueba" | No |
| **En producción** | Cualquier cuenta Google | **Sí** (ver más abajo) |

Una vez que se habilita “Configuración del sistema → OAuth”, la página de inicio de sesión del portal muestra el botón Google a cada visitante;
Si la aplicación aún está en prueba, los visitantes que no están en la lista de usuarios de prueba ven
`Error 403: access_denied` (aplicación no verificada/no eres un usuario de prueba).
Para ofrecer el inicio de sesión Google públicamente, **cambie la aplicación a producción y pase la verificación**.

## 2. Requisitos estrictos para la verificación

La revisión de Google (verificación básica para alcances no restringidos) requiere:

1. **Página de inicio**: un URL de acceso público en el dominio autorizado (mismo dominio o un subdominio).
2. **Enlace de Política de Privacidad**: debe ser **accesible públicamente** (sin iniciar sesión) y explicar qué datos recopila y cómo los usa/almacena/elimina. Para el inicio de sesión Google debe cubrir al menos:
 los datos de la cuenta Google recopilados (correo electrónico, nombre, avatar), el propósito (crear/iniciar sesión en la cuenta del portal), no vender a terceros,
 y cómo los usuarios pueden eliminar su cuenta y sus datos.
3. **Enlace de Términos de servicio**: accesible públicamente, que describe las reglas del servicio.
4. **Dominios autorizados**: registre el dominio de la aplicación en la pantalla de consentimiento (por ejemplo, `example.com`); Los enlaces de inicio/privacidad/términos deben estar en él o en un subdominio.
5. **Logotipo de la aplicación (opcional pero recomendado)**: 120×120px.
6. **Justificación del alcance**: este proyecto solo utiliza `openid / email / profile` (no restringido, no sensible),
 lo cual se incluye en la verificación básica: sin justificación por alcance y **sin evaluación de seguridad de terceros**.
 (Los ámbitos restringidos, como el acceso completo a Gmail, los requerirían; este proyecto no utiliza ninguno).

Entrada de verificación: [Google Cloud Console](https://console.cloud.google.com/) →
**API y servicios → Pantalla de consentimiento de OAuth → Publicar aplicación → Enviar para verificación**.
La verificación básica generalmente se completa en unos días; Mientras tanto, puede operar en modo de prueba.

## 3. Qué configurar para este proyecto

### 3.1 Redirección autorizada URI

La devolución de llamada se genera dinámicamente desde el dominio que inicia el inicio de sesión (consulte `Authorize` en `internal/api/handlers/oauth/handler.go`):

```
https://<portal-domain>/api/v1/client-auth/oauth/google/callback
```

- El inicio de sesión se inició desde el dominio del portal (por ejemplo, `https://p.example.com`) → registrar `https://p.example.com/...`;
- Iniciado desde el dominio del panel `/p` → registrar `https://panel.example.com/...`.
- Google permite **múltiples** URI de redireccionamiento; registrar ambos dominios.
- ⚠ Google requiere que los URI de redireccionamiento sean **https** (excepto `http://localhost`);
 el proxy inverso debe reenviar `X-Forwarded-Host` / `X-Forwarded-Proto`; de lo contrario, el panel crea un
 http o redirección_uri interna → Google lo rechaza (`redirect_uri_mismatch`).

Después de configurar "Configuración del sistema → OAuth → Dominio de devolución de llamada del portal", la devolución de llamada URL que se muestra en la página es la que se debe pegar en Google.

### 3.2 Enlaces de Política de Privacidad/Términos en el portal

La tarjeta de inicio de sesión incluye enlaces "Política de privacidad · Términos de servicio" (que se muestran una vez configurados) y ambos documentos tienen **páginas independientes**
(el cuadro de diálogo y la página independiente comparten el mismo contenido):

- Dominio del panel principal: `https://<panel-domain>/p/legal/privacy`, `/p/legal/terms`
- Dominio de portal dedicado (implementación raíz): `https://<portal-domain>/legal/privacy`, `/legal/terms`
- Soporta `?lang=xx`; sin él, se utiliza el `Accept-Language` del navegador y luego el portal predeterminado.
- Las páginas se **representan en el servidor** (sin JS), lo que ayuda al rastreador de verificación de Google.

**Configuración (de cualquier manera; los enlaces ganan)**:

1. **Modo de contenido (recomendado, funciona de inmediato)**: editor del portal → Configuración → General → Documentos de cumplimiento →
 elija "Editar idioma" y pegue las plantillas del §4 en "Contenido de la política de privacidad" / "Contenido de los términos de servicio"
 (**Markdown, editado por idioma**) → después de guardar, la tarjeta de inicio de sesión muestra enlaces que abren un **diálogo**;
 El “enlace independiente” del editor se puede ingresar directamente en la pantalla de consentimiento Google.
2. **Modo de enlace externo**: ingresa “Política de privacidad URL” / “Términos de servicio URL” (apuntando a las páginas que alojas) → se abre en una nueva pestaña.

El contenido y los enlaces se entregan a través de `/api/v1/public/portal-config` (`privacy_policy_content` /
`terms_of_service_content` son **mapas de Markdown por idioma** `{"zh-CN":"...","en":"..."}`;
`privacy_policy_url` / `terms_of_service_url` son enlaces externos opcionales). **Deben ser accesibles sin iniciar sesión** ✓.

### 3.3 Interruptor del lado del panel

`System settings → OAuth`: habilite Google e ingrese ID de cliente / Secreto de cliente (el secreto está cifrado con AES-256-GCM en reposo).
El botón Google aparece en la página de inicio de sesión del portal solo cuando todo está configurado (`/api/v1/client-auth/oauth/providers` vuelve habilitado).

## 4. Política de privacidad/Plantillas de términos

> Plantillas de cumplimiento mínimas que puedes adaptar. Antes de usar: reemplace los marcadores de posición `【】`, agregue/elimine cláusulas para su negocio,
> y que la entidad operadora los revise. Estas plantillas no son asesoramiento legal.

### 4.1 Política de privacidad (plantilla)

```markdown
# Privacy Policy

Last updated: [DATE]

[OPERATOR NAME] ("we") operates [PORTAL NAME] ("the Service"). We respect your privacy;
this policy explains what information we collect and how we use it.

## 1. Information we collect
- **Account information**: the email address and password you provide at registration (stored only as a one-way hash);
- **Google sign-in information**: when you choose "Sign in with Google", we receive, with your authorization,
  your Google account email, display name and avatar URL (scopes: openid, email, profile);
- **Service data**: subscription configuration, traffic usage, device IP allow-list, tickets and order records.

## 2. How we use information
Only to: create and sign in to your account, provide the proxy subscription service, bill you and provide support,
and for security controls (sign-in protection, IP allow-list). We **do not** sell your personal information.

## 3. Third-party services
- **Google**: receives account information within the authorized scope only when you actively use "Sign in with Google";
- Payment providers: receive order information only when you initiate a payment.

## 4. Storage and security
Data is stored on our servers; credentials are encrypted at rest and all traffic uses TLS.

## 5. Your rights
You can update your profile/password and end sessions in the portal at any time; to delete your account and all data,
contact [CONTACT EMAIL] and we will process the request within [N] business days.

## 6. Policy changes
Material changes will be announced in the portal notice board.
```

### 4.2 Términos de servicio (plantilla)

```markdown
# Terms of Service

Last updated: [DATE]

## 1. Service
The Service provides [PROXY / NETWORK ACCESS] to registered users; specific plans are described on the store page.

## 2. Accounts
- You are responsible for the security of your account and password; for accounts created with Google sign-in, credentials are managed by Google;
- You may not lend or transfer your account, or use the Service for any unlawful purpose (including but not limited to:
  unauthorized access to others' systems, distributing illegal content, or circumventing mandatory legal requirements).

## 3. Payment and refunds
Plans are billed [monthly / by usage]; traffic and device limits follow the plan description;
[REFUND POLICY: e.g. full refund within 7 days if unused].

## 4. Changes and termination
We may suspend or terminate the Service due to force majeure or your breach of these terms; deleting an account deletes the related data.

## 5. Disclaimer
The Service is provided "as is"; to the extent permitted by law, we are not liable for indirect losses caused by use of the Service.

## 6. Miscellaneous
These terms are governed by the laws of [JURISDICTION]. Contact: [CONTACT EMAIL].
```

## 5. Lista de verificación previa al lanzamiento

- [] Pantalla de consentimiento de OAuth: nombre de la aplicación/logotipo/página de inicio/política de privacidad/términos todos presentes; dominio registrado
- [] Páginas de Política de privacidad y Términos **accesibles sin iniciar sesión** (página personalizada del portal o página estática independiente)
- [] Ambos enlaces visibles en la tarjeta/pie de página de inicio de sesión del portal
- [] redirección URI coincide con el dominio real y es https
- [] Publicar aplicación → enviar para verificación; la restricción del usuario de prueba se levanta después de la aprobación
- [] Configuración de OAuth del panel: habilitado = verdadero con ID de cliente/Secreto completado (el punto final del proveedor devuelve habilitado: verdadero)

## 6. Comparación de GitHub

Las aplicaciones GitHub OAuth **no** tienen estado de publicación ni flujo de verificación: cree la aplicación, configure la devolución de llamada URL y funcionará públicamente.
Tenga en cuenta que una aplicación OAuth GitHub acepta solo **una** devolución de llamada de autorización URL (coincidencia de prefijo); para múltiples dominios,
utilice el enfoque multi-URI de Google o cree una aplicación independiente por dominio del portal.
