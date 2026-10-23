<!-- source: README.md | locale: es | updated: 2026-10-23 -->
# Documentación CoreFusion

> Idiomas: [简体中文](README.md) · [English](README.en.md) · [繁體中文](README.zh-TW.md) · [Русский](README.ru.md) · [日本語](README.ja.md) · [한국어](README.ko.md) · [Deutsch](README.de.md) · [Français](README.fr.md) · Español · [Português](README.pt.md) · [Türkçe](README.tr.md) · [Bahasa Indonesia](README.id.md) · [ไทย](README.th.md) · [Tiếng Việt](README.vi.md) · [العربية](README.ar.md) · [فارسی](README.fa.md)

## [USER-GUIDE.es.md](./USER-GUIDE.es.md) — Guía del usuario

Una guía de operaciones completa para **usuarios** (operaciones/administradores/operadores): instalación, configuración inicial, módulos de funciones, preguntas frecuentes y apéndice.

## [api/](./api/) — Referencia pública de API

Para integradores, desarrollo secundario y clientes externos:

- [api/README.es.md](./api/README.es.md) — descripción general: base URL, autenticación, formato de respuesta, límites de velocidad
- [api/subscription.es.md](./api/subscription.es.md): punto final de suscripción (`/sub/:token`), formatos, encabezados de respuesta, ejemplos
- [api/portal-api.es.md](./api/portal-api.es.md) — portal del cliente API (registro/inicio de sesión, planes, pedidos, saldo, tickets)
- [api/charge-callback.es.md](./api/charge-callback.es.md) — firma y verificación de devolución de llamada de pago
- [api/admin-api.es.md](./api/admin-api.es.md) — admin Tabla de rutas API agrupada por módulo

## Documentos de diseño

Los documentos de diseño son materiales internos (propiedad intelectual principal) y **no se distribuyen** con este repositorio. Póngase en contacto con el mantenedor si los necesita.

> **Alcance**: la aplicación cliente (UI local, adquisición de TUN, bandeja, fusión de suscripciones múltiples) es un producto independiente y está fuera del alcance de este repositorio. La documentación aquí cubre únicamente el lado del servidor (panel/nodos/entrega de suscripción).

## Otras guías de operaciones

- `cert-google-eab.md` — Google EAB emisión de certificado *(chino)*
- `oauth-google-verification.md` — Google Verificación OAuth *(chino)*
- `notification-templates.md` — variables de plantilla de notificación *(chino)*

## Cobertura de traducción

| Documento | zh-CN | en | zh-TW | Otros locales |
|----------|-------|----|-------|---------------|
| README.md | ✅ | ✅ | ✅ | ru ✅ |
| USER-GUIDE.md | ✅ | ✅ | ✅ | ru ✅ |
| I18N.md | ✅ | — | — | — |
| api/README.md | ✅ | ✅ | ✅ | ru ✅ |
| api/subscription.md | ✅ | ✅ | ✅ | ru ✅ |
| api/portal-api.md | ✅ | ✅ | ✅ | ru ✅ |
| api/charge-callback.md | ✅ | ✅ | ✅ | ru ✅ |
| api/admin-api.md | ✅ | ✅ | ✅ | ru ✅ |
| cert-google-eab.md | ✅ | ✅ | ✅ | ru ✅ |
| oauth-google-verification.md | ✅ | ✅ | ✅ | ru ✅ |
| notification-templates.md | ✅ | ✅ | ✅ | ru ✅ |
| LEGAL.md | ✅ | ✅ | ✅ | ru ✅ |
| THIRD_PARTY_LICENSES.md | ✅ | ✅ | ✅ | ru ✅ |

Consulte [I18N.md](README.es.md) para conocer nombres, navegación por idiomas y reglas de revisión *(chino)*.
