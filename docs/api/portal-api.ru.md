<!-- source: portal-api.md | locale: ru | updated: 2026-10-23 -->
# Справочник по порталу API (Клиент/портал API)

> Языки: [简体中文](portal-api.md) · [English](portal-api.en.md) · [繁體中文](portal-api.zh-TW.md) · Русский · [日本語](portal-api.ja.md) · [한국어](portal-api.ko.md) · [Deutsch](portal-api.de.md) · [Français](portal-api.fr.md) · [Español](portal-api.es.md) · [Português](portal-api.pt.md) · [Türkçe](portal-api.tr.md) · [Bahasa Indonesia](portal-api.id.md) · [ไทย](portal-api.th.md) · [Tiếng Việt](portal-api.vi.md) · [العربية](portal-api.ar.md) · [فارسی](portal-api.fa.md)

> **Полная** ссылка на интерфейс внешнего интерфейса портала (`/p` или выделенный порт портала) и сторонних клиентов.
> Определения маршрутов: `internal/api/router/router.go` (`clientAuthGroup` / `clientAuthProtected` / `/api/v1/public/*`).

## 1. Общие соглашения

### 1.1 База URL

| Развертывание | База URL |
|---|---|
| Основная панель двигателя | `https://<panel-domain>/api/v1` (страница портала `/p`) |
| Выделенный портальный порт | `https://<portal-domain>[:port]/api/v1` (страница портала `/`) |
| Выделенный домен портала + обратный прокси | То же, что и выше; прокси должен пересылать оба пути к страницам и `/api/v1` |

### 1.2 Аутентификация

- После входа в систему отправьте `Authorization: Bearer <access_token>`;
- Токены доступа действуют 24 часа, токены обновляются 7 дней;
- JWT содержит `tv` (token_version): выход из системы/принудительный выход увеличивает его и делает недействительными все старые токены.
 (`clients.token_version`, см. `internal/service/clientauth/service.go`);
- Каждая защищенная конечная точка проходит через `ClientAuthMiddleware` (проверка версии + защита режима обслуживания).

### 1.3 Конверт ответа

Большинство конечных точек возвращают `{ "code": 0, "data": ..., "message": "..." }`;
некоторые конечные точки списка возвращают `{ "items": [...], "total": N }` или пустой массив (отмечено для каждой конечной точки ниже).
Ошибки возвращаются `{ "code": "ERROR_CODE", "message": "...", "request_id": "...", "can_retry": ... }`
(HTTP 4xx/5xx).

### 1.4 Единицы измерения

Поля заказа/баланса представляют собой **целые числа в центах**: `990` = 9,90 иен (`currency: "CNY"`).

### 1.5 Реальный пример (войти в систему → загрузить профиль)

```bash
# Register (when registration_enabled=true)
curl -X POST https://<portal-domain>/api/v1/client-auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"plaintext-password","referral_code":""}'
# → 201 {"data":{"access_token":"<JWT>","refresh_token":"<JWT>","client":{...}}}

# Sign in
curl -X POST https://<portal-domain>/api/v1/client-auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"plaintext-password"}'
# → 200 same as above

# Fetch the current client profile (with token)
curl https://<portal-domain>/api/v1/client-auth/me \
  -H 'Authorization: Bearer <access_token>'
# → 200 {"data":{"id":27,"email":"user@example.com","nickname":"","status":"active","quota_bytes":10737418240,"used_bytes":0,...}}

# Fetch a subscription (no header, token in the path)
curl 'https://<portal-domain>/sub/<subscription_token>?format=mihomo_yaml'
# → 200 Mihomo YAML (or base64_uri / singbox_json / xray_json)

# Error example: expired token
# → 401 {"code":"errClientTokenRevoked","message":"...","request_id":"...","can_retry":false}
```

По номеру 401 позвоните по `POST /api/v1/client-auth/refresh` один раз и повторите попытку; если это по-прежнему не удается, очистите локальный сеанс (шаблон портала уже делает это).

---

## 2. Публичные конечные точки (без входа)

| Метод | Путь | Описание |
|---|---|---|
| GET | `/p` (`/` на выделенном движке портала) | HTML портала (отображается на сервере + среда выполнения Vue) |
| GET | `/api/v1/public/portal-config` | Конфигурация среды выполнения портала: имя сайта, цвета темы, навигация, модули, языковые пакеты, `registration_enabled`, `regions` (источник региона для входа), `privacy_policy_url` / `terms_of_service_url` / `privacy_policy_content` / `terms_of_service_content` (документы соответствия; содержимое сопоставлено для каждого языка; отдельные страницы `/p/legal/*` ✓) |
| GET | `/api/v1/public/settings` | Публичные настройки (на данный момент `registration_enabled`) |
| GET | `/api/v1/public/subscription-endpoints` | Список конечных точек подписки (пустой массив) |
| GET | `/api/v1/public/articles` | Публичные статьи (только опубликованные) |
| GET | `/api/v1/public/articles/:slug` | Подробности статьи |
| GET | `/api/v1/public/client-configs` | Конфигурация загрузки клиента |
| GET | `/sub/:token` | Доставка подписки (`format` возвращает base64_uri/raw_uri/mihomo_yaml/singbox_json/xray_json и т. д.; отозвана/отсутствует → 404, приостановлена/истекла/неактивна → 403. См. [subscription.md](portal-api.ru.md)) |
| GET | `/api/v1/client-auth/oauth/providers` | Видимость поставщика OAuth: `{"data":{"providers":[{"provider":"github","enabled":true},...]}}`. |
| GET | `/api/v1/system/maintenance` | Статус обслуживания (общедоступный). Портал показывает страницу обслуживания, когда он включен. |

> Все это включает в себя охрану обслуживания и общедоступное ограничение скорости API.

## 3. Аутентификация/ОАутентификация

| Метод | Путь | Авторизация | Описание |
|---|---|---|---|
| POST | `/api/v1/client-auth/register` | — | Зарегистрируйтесь. Тело: `{email, password, referral_code?}`. Возвращает `{access_token, refresh_token, client}`; скорость регистрации ограничена |
| POST | `/api/v1/client-auth/login` | — | Войдите в систему. Тело: `{email, password}`. Тот же ответ; выдает JWT с `token_version` |
| POST | `/api/v1/client-auth/refresh` | — | Обновить. Тело: `{refresh_token}`. Возвращает новые токены доступа/обновления (версия проверена; старые токены обновления исчезают сразу после выхода из системы) |
| POST | `/api/v1/client-auth/logout` | ✓ | Выход из системы: отзыв на стороне сервера (`token_version +1`) → все старые токены клиента недействительны. |
| POST | `/api/v1/client-auth/rotate-token` | ✓ | Поворот токена подписки |
| POST | `/api/v1/client-auth/ticket` | ✓ | Выдать **одноразовый билет с коротким сроком действия** (для потоков `window.open`, таких как «загрузка пакета журналов», где заголовки не могут быть установлены). |
| GET | `/api/v1/client-auth/oauth/:provider` | — | Запустить OAuth: 307 → согласие GitHub/Google (`state` anti-CSRF, 10 минут); поставщики-инвалиды возвращают 400 |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | — | Обратный вызов OAuth: код обмена → создать/связать клиент → выдать версию JWT → 302 вернуться на портал (`/<portalPath>?access_token=...&refresh_token=...`) |

> Обратный вызов OAuth — это **перенаправление браузера** (а не вызов API): после успешного входа в систему портал считывает токен.
> из URL в `onMounted` (или через `postMessage` того же происхождения, см. `handleOAuthCallback` в шаблоне портала `web`).

## 4. Аккаунт

| Метод | Путь | Описание |
|---|---|---|
| GET | `/api/v1/client-auth/me` | Текущий клиент: `{id, email, plan_id, plan_name, status, quota_bytes, used_bytes, upload_bytes, download_bytes, expires_at, ip_whitelist, device_limit, sub_token, sub_status, created_at, ...}` (конверт `data`) |
| POST | `/api/v1/client-auth/change-password` | Корпус: `{old_password, new_password}` |
| GET | `/api/v1/client-auth/checktime` | Серверное время (синхронизация часов клиента) |
| GET | `/api/v1/client-auth/connection-info` | Параметры подключения текущего плана: `{concurrent, device_limit, protocol, speed_limit, tls, transport}` |

## 5. Подписки

| Метод | Путь | Описание |
|---|---|---|
| POST | `/api/v1/client-auth/reset-subscription` | Сбросить подписку; это вращается `sub_token` |

> Ссылка на подписку создается на основе интерфейса `sub_token`, возвращаемого `/me` (`/sub/:token`); нет
> отдельные конечные точки `subscription-link`/`subscription-formats` больше.

## 6. Узлы

| Метод | Путь | Описание |
|---|---|---|
| GET | `/api/v1/client-auth/nodes` | Узлы, видимые для текущего плана: `{items:[...]}` (поля отображения, такие как имя/протокол/транспорт/регион) |
| GET | `/api/v1/client-auth/nodes/:id/config` | Полная конфигурация узла |
| GET | `/api/v1/client-auth/nodes/:id/link` | Поделиться ссылкой (vless:// и т. д.) |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | Изображение QR-кода |

## 7. Магазин/планы/заказы

| Метод | Путь | Описание |
|---|---|---|
| GET | `/api/v1/client-auth/plans` | Список планов (пустой массив): `{id, name, description, category, sort_order, variants:[{id, name, billing_cycle, duration_days, price(cents), original_price, currency, quota_bytes, ...}]}` |
| GET | `/api/v1/client-auth/plans/:id` | Детали плана |
| GET | `/api/v1/client-auth/traffic-packs` | Список пакетов трафика |
| POST | `/api/v1/client-auth/traffic-packs/buy` | Купить пакет трафика |
| GET | `/api/v1/client-auth/orders` | Мои плановые заказы (с разбивкой на страницы) |
| POST | `/api/v1/client-auth/orders` | Создать заказ (купить/продлить вариант плана) |
| GET | `/api/v1/client-auth/orders/:id` | Детали заказа |
| GET | `/api/v1/client-auth/invoices` | Счета-фактуры |
| GET | `/api/v1/client-auth/invoices/:id` | Детали счета |

## 8. Баланс/пополнение

| Метод | Путь | Описание |
|---|---|---|
| GET | `/api/v1/client-auth/balance` | `{data:{client_id, balance(cents), frozen}}` |
| GET | `/api/v1/client-auth/balance/transactions` | Балансовые транзакции (`type`: начисление/потребление/возврат; с разбивкой на страницы) |
| GET | `/api/v1/client-auth/charge/sites` | Доступные сайты пополнения счета (`ListChargeSites`: только включено) |
| POST | `/api/v1/client-auth/charge/create` | Создать заказ на пополнение счета (перенаправляет на сторонний платеж) |
| GET | `/api/v1/client-auth/charge/orders` | Список заказов на пополнение счета |
| GET | `/api/v1/client-auth/charge/orders/:id` | Детали заказа пополнения счета |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | Опросить статус оплаты по trade_no |
| POST | `/api/v1/client-auth/charge/redeem-card` | Погасить карту пополнения |

> Удаление клиента **каскадом** к его балансу и транзакциям (см. `DeleteClient` в `internal/service/client/service.go`).

## 9. Подарки/история трафика

| Метод | Путь | Описание |
|---|---|---|
| GET | `/api/v1/client-auth/gifts` | Заявленные подарки (пустой массив; `type`: дни/байты) |
| POST | `/api/v1/client-auth/gifts/:id/claim` | Претензия |
| GET | `/api/v1/client-auth/traffic-history` | Ежемесячная история трафика: `{data:{history:[{period_start,total_bytes,...}], months}}` |

## 10. Билеты

| Метод | Путь | Описание |
|---|---|---|
| GET | `/api/v1/client-auth/tickets` | Список билетов: `{items,total}` |
| POST | `/api/v1/client-auth/tickets` | Создать билет |
| GET | `/api/v1/client-auth/tickets/:id` | Детали билета (с ответами) |
| POST | `/api/v1/client-auth/tickets/:id/reply` | Ответить |
| POST | `/api/v1/client-auth/tickets/:id/close` | Закрыть |

## 11. Статьи/руководства

| Метод | Путь | Описание |
|---|---|---|
| GET | `/api/v1/client-auth/articles` | Список статей (с фильтром по категориям) |
| GET | `/api/v1/client-auth/articles/:slug` | Подробности статьи |

## 12. Рефералы (по умолчанию отключены; можно включить в редакторе портала)

| Метод | Путь | Описание |
|---|---|---|
| GET | `/api/v1/client-auth/referral/code` | Мой реферальный код и ссылка |
| GET | `/api/v1/client-auth/referral/stats` | Статистика комиссий (`commission_rate`, `available_commission`, `min_withdraw_amount`) |
| GET | `/api/v1/client-auth/referral/records` | Реферальные записи |
| GET | `/api/v1/client-auth/referral/withdrawals` | Записи о снятии средств |
| POST | `/api/v1/client-auth/referral/withdraw` | Запросить вывод средств |

## 13. Общие аккаунты

| Метод | Путь | Описание |
|---|---|---|
| GET | `/api/v1/client-auth/shared-accounts` | Доступные общие учетные записи (потоковое вещание и т. д.) |
| GET | `/api/v1/client-auth/shared-accounts/:id` | Деталь |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | Запросить доступ |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | Ремонт в один клик |

## 14. Пакеты журналов (диагностика подписчиков)

| Метод | Путь | Описание |
|---|---|---|
| GET | `/api/v1/client-auth/logs/types` | Типы журналов, которые можно объединить |
| GET | `/api/v1/client-auth/logs/packages` | Мои пакеты журналов |
| POST | `/api/v1/client-auth/logs/package` | Создайте задание по объединению журналов (только входные узлы) |
| GET | `/api/v1/client-auth/logs/packages/:id` | Статус задания (готово/обработка/не выполнено) |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | Скачать (предпочитайте `POST /ticket` и добавьте `?ticket=`) |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | Удаление пакета журналов |

## 15. Настройки безопасности / список разрешенных IP-адресов / самостоятельная разблокировка

| Метод | Путь | Описание |
|---|---|---|
| GET/PUT | `/api/v1/client-auth/settings` | Настройки клиента: `{ip_whitelist, device_limit}` (PUT) |
| GET | `/api/v1/client-auth/settings/whitelist` | Список разрешений (голый массив) |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | Удаление записи из списка разрешенных |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | Добавить текущий IP в список разрешенных |
| POST | `/api/v1/client-auth/self-unban` | Самостоятельная разблокировка (попытки ограничены политикой) |

> Список разрешенных IP-адресов **применяется только к URL-адресам подписки** (`/sub/:token`); он не блокирует вход на портал/OAuth или
> другие конечные точки аутентификации клиента. Если у пользователя нет списка разрешенных или подписки URL, доступ всегда разрешен.
> список разрешений — это настройка для каждого пользователя; глобального переключателя нет.

**Поток саморазблокировки**: доступен только для `auto_quota` (автоматический запрет при превышении трафика), если на стороне администратора
`self_unban.enabled` включен. Вход (пароль или обратный вызов OAuth) возвращает `require_self_unban=true` + `self_unban_token`.
(тип=self_unban, действителен 15 минут, **можно использовать только на `POST /self-unban`; другие конечные точки возвращают 403/`SELF_UNBAN_TOKEN_SCOPE`).
Затем портал показывает карту разблокировки; после успешной разбанки пользователь снова авторизуется. Обратные вызовы OAuth доставляют это через
фрагмент URL `#require_self_unban=1&self_unban_token=..&self_unban_hint=..`.

## 16. Коды ошибок (выдержка)

| код | Сценарий |
|---|---|
| `INVALID_TOKEN` | Токен обновления недействителен/истёк срок действия/несоответствие версии |
| `INVALID_REQUEST` | Проверка параметра не удалась |
| `NOT_FOUND` | Отсутствует ресурс (билет/заказ/запись в белом списке и т. д.) |
| `RELOAD_FAILED` / `SYNC_FAILED` | (Со стороны администратора) не удалось синхронизировать конфигурацию прокси-сервера. |
| 401 + `errClientTokenRevoked` | несоответствие token_version (выход из системы/выкидывание) → клиенты должны обновиться; если это не помогло, очистите локальные токены |

## 17. Соответствующие документы (политика конфиденциальности/условия обслуживания)

Ссылки «Политика конфиденциальности · Условия обслуживания» находятся в нижней части карточки входа (так же, как и на обычных сайтах; нажатие открывает содержимое):

- **Конфигурация**: редактор портала → Настройки → Общие → Документы о соответствии (**редактирование контента для каждого языка**);
 вы также можете использовать `PUT /api/v1/portal/active` со структурированными полями `privacy_policy_url`/`privacy_policy_content`/
 `terms_of_service_url` / `terms_of_service_content` (ноль = без изменений; содержимое представляет собой карту `{"zh-CN":"...","en":"..."}`).
- **Отдельные доступные страницы** (отрисовываются на сервере, легко подходят для сканеров проверки Google; поддерживает `?lang=xx`):
  - Домен главной панели: `GET /p/legal/privacy`, `GET /p/legal/terms`.
  - Выделенный домен портала (корневое развертывание): `GET /legal/privacy`, `GET /legal/terms`.
- **Логика отображения**: URL или контент, **URL побеждает** (открывается на новой вкладке); если контент предоставлен, нажмите кнопку
 Карта входа отображает Markdown в диалоговом окне (`renderMd`) с использованием текущего языка портала.
- Доставляется во время выполнения через `/api/v1/public/portal-config`; компонент входа потребляет
 `ctx.privacyUrl / termsUrl / privacyContent / termsContent / openLegal / hasLegalLinks`.
- Google Проверка OAuth требует, чтобы эти ссылки были общедоступны → используйте отдельные ссылки выше в
 экран согласия; см. `docs/oauth-google-verification.md` (содержит шаблоны политики конфиденциальности/условий).

## 18. Пример интеграции

```js
// Sign in
const { data } = await fetch('/api/v1/client-auth/login', {
  method: 'POST', headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ email, password })
}).then(r => r.json());
localStorage.setItem('client_token', data.access_token);
localStorage.setItem('client_refresh_token', data.refresh_token);

// Authenticated request
const me = await fetch('/api/v1/client-auth/me', {
  headers: { Authorization: 'Bearer ' + data.access_token }
}).then(r => r.json());
```

По номеру 401 позвоните по `POST /refresh` один раз и повторите попытку; если это по-прежнему не удается, очистите локальный сеанс
(это уже реализовано в шаблоне портала `api()`).
