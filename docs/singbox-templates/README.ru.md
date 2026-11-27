# Шаблон формата подписки sing-box

<sub>[简体中文](README.md) · [English](README.en.md) · Русский · [فارسی](README.fa.md)</sub>

[`config.json`](./config.json) — это `config_json` формата подписки **sing-box** в панели.
Он содержит только клиентские фиксированные секции:

```text
log / dns / inbounds / route / experimental
```

`outbounds` генерирует панель из узлов подписки и **всегда добавляет selector
`tag: proxy`**. То есть в подписке всегда есть `proxy` — правила можно ссылать на него
без ручной правки. Если узел называется `proxy` / `direct` / `block`, он
переименовывается (`proxy-2`, …), чтобы не было дублей tag.

## 1. Загрузка

```text
Панель → Формат подписки → Создать
  client_type: singbox
  format:      singbox_json
  содержимое:  вставить config.json
```

Ссылка подписки:

```text
https://<panel>/sub/<token>?format=singbox_json&config=<format>
```

> Раньше `inbounds` не объединялся (только log/dns/route/experimental) —
> теперь объединяется, иначе клиент получал конфиг без слушающих портов.

## 2. Подписчики и автономное использование

- **Подписчику нужна только ссылка подписки** — без аккаунта панели и ручной правки.
- **Без панели** используйте [`standalone.json`](./standalone.json): полный конфиг,
  проходит `sing-box check`. Замените `node-example` в `outbounds`
  (заглушка `127.0.0.1:1080` socks5) на свой узел.

## 3. Структура

| Секция | Назначение |
| --- | --- |
| `inbounds` | `mixed` на `127.0.0.1:7890`; TUN добавьте сами при необходимости |
| `dns` | новый формат DNS (sing-box 1.12+): DoH для Китая → `direct`, глобальный → `proxy` |
| `route.rule_set` | удалённые `.srs` MetaCubeX: `cn`, `category-ads-all`, `geolocation-!cn`, `geoip/cn` |
| `route.rules` | реклама → reject; STUN (3478/19302) → reject; Китай → direct; остальное → proxy |
| `route.final` | `proxy` (добавляемый selector) |

## 4. DNS (новый формат)

```json
{ "type": "https", "tag": "dns-cn", "server": "223.5.5.5", "path": "/dns-query", "detour": "direct" }
```

Старый вид `{"address": "...", "detour": "..."}` удалён в sing-box 1.12.

## 5. TUN (опционально)

```json
{
  "type": "tun",
  "tag": "tun-in",
  "address": ["172.19.0.1/30"],
  "auto_route": true,
  "strict_route": false,
  "stack": "system"
}
```

`stack` устарел в sing-box 1.15, но пока работает (только WARN).

## 6. Наборы правил

```text
https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geosite/<name>.srs
https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geoip/<name>.srs
```

Clash `.txt`/`.yaml` напрямую не подходят — сначала конвертируйте в `.srs`
(`sing-box rule-set convert`).

## 7. Проверка

```bash
curl -s "https://<panel>/sub/<token>?format=singbox_json&config=<format>" -o sb.json
sing-box check -c sb.json
# exit 0 = OK
```

## 8. Внешние ссылки

- [Каталог в репозитории](https://github.com/qist/corefusion/tree/main/docs/singbox-templates)
- Исходные файлы:
  - [config.json](https://raw.githubusercontent.com/qist/corefusion/main/docs/singbox-templates/config.json)
  - [standalone.json](https://raw.githubusercontent.com/qist/corefusion/main/docs/singbox-templates/standalone.json)
- Документация:
  - sing-box: <https://sing-box.sagernet.org/>
  - наборы правил: <https://github.com/MetaCubeX/meta-rules-dat>

- Репозиторий проекта: <https://github.com/qist/corefusion>
- Репозиторий правил (clash-правила для шаблонов): <https://github.com/qist/xray>
