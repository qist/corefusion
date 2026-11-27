# Шаблон формата подписки Xray

<sub>[简体中文](README.md) · [English](README.en.md) · Русский · [فارسی](README.fa.md)</sub>

[`config.json`](./config.json) — это `config_json` формата подписки **xray_json**.
Он содержит только клиентские фиксированные секции:

```text
log / dns / inbounds / routing
```

`outbounds` генерирует панель, плюс добавляются `direct` / `block` и **balancer**
`tag: proxy` (в Xray нет selector — random-balancer играет роль прокси-группы).
Правила с `"outboundTag": "proxy"` переписываются в `"balancerTag": "proxy"`.
**В подписке всегда есть `proxy`** — ручная правка не нужна. Имена узлов,
совпадающие с `proxy` / `direct` / `block`, переименовываются автоматически.

## 1. Загрузка

```text
Панель → Формат подписки → Создать
  client_type: v2ray
  format:      xray_json
  содержимое:  вставить config.json
```

```text
https://<panel>/sub/<token>?format=xray_json&config=<format>
```

## 2. Подписчики и автономное использование

- **Подписчику нужна только ссылка подписки** — без аккаунта панели и ручной правки.
- **Без панели** используйте [`standalone.json`](./standalone.json): полный конфиг,
  проходит `xray run -test`. Замените `node-example` в `outbounds`
  (заглушка `127.0.0.1:1080` socks) на свой узел.

## 3. Структура

| Секция | Назначение |
| --- | --- |
| `inbounds` | SOCKS `127.0.0.1:10808` + HTTP `127.0.0.1:10809` |
| `dns` | домены Китая → Alibaba DoH, глобальные → Google DoH, остальное → `1.1.1.1` |
| `routing.rules` | реклама → `block`; STUN (3478/19302) → `block`; Китай → `direct`; остальное → `proxy` |
| `routing.domainStrategy` | `IPIfNonMatch` |

## 4. Geodata

Правила используют встроенные теги `geosite:` / `geoip:`:

```text
geosite:category-ads-all
geosite:cn
geosite:geolocation-!cn
geoip:cn
```

Клиенту нужны `geosite.dat` / `geoip.dat` (панель синхронизирует их в `data/rules/`).
Для своих данных положите их в каталог `XRAY_LOCATION_ASSET`.

## 5. Проверка

```bash
curl -s "https://<panel>/sub/<token>?format=xray_json&config=<format>" -o xr.json
XRAY_LOCATION_ASSET=/opt/corefusion/data/rules xray run -test -c xr.json
# Configuration OK.
```

## 6. Известные предупреждения

WebSocket `headers.Host` и Trojan без flow вызывают предупреждения об устаревании
в Xray 26.x. Это особенности протокола узла, загрузку они не блокируют.

## 7. Внешние ссылки

- [Каталог в репозитории](https://github.com/qist/corefusion/tree/main/docs/xray-templates)
- Исходные файлы:
  - [config.json](https://raw.githubusercontent.com/qist/corefusion/main/docs/xray-templates/config.json)
  - [standalone.json](https://raw.githubusercontent.com/qist/corefusion/main/docs/xray-templates/standalone.json)
- Документация:
  - Xray-core: <https://xtls.github.io/>
  - geosite / geoip: <https://github.com/v2fly/domain-list-community>

- Репозиторий проекта: <https://github.com/qist/corefusion>
- Репозиторий правил (clash-правила для шаблонов): <https://github.com/qist/xray>
