<!-- source: LEGAL.md | locale: es | updated: 2026-10-23 -->
# Notas legales y de cumplimiento

> Esta traducción se ofrece únicamente para facilitar la consulta. En caso de discrepancia, prevalece el original en chino.

> Idiomas: [简体中文](LEGAL.md) · [English](LEGAL.en.md) · [繁體中文](LEGAL.zh-TW.md) · [Русский](LEGAL.ru.md) · [日本語](LEGAL.ja.md) · [한국어](LEGAL.ko.md) · [Deutsch](LEGAL.de.md) · [Français](LEGAL.fr.md) · Español · [Português](LEGAL.pt.md) · [Türkçe](LEGAL.tr.md) · [Bahasa Indonesia](LEGAL.id.md) · [ไทย](LEGAL.th.md) · [Tiếng Việt](LEGAL.vi.md) · [العربية](LEGAL.ar.md) · [فارسی](LEGAL.fa.md)

> Este documento explica la licencia de CoreFusion, la política de uso comercial, las obligaciones de terceros y los puntos de cumplimiento que los usuarios deben observar en sus propias jurisdicciones.
>
> **Este documento no es asesoramiento legal.** El cumplimiento depende de su región de implementación, clientes y modelo operativo; consulte a un abogado local calificado.
>
> *Esta traducción se proporciona para su comodidad. Si difiere del original chino, prevalecerá la versión china.*

## 1. Licencia de proyecto

CoreFusion se publica bajo la **Licencia pública general GNU Affero v3.0 (AGPL-3.0)**; consulte [LICENSE](./LICENSE) para obtener el texto completo.

```
Copyright (C) 2026 qist

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published
by the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
```

## 2. Uso comercial y hosting comercial: expresamente permitido

**Cualquier individuo u organización puede utilizar CoreFusion con fines comerciales, incluidos, entre otros:**

- Autohospedaje e implementación interna;
- Ejecutar un servicio de proxy/nodo pago para terceros;
- Proporcionar SaaS, alojamiento, operaciones gestionadas o integración de sistemas a terceros a cambio de una tarifa;
- Desarrollo secundario, personalización y distribución (incluida la entrega con hardware o como parte de una solución).

**El uso comercial y el alojamiento comercial mencionados anteriormente no requieren autorización por separado ni tarifa de licencia ni participación en los ingresos.**

Las únicas obligaciones son las de la propia AGPL-3.0, principalmente dos:

1. **Al distribuir archivos binarios** (§4–§6): proporcione a los destinatarios la fuente correspondiente (fuente correspondiente completa) o una oferta escrita efectiva conforme al §6; conservar los avisos de derechos de autor y el texto de la licencia.
2. **Al proporcionar una versión modificada a través de una red** (§13): si publica una versión modificada y permite que terceros interactúen con ella a través de una red, debe **ofrecer a esos usuarios la fuente correspondiente de la versión modificada de forma gratuita** (por ejemplo, un enlace destacado de descarga/repositorio de la fuente en el panel).

Notas adicionales:

- Implementar una copia **sin modificar** y servir a los usuarios no activa, por sí solo, la obligación de fuente §13; la distribución de binarios todavía se incluye en el punto 1.
- No está prohibido cobrar dinero; AGPL rige la **disponibilidad de la fuente**, no el modelo de negocio. No puede rechazar el artículo 13 de una versión modificada simplemente porque opera comercialmente o es de código cerrado.
- Si desea una **licencia comercial de código cerrado no sujeta a AGPL**, comuníquese con el titular de los derechos de autor por separado. Tenga en cuenta que este proyecto vincula componentes GPL-3.0 de terceros (consulte la sección 3); cualquier versión de código cerrado debe eliminarlos primero.

## 3. Componentes de terceros y redistribución

Los principales componentes de terceros y sus licencias se enumeran en [THIRD_PARTY_LICENSES.md](LEGAL.es.md).

Notas especiales:

- A través de `github.com/xtls/xray-core`, este proyecto vincula indirectamente **GPL-3.0 o posterior** `github.com/sagernet/sing` y `sing-shadowsocks`. Por lo tanto, **la distribución de binarios del panel o del Agente también requiere satisfacer las obligaciones de fuente correspondiente de GPL-3.0**.
- En tiempo de ejecución, el panel puede descargar o invocar programas independientes como Xray-core, sing-box y Caddy. Estos **no se distribuyen en este repositorio**, pero si los envía con sus entregables debe cumplir con sus licencias (especialmente la GPL-3.0 de sing-box: proporcione su fuente correspondiente). Hysteria2 y TUIC son protocolos transportados por sing-box, no binarios independientes.
- Los datos GeoIP/GeoSite incluidos (GeoLite2, `geoip.dat`, `geosite.dat`) están sujetos a sus propias licencias de datos; Verifique los términos antes del uso comercial.

## 4. Cumplimiento regional y responsabilidad del usuario

CoreFusion es **software de uso general**: no proporciona ningún proxy ni servicio de red, no envía nodos, ancho de banda ni credenciales, y no cobra tarifas. Las operaciones son responsabilidad del implementador.

Los implementadores/operadores deben determinar si su negocio requiere licencias, presentaciones o evaluaciones de seguridad en su jurisdicción, incluidos, entre otros:

- **China continental**: licencias comerciales de telecomunicaciones de valor agregado (ICP/IDC/ISP, etc.), presentación de ICP, MLPS, evaluación de seguridad de transferencia de datos transfronteriza y obligaciones bajo la Ley de Ciberseguridad, la Ley de Seguridad de Datos y la Ley de Protección de Información Personal. No utilice este software para fines que violen las leyes locales.
- **UE/Reino Unido**: RGPD/RGPD del Reino Unido, privacidad electrónica, obligaciones del procesador/controlador de la Ley de Servicios Digitales (DSA), registro y cumplimiento de cookies.
- **Estados Unidos**: leyes estatales de privacidad, controles de exportación de cifrado (EAR) y control de sanciones.
- **Otras regiones**: telecomunicaciones locales, comunicaciones cifradas, gestión de contenidos, localización de datos y normas de protección al consumidor.

Los usuarios aceptan no utilizar este software para:

- Actividades que violen la ley aplicable o las normas de control/sanciones de exportaciones;
- Infringir la propiedad intelectual, la privacidad u otros derechos legales;
- Distribuir contenido ilegal, ataques a la red, fraude u otra actividad delictiva.

**Cualquier responsabilidad legal que surja del uso o mal uso de este software recae en el usuario.**

## 5. Protección de datos y privacidad

El panel puede almacenar cuentas, detalles de contacto, pedidos, devoluciones de llamadas de pago, suscripciones, estadísticas de tráfico y registros de inicio de sesión/operaciones. Como procesador/controlador, los operadores deben:

- Proporcionar a los usuarios una política de privacidad y obtener el consentimiento necesario;
- Seguir la minimización de datos y establecer períodos de retención razonables para registros y estadísticas (el proyecto proporciona políticas de limpieza de registros que los operadores deben configurar);
- Solicitudes de soporte para acceder, corregir y eliminar información personal;
- Aplicar control de acceso y cifrado a bases de datos, copias de seguridad, claves privadas de certificados y credenciales de pago;
- Complete evaluaciones de transferencia de datos transfronteriza cuando realice implementaciones transfronterizas.

## 6. Descargo de responsabilidad

Este software se proporciona "tal cual" según AGPL-3.0 §§15-17, sin ninguna garantía expresa o implícita, incluida la comerciabilidad, la idoneidad para un propósito particular y la no infracción. El titular de los derechos de autor no es responsable de los daños directos, indirectos, incidentales o consecuentes que surjan del uso de este software.

## 7. Contacto

- Proyecto: <https://github.com/qist/corefusion>
- Preguntas sobre licencias comerciales/cumplimiento: utilice los problemas del proyecto o los datos de contacto publicados en la página del repositorio.
