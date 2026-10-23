<!-- source: README.md | locale: ru | updated: 2026-10-23 -->
# CoreFusion Документация

> Языки: [简体中文](README.md) · [English](README.en.md) · [繁體中文](README.zh-TW.md) · Русский · [日本語](README.ja.md) · [한국어](README.ko.md) · [Deutsch](README.de.md) · [Français](README.fr.md) · [Español](README.es.md) · [Português](README.pt.md) · [Türkçe](README.tr.md) · [Bahasa Indonesia](README.id.md) · [ไทย](README.th.md) · [Tiếng Việt](README.vi.md) · [العربية](README.ar.md) · [فارسی](README.fa.md)

## [USER-GUIDE.ru.md](./USER-GUIDE.ru.md) — Руководство пользователя

Полное руководство по эксплуатации для **пользователей** (операторов/администраторов/операторов): установка, первоначальная настройка, функциональные модули, часто задаваемые вопросы и приложение.

## [api/](./api/) — общедоступная ссылка на API.

Для интеграторов, вторичной разработки и сторонних клиентов:

- [api/README.ru.md](./api/README.ru.md) — обзор: база URL, аутентификация, формат ответа, ограничения скорости
- [api/subscription.ru.md](./api/subscription.ru.md) — конечная точка подписки (`/sub/:token`), форматы, заголовки ответов, примеры
- [api/portal-api.ru.md](./api/portal-api.ru.md) — клиентский портал API (регистрация/вход, планы, заказы, баланс, билеты)
- [api/charge-callback.ru.md](./api/charge-callback.ru.md) — подпись обратного платежа и проверка
- [api/admin-api.ru.md](./api/admin-api.ru.md) — таблица маршрутов администратора API, сгруппированная по модулям

## Проектная документация

Проектные документы являются внутренними материалами (основная интеллектуальная собственность) и **не распространяются** вместе с этим репозиторием. Если они вам нужны, свяжитесь с сопровождающим.

> **Объем**: клиентское приложение (локальный пользовательский интерфейс, управление TUN, лоток, объединение нескольких подписок) является отдельным продуктом и выходит за рамки этого репозитория. Документация здесь охватывает только серверную часть (панель/узлы/доставка подписки).

## Другие руководства по эксплуатации

- `cert-google-eab.md` — Google EAB Выдача сертификата *(китайский)*
- `oauth-google-verification.md` — Google Проверка OAuth *(китайский)*
- `notification-templates.md` — переменные шаблона уведомления *(китайский)*

## Охват перевода

| Документ | zh-CN | en | zh-TW | Другие языки |
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

См. [I18N.md](README.ru.md) для получения информации об именовании, языковой навигации и правилах просмотра *(китайский)*.
