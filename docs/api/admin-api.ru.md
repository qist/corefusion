<!-- source: admin-api.md | locale: ru | updated: 2026-10-23 -->
# Администратор API

> Языки: [简体中文](admin-api.md) · [English](admin-api.en.md) · [繁體中文](admin-api.zh-TW.md) · Русский · [日本語](admin-api.ja.md) · [한국어](admin-api.ko.md) · [Deutsch](admin-api.de.md) · [Français](admin-api.fr.md) · [Español](admin-api.es.md) · [Português](admin-api.pt.md) · [Türkçe](admin-api.tr.md) · [Bahasa Indonesia](admin-api.id.md) · [ไทย](admin-api.th.md) · [Tiếng Việt](admin-api.vi.md) · [العربية](admin-api.ar.md) · [فارسی](admin-api.fa.md)

Полная таблица маршрутов администратора API для **разработчиков панелей/автоматизации/интеграторов**.

- База URL: `https://<panel-domain>/api/v1`
- Аутентификация: `Authorization: Bearer <admin_token>` (из `POST /api/v1/auth/login`)
- Общие форматы ответов/ошибок и соглашения об ограничении скорости: см. [README.md](./README.ru.md).
- Источник истины: `internal/api/router/router.go` (739 административных маршрутов); если эта таблица не согласна, **код выигрывает**

## Маркеры разрешений

| Маркер | Значение |
|---|---|
| Публичный | Вход в систему не требуется (защита обслуживания/ограничение скорости все еще могут применяться) |
| Публичный · с ограниченной ставкой | Без входа в систему, с выделенным ограничением скорости |
| Требуется вход | Требуется токен администратора для входа в систему (`AuthRequired`). |
| Администратор | Требуется роль ≥ `admin` (`RequireRole(RoleAdmin)`) |
| Суперадминистратор | Требуется роль = `super_admin`. |

Уровни ролей: `viewer(10) < operator(20) < admin(30) < super_admin(40)`; недостаточно ролей возвращает 403 `INSUFFICIENT_ROLE`.

---

## Быстрый старт (реальные примеры)

Эти примеры основаны на реальных местных реакциях; токены и данные редактируются.

### 1. Войдите и получите токен

```bash
curl -X POST https://<panel-domain>/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"your-password"}'
```

Ответ:

```json
{
  "data": {
    "access_token": "<JWT, valid 24h>",
    "refresh_token": "<JWT, valid 7 days>",
    "user": { "id": 1, "role": "super_admin", "username": "admin" }
  }
}
```

### 2. Вызов конечной точки бизнеса с помощью токена

```bash
curl https://<panel-domain>/api/v1/clients?limit=1 \
  -H 'Authorization: Bearer <access_token>'
```

Ответ (списки используют структуру `items` + `total`):

```json
{
  "data": {
    "items": [
      {
        "id": 27,
        "email": "user@example.com",
        "status": "active",
        "quota_bytes": 10737418240,
        "used_bytes": 0,
        "plan_id": 1,
        "expires_at": null,
        "created_at": "2026-09-30T15:54:32+08:00"
      }
    ],
    "total": 22,
    "limit": 1,
    "offset": 0
  }
}
```

### 3. Примеры ответов об ошибках

**401 не авторизован:**

```json
{
  "code": "MISSING_TOKEN",
  "message": "missing authorization header",
  "request_id": "a02cfdf9-6be0-46be-8584-824caab34dc7",
  "can_retry": false,
  "requires_action": false
}
```

**Ресурс 404 не найден:**

```json
{
  "code": "CLIENT_NOT_FOUND",
  "message": "client not found",
  "request_id": "099949bd-4ded-468d-a6f8-f8e798af2273",
  "can_retry": false,
  "requires_action": false
}
```

### 4. Пагинация

Все конечные точки списка используют `limit` (размер страницы, по умолчанию 20) и `offset` (отсчет от 0) и возвращают `{items, total, limit, offset}`. Некоторые устаревшие конечные точки возвращают пустой массив (отмечено для каждой таблицы).

---

## Содержание

- [Аутентификация и безопасность администратора](#аутентификация-и-безопасность-администратора)
- [Учетные записи администратора](#учетные-записи-администратора)
- [Управление клиентами](#управление-клиентами)
- [Шаблоны планов](#шаблоны-планов)
- [Варианты плана](#варианты-плана)
- [Пакеты трафика](#пакеты-трафика)
- [Купоны](#купоны)
- [Группы узлов](#группы-узлов)
- [Узловые серверы](#узловые-серверы)
- [Журнал аудита узла](#журнал-аудита-узла)
- [Узлы (устаревшая модель, снята с производства)](#узлы-устаревшая-модель-снята-с-производства)
- [Обзор региона](#обзор-региона)
- [Подписки](#подписки)
- [Форматы подписки](#форматы-подписки)
- [Конечные точки подписки](#конечные-точки-подписки)
- [Делитесь доменами](#делитесь-доменами)
- [Cloudflare ИП](#cloudflare-ип)
- [Конфигурация подписки (Mihomo/Clash)](#конфигурация-подписки-mihomoclash)
- [Сайты пополнения счета](#сайты-пополнения-счета)
- [Заказы на пополнение счета](#заказы-на-пополнение-счета)
- [Карты пополнения](#карты-пополнения)
- [События обратного вызова для оплаты](#события-обратного-вызова-для-оплаты)
- [Повторные попытки обратного вызова](#повторные-попытки-обратного-вызова)
- [Баланс клиента](#баланс-клиента)
- [Подарки](#подарки)
- [Реферальная конфигурация](#реферальная-конфигурация)
- [Вывод средств по рефералам](#вывод-средств-по-рефералам)
- [Списки разрешенных IP-адресов (сводка)](#списки-разрешенных-ip-адресов-сводка)
- [Привязки OAuth клиента](#привязки-oauth-клиента)
- [Билеты (админ)](#билеты-админ)
- [Статьи](#статьи)
- [Общие аккаунты](#общие-аккаунты)
- [Сертификаты](#сертификаты)
- [Каналы сертификатов](#каналы-сертификатов)
- [Конфигурация EAB](#конфигурация-eab)
- [Счета ACME](#счета-acme)
- [Встроенный центр сертификации](#встроенный-центр-сертификации)
- [Гео правила](#гео-правила)
- [Библиотека правил WAF](#библиотека-правил-waf)
- [Управление журналами](#управление-журналами)
- [Резервные копии](#резервные-копии)
- [Системные настройки](#системные-настройки)
- [Аудит безопасности](#аудит-безопасности)
- [Метрики](#метрики)
- [Шаблоны порталов](#шаблоны-порталов)
- [Установка агента](#установка-агента)
- [Запись об обратном вызове](#запись-об-обратном-вызове)
- [Входящие (с узлами)](#входящие-с-узлами)
- [Публичная конфигурация](#публичная-конфигурация)
- [Аутентификация и бизнес клиента (то же, что и в системе администратора)](#аутентификация-и-бизнес-клиента-то-же-что-и-в-системе-администратора)
- [Доставка подписки](#доставка-подписки)
- [Рабочий процесс настройки среды выполнения](#рабочий-процесс-настройки-среды-выполнения)
- [Политики доступа к услугам (устарело)](#политики-доступа-к-услугам-устарело)
- [Мониторинг квот трафика](#мониторинг-квот-трафика)
- [Проверка здоровья](#проверка-здоровья)
- [Страница портала](#страница-портала)
- [ACME задачи](#acme-задачи)

---

### Аутентификация и безопасность администратора

Вход, выход, смена пароля, 2FA (TOTP), Passkey (WebAuthn) и разовые билеты для админ-панели.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| POST | `/api/v1/auth/2fa/disable` | Требуется вход | Отключить |
| POST | `/api/v1/auth/2fa/enable` | Требуется вход | Включить |
| POST | `/api/v1/auth/2fa/recovery-codes` | Требуется вход | Восстановить коды восстановления |
| POST | `/api/v1/auth/2fa/setup` | Требуется вход | Настройки |
| GET | `/api/v1/auth/2fa/status` | Требуется вход | Статус |
| POST | `/api/v1/auth/change-password` | Требуется вход | Изменить пароль |
| POST | `/api/v1/auth/login` | Публичный | Войти |
| POST | `/api/v1/auth/login/2fa` | Публичный | Вход с динамическим кодом (2FA) |
| GET | `/api/v1/auth/login/2fa-status` | Публичный | Проверьте, включена ли в учетной записи 2FA |
| POST | `/api/v1/auth/logout` | Требуется вход | Выйти |
| GET | `/api/v1/auth/me` | Требуется вход | Текущий вошедший пользователь |
| GET | `/api/v1/auth/passkey/credentials` | Требуется вход | Список учетных данных Passkey |
| DELETE | `/api/v1/auth/passkey/credentials/:id` | Требуется вход | Удалить учетные данные Passkey |
| POST | `/api/v1/auth/passkey/login/begin` | Публичный | Начать вход |
| POST | `/api/v1/auth/passkey/login/finish` | Публичный | Завершить вход |
| POST | `/api/v1/auth/passkey/register/begin` | Требуется вход | Начать регистрацию |
| POST | `/api/v1/auth/passkey/register/finish` | Требуется вход | Завершить регистрацию |
| GET | `/api/v1/auth/passkey/rpids` | Требуется вход | Список разрешенных идентификаторов RP |
| POST | `/api/v1/auth/passkey/rpids` | Требуется вход | Обновить список разрешенных идентификаторов RP |
| GET | `/api/v1/auth/passkey/status` | Публичный | Статус |
| POST | `/api/v1/auth/refresh` | Публичный | Обновить |
| POST | `/api/v1/auth/ticket` | Требуется вход | Выдать одноразовый билет с коротким сроком действия |

### Учетные записи администратора

Учетные записи пользователей панели администратора. Для листинга требуется администратор; для создания/обновления/удаления требуется суперадминистратор (предотвращает повышение привилегий).

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/users` | Администратор | Список |
| POST | `/api/v1/users` | Суперадминистратор | Создать |
| DELETE | `/api/v1/users/:id` | Суперадминистратор | Удалить |
| PUT | `/api/v1/users/:id` | Суперадминистратор | Обновить |

### Управление клиентами

CRUD, приостановка/возобновление, обновление/понижение версии, продление и сброс использования для подписчиков (учетных записей портала).

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/clients` | Требуется вход | Список клиентов |
| POST | `/api/v1/clients` | Требуется вход | Создать клиента. корпус: `{"email":"user@example.com","password":"plaintext-password","plan_id":1,"quota_bytes":10737418240,"expires_at":"2026-12-31T00:00:00Z","remark":""}`. Требуется `plan_id`; `password` может быть пустым (клиенты могут установить его позже на портале) |
| DELETE | `/api/v1/clients/:id` | Требуется вход | Удалить клиента |
| GET | `/api/v1/clients/:id` | Требуется вход | Получить клиента |
| PUT | `/api/v1/clients/:id` | Требуется вход | Обновить клиента |
| POST | `/api/v1/clients/:id/downgrade` | Требуется вход | Понизить версию клиента |
| GET | `/api/v1/clients/:id/orders` | Требуется вход | Список заказов клиентов |
| POST | `/api/v1/clients/:id/renew` | Требуется вход | Продлить клиента |
| POST | `/api/v1/clients/:id/reset-usage` | Требуется вход | Сбросить использование |
| POST | `/api/v1/clients/:id/restore` | Требуется вход | Возобновить клиента |
| POST | `/api/v1/clients/:id/suspend` | Требуется вход | Заблокировать клиента |
| POST | `/api/v1/clients/:id/upgrade` | Требуется вход | Обновить клиента |

### Шаблоны планов

Планы (варианты, циклы оплаты, цены, квоты трафика) и авторизация узла на уровне шаблона.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/plan-templates` | Требуется вход | Список шаблонов |
| POST | `/api/v1/plan-templates` | Требуется вход | Создать шаблон |
| DELETE | `/api/v1/plan-templates/:id` | Требуется вход | Удалить шаблон |
| GET | `/api/v1/plan-templates/:id` | Требуется вход | Детали шаблона |
| PUT | `/api/v1/plan-templates/:id` | Требуется вход | Обновить шаблон |
| GET | `/api/v1/plan-templates/:id/accessible-nodes` | Требуется вход | Получить доступные узлы |
| GET | `/api/v1/plan-templates/:id/billing-node-options` | Требуется вход | Получите оплачиваемые варианты узлов |
| PUT | `/api/v1/plan-templates/:id/node-groups` | Требуется вход | Установить группы узлов шаблона |
| POST | `/api/v1/plan-templates/:id/sync-accounts` | Требуется вход | Синхронизировать аккаунты с шаблоном |
| GET | `/api/v1/plan-templates/:id/variants` | Требуется вход | Список вариантов |
| POST | `/api/v1/plan-templates/:id/variants` | Требуется вход | Создать вариант |
| POST | `/api/v1/plan-templates/sync-all-accounts` | Требуется вход | Синхронизировать все учетные записи |

### Варианты плана

Обновление и удаление вариантов плана (создание вложено в шаблоны планов).

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| DELETE | `/api/v1/plan-variants/:id` | Требуется вход | Удалить вариант |
| PUT | `/api/v1/plan-variants/:id` | Требуется вход | Обновить вариант |

### Пакеты трафика

Пакеты трафика, которые можно приобрести отдельно.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/traffic-packs` | Требуется вход | Список пакетов трафика |
| POST | `/api/v1/traffic-packs` | Требуется вход | Создать пакет трафика |
| DELETE | `/api/v1/traffic-packs/:id` | Требуется вход | Удалить пакет трафика |
| PUT | `/api/v1/traffic-packs/:id` | Требуется вход | Обновить пакет трафика |

### Купоны

Создавайте, подтверждайте и активируйте купоны.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/promotions` | Требуется вход | Список купонов |
| POST | `/api/v1/promotions` | Требуется вход | Создать купон |
| DELETE | `/api/v1/promotions/:id` | Требуется вход | Удалить купон |
| PUT | `/api/v1/promotions/:id` | Требуется вход | Обновить купон |
| POST | `/api/v1/promotions/validate` | Требуется вход | Подтвердить купон |

### Группы узлов

Организуйте узлы в группы, на которые ссылаются планы.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/node-groups` | Требуется вход | Список групп узлов |
| POST | `/api/v1/node-groups` | Требуется вход | Создать группу узлов |
| DELETE | `/api/v1/node-groups/:id` | Требуется вход | Удалить группу узлов |
| PUT | `/api/v1/node-groups/:id` | Требуется вход | Обновить группу узлов |
| GET | `/api/v1/node-groups/:id/members` | Требуется вход | Список участников группы |
| POST | `/api/v1/node-groups/:id/members` | Требуется вход | Добавить участника группы |
| DELETE | `/api/v1/node-groups/:id/members/:mid` | Требуется вход | Удалить участника группы |

### Узловые серверы

Основные ресурсы: узел CRUD, доставка конфигурации ядра (xray/sing-box), конфигурация входящей/исходящей/конечной точки, управление процессами, сертификаты, WAF, ограничения скорости, переключение портов, цепочки прокси, зеркало/основное резервное копирование, установка/обновление агента, управление временем выполнения xray. Множество подпутей, сгруппированных по семействам объектов.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/node-servers` | Администратор | Список |
| POST | `/api/v1/node-servers` | Администратор | Создать |
| DELETE | `/api/v1/node-servers/:id` | Администратор | Удалить |
| GET | `/api/v1/node-servers/:id` | Администратор | Подробности |
| PUT | `/api/v1/node-servers/:id` | Администратор | Обновить |
| GET | `/api/v1/node-servers/:id/binaries` | Администратор | Установленный основной список |
| POST | `/api/v1/node-servers/:id/binaries/ech-generate` | Администратор | Создать пару ключей ECH. |
| POST | `/api/v1/node-servers/:id/binaries/install` | Администратор | Установить/обновить ядро |
| GET | `/api/v1/node-servers/:id/binaries/interfaces` | Администратор | Список сетевых интерфейсов |
| POST | `/api/v1/node-servers/:id/binaries/mldsa65-key` | Администратор | Сгенерировать ключ MLDSA65 |
| POST | `/api/v1/node-servers/:id/binaries/reality-key` | Администратор | Создать пару ключей REALITY. |
| POST | `/api/v1/node-servers/:id/binaries/reality-public-key` | Администратор | Получить открытый ключ из закрытого ключа REALITY. |
| GET | `/api/v1/node-servers/:id/binaries/releases` | Администратор | Доступные версии |
| POST | `/api/v1/node-servers/:id/binaries/vlessenc` | Администратор | Сгенерируйте параметры шифрования VLESS |
| POST | `/api/v1/node-servers/:id/binaries/wg-key` | Администратор | Сгенерировать ключ WireGuard |
| POST | `/api/v1/node-servers/:id/binaries/x25519-key` | Администратор | Создайте пару ключей X25519. |
| GET | `/api/v1/node-servers/:id/bootstrap-cmd` | Администратор | Получить команду начальной загрузки агента |
| POST | `/api/v1/node-servers/:id/bootstrap-token` | Администратор | Создать токен начальной загрузки |
| POST | `/api/v1/node-servers/:id/caddy/batch-update-log-enabled` | Администратор | Переключатели журнала массового обновления |
| POST | `/api/v1/node-servers/:id/caddy/batch-update-ports` | Администратор | Массовое обновление портов |
| POST | `/api/v1/node-servers/:id/caddy/generate-config` | Администратор | Создать конфигурацию |
| DELETE | `/api/v1/node-servers/:id/caddy/l4-rules` | Администратор | Удалить все правила L4 |
| POST | `/api/v1/node-servers/:id/caddy/save-and-apply` | Администратор | Сохраните и примените |
| GET | `/api/v1/node-servers/:id/caddy/sites` | Администратор | Список сайтов |
| POST | `/api/v1/node-servers/:id/caddy/sites` | Администратор | Создать сайт |
| DELETE | `/api/v1/node-servers/:id/caddy/sites/:siteId` | Администратор | Удалить сайт |
| PUT | `/api/v1/node-servers/:id/caddy/sites/:siteId` | Администратор | Обновить сайт |
| DELETE | `/api/v1/node-servers/:id/caddy/sni-rules` | Администратор | Удалить все правила SNI. |
| POST | `/api/v1/node-servers/:id/caddy/sni-rules/toggle` | Администратор | Переключить правила SNI |
| GET | `/api/v1/node-servers/:id/caddy/sni-settings` | Администратор | Настройки SNI |
| POST | `/api/v1/node-servers/:id/caddy/sni-settings` | Администратор | Сохраните настройки SNI. |
| GET | `/api/v1/node-servers/:id/certificates` | Администратор | Список сертификатов узла |
| DELETE | `/api/v1/node-servers/:id/certificates/:cid` | Администратор | Отвязать сертификат |
| POST | `/api/v1/node-servers/:id/certificates/:cid` | Администратор | Привязать сертификат |
| GET | `/api/v1/node-servers/:id/chain-config` | Администратор | Создать связанную конфигурацию |
| GET | `/api/v1/node-servers/:id/chain-health` | Администратор | Статус работоспособности цепи |
| POST | `/api/v1/node-servers/:id/control` | Суперадминистратор | Ядро узла управления. тело: `{"core_type":"xray","action":"start"}` (действие = старт/стоп/перезапуск). `core_type` может быть `xray`/`singbox` |
| GET | `/api/v1/node-servers/:id/degradation-status` | Администратор | Ухудшенный статус |
| POST | `/api/v1/node-servers/:id/distribute-cert` | Администратор | Распространение сертификата |
| GET | `/api/v1/node-servers/:id/endpoint-configs` | Администратор | Список |
| POST | `/api/v1/node-servers/:id/endpoint-configs` | Администратор | Создать |
| DELETE | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | Администратор | Удалить |
| GET | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | Администратор | Подробности |
| PUT | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | Администратор | Обновить |
| POST | `/api/v1/node-servers/:id/failover` | Администратор | Запустить отработку отказа для этого узла |
| GET | `/api/v1/node-servers/:id/failover-status` | Администратор | Статус аварийного переключения |
| POST | `/api/v1/node-servers/:id/force-reconnect` | Администратор | Принудительное переподключение |
| GET | `/api/v1/node-servers/:id/free-port` | Администратор | Запросить бесплатный порт |
| POST | `/api/v1/node-servers/:id/geo-rules/push` | Администратор | Нажмите на этот узел |
| GET | `/api/v1/node-servers/:id/inbound-configs` | Администратор | Список |
| POST | `/api/v1/node-servers/:id/inbound-configs` | Администратор | Создать |
| DELETE | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | Администратор | Удалить |
| GET | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | Администратор | Подробности |
| PUT | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | Администратор | Обновить |
| GET | `/api/v1/node-servers/:id/inbound-configs/:cfgId/share-links` | Администратор | Делитесь ссылками и фрагментами |
| POST | `/api/v1/node-servers/:id/inbound-configs/:cfgId/users/:email/pause` | Администратор | Заблокировать этого пользователя |
| POST | `/api/v1/node-servers/:id/inbound-configs/:cfgId/users/:email/resume` | Администратор | Восстановить этого пользователя |
| POST | `/api/v1/node-servers/:id/inbound-configs/reset-all-traffic` | Администратор | Сбросить весь входящий трафик |
| GET | `/api/v1/node-servers/:id/install-script` | Администратор | Получить скрипт установки агента |
| GET | `/api/v1/node-servers/:id/metrics-history` | Администратор | Получить историю показателей |
| GET | `/api/v1/node-servers/:id/mirror-status` | Администратор | Статус зеркала |
| POST | `/api/v1/node-servers/:id/nginx/batch-update-ports` | Администратор | Массовое обновление портов |
| POST | `/api/v1/node-servers/:id/nginx/generate-config` | Администратор | Создать конфигурацию |
| DELETE | `/api/v1/node-servers/:id/nginx/l4-rules` | Администратор | Удалить все правила L4 |
| POST | `/api/v1/node-servers/:id/nginx/save-and-apply` | Администратор | Сохраните и примените |
| GET | `/api/v1/node-servers/:id/nginx/settings` | Администратор | Чтение настроек |
| POST | `/api/v1/node-servers/:id/nginx/settings` | Администратор | Сохранить настройки |
| GET | `/api/v1/node-servers/:id/nginx/sites` | Администратор | Список сайтов |
| POST | `/api/v1/node-servers/:id/nginx/sites` | Администратор | Создать сайт |
| DELETE | `/api/v1/node-servers/:id/nginx/sites/:siteId` | Администратор | Удалить сайт |
| PUT | `/api/v1/node-servers/:id/nginx/sites/:siteId` | Администратор | Обновить сайт |
| DELETE | `/api/v1/node-servers/:id/nginx/sni-rules` | Администратор | Удалить все правила SNI. |
| POST | `/api/v1/node-servers/:id/nginx/sni-rules/toggle` | Администратор | Переключить правила SNI |
| PUT | `/api/v1/node-servers/:id/node-type` | Администратор | Обновить тип узла |
| GET | `/api/v1/node-servers/:id/online-users` | Администратор | Онлайн-пользователи |
| GET | `/api/v1/node-servers/:id/outbound-configs` | Администратор | Список |
| POST | `/api/v1/node-servers/:id/outbound-configs` | Администратор | Создать |
| DELETE | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | Администратор | Удалить |
| GET | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | Администратор | Подробности |
| PUT | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | Администратор | Обновить |
| PUT | `/api/v1/node-servers/:id/outbound-configs/reorder` | Администратор | Сортировать |
| GET | `/api/v1/node-servers/:id/port-hop/rules` | Администратор | Предварительный просмотр правила переключения портов |
| GET | `/api/v1/node-servers/:id/protocols` | Администратор | Список протоколов |
| GET | `/api/v1/node-servers/:id/protocols/:coreType` | Администратор | Конфигурация протокола |
| PUT | `/api/v1/node-servers/:id/protocols/:coreType` | Администратор | Сохранить конфигурацию протокола |
| GET | `/api/v1/node-servers/:id/protocols/:coreType/config-file` | Администратор | Чтение основного файла конфигурации |
| GET | `/api/v1/node-servers/:id/protocols/:coreType/section/:section` | Администратор | Прочтите раздел протокола |
| PUT | `/api/v1/node-servers/:id/protocols/:coreType/section/:section` | Администратор | Сохранить раздел протокола |
| POST | `/api/v1/node-servers/:id/protocols/:coreType/validate` | Администратор | Проверка конфигурации (реальный пробный прогон ядра) |
| GET | `/api/v1/node-servers/:id/proxy-status` | Администратор | Статус удаленного прокси |
| POST | `/api/v1/node-servers/:id/proxy/batch-update-ports` | Администратор | Массовое обновление портов |
| GET | `/api/v1/node-servers/:id/proxy/config` | Администратор | Чтение конфигурации |
| PUT | `/api/v1/node-servers/:id/proxy/config` | Администратор | Обновить конфигурацию |
| POST | `/api/v1/node-servers/:id/proxy/control` | Администратор | Контроль регистрации связанных прокси |
| POST | `/api/v1/node-servers/:id/proxy/download` | Администратор | Скачать базовую версию прокси |
| POST | `/api/v1/node-servers/:id/proxy/generate-config` | Администратор | Создать конфигурацию |
| POST | `/api/v1/node-servers/:id/proxy/guardian` | Администратор | Обновить конфигурацию супервизора |
| POST | `/api/v1/node-servers/:id/proxy/install` | Администратор | Установить прокси |
| DELETE | `/api/v1/node-servers/:id/proxy/l4-rules` | Администратор | Удалить все правила L4 |
| GET | `/api/v1/node-servers/:id/proxy/l4-settings` | Администратор | Настройки L4 |
| POST | `/api/v1/node-servers/:id/proxy/l4-settings` | Администратор | Сохранить настройки L4 |
| GET | `/api/v1/node-servers/:id/proxy/log-settings` | Администратор | Настройки журнала |
| POST | `/api/v1/node-servers/:id/proxy/log-settings` | Администратор | Сохранить настройки журнала |
| GET | `/api/v1/node-servers/:id/proxy/nginx-settings` | Администратор | Настройки nginx |
| POST | `/api/v1/node-servers/:id/proxy/nginx-settings` | Администратор | Сохраните настройки nginx. |
| GET | `/api/v1/node-servers/:id/proxy/releases` | Администратор | Доступные версии |
| POST | `/api/v1/node-servers/:id/proxy/reload` | Администратор | Перезагрузить конфигурацию |
| POST | `/api/v1/node-servers/:id/proxy/restart` | Администратор | Перезапустить |
| GET | `/api/v1/node-servers/:id/proxy/sites` | Администратор | Список сайтов |
| POST | `/api/v1/node-servers/:id/proxy/sites` | Администратор | Создать сайт |
| DELETE | `/api/v1/node-servers/:id/proxy/sites/:siteId` | Администратор | Удалить сайт |
| PUT | `/api/v1/node-servers/:id/proxy/sites/:siteId` | Администратор | Обновить сайт |
| DELETE | `/api/v1/node-servers/:id/proxy/sni-rules` | Администратор | Удалить все правила SNI. |
| POST | `/api/v1/node-servers/:id/proxy/sni-rules/toggle` | Администратор | Переключить правила SNI |
| GET | `/api/v1/node-servers/:id/proxy/sni-settings` | Администратор | Настройки SNI |
| POST | `/api/v1/node-servers/:id/proxy/sni-settings` | Администратор | Сохраните настройки SNI. |
| POST | `/api/v1/node-servers/:id/proxy/start` | Администратор | Старт |
| GET | `/api/v1/node-servers/:id/proxy/status` | Администратор | Статус прокси-процесса |
| POST | `/api/v1/node-servers/:id/proxy/stop` | Администратор | Стоп |
| POST | `/api/v1/node-servers/:id/proxy/sync` | Администратор | Синхронизировать конфигурацию прокси |
| GET | `/api/v1/node-servers/:id/rate-limit` | Администратор | Чтение конфигурации |
| PUT | `/api/v1/node-servers/:id/rate-limit` | Администратор | Обновить конфигурацию |
| GET | `/api/v1/node-servers/:id/rate-limit/status` | Администратор | Статус |
| GET | `/api/v1/node-servers/:id/recover-mode` | Администратор | Чтение режима восстановления узла |
| PUT | `/api/v1/node-servers/:id/recover-mode` | Администратор | Установить режим восстановления узла |
| POST | `/api/v1/node-servers/:id/revoke-cert` | Администратор | Отозвать сертификат агента |
| GET | `/api/v1/node-servers/:id/services` | Администратор | Список |
| POST | `/api/v1/node-servers/:id/services` | Администратор | Создать |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId` | Администратор | Удалить |
| GET | `/api/v1/node-servers/:id/services/:serviceId` | Администратор | Подробности |
| PUT | `/api/v1/node-servers/:id/services/:serviceId` | Администратор | Обновить |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId/crash-logs` | Администратор | Очистить журналы сбоев |
| GET | `/api/v1/node-servers/:id/services/:serviceId/crash-logs` | Администратор | Список журналов сбоев |
| GET | `/api/v1/node-servers/:id/services/:serviceId/crash-logs/:logId` | Администратор | Подробности журнала сбоев |
| GET | `/api/v1/node-servers/:id/services/:serviceId/endpoints` | Администратор | Список конечных точек |
| POST | `/api/v1/node-servers/:id/services/:serviceId/endpoints` | Администратор | Создать конечную точку |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId/endpoints/:endpointId` | Администратор | Удалить конечную точку |
| PUT | `/api/v1/node-servers/:id/services/:serviceId/endpoints/:endpointId` | Администратор | Обновить конечную точку |
| PUT | `/api/v1/node-servers/:id/services/:serviceId/guardian` | Администратор | Обновить конфигурацию супервизора |
| GET | `/api/v1/node-servers/:id/services/:serviceId/process-status` | Администратор | Получить статус обработки |
| POST | `/api/v1/node-servers/:id/services/:serviceId/restart` | Администратор | Перезапустить |
| POST | `/api/v1/node-servers/:id/services/:serviceId/start` | Администратор | Старт |
| POST | `/api/v1/node-servers/:id/services/:serviceId/stop` | Администратор | Стоп |
| GET | `/api/v1/node-servers/:id/services/process-status` | Администратор | Все статусы процессов |
| GET | `/api/v1/node-servers/:id/share-domains` | Администратор | Поделиться доменами для этого узла |
| POST | `/api/v1/node-servers/:id/share-domains` | Администратор | Создать |
| DELETE | `/api/v1/node-servers/:id/share-domains/:domainId` | Администратор | Удалить |
| PUT | `/api/v1/node-servers/:id/share-domains/:domainId` | Администратор | Обновить |
| DELETE | `/api/v1/node-servers/:id/share-domains/all` | Администратор | Очистить все |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/apply` | Администратор | Применить правила брандмауэра с переключением портов |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/remove` | Администратор | Удалить правила брандмауэра с переключением портов |
| POST | `/api/v1/node-servers/:id/speed-test` | Администратор | Тест скорости узла |
| GET | `/api/v1/node-servers/:id/status` | Администратор | Статус узла |
| POST | `/api/v1/node-servers/:id/sync` | Администратор | Конфигурация узла синхронизации (генерация и отправка) |
| POST | `/api/v1/node-servers/:id/sync-data` | Суперадминистратор | Передача данных панели на узлы (полная синхронизация пользователей/подписок/планов) |
| POST | `/api/v1/node-servers/:id/sync-mirror` | Администратор | Синхронизировать зеркальный узел |
| GET | `/api/v1/node-servers/:id/tags` | Администратор | Список тегов |
| POST | `/api/v1/node-servers/:id/tags` | Администратор | Добавить тег |
| DELETE | `/api/v1/node-servers/:id/tags/:tagKey` | Администратор | Удалить тег |
| GET | `/api/v1/node-servers/:id/traffic-history` | Администратор | История трафика |
| POST | `/api/v1/node-servers/:id/traffic-resume` | Администратор | Восстановить трафик, приостановленный за превышение лимита |
| GET | `/api/v1/node-servers/:id/traffic-usage` | Администратор | Использование трафика |
| POST | `/api/v1/node-servers/:id/update-agent` | Администратор | Агент обновления |
| GET | `/api/v1/node-servers/:id/used-server-names` | Администратор | Список используемых REALITY SNI |
| GET | `/api/v1/node-servers/:id/waf/config` | Администратор | Чтение конфигурации |
| PUT | `/api/v1/node-servers/:id/waf/config` | Администратор | Сохранить конфигурацию |
| POST | `/api/v1/node-servers/:id/waf/disable-domain` | Администратор | Отключить защиту домена |
| GET | `/api/v1/node-servers/:id/waf/domain-config` | Администратор | Список конфигурации домена |
| PUT | `/api/v1/node-servers/:id/waf/domain-config` | Администратор | Массовое сохранение конфигураций домена |
| DELETE | `/api/v1/node-servers/:id/waf/domain-config/:domain` | Администратор | Удалить конфигурацию домена |
| GET | `/api/v1/node-servers/:id/waf/domain-config/:domain` | Администратор | Подробности конфигурации домена |
| PUT | `/api/v1/node-servers/:id/waf/domain-config/:domain` | Администратор | Сохранить конфигурацию домена |
| GET | `/api/v1/node-servers/:id/waf/domain-rules` | Администратор | Каталог правил домена |
| GET | `/api/v1/node-servers/:id/waf/domain-rules/:domain/:name` | Администратор | Чтение файла правил домена |
| PUT | `/api/v1/node-servers/:id/waf/domain-rules/:domain/:name` | Администратор | Сохранить файл правил домена |
| GET | `/api/v1/node-servers/:id/waf/domain-status` | Администратор | Статус защиты домена |
| POST | `/api/v1/node-servers/:id/waf/enable-domain` | Администратор | Включить защиту домена |
| POST | `/api/v1/node-servers/:id/waf/push` | Администратор | Нажмите на этот узел |
| GET | `/api/v1/node-servers/:id/waf/rules` | Администратор | Список правил узла WAF |
| DELETE | `/api/v1/node-servers/:id/waf/rules/:name` | Администратор | Удалить правила узла WAF |
| GET | `/api/v1/node-servers/:id/waf/rules/:name` | Администратор | Подробности правила узла WAF |
| PUT | `/api/v1/node-servers/:id/waf/rules/:name` | Администратор | Сохранить правила узла WAF |
| GET | `/api/v1/node-servers/:id/waf/status` | Администратор | Статус узла WAF |
| PUT | `/api/v1/node-servers/:id/waf/status` | Администратор | Переключить узел WAF |
| POST | `/api/v1/node-servers/:id/waf/sync` | Администратор | Синхронизировать с этим узлом |
| GET | `/api/v1/node-servers/:id/warp` | Администратор | Список |
| DELETE | `/api/v1/node-servers/:id/warp/:warpId` | Администратор | Удалить |
| GET | `/api/v1/node-servers/:id/warp/config` | Администратор | Чтение конфигурации |
| POST | `/api/v1/node-servers/:id/warp/generate-config` | Администратор | Создать конфигурацию |
| POST | `/api/v1/node-servers/:id/warp/register` | Администратор | Зарегистрироваться |
| GET | `/api/v1/node-servers/:id/warp/status` | Администратор | Статус |
| GET | `/api/v1/node-servers/:id/xray/control/balancer/:tag` | Администратор | Получить информацию о балансировщике |
| POST | `/api/v1/node-servers/:id/xray/control/balancer/:tag` | Администратор | Установить цель балансировщика |
| POST | `/api/v1/node-servers/:id/xray/control/firewall/apply` | Администратор | Применить правила брандмауэра с переключением портов |
| POST | `/api/v1/node-servers/:id/xray/control/firewall/remove` | Администратор | Удалить правила брандмауэра с переключением портов |
| POST | `/api/v1/node-servers/:id/xray/control/inbound` | Администратор | Добавить входящий |
| DELETE | `/api/v1/node-servers/:id/xray/control/inbound/:tag` | Администратор | Удалить входящие |
| POST | `/api/v1/node-servers/:id/xray/control/inbound/:tag/user` | Администратор | Добавить пользователя |
| DELETE | `/api/v1/node-servers/:id/xray/control/inbound/:tag/user/:email` | Администратор | Удалить пользователя |
| POST | `/api/v1/node-servers/:id/xray/control/outbound` | Администратор | Добавить исходящие |
| DELETE | `/api/v1/node-servers/:id/xray/control/outbound/:tag` | Администратор | Удалить исходящие |
| POST | `/api/v1/node-servers/:id/xray/control/routing` | Администратор | Запросить маршрут |
| POST | `/api/v1/node-servers/:id/xray/control/routing/rule` | Администратор | Добавить правило маршрутизации |
| DELETE | `/api/v1/node-servers/:id/xray/control/routing/rule/:tag` | Администратор | Удалить правило маршрутизации |
| GET | `/api/v1/node-servers/:id/xray/control/routing/rules` | Администратор | Список правил маршрутизации |
| POST | `/api/v1/node-servers/:id/xray/control/sync` | Администратор | Вручную запустить синхронизацию конфигурации xray |
| GET | `/api/v1/node-servers/:id/xray/stats` | Администратор | Получить статистику |
| POST | `/api/v1/node-servers/:id/xray/stats/reset/:email` | Администратор | Сбросить статистику пользователя |
| GET | `/api/v1/node-servers/:id/xray/stats/users` | Администратор | Получить статистику пользователей |
| GET | `/api/v1/node-servers/billable` | Администратор | Список оплачиваемых узлов |
| GET | `/api/v1/node-servers/ca-cert` | Администратор | Сертификат ЦС |
| GET | `/api/v1/node-servers/ca-status` | Администратор | Статус ЦС |
| GET | `/api/v1/node-servers/dashboard` | Администратор | Статус информационной панели |
| POST | `/api/v1/node-servers/distribute-cert` | Администратор | Распространить сертификат на все узлы |
| POST | `/api/v1/node-servers/failover-check` | Администратор | Запустить проверку аварийного переключения |
| GET | `/api/v1/node-servers/metrics/aggregated` | Администратор | Агрегированные показатели |
| POST | `/api/v1/node-servers/migrate-cert` | Администратор | Перенести сертификат |
| GET | `/api/v1/node-servers/online-summary` | Администратор | Онлайн-резюме |
| GET | `/api/v1/node-servers/online-users` | Администратор | Все онлайн-пользователи |
| GET | `/api/v1/node-servers/pending-agent-updates` | Администратор | Агенты ожидают обновления |
| POST | `/api/v1/node-servers/recommend-region` | Администратор | Рекомендуемые регионы |
| POST | `/api/v1/node-servers/select-nodes` | Администратор | Выберите узлы для подписки |
| POST | `/api/v1/node-servers/speed-test-all` | Администратор | Тест скорости всех узлов |
| GET | `/api/v1/node-servers/speed-test-results` | Администратор | Результаты теста скорости |
| GET | `/api/v1/node-servers/traffic/stats` | Администратор | Статистика трафика |
| POST | `/api/v1/node-servers/update-all-agents` | Администратор | Обновите всех агентов |
| DELETE | `/api/v1/node-servers/users/:client_id` | Администратор | Синхронизировать-удалить клиента |
| GET | `/api/v1/node-servers/users/retry-queue` | Администратор | Состояние очереди повторных попыток |
| POST | `/api/v1/node-servers/users/retry-queue/process` | Администратор | Очередь повторов процесса |
| POST | `/api/v1/node-servers/users/sync` | Администратор | Синхронизировать одного клиента |
| POST | `/api/v1/node-servers/validate-chain-config` | Администратор | Проверка связанной конфигурации |

### Журнал аудита узла

Аудит и экспорт операций на стороне узла.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/node-audit-logs` | Требуется вход | Журнал аудита узла |
| GET | `/api/v1/node-audit-logs/export` | Требуется вход | Экспортировать журналы аудита узла |

### Узлы (устаревшая модель, снята с производства)

> `/api/v1/nodes` CRUD был прекращен 20 октября 2026 г.: узлы управляются единообразно через `/api/v1/node-servers`.
> Запросы онлайн-пользователей остаются по адресу `/api/v1/node-servers/{online-users,online-summary,:id/online-users}`.

### Обзор региона

Представление узла, агрегированное по регионам.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/regions/:region/servers` | Требуется вход | Список узлов для этого региона |
| GET | `/api/v1/regions/overview` | Требуется вход | Обзор региона |

### Подписки

Управление записями о подписке: отзыв, ротация токена, журналы доступа, сброс использования.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/subscriptions` | Требуется вход | Список |
| POST | `/api/v1/subscriptions` | Требуется вход | Создать |
| DELETE | `/api/v1/subscriptions/:id` | Требуется вход | Удалить |
| GET | `/api/v1/subscriptions/:id` | Требуется вход | Подробности |
| PUT | `/api/v1/subscriptions/:id` | Требуется вход | Обновить |
| GET | `/api/v1/subscriptions/:id/access-logs` | Требуется вход | Журнал доступа |
| POST | `/api/v1/subscriptions/:id/enable` | Требуется вход | Включить |
| POST | `/api/v1/subscriptions/:id/reset-usage` | Требуется вход | Сбросить использование |
| POST | `/api/v1/subscriptions/:id/restore` | Требуется вход | Резюме |
| POST | `/api/v1/subscriptions/:id/revoke` | Требуется вход | Отозвать |
| POST | `/api/v1/subscriptions/:id/rotate` | Требуется вход | Поворот |
| GET | `/api/v1/subscriptions/:id/rotations` | Требуется вход | История ротации |

### Форматы подписки

> Поддержка `/api/v1/subscription-formats` CRUD на стороне администратора была прекращена 20 октября 2026 г.: фактическая точка входа:
> `/api/v1/mihomo/configs` (центр формата подписки). Таблица `subscription_formats` остается,
> используется только в качестве начальных данных для общедоступной конечной точки `/api/v1/public/client-configs`.

### Конечные точки подписки

Выделенные домены подписки (обратный прокси-сервер по пути `/sub` панели).

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/subscription-endpoints` | Требуется вход | Список |
| POST | `/api/v1/subscription-endpoints` | Требуется вход | Создать |
| DELETE | `/api/v1/subscription-endpoints/:id` | Требуется вход | Удалить |
| GET | `/api/v1/subscription-endpoints/:id` | Требуется вход | Подробности |
| PUT | `/api/v1/subscription-endpoints/:id` | Требуется вход | Обновить |

### Делитесь доменами

Доступ к пулу доменов/IP-адресов, используемому в клиентских ссылках.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/share-domains` | Требуется вход | Список |
| POST | `/api/v1/share-domains` | Требуется вход | Создать |
| DELETE | `/api/v1/share-domains/:id` | Требуется вход | Удалить |
| PUT | `/api/v1/share-domains/:id` | Требуется вход | Обновить |
| DELETE | `/api/v1/share-domains/all` | Требуется вход | Очистить все |
| POST | `/api/v1/share-domains/seed` | Требуется вход | Инициализировать данные по умолчанию |

### Cloudflare ИП

Cloudflare CDN Управление пулом IP-адресов и источниками.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| DELETE | `/api/v1/cfip/entries` | Требуется вход | Очистить записи |
| GET | `/api/v1/cfip/entries` | Требуется вход | Список участников |
| POST | `/api/v1/cfip/entries` | Требуется вход | Добавить запись |
| DELETE | `/api/v1/cfip/entries/:id` | Требуется вход | Удалить запись |
| POST | `/api/v1/cfip/fetch-all` | Требуется вход | Получить все источники |
| POST | `/api/v1/cfip/seed` | Требуется вход | Инициализировать данные по умолчанию |
| GET | `/api/v1/cfip/sources` | Требуется вход | Список источников данных |
| POST | `/api/v1/cfip/sources` | Требуется вход | Создать источник данных |
| DELETE | `/api/v1/cfip/sources/:id` | Требуется вход | Удалить источник данных |
| PUT | `/api/v1/cfip/sources/:id` | Требуется вход | Обновить источник данных |
| POST | `/api/v1/cfip/sources/:id/fetch` | Требуется вход | Получить этот источник |

### Конфигурация подписки (Mihomo/Clash)

Конфигурация шаблона подписки: группы, правила, поставщики правил, предварительный просмотр YAML.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/mihomo/configs` | Требуется вход | Список |
| POST | `/api/v1/mihomo/configs` | Требуется вход | Создать |
| DELETE | `/api/v1/mihomo/configs/:id` | Требуется вход | Удалить |
| GET | `/api/v1/mihomo/configs/:id` | Требуется вход | Подробности |
| PUT | `/api/v1/mihomo/configs/:id` | Требуется вход | Обновить |
| GET | `/api/v1/mihomo/configs/:id/proxy-groups` | Требуется вход | Список групп |
| POST | `/api/v1/mihomo/configs/:id/proxy-groups` | Требуется вход | Создать группу |
| GET | `/api/v1/mihomo/configs/:id/rule-providers` | Требуется вход | Список поставщиков правил |
| POST | `/api/v1/mihomo/configs/:id/rule-providers` | Требуется вход | Создать поставщика правил |
| GET | `/api/v1/mihomo/configs/:id/rules` | Требуется вход | Список правил |
| POST | `/api/v1/mihomo/configs/:id/rules` | Требуется вход | Создать правило |
| GET | `/api/v1/mihomo/configs/:id/yaml` | Требуется вход | Предварительный просмотр создан YAML |
| DELETE | `/api/v1/mihomo/proxy-groups/:id` | Требуется вход | Удалить группу |
| DELETE | `/api/v1/mihomo/rule-providers/:id` | Требуется вход | Удалить поставщика правил |
| DELETE | `/api/v1/mihomo/rules/:id` | Требуется вход | Удалить правило |

### Сайты пополнения счета

Интеграция сторонних платежных сайтов: ключи подписи, файлы проверки, проверка домена, документы по интеграции.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/charge-sites` | Требуется вход | Список сайтов |
| POST | `/api/v1/charge-sites` | Требуется вход | Создать сайт |
| DELETE | `/api/v1/charge-sites/:id` | Требуется вход | Удалить сайт |
| GET | `/api/v1/charge-sites/:id` | Требуется вход | Детали сайта |
| PUT | `/api/v1/charge-sites/:id` | Требуется вход | Обновить сайт |
| POST | `/api/v1/charge-sites/:id/clear-prev-secret` | Требуется вход | Очистить старый ключ |
| POST | `/api/v1/charge-sites/:id/generate-verify` | Требуется вход | Создать файл проверки |
| POST | `/api/v1/charge-sites/:id/generate-verify-file` | Требуется вход | Создать файл проверки |
| GET | `/api/v1/charge-sites/:id/integration-doc` | Требуется вход | Получить документацию по интеграции |
| POST | `/api/v1/charge-sites/:id/rotate-secret` | Требуется вход | Поворот ключа подписи |
| POST | `/api/v1/charge-sites/:id/verify-dns` | Требуется вход | Подтвердить домен DNS |
| POST | `/api/v1/charge-sites/:id/verify-domain` | Требуется вход | Проверка домена HTTP |
| POST | `/api/v1/charge-sites/:id/verify-domain-dns` | Требуется вход | Подтвердить домен DNS |
| POST | `/api/v1/charge-sites/:id/verify-domain-http` | Требуется вход | Проверка домена HTTP |

### Заказы на пополнение счета

Запросы на пополнение счета, возврат средств, экспорт CSV.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/charge-orders` | Требуется вход | Список заказов |
| GET | `/api/v1/charge-orders/:id` | Требуется вход | Детали заказа |
| POST | `/api/v1/charge-orders/:id/refund` | Требуется вход | Возврат |
| GET | `/api/v1/charge-orders/export-csv` | Требуется вход | Экспортные заказы на пополнение счета CSV |

### Карты пополнения

Массовое создание карт, записи использования, экспорт CSV.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/charge-cards` | Требуется вход | Список карт |
| POST | `/api/v1/charge-cards` | Требуется вход | Создать карту |
| DELETE | `/api/v1/charge-cards/:id` | Требуется вход | Удалить карту |
| GET | `/api/v1/charge-cards/:id/usages` | Требуется вход | Записи об использовании карты |
| POST | `/api/v1/charge-cards/batch` | Требуется вход | Массовое создание карточек |
| GET | `/api/v1/charge-cards/export-csv` | Требуется вход | Экспортные карты CSV |

### События обратного вызова для оплаты

Аудит событий обратного вызова.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/charge-callback-events` | Требуется вход | Список событий обратного вызова |

### Повторные попытки обратного вызова

Ручная повторная попытка неудачных обратных вызовов.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/charge-callback-retries` | Требуется вход | Список неудачных обратных вызовов |
| POST | `/api/v1/charge-callback-retries/:id/retry` | Администратор | Повторить обратный вызов |

### Баланс клиента

Просмотр балансов и транзакций администратором.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/client-balances` | Требуется вход | Список баланса клиента |
| GET | `/api/v1/client-balances/transactions` | Требуется вход | Балансовые операции (администратор) |
| GET | `/api/v1/client-balances/transactions/export-csv` | Требуется вход | Транзакции экспортного баланса CSV |

### Подарки

Выдавать требуемые подарки (дни/трафик).

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/client-gifts` | Требуется вход | Список подарков |
| POST | `/api/v1/client-gifts` | Требуется вход | Создать подарок |
| POST | `/api/v1/client-gifts/batch` | Требуется вход | Массовое создание подарков |
| GET | `/api/v1/client-gifts/search-clients` | Требуется вход | Поиск клиентов |

### Реферальная конфигурация

Настройка и статистика реферальной комиссии.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/referral/config` | Требуется вход | Реферальная конфигурация |
| PUT | `/api/v1/referral/config` | Администратор | Сохранить реферальную конфигурацию |
| GET | `/api/v1/referral/records` | Требуется вход | Список реферальных записей |
| GET | `/api/v1/referral/stats` | Требуется вход | Статистика рефералов |

### Вывод средств по рефералам

Рассмотрите запросы на вывод средств.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/referral-withdrawals` | Требуется вход | Список запросов на вывод средств |
| POST | `/api/v1/referral-withdrawals/:id/approve` | Требуется вход | Одобрить вывод |
| POST | `/api/v1/referral-withdrawals/:id/reject` | Требуется вход | Отклонить вывод средств |

### Списки разрешенных IP-адресов (сводка)

Просмотр списка разрешенных IP-адресов для всех клиентов.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/ip-whitelists` | Требуется вход | Все списки разрешенных IP-адресов клиентов |

### Привязки OAuth клиента

Запрос привязок сторонних учетных записей клиентов.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/client-oauth-bindings` | Требуется вход | Список привязки OAuth клиента |

### Билеты (админ)

Ответ на заявку, назначение и закрытие.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/tickets` | Требуется вход | Список билетов |
| GET | `/api/v1/tickets/:id` | Требуется вход | Детали билета |
| POST | `/api/v1/tickets/:id/assign` | Требуется вход | Назначить билет |
| POST | `/api/v1/tickets/:id/close` | Требуется вход | Закрыть заявку |
| POST | `/api/v1/tickets/:id/reply` | Требуется вход | Ответ на билет |

### Статьи

CRUD для статей портала.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/articles` | Требуется вход | Список статей |
| POST | `/api/v1/articles` | Требуется вход | Создать статью |
| DELETE | `/api/v1/articles/:id` | Требуется вход | Удалить статью |
| GET | `/api/v1/articles/:id` | Требуется вход | Подробности статьи |
| PUT | `/api/v1/articles/:id` | Требуется вход | Обновить статью |

### Общие аккаунты

Записи о выдаче и исправлении общих учетных записей (стриминг и т. д.).

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/shared-accounts` | Требуется вход | Общий список аккаунтов |
| POST | `/api/v1/shared-accounts` | Требуется вход | Создать общий аккаунт |
| DELETE | `/api/v1/shared-accounts/:id` | Требуется вход | Удалить общий аккаунт |
| GET | `/api/v1/shared-accounts/:id` | Требуется вход | Общие данные аккаунта |
| PUT | `/api/v1/shared-accounts/:id` | Требуется вход | Обновить общий аккаунт |
| GET | `/api/v1/shared-accounts/:id/repair-logs` | Требуется вход | Восстановить записи |

### Сертификаты

Выдача, загрузка, продление и экспорт публичного сертификата (ACME); поддерживается выдача на конкретном узле.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/certificates` | Требуется вход | Список |
| POST | `/api/v1/certificates` | Требуется вход | Создать |
| DELETE | `/api/v1/certificates/:id` | Требуется вход | Удалить |
| GET | `/api/v1/certificates/:id` | Требуется вход | Подробности |
| PUT | `/api/v1/certificates/:id` | Требуется вход | Обновить |
| GET | `/api/v1/certificates/:id/download` | Требуется вход | Скачать |
| POST | `/api/v1/certificates/:id/renew` | Требуется вход | Продлить клиента |
| GET | `/api/v1/certificates/check-port` | Требуется вход | Проверьте порт |
| GET | `/api/v1/certificates/detect-ip` | Требуется вход | IP-зонд |
| GET | `/api/v1/certificates/export` | Требуется вход | Экспорт |
| POST | `/api/v1/certificates/issue` | Требуется вход | Проблема. корпус: `{"domains":["example.com","*.example.com"],"challenge_type":"dns-01","dns_channel_id":2,"key_type":"rsa","key_bits":4096}`. Для `challenge_type` по умолчанию используется http-01; `dns_channel_id` выбирает канал DNS (настроенный провайдер DNS + учетные данные) |
| POST | `/api/v1/certificates/issue-on-node` | Требуется вход | Выдать на узел |
| POST | `/api/v1/certificates/upload` | Требуется вход | Загрузить |

### Каналы сертификатов

Конфигурация канала ACME (CA).

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/cert-channels` | Требуется вход | Список |
| POST | `/api/v1/cert-channels` | Требуется вход | Создать |
| DELETE | `/api/v1/cert-channels/:id` | Требуется вход | Удалить |
| PUT | `/api/v1/cert-channels/:id` | Требуется вход | Обновить |
| GET | `/api/v1/cert-channels/providers` | Требуется вход | Поставщик |

### Конфигурация EAB

Учетные данные, привязывающие внешние учетные записи, такие как ZeroSSL/Google.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/eab-configs` | Требуется вход | Список |
| POST | `/api/v1/eab-configs` | Требуется вход | Создать |
| DELETE | `/api/v1/eab-configs/:id` | Требуется вход | Удалить |
| PUT | `/api/v1/eab-configs/:id` | Требуется вход | Обновить |
| POST | `/api/v1/eab-configs/fetch-google` | Требуется вход | Получить Google EAB из GCP |
| POST | `/api/v1/eab-configs/fetch-zerossl` | Требуется вход | Получите ZeroSSL EAB по электронной почте в один клик |

### Счета ACME

Управление учетной записью ACME (контактный адрес электронной почты) на панели.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/acme-accounts` | Требуется вход | Список |
| POST | `/api/v1/acme-accounts` | Требуется вход | Создать |
| DELETE | `/api/v1/acme-accounts/:id` | Требуется вход | Удалить |
| PUT | `/api/v1/acme-accounts/:id` | Требуется вход | Обновить |
| GET | `/api/v1/acme-accounts/default` | Требуется вход | Получить значения по умолчанию |

### Встроенный центр сертификации

Центр сертификации внутренней панели: выдача/обновление/аннулирование сертификата агента, экспорт списка отзыва сертификатов, очередь распространения.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/ca/agent-certs` | Требуется вход | Список сертификатов агента |
| POST | `/api/v1/ca/agent-certs/:server_id/renew` | Требуется вход | Продлить сертификат агента |
| GET | `/api/v1/ca/distribution-queue` | Требуется вход | Очередь распространения сертификатов |
| DELETE | `/api/v1/ca/distribution-queue/:id` | Требуется вход | Удалить запись очереди распределения |
| POST | `/api/v1/ca/distribution-queue/:id/retry` | Требуется вход | Повторить распространение сертификата |
| GET | `/api/v1/ca/revocations` | Требуется вход | Список отзыва |
| GET | `/api/v1/ca/revocations/export` | Требуется вход | Экспорт списка отзыва сертификатов |
| GET | `/api/v1/ca/status` | Требуется вход | Статус ЦС |

### Гео правила

Синхронизация и доставка источников и файлов данных geoip/геосайта.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/geo-rules/files` | Требуется вход | Список файлов |
| DELETE | `/api/v1/geo-rules/files/:fileId` | Требуется вход | Удалить файл |
| PUT | `/api/v1/geo-rules/files/:fileId/auto-sync` | Требуется вход | Включить автоматическую синхронизацию |
| POST | `/api/v1/geo-rules/files/:fileId/download` | Требуется вход | Скачать |
| GET | `/api/v1/geo-rules/files/:fileId/download-file` | Требуется вход | Скачать файл |
| GET | `/api/v1/geo-rules/github-assets` | Требуется вход | Получить ресурсы репозитория GitHub. |
| POST | `/api/v1/geo-rules/push-all` | Требуется вход | Отправить на все узлы |
| POST | `/api/v1/geo-rules/seed` | Требуется вход | Инициализировать данные по умолчанию |
| GET | `/api/v1/geo-rules/sources` | Требуется вход | Список источников данных |
| POST | `/api/v1/geo-rules/sources` | Требуется вход | Создать источник данных |
| DELETE | `/api/v1/geo-rules/sources/:sourceId` | Требуется вход | Удалить источник данных |
| PUT | `/api/v1/geo-rules/sources/:sourceId` | Требуется вход | Обновить источник данных |
| GET | `/api/v1/geo-rules/sources/:sourceId/assets` | Требуется вход | Получить ресурсы выпуска |
| POST | `/api/v1/geo-rules/sources/:sourceId/files` | Требуется вход | Добавить файл |
| PUT | `/api/v1/geo-rules/sources/:sourceId/schedule` | Требуется вход | Переключить синхронизацию по расписанию |
| POST | `/api/v1/geo-rules/sources/:sourceId/select` | Требуется вход | Выберите файл |
| POST | `/api/v1/geo-rules/sources/:sourceId/sync` | Требуется вход | Синхронизировать этот источник данных |
| GET | `/api/v1/geo-rules/status` | Требуется вход | Статус |
| POST | `/api/v1/geo-rules/sync-all` | Требуется вход | Синхронизировать все |
| GET | `/api/v1/geo-rules/sync-status/:key` | Требуется вход | Синхронизировать прогресс |

### Библиотека правил WAF

Глобальные правила WAF: источник GitHub, синхронизация, доставка на все узлы.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/waf/github-source` | Требуется вход | Источник данных GitHub |
| PUT | `/api/v1/waf/github-source` | Требуется вход | Сохранить источник данных GitHub |
| POST | `/api/v1/waf/push-all` | Требуется вход | Отправить на все узлы |
| GET | `/api/v1/waf/rules` | Требуется вход | Список правил |
| DELETE | `/api/v1/waf/rules/:name` | Требуется вход | Удалить правило |
| GET | `/api/v1/waf/rules/:name` | Требуется вход | Получить правило |
| PUT | `/api/v1/waf/rules/:name` | Требуется вход | Сохранить правило |
| POST | `/api/v1/waf/sync-all` | Требуется вход | Синхронизировать все |
| POST | `/api/v1/waf/sync-github` | Требуется вход | Синхронизация с GitHub |

### Управление журналами

Чтение и потоковая передача журналов узла/ядра, политики очистки, управление записью журналов.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| POST | `/api/v1/log-manage/cleanup-all` | Требуется вход | Очистите все сейчас |
| GET | `/api/v1/log-manage/cleanup-rules` | Требуется вход | Список правил очистки |
| PUT | `/api/v1/log-manage/cleanup-rules/:id` | Требуется вход | Обновить правило очистки |
| POST | `/api/v1/log-manage/cleanup-table/:table` | Требуется вход | Очистить указанную таблицу сейчас |
| GET | `/api/v1/log-manage/core-log-output` | Требуется вход | Конфигурация вывода основного журнала |
| PUT | `/api/v1/log-manage/core-log-output` | Требуется вход | Сохранить конфигурацию вывода основного журнала |
| GET | `/api/v1/log-manage/core/:type/stdout-log` | Требуется вход | Чтение основных журналов стандартного вывода |
| GET | `/api/v1/log-manage/core/:type/stdout-stream` | Требуется вход | Поток основного стандартного вывода |
| POST | `/api/v1/log-manage/discover` | Требуется вход | Откройте для себя файлы журналов узла |
| GET | `/api/v1/log-manage/log-output` | Требуется вход | Конфигурация вывода журнала |
| PUT | `/api/v1/log-manage/log-output` | Требуется вход | Сохранить конфигурацию вывода журнала |
| POST | `/api/v1/log-manage/log-output/release/:type` | Требуется вход | Перехват стандартного вывода журнала выпуска |
| POST | `/api/v1/log-manage/log-output/takeover/:type` | Требуется вход | Взять на себя стандартный вывод журнала |
| GET | `/api/v1/log-manage/nodes/:serverId/logs` | Требуется вход | Список файлов журнала узла |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type` | Требуется вход | Чтение журналов узлов |
| POST | `/api/v1/log-manage/nodes/:serverId/logs/:type/rotate` | Требуется вход | Поворот журналов узлов |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type/stream` | Требуется вход | Журналы потокового узла |
| GET | `/api/v1/log-manage/policies` | Требуется вход | Список политик |
| POST | `/api/v1/log-manage/policies` | Требуется вход | Создать политику |
| DELETE | `/api/v1/log-manage/policies/:id` | Требуется вход | Удалить политику |
| GET | `/api/v1/log-manage/policies/:id` | Требуется вход | Подробности политики |
| PUT | `/api/v1/log-manage/policies/:id` | Требуется вход | Обновить политику |
| GET | `/api/v1/log-manage/policies/:id/jobs` | Требуется вход | Список заданий по очистке |
| GET | `/api/v1/log-manage/policies/:id/log` | Требуется вход | Чтение журналов |
| GET | `/api/v1/log-manage/policies/:id/log-stream` | Требуется вход | Журналы потоков |
| POST | `/api/v1/log-manage/policies/:id/rotate` | Требуется вход | Поворот |
| GET | `/api/v1/log-manage/proxy/:type/stdout-log` | Требуется вход | Чтение основных журналов стандартного вывода |
| GET | `/api/v1/log-manage/proxy/:type/stdout-stream` | Требуется вход | Поток основного стандартного вывода |
| GET | `/api/v1/log-manage/scheduler-config` | Требуется вход | Конфигурация планирования |
| PUT | `/api/v1/log-manage/scheduler-config` | Требуется вход | Обновить конфигурацию планирования |
| GET | `/api/v1/log-manage/status` | Требуется вход | Статус |
| POST | `/api/v1/log-manage/sync` | Требуется вход | Синхронизировать политики очистки |
| GET | `/api/v1/log-manage/writers` | Требуется вход | Статус писателя |
| PUT | `/api/v1/log-manage/writers/:type/config` | Требуется вход | Обновить конфигурацию записи |
| GET | `/api/v1/log-manage/writers/:type/files` | Требуется вход | Список файлов журнала записи |
| POST | `/api/v1/log-manage/writers/:type/start` | Требуется вход | Начать писать |
| POST | `/api/v1/log-manage/writers/:type/stop` | Требуется вход | Остановить писателя |

### Резервные копии

База данных/полное резервное копирование, восстановление, запланированные задания, удаленное хранилище FTP/S3/WebDAV.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| DELETE | `/api/v1/backups/:id` | Администратор | Удалить резервную копию |
| GET | `/api/v1/backups/:id/download` | Администратор | Скачать резервную копию |
| POST | `/api/v1/backups/:id/restore` | Администратор | Восстановление из резервной копии |
| POST | `/api/v1/backups/:id/upload-ftp` | Администратор | Загрузить в FTP |
| POST | `/api/v1/backups/:id/upload-s3` | Администратор | Загрузить в S3 |
| POST | `/api/v1/backups/:id/upload-webdav` | Администратор | Загрузить в WebDAV |
| POST | `/api/v1/backups/create` | Администратор | Создать полную резервную копию (с дампом БД) |
| GET | `/api/v1/backups/list` | Администратор | Полный список резервных копий |
| POST | `/api/v1/backups/remote-restore` | Администратор | Восстановление с удаленного доступа |
| GET | `/api/v1/backups/restore-status` | Администратор | Восстановить прогресс |
| GET | `/api/v1/backups/schedules` | Администратор | Список запланированных работ |
| POST | `/api/v1/backups/schedules` | Администратор | Создать запланированное задание |
| DELETE | `/api/v1/backups/schedules/:id` | Администратор | Удалить запланированное задание |
| PUT | `/api/v1/backups/schedules/:id` | Администратор | Обновить запланированное задание |
| GET | `/api/v1/backups/storages` | Администратор | Список конфигурации хранилища |
| POST | `/api/v1/backups/storages` | Администратор | Создать конфигурацию хранилища |
| DELETE | `/api/v1/backups/storages/:id` | Администратор | Удалить конфигурацию хранилища |
| PUT | `/api/v1/backups/storages/:id` | Администратор | Обновить конфигурацию хранилища |
| GET | `/api/v1/backups/storages/:id/files` | Администратор | Список файлов удаленных резервных копий |
| POST | `/api/v1/backups/upload-restore` | Администратор | Загрузите резервную копию и восстановите |

### Системные настройки

Конфигурация и миграция базы данных, настройки панели, ключи JWT, тесты SMTP/Telegram, шаблоны уведомлений, онлайн-обновления (в основном для администратора).

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| PUT | `/api/v1/system/bootstrap-finish` | Администратор | Полная настройка при первой загрузке |
| GET | `/api/v1/system/bootstrap-state` | Администратор | Статус первой загрузки |
| GET | `/api/v1/system/config-file/:coreType` | Администратор | Чтение основного файла конфигурации |
| GET | `/api/v1/system/db-config` | Администратор | Конфигурация базы данных |
| PUT | `/api/v1/system/db-config` | Администратор | Обновить конфигурацию базы данных |
| POST | `/api/v1/system/db-config/migrate` | Администратор | Перенос данных базы данных |
| POST | `/api/v1/system/db-config/switch` | Администратор | Переключить базу данных (с одноэтапной миграцией) |
| POST | `/api/v1/system/db-config/test` | Администратор | Проверить подключение к базе данных |
| GET | `/api/v1/system/db-pool-stats` | Администратор | Статистика пула подключений |
| GET | `/api/v1/system/db-stats` | Администратор | Статистика базы данных |
| GET | `/api/v1/system/db-stats/ping` | Администратор | Зонд базы данных |
| POST | `/api/v1/system/db-stats/reset` | Администратор | Сбросить статистику базы данных |
| GET | `/api/v1/system/encryption-key-status` | Администратор | Статус ключа шифрования |
| GET | `/api/v1/system/env-config` | Администратор | Конфигурация .env |
| PUT | `/api/v1/system/env-config` | Администратор | Обновить конфигурацию .env |
| POST | `/api/v1/system/generate-encryption-key` | Администратор | Создать ключ шифрования |
| GET | `/api/v1/system/github-config` | Администратор | Конфигурация GitHub |
| PUT | `/api/v1/system/github-config` | Администратор | Обновить конфигурацию GitHub |
| GET | `/api/v1/system/jwt-secret-status` | Администратор | Статус ключа JWT |
| GET | `/api/v1/system/maintenance` | Публичный | Статус режима обслуживания (общедоступный) |
| PUT | `/api/v1/system/maintenance` | Администратор | Переключить режим обслуживания |
| POST | `/api/v1/system/migrate-data` | Администратор | Перенос данных |
| GET | `/api/v1/system/notification-config` | Администратор | Конфигурация уведомлений |
| PUT | `/api/v1/system/notification-config` | Администратор | Обновить конфигурацию уведомлений |
| GET | `/api/v1/system/notification-templates` | Администратор | Список шаблонов |
| POST | `/api/v1/system/notification-templates` | Администратор | Создать шаблон |
| DELETE | `/api/v1/system/notification-templates/:id` | Администратор | Удалить шаблон |
| PUT | `/api/v1/system/notification-templates/:id` | Администратор | Обновить шаблон |
| POST | `/api/v1/system/notification-templates/:id/broadcast` | Администратор | Широковещательное уведомление по шаблону |
| POST | `/api/v1/system/notification-templates/:id/send` | Администратор | Отправить тестовое уведомление |
| GET | `/api/v1/system/panel-config` | Администратор | Конфигурация панели |
| PUT | `/api/v1/system/panel-config` | Администратор | Обновить конфигурацию панели |
| GET | `/api/v1/system/reality-targets` | Администратор | Целевой список REALITY |
| PUT | `/api/v1/system/reality-targets` | Администратор | Обновить цель REALITY |
| POST | `/api/v1/system/reality-targets/reset` | Администратор | Сбросить цель REALITY по умолчанию |
| POST | `/api/v1/system/regenerate-jwt-secret` | Администратор | Восстановить ключ JWT |
| GET | `/api/v1/system/releases` | Администратор | Доступные версии |
| POST | `/api/v1/system/restart` | Администратор | Перезапустить |
| POST | `/api/v1/system/rollback` | Администратор | Откат системы |
| GET | `/api/v1/system/settings` | Администратор | Чтение настроек |
| PUT | `/api/v1/system/settings/batch` | Администратор | Настройки массового обновления |
| GET | `/api/v1/system/stats` | Администратор | Статус системы |
| GET | `/api/v1/system/subscription-billing-templates` | Администратор | Шаблон оплаты подписки |
| PUT | `/api/v1/system/subscription-billing-templates` | Администратор | Обновить шаблон оплаты подписки |
| POST | `/api/v1/system/telegram-chat-id` | Администратор | Получить идентификатор чата Telegram |
| POST | `/api/v1/system/test-smtp` | Администратор | Тест SMTP |
| POST | `/api/v1/system/test-telegram` | Администратор | Тест Telegram |
| POST | `/api/v1/system/update` | Администратор | Обновление системы |
| GET | `/api/v1/system/update-jobs` | Администратор | Обновить список вакансий |
| POST | `/api/v1/system/upload-mtls-ca` | Администратор | Загрузите сертификат CA mTLS |

### Аудит безопасности

Аудит запросов событий и экспорт.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/security/audit-events` | Требуется вход | Список событий аудита |
| GET | `/api/v1/security/audit-events/export` | Требуется вход | События экспортного аудита |

### Метрики

Панельные агрегированные метрики (узлы/сертификаты/сервисы).

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/metrics/certificates` | Требуется вход | Метрики сертификата |
| GET | `/api/v1/metrics/nodes` | Требуется вход | Метрики узла |
| GET | `/api/v1/metrics/overview` | Требуется вход | Обзор |

### Шаблоны порталов

Шаблоны портала, конфигурация активации и управление кэшем.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/portal/active` | Требуется вход | Активный шаблон |
| PUT | `/api/v1/portal/active` | Требуется вход | Обновить активный шаблон |
| DELETE | `/api/v1/portal/cache` | Требуется вход | Очистить кэш портала |
| GET | `/api/v1/portal/cache` | Требуется вход | Статус кэша |
| PUT | `/api/v1/portal/cache/dev-mode` | Требуется вход | Переключить режим разработки кэша |
| POST | `/api/v1/portal/cache/rebuild` | Требуется вход | Восстановить кэш портала |
| POST | `/api/v1/portal/reset` | Требуется вход | Восстановить шаблон по умолчанию |
| GET | `/api/v1/portal/templates` | Требуется вход | Список |
| POST | `/api/v1/portal/templates` | Требуется вход | Создать |
| DELETE | `/api/v1/portal/templates/:id` | Требуется вход | Удалить |
| GET | `/api/v1/portal/templates/:id` | Требуется вход | Подробности |
| PUT | `/api/v1/portal/templates/:id` | Требуется вход | Обновить |
| POST | `/api/v1/portal/templates/:id/activate` | Требуется вход | Активировать шаблон |

> `/portal/templates` CRUD — это **расширенный API**: пользовательский интерфейс портала в основном использует `PUT /api/v1/portal/active`.
> (однократная полная конфигурация сайта); управлять/переключать несколько шаблонов с помощью этой группы. Оба поддерживаются и не конфликтуют.

### Установка агента

Сценарий установки и загрузки Node Agent (общедоступный).

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| POST | `/api/v1/agent/bootstrap` | Публичный | Регистрация агента при первой загрузке (агент сообщает с помощью токена начальной загрузки; общедоступный) |
| GET | `/api/v1/agent/releases` | Администратор | Доступные версии Агента (для страницы обновления) |
| GET | `/api/v1/agent/version` | Администратор | Текущая версия Агента на панели |
| GET | `/api/v1/agent/download` | Публичный | Скачать агент |
| GET | `/api/v1/agent/install` | Публичный | Получить скрипт установки |

### Запись об обратном вызове

Обратный вызов сторонней платежной платформы (публичный, подпись проверена).

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| POST | `/api/v1/charge/callback/:site_id` | Публичный | Обработка обратного вызова по оплате |

### Входящие (с узлами)

Входящий список, используемый для выбора узла плана.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/inbounds-with-server` | Требуется вход | Все входящие (включая их узлы) |

### Публичная конфигурация

Конфигурация среды выполнения портала без входа в систему (защита обслуживания + общедоступное ограничение скорости).

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/public/articles` | Публичный | Публичный список статей |
| GET | `/api/v1/public/articles/:slug` | Публичный | Подробности общедоступной статьи |
| GET | `/api/v1/public/client-configs` | Публичный | Конфигурация загрузки клиента (общедоступная) |
| GET | `/api/v1/public/portal-config` | Публичный | Конфигурация среды выполнения портала (имя сайта, тема, языки, документы соответствия; общедоступные) |
| GET | `/api/v1/public/settings` | Публичный | Публичные настройки (например, Registration_enabled; public) |
| GET | `/api/v1/public/subscription-endpoints` | Публичный | Включенный список конечных точек подписки (общедоступный) |

### Аутентификация и бизнес клиента (то же, что и в системе администратора)

Клиентские конечные точки, идентичные порталу; подробности см. в `portal-api.en.md`.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/api/v1/client-auth/articles` | Публичный | Список статей |
| GET | `/api/v1/client-auth/articles/:slug` | Публичный | Получить статью |
| GET | `/api/v1/client-auth/balance` | Публичный | Получить баланс |
| GET | `/api/v1/client-auth/balance/transactions` | Публичный | Балансовые операции |
| GET | `/api/v1/client-auth/bills` | Публичный | Список заказов |
| POST | `/api/v1/client-auth/change-password` | Публичный | Изменить пароль |
| POST | `/api/v1/client-auth/charge/create` | Публичный | Создать заказ на пополнение счета |
| GET | `/api/v1/client-auth/charge/orders` | Публичный | Список заказов на пополнение счета |
| GET | `/api/v1/client-auth/charge/orders/:id` | Публичный | Получить подробную информацию о заказе на пополнение счета |
| POST | `/api/v1/client-auth/charge/redeem-card` | Публичный | Погасить карту |
| GET | `/api/v1/client-auth/charge/sites` | Публичный | Список сайтов пополнения счета |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | Публичный | Запросить статус пополнения счета |
| GET | `/api/v1/client-auth/checktime` | Публичный | Получить время сервера (синхронизация часов) |
| GET | `/api/v1/client-auth/connection-info` | Публичный | Получить параметры подключения плана |
| GET | `/api/v1/client-auth/gifts` | Публичный | Список подарков |
| POST | `/api/v1/client-auth/gifts/:id/claim` | Публичный | Получить подарок |
| GET | `/api/v1/client-auth/invoices` | Публичный | Список заказов |
| GET | `/api/v1/client-auth/invoices/:id` | Публичный | Детали заказа |
| GET | `/api/v1/client-auth/ip-whitelist` | Публичный | Список разрешенных IP-адресов |
| DELETE | `/api/v1/client-auth/ip-whitelist/:id` | Публичный | Удалить из списка разрешенных IP-адресов |
| POST | `/api/v1/client-auth/ip-whitelist/auto-add` | Публичный | Автоматическое добавление IP |
| POST | `/api/v1/client-auth/login` | Публичный | Войти |
| POST | `/api/v1/client-auth/logout` | Публичный | Выйти |
| POST | `/api/v1/client-auth/logs/package` | Публичный | Создать пакет журналов |
| GET | `/api/v1/client-auth/logs/packages` | Публичный | Получение списка пакетов журналов |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | Публичный | Удалить пакет журналов |
| GET | `/api/v1/client-auth/logs/packages/:id` | Публичный | Получить пакет журналов |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | Публичный | Скачать пакет журналов |
| GET | `/api/v1/client-auth/logs/types` | Публичный | Проверка работоспособности пакета журналов |
| GET | `/api/v1/client-auth/me` | Публичный | Получить текущего вошедшего пользователя |
| GET | `/api/v1/client-auth/nodes` | Публичный | Мой список узлов |
| GET | `/api/v1/client-auth/nodes/:id/config` | Публичный | Конфигурация узла |
| GET | `/api/v1/client-auth/nodes/:id/link` | Публичный | Ссылка на общий доступ к узлу |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | Публичный | QR-код узла |
| GET | `/api/v1/client-auth/oauth/:provider` | Публичный | Авторизовать |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | Публичный | Обратный звонок |
| GET | `/api/v1/client-auth/oauth/providers` | Публичный | Статус провайдера |
| GET | `/api/v1/client-auth/orders` | Публичный | Список заказов |
| POST | `/api/v1/client-auth/orders` | Публичный | Создать заказ |
| GET | `/api/v1/client-auth/orders/:id` | Публичный | Детали заказа |
| GET | `/api/v1/client-auth/plans` | Публичный | Список планов |
| GET | `/api/v1/client-auth/plans/:id` | Публичный | Детали плана |
| GET | `/api/v1/client-auth/profile` | Публичный | Получить профиль |
| PUT | `/api/v1/client-auth/profile` | Публичный | Обновить профиль |
| POST | `/api/v1/client-auth/profile/bind-oauth` | Публичный | Привязать сторонний аккаунт |
| DELETE | `/api/v1/client-auth/profile/unbind-oauth/:provider` | Публичный | Отвязать сторонний аккаунт |
| GET | `/api/v1/client-auth/referral/code` | Публичный | Получить код подтверждения реферала |
| GET | `/api/v1/client-auth/referral/records` | Публичный | Список реферальных записей |
| GET | `/api/v1/client-auth/referral/stats` | Публичный | Получить статистику рефералов |
| POST | `/api/v1/client-auth/referral/withdraw` | Публичный | Запросить вывод средств |
| GET | `/api/v1/client-auth/referral/withdrawals` | Публичный | Список выводов рефералов |
| POST | `/api/v1/client-auth/refresh` | Публичный | Обновить |
| POST | `/api/v1/client-auth/register` | Публичный | Зарегистрироваться |
| POST | `/api/v1/client-auth/reset-subscription` | Публичный | Сбросить подписку |
| POST | `/api/v1/client-auth/rotate-token` | Публичный | Ротация токена подписки |
| POST | `/api/v1/client-auth/self-unban` | Публичный | Самообслуживание разблокирует |
| PUT | `/api/v1/client-auth/settings` | Публичный | Обновить настройки |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | Публичный | Автоматическое добавление IP |
| GET | `/api/v1/client-auth/settings/whitelist` | Публичный | Список разрешенных IP-адресов |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | Публичный | Удалить из списка разрешенных IP-адресов |
| GET | `/api/v1/client-auth/shared-accounts` | Публичный | Список общих учетных записей |
| GET | `/api/v1/client-auth/shared-accounts/:id` | Публичный | Получить общий аккаунт |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | Публичный | Запросить общий аккаунт |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | Публичный | Заявка на ремонт |
| POST | `/api/v1/client-auth/ticket` | Публичный | Выдать одноразовый билет с коротким сроком действия |
| GET | `/api/v1/client-auth/tickets` | Публичный | Список билетов |
| POST | `/api/v1/client-auth/tickets` | Публичный | Создать билет |
| GET | `/api/v1/client-auth/tickets/:id` | Публичный | Получить билет |
| POST | `/api/v1/client-auth/tickets/:id/close` | Публичный | Закрыть заявку |
| POST | `/api/v1/client-auth/tickets/:id/reply` | Публичный | Ответ на билет |
| GET | `/api/v1/client-auth/traffic-history` | Публичный | История трафика |
| GET | `/api/v1/client-auth/traffic-packs` | Публичный | Список пакетов трафика |
| POST | `/api/v1/client-auth/traffic-packs/buy` | Публичный | Купить пакет трафика |

### Доставка подписки

Публичная точка входа для клиентов для получения подписок; см. `subscription.en.md`.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/sub/:token` | Публичный | Обслуживать подписку |

### Рабочий процесс настройки среды выполнения

Снимки конфигурации/пробный запуск/применение/откат (для `apply`, `rollback` и `disable-proxy` требуется администратор).

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| POST | `/runtime/apply` | Публичный | Применить |
| GET | `/runtime/apply-jobs` | Требуется вход | Список вакансий |
| GET | `/runtime/apply-jobs/:id` | Требуется вход | Получить работу |
| POST | `/runtime/apply-jobs/:id/cancel` | Публичный | Отменить задание |
| POST | `/runtime/apply-jobs/:id/retry` | Публичный | Повторить задание |
| POST | `/runtime/disable-proxy` | Публичный | Отключить прокси |
| POST | `/runtime/dry-run` | Требуется вход | Исполнение всухую |
| GET | `/runtime/nodes` | Требуется вход | Список узлов |
| GET | `/runtime/overview` | Требуется вход | Обзор |
| POST | `/runtime/rollback` | Публичный | Откат назад |
| GET | `/runtime/snapshots` | Требуется вход | Получение списка снимков |
| POST | `/runtime/snapshots` | Требуется вход | Создать снимок |
| DELETE | `/runtime/snapshots/:id` | Публичный | Удалить снимок |
| GET | `/runtime/snapshots/:id` | Требуется вход | Получить снимок |
| POST | `/runtime/verify` | Требуется вход | Проверить |

### Политики доступа к услугам (устарело)

> `/services/:id/access-policies` и `/services/:id/ip-bans` были выведены из эксплуатации 20 октября 2026 г.:
> нет вызовов администратора, и ни один путь подключения не использует эти политики.

### Мониторинг квот трафика

Доступное только для чтения представление мониторинга квот и использования.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/traffic/quotas` | Требуется вход | Список квот |
| GET | `/traffic/quotas/:id/usage` | Требуется вход | Использование квоты |
| GET | `/traffic/reset-logs` | Требуется вход | Записи сброса квоты |
| GET | `/traffic/usage-history` | Требуется вход | История использования |

### Проверка здоровья

Зонд живости.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/health` | Публичный | Зонд живости; 200 значит здоров |

### Страница портала

Страница SPA портала.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/p` | Публичный | HTML портала (отображается на сервере + среда выполнения Vue) |

### ACME задачи

Обратный вызов вызова HTTP-01.

| Метод | Путь | Разрешение | Описание |
|---|---|---|---|
| GET | `/.well-known/acme-challenge/:token` | Публичный | ACME HTTP-01 обратный вызов запроса (проверяет домен во время выдачи) |
