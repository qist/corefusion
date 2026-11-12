<!-- source: subscription.md | locale: de | updated: 2026-11-12 -->
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
| `platform` | Ordnen Sie eine Konfigurationsvorlage anhand des Plattformnamens zu (wird verwendet, wenn `config` weggelassen wird); Auch das Format muss übereinstimmen |
| `providers` | Nur `mihomo_yaml`: `1` / `true` aktiviert den **Provider-Modus** (`proxy-providers` ausgeben und über `use` referenzieren); Der Standardwert oder ein anderer Wert sorgt dafür, dass die Knoten unter `proxies` inline bleiben. Siehe §6 |

### Unterstützte `format`-Werte

| Wert | Ausgabe | Kunden |
|---|---|---|
| `base64_uri` (Standard) | Durch Zeilenumbrüche verbundene Freigabelinks, als Ganzes base64-kodiert | V2RayN, Shadowrocket, Surge und andere generische Clients |
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
| `Profile-Web-Page-Url` | `https://portal.example.com` | Portal URL („Webseite öffnen“ in Clients; Root-Pfad, wenn das Portal über einen eigenen Port verfügt, andernfalls /p) |
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

# 5) Clash dynamic subscription: config template + Provider mode
curl -L "https://sub.example.com/abcd1234efgh5678?format=clash&config=out&providers=1"

# 6) sing-box / Xray with their own config templates (rules come from the template)
curl -L "https://sub.example.com/abcd1234efgh5678?format=singbox_json&config=sing-box-home"
curl -L "https://sub.example.com/abcd1234efgh5678?format=xray_json&config=xray-home"
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

## 6. Anbietermodus und mehrere Abonnementdomänen

### 6.1 Was `providers=1` ausstrahlt

Mit `format=mihomo_yaml&providers=1` (oder `format=clash&providers=1`) der YAML:

- Inline-Knoten nicht mehr auf der obersten Ebene – es wird `proxy-providers:` ausgegeben;
- Proxy-Gruppen verweisen auf Knotenquellen über `use: [<provider-key>...]`;
- Alle `proxy-providers`, die in einem gemeinsam genutzten Abonnementformat (`mihomo_configs.config_json`) geschrieben sind, werden **ignoriert** (eine gemeinsam genutzte Konfiguration bedient mehrere Pläne; fest codierte Anbieter würden zwischen den Plänen durchsickern). Gruppen → Quellzuordnungen live unter „Planverwaltung → Gruppe → Quellzuordnung“.

Anbieterarten:

| Freundlich | Schlüssel | Notizen |
|---|---|---|
| Intern | Bereinigter Knotengruppenname | Pro Planknotengruppe generiert; enthält nur die Knoten dieser Gruppe |
| Reserviert | `subscription-nodes` | Alle Knoten des Plans; In der Quelle URL wird der Parameter `provider` **weggelassen** |
| Extern | Im Plan konfigurierter Name | Externe Quelle (`http` YAML oder `file`) |

Gemeinsame Felder pro Anbieter:

- `url` – verweist auf dieses Abonnement mit `format=mihomo_proxies_yaml` und erbt `config` / `platform` von der Hauptanfrage; interne Anbieter hängen `provider=<key>` an;
- `path` – **immer abgeleitet von „internen Anbieterstandards → Cache-Pfadvorlage“** (Standard: `./providers/{key}.yaml`, ersetzt durch `{key}`). Auch externe Anbieter können keinen eigenen Weg vorgeben;
- `health-check` — `enable / url / interval`; interne Anbieter erben die Standardeinstellungen, externe Anbieter konfigurieren sie im Plan;
- `interval` – Aktualisierungsintervall in Sekunden.

### 6.2 Welche Domain geht in den Provider URL (Multi-Domain)

„Abonnementcenter → Abonnementendpunkte“ kann **mehrere** Datensätze enthalten (Domäne + Pfad + aktiviert). Der Anbieter `url` wählt die Domain in dieser Reihenfolge aus:

1. **Die von dieser Anfrage tatsächlich verwendete Domäne**: der erste Wert von `X-Forwarded-Host` (Port entfernt), andernfalls `Host`; es muss mit einem **aktivierten** Endpunkt übereinstimmen;
2. Wenn nichts übereinstimmt, greifen Sie auf den aktivierten Endpunkt mit der **niedrigsten ID** zurück.
3. Der Pfad kommt vom übereinstimmenden Endpunkt (`path`, Standard `/`) und ergibt `https://<domain><path>/<token>?format=mihomo_proxies_yaml...`.

Folgen:

- Ein Client, der über eine neue Domäne abruft, erhält diese Domäne innerhalb des Providers – **keine Panel-Änderung erforderlich**;
- Eine alte, noch in der Liste aktivierte Domain ist nicht mehr fest codiert;
- Wenn ein Client den alten Anbieter URL zwischengespeichert hat, wird er durch Aktualisieren des Abonnements (oder Löschen des Anbietercaches/Neustarten des Clients) umgeschaltet.

Reverse-Proxys müssen `X-Forwarded-Proto` und `X-Forwarded-Host` weiterleiten; andernfalls wird der Fallback-Endpunkt verwendet.

## 7. Notizen

- **Format und Konfiguration sind eng miteinander verbunden**: Die von `config`/`platform` ausgewählte Vorlage muss mit der `format` dieser Anfrage übereinstimmen; Nicht übereinstimmende Vorlagen werden ignoriert – dies verhindert das Einfügen von Mihomo-Regeln in sing-box/Xray JSON.
- **`providers=1` betrifft nur Mihomo YAML**: sing-box / Xray / base64 / raw ignorieren es (Knoten bleiben inline).
- **Ratenbegrenzung**: Der Abonnementendpunkt hat sein eigenes Limit (`SubscribeRateLimit`); Beleidigende Klienten werden abgewiesen.
- **Zugriffsprotokoll**: Bei jedem Abruf werden IP-/UA-/CDN-Informationen aufgezeichnet, sichtbar unter „Abonnement-Zugriffsprotokoll“ in der Admin-Benutzeroberfläche.
- Ohne eine auf einem dedizierten Abonnement-Port konfigurierte Domäne werden die `X-Forwarded-Proto`/`X-Forwarded-Host` der Anfrage verwendet; Stellen Sie sicher, dass Ihr Reverse-Proxy beide Header weiterleitet.
- Achten Sie bei Bereitstellungen mit mehreren Domänen darauf, dass die Endpunktliste an den Domänen ausgerichtet ist, die Clients tatsächlich verwenden. siehe §6.2.
