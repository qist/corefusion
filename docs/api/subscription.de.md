<!-- source: subscription.md | locale: de | updated: 2026-10-23 -->
# Abonnement API (öffentlich)

> Sprachen: [简体中文](subscription.md) · [English](subscription.en.md) · [繁體中文](subscription.zh-TW.md) · [Русский](subscription.ru.md) · [日本語](subscription.ja.md) · [한국어](subscription.ko.md) · Deutsch · [Français](subscription.fr.md) · [Español](subscription.es.md) · [Português](subscription.pt.md) · [Türkçe](subscription.tr.md) · [Bahasa Indonesia](subscription.id.md) · [ไทย](subscription.th.md) · [Tiếng Việt](subscription.vi.md) · [العربية](subscription.ar.md) · [فارسی](subscription.fa.md)

Ruft die Abonnementknotenliste eines Benutzers ab. **Dies ist der am häufigsten verwendete Endpunkt für Endbenutzer**; Es sind keine Header erforderlich – der Zugriff wird durch das Token im URL gewährt.

## 1. Endpunkt

| Bereitstellung | URL |
|---|---|
| Hauptpanel | `GET https://<panel-domain>/sub/<token>` |
| Dedizierter Abonnement-Port | `GET https://<sub-domain>:<port>/<token>` (kein `/sub`-Präfix) |

`<token>` ist das eindeutige Zugriffstoken eines Abonnements, das von der Administratorseite generiert wird (sichtbar unter „Meine Abonnements“ im Portal).

## 2. Abfrageparameter

| Parameter | Beschreibung |
|---|---|
| `format` | Ausgabeformat; überschreibt den Abonnementstandard. Siehe unten |
| `config` | Name der Konfigurationsvorlage (definiert unter „Abonnementformate“ in der Admin-Benutzeroberfläche). Muss dasselbe Format wie `format` dieser Anfrage verwenden, andernfalls wird es ignoriert |
| `platform` | Ordnen Sie eine Konfigurationsvorlage anhand des Plattformnamens zu (wird verwendet, wenn `config` weggelassen wird). Auch das Format muss übereinstimmen |

### Unterstützte `format`-Werte

| Wert | Ausgabe | Kunden |
|---|---|---|
| `base64_uri` (Standard) | Durch Zeilenumbrüche verbundene Freigabelinks, als Ganzes base64-codiert | V2RayN, Shadowrocket, Surge und andere generische Clients |
| `raw_uri` | Mit Zeilenumbruch verbundene Freigabelinks, einfacher Text | Clients, die einfache Links/Debugging benötigen |
| `mihomo_yaml` | Vollständige Clash/Mihomo YAML-Konfiguration (mit Proxy-Gruppen, Regeln) | Clash.Meta / mihomo / Clash Verge Rev |
| `mihomo_proxies_yaml` | YAML nur mit der Knotenliste `proxies:` | Mihomo-Proxy-Provider-Quelle (mit geplanter Aktualisierung) |
| `singbox_json` | Vollständige sing-box JSON-Konfiguration | sing-box / SFI / FLClash (sing-box-Kern) |
| `xray_json` | Vollständige Xray JSON-Konfiguration | Xray-Kern |

> Es gilt Protokoll-/Formatkompatibilität: Beispielsweise erscheint `anytls` nur in den sing-box/Mihomo-Formaten; REALITY-Knoten auf einem sing-box-Server werden in Xray/sing-box JSON-Abonnements nicht angezeigt. Knoten, die der Client nicht erkennen kann, werden automatisch gelöscht, anstatt dass das gesamte Abonnement fehlschlägt.

## 3. Antwortheader

| Kopfzeile | Beispiel | Beschreibung |
|---|---|---|
| `Subscription-Userinfo` | `upload=1024; download=2048; total=1073741824; expire=1760000000` | Nutzung und Ablauf (Bytes/Unix-Sekunden). **Wird auch auf 403 gesendet**, damit Kunden das verbleibende Guthaben anzeigen können |
| `Profile-Update-Interval` | `6` | Empfohlenes Aktualisierungsintervall (Stunden) |
| `Profile-Title` | `CoreFusion` | Profilname (angezeigt von Clash Verge Rev / V2RayN usw.) |
| `Profile-Web-Page-Url` | `https://…/portal` | Link zur Portalseite („Webseite öffnen“ in Clients) |
| `Content-Type` | Pro Format: `text/plain` / `text/yaml` / `application/json` |  |
| `Content-Disposition` | `attachment; filename="subscription.txt"` |  |

## 4. Statuscodes

| HTTP | Code | Bedeutung |
|---|---|---|
| 200 | — | Normal (Hauptteil ist das in `format` gerenderte Abonnement) |
| 400 | `MISSING_TOKEN` | Kein Token im URL |
| 403 | (Klartext) | Abonnement ausgesetzt / abgelaufen / inaktiv. Die Nachricht wird über `Accept-Language` lokalisiert (zh/en/ja/ko) und ist ein einzeiliger Grund |
| 404 | `SUBSCRIPTION_NOT_FOUND` | Token existiert nicht |

> Widerrufene Abonnements werden als nicht vorhanden behandelt (404).

## 5. Beispiele

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

Dynamisches Clash/Mihomo-Abonnement (regelmäßige Aktualisierung):

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

## 6. Notizen

- **Format und Konfiguration sind eng miteinander verbunden**: Die von `config`/`platform` ausgewählte Vorlage muss mit der `format` dieser Anfrage übereinstimmen. Nicht übereinstimmende Vorlagen werden ignoriert – dies verhindert das Einfügen von Mihomo-Regeln in sing-box/Xray JSON.
- **Ratenbegrenzung**: Der Abonnementendpunkt hat sein eigenes Limit (`SubscribeRateLimit`); Beleidigende Klienten werden abgewiesen.
- **Zugriffsprotokoll**: Bei jedem Abruf werden IP-/UA-/CDN-Informationen aufgezeichnet, sichtbar unter „Abonnement-Zugriffsprotokoll“ in der Admin-Benutzeroberfläche.
- Ohne eine auf einem dedizierten Abonnement-Port konfigurierte Domäne werden die `X-Forwarded-Proto`/`X-Forwarded-Host` der Anfrage verwendet; Stellen Sie sicher, dass Ihr Reverse-Proxy beide Header weiterleitet.
