<!-- source: subscription.md | locale: ru | updated: 2026-10-23 -->
# Подписка API (общедоступная)

> Языки: [简体中文](subscription.md) · [English](subscription.en.md) · [繁體中文](subscription.zh-TW.md) · Русский · [日本語](subscription.ja.md) · [한국어](subscription.ko.md) · [Deutsch](subscription.de.md) · [Français](subscription.fr.md) · [Español](subscription.es.md) · [Português](subscription.pt.md) · [Türkçe](subscription.tr.md) · [Bahasa Indonesia](subscription.id.md) · [ไทย](subscription.th.md) · [Tiếng Việt](subscription.vi.md) · [العربية](subscription.ar.md) · [فارسی](subscription.fa.md)

Извлекает список узлов подписки пользователя. **Это наиболее часто используемая конечная точка для конечных пользователей**; заголовки не требуются — доступ предоставляется по токену в URL.

## 1. Конечная точка

| Развертывание | URL |
|---|---|
| Основная панель | `GET https://<panel-domain>/sub/<token>` |
| Выделенный порт подписки | `GET https://<sub-domain>:<port>/<token>` (без префикса `/sub`) |

`<token>` — это уникальный токен доступа к подписке, созданный администратором (его можно увидеть в разделе «Мои подписки» на портале).

## 2. Параметры запроса

| Параметр | Описание |
|---|---|
| `format` | Выходной формат; переопределяет подписку по умолчанию. См. ниже |
| `config` | Имя шаблона конфигурации (определено в разделе «Форматы подписки» в пользовательском интерфейсе администратора). Должен использовать тот же формат, что и `format` этого запроса, в противном случае он игнорируется. |
| `platform` | Сопоставьте шаблон конфигурации с именем платформы (используется, если `config` опущен); формат также должен совпадать |

### Поддерживаемые значения `format`

| Значение | Выход | Клиенты |
|---|---|---|
| `base64_uri` (по умолчанию) | Ссылки общего доступа, соединенные новой строкой, в целом в кодировке Base64 | V2RayN, Shadowrocket, Surge и другие универсальные клиенты. |
| `raw_uri` | Ссылки общего доступа, объединенные новой строкой, простой текст | Клиенты, которым нужны простые ссылки/отладка |
| `mihomo_yaml` | Полная конфигурация Clash/Mihomo YAML (с прокси-группами, правилами) | Clash.Meta / mihomo / Clash Verge Rev |
| `mihomo_proxies_yaml` | YAML только со списком узлов `proxies:` | источник прокси-провайдера mihomo (с запланированным обновлением) |
| `singbox_json` | Полная конфигурация sing-box JSON | sing-box/SFI/FLClash (ядро sing-box) |
| `xray_json` | Полная конфигурация Xray JSON | Xray-ядро |

> Применяется совместимость протокола/формата: например, `anytls` появляется только в форматах sing-box/Mihomo; Узлы REALITY на сервере sing-box не отображаются в подписках Xray/sing-box JSON. Узлы, которые клиент не может распознать, автоматически удаляются, а не прерывают всю подписку.

## 3. Заголовки ответов

| Заголовок | Пример | Описание |
|---|---|---|
| `Subscription-Userinfo` | `upload=1024; download=2048; total=1073741824; expire=1760000000` | Использование и срок действия (байты/Unix-секунды). **Также отправляется по номеру 403**, чтобы клиенты могли показать остаток суммы. |
| `Profile-Update-Interval` | `6` | Рекомендуемый интервал обновления (часы) |
| `Profile-Title` | `CoreFusion` | Имя профиля (показано Clash Verge Rev / V2RayN и т. д.) |
| `Profile-Web-Page-Url` | `https://…/portal` | Ссылка на страницу портала («открыть веб-страницу» в клиентах) |
| `Content-Type` | По формату: `text/plain` / `text/yaml` / `application/json`. |  |
| `Content-Disposition` | `attachment; filename="subscription.txt"` |  |

## 4. Коды состояния

| HTTP | код | Значение |
|---|---|---|
| 200 | — | Обычный (тело — это подписка, отображаемая в `format`) |
| 400 | `MISSING_TOKEN` | Нет токена в URL |
| 403 | (обычный текст) | Подписка приостановлена/истекла/неактивна. Сообщение локализовано (zh/en/ja/ko) через `Accept-Language` и представляет собой однострочную причину. |
| 404 | `SUBSCRIPTION_NOT_FOUND` | Токен не существует |

> Отозванные подписки считаются несуществующими (404).

## 5. Примеры

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

Динамическая подписка Clash/Mihomo (периодическое обновление):

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

## 6. Примечания

- **Формат и конфигурация тесно связаны**: шаблон, выбранный `config`/`platform`, должен соответствовать `format` этого запроса; несовпадающие шаблоны игнорируются — это предотвращает внедрение правил Mihomo в sing-box/Xray JSON.
- **Ограничение скорости**: конечная точка подписки имеет собственный лимит (`SubscribeRateLimit`); злоупотребляющие клиенты отклоняются.
- **Журнал доступа**: при каждой выборке записывается информация об IP/UA/CDN, которая отображается в разделе «Журнал доступа к подписке» в пользовательском интерфейсе администратора.
- Без домена, настроенного на выделенном порту подписки, используются запросы `X-Forwarded-Proto`/`X-Forwarded-Host`; убедитесь, что ваш обратный прокси пересылает оба заголовка.
