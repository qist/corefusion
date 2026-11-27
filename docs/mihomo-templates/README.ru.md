# Шаблоны формата подписки mihomo

<sub>[简体中文](README.md) · [English](README.en.md) · Русский · [فارسی](README.fa.md)</sub>

Три шаблона: **Windows / Android / OpenClash (роутер)**.

Это YAML, который хранится в панели в разделе **Формат подписки**: без `proxies`
и без `proxy-providers`. Узлы генерируются панелью для каждого тарифа при выдаче.

| Файл | Клиент | Перехват трафика |
| --- | --- | --- |
| [`mihomo-windows.yaml`](./mihomo-windows.yaml) | Clash Verge Rev / Mihomo Party / FlClash PC | `mixed-port` + системный прокси |
| [`mihomo-android.yaml`](./mihomo-android.yaml) | Clash Meta for Android / FlClash | VPN/TUN создаёт приложение |
| [`mihomo-openclash.yaml`](./mihomo-openclash.yaml) | OpenClash (OpenWrt) | redirect/tproxy, TUN опционально |

## 1. Динамическая генерация

| Место | Значение |
| --- | --- |
| `_hasExternalNodes: true` | «в эту группу добавить внешние узлы»: в обычном режиме — имена узлов, в provider-режиме — `use` |
| верхний `proxies:` | только специальные outbound (например `dns-out`) |
| верхний `proxy-providers:` | **не добавлять**: панель его игнорирует (один YAML используется многими тарифами) |
| `rules:` | после импорта правила выводит панель; в шаблоне это только образец |

```text
вставить шаблон → сохранить как формат подписки → клиент тянет /sub/<token>
    по умолчанию : узлы вставляются в группы с _hasExternalNodes
    ?providers=1 : выводится proxy-providers, группам добавляется `use`
```

## 2. Импорт в панель

1. Панель → **Формат подписки** (конфигурация mihomo) → создать/изменить;
2. Вставить YAML → сохранить;
3. Привязать к тарифу и настроить источники: **Тариф → Источники подписки**.

Ссылки для клиента:

```text
https://<panel>/sub/<token>
https://<panel>/sub/<token>?format=mihomo_yaml&config=<format>&providers=1
```

## 3. SSH (порт 22) только через выбранные узлы

```yaml
- DST-PORT,22,🖥️ SSH,no-resolve

- name: 🖥️ SSH
  type: select
  proxies:
    - DIRECT
```

По умолчанию в группе только `DIRECT` — узлы не добавляются автоматически.
Чтобы разрешить конкретную группу узлов:

```text
Панель → Тариф → Источники подписки → группа → источники
  группа: 🖥️ SSH
  источник: <группа узлов, например hysteria>
```

Итог: `22 → 🖥️ SSH → use: [hysteria]` (только эти узлы), остальные группы не затрагиваются.

> Привязка задаётся в тарифе (не в шаблоне) и сравнивается по **точному имени группы**.
> Раньше группа называлась `ACCESS-DENIED`; после перехода на `🖥️ SSH` обновите
> привязку, иначе порт 22 пойдёт только через `DIRECT`.

## 4. Правила и DNS

- `rule-providers` указывают на `https://rules.example.com/clash/Ruleset/...` —
  замените на свою библиотеку правил (публичные источники — в китайской версии README).
- geodata берётся из публичного релиза MetaCubeX:

```yaml
geox-url:
  geoip: "https://fastly.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geoip.dat"
  geosite: "https://fastly.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geosite.dat"
  mmdb: "https://fastly.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geoip.metadb"
```

- DNS только публичный DoH: Alibaba/Tencent для Китая, Google/Cloudflare для остального.

## 5. Различия платформ

| Параметр | Windows | Android | OpenClash |
| --- | --- | --- | --- |
| `allow-lan` | false | false | **true** |
| `ipv6` | false | true | true |
| `find-process-mode` | strict | strict | **"off"** (в кавычках, иначе YAML превратит в `false`) |
| `tun` | нет (системный прокси) | нет (приложение само) | **включён**, `stack: system` |
| `external-controller` | `127.0.0.1:9090` | `127.0.0.1:9090` | `0.0.0.0:9090` |

## 6. Проверка

```bash
curl -s "https://<panel>/sub/<token>?format=mihomo_yaml" -o test.yaml
mihomo -t -d <tmpdir> -f test.yaml
# configuration file ... test is successful
```

## 7. Внешние ссылки

- [Каталог в репозитории](https://github.com/qist/corefusion/tree/main/docs/mihomo-templates)
- Исходные файлы:
  - [mihomo-windows.yaml](https://raw.githubusercontent.com/qist/corefusion/main/docs/mihomo-templates/mihomo-windows.yaml)
  - [mihomo-android.yaml](https://raw.githubusercontent.com/qist/corefusion/main/docs/mihomo-templates/mihomo-android.yaml)
  - [mihomo-openclash.yaml](https://raw.githubusercontent.com/qist/corefusion/main/docs/mihomo-templates/mihomo-openclash.yaml)
- Публичные источники правил:
  - Loyalsoldier/clash-rules: <https://github.com/Loyalsoldier/clash-rules>
  - ispip (китайские IP): <https://ispip.clang.cn/>
  - MetaCubeX meta-rules-dat: <https://github.com/MetaCubeX/meta-rules-dat>

- Репозиторий проекта: <https://github.com/qist/corefusion>
- Репозиторий правил (clash-правила для шаблонов): <https://github.com/qist/xray>
