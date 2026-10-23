<!-- source: oauth-google-verification.md | locale: ru | updated: 2026-10-23 -->
# Google Руководство по проверке OAuth

> Языки: [简体中文](oauth-google-verification.md) · [English](oauth-google-verification.en.md) · [繁體中文](oauth-google-verification.zh-TW.md) · Русский · [日本語](oauth-google-verification.ja.md) · [한국어](oauth-google-verification.ko.md) · [Deutsch](oauth-google-verification.de.md) · [Français](oauth-google-verification.fr.md) · [Español](oauth-google-verification.es.md) · [Português](oauth-google-verification.pt.md) · [Türkçe](oauth-google-verification.tr.md) · [Bahasa Indonesia](oauth-google-verification.id.md) · [ไทย](oauth-google-verification.th.md) · [Tiếng Việt](oauth-google-verification.vi.md) · [العربية](oauth-google-verification.ar.md) · [فارسی](oauth-google-verification.fa.md)

> Область: вход Google на портал CoreFusion (`internal/service/oauth`, области: `openid`, `email`, `profile`).
> Сравнение GitHub в конце (GitHub не имеет потока проверки).

## 1. Тестирование против производственного режима

Экран согласия OAuth Google имеет два состояния публикации:

| Государство | Кто может войти в систему | Требуется проверка |
|---|---|---|
| **Тестирование** | В списке «тестовых пользователей» только аккаунты Google. | Нет |
| **В производстве** | Любой аккаунт Google | **Да** (см. ниже) |

После включения «Системные настройки → OAuth» на странице входа в портал каждому посетителю отображается кнопка Google;
если приложение все еще находится в стадии тестирования, посетители, не входящие в список тестовых пользователей, увидят
`Error 403: access_denied` (приложение не проверено/вы не тестовый пользователь).
Чтобы предложить общедоступный вход в систему с помощью Google, **переключите приложение в рабочий режим и пройдите проверку**.

## 2. Жесткие требования к верификации

Для проверки Google (базовая проверка для неограниченных областей применения) требуется:

1. **Домашняя страница**: общедоступная страница URL в авторизованном домене (тот же домен или поддомен).
2. **Ссылка на Политику конфиденциальности**: должна быть **общедоступной** (без входа в систему) и объяснять, какие данные вы собираете и как вы их используете/храните/удаляете. Для входа в систему Google он должен как минимум охватывать:
 собранные данные учетной записи Google (адрес электронной почты, имя, аватар), цель (создание/вход в учетную запись портала), отсутствие продажи третьим лицам,
 и как пользователи могут удалить свою учетную запись и данные.
3. **Ссылка на Условия использования**: общедоступная, с описанием правил сервиса.
4. **Авторизованные домены**: зарегистрируйте домен приложения на экране согласия (например, `example.com`); Ссылки home/privacy/terms должны находиться на нем или на субдомене.
5. **Логотип приложения (необязательно, но рекомендуется)**: 120×120 пикселей.
6. **Обоснование объема**: в этом проекте используется только `openid / email / profile` (неограниченный, неконфиденциальный),
 которая подпадает под базовую проверку — без обоснования по каждому объему и **никакой сторонней оценки безопасности**.
 (Они потребуются для ограниченных областей, таких как полный доступ к Gmail; в этом проекте они не используются.)

Запись для подтверждения: [Google Cloud Console](https://console.cloud.google.com/) →
**API и службы → Экран согласия OAuth → Опубликовать приложение → Отправить на проверку**.
Базовая проверка обычно завершается в течение нескольких дней; Пока вы можете работать в тестовом режиме.

## 3. Что настроить для этого проекта

### 3.1 Авторизованное перенаправление URI

Обратный вызов генерируется динамически из домена, с которого начинается вход в систему (см. `Authorize` в `internal/api/handlers/oauth/handler.go`):

```
https://<portal-domain>/api/v1/client-auth/oauth/google/callback
```

- Вход начат с домена портала (например `https://p.example.com`) → зарегистрируйте `https://p.example.com/...`;
- Начал с панели домена `/p` → прописать `https://panel.example.com/...`.
- Google допускает **несколько** URI перенаправления; зарегистрировать оба домена.
- ⚠ Google требует, чтобы URI перенаправления были **https** (кроме `http://localhost`);
 обратный прокси должен пересылать `X-Forwarded-Host`/`X-Forwarded-Proto`, иначе панель создаст
 http или внутреннее redirect_uri → Google отклоняет его (`redirect_uri_mismatch`).

После настройки «Системные настройки → OAuth → Домен обратного вызова портала» обратный вызов URL, показанный на странице, — это тот обратный вызов, который нужно вставить в Google.

### 3.2 Политика конфиденциальности/Условия ссылки на портале

Карточка для входа содержит ссылки «Политика конфиденциальности · Условия использования» (отображаются после настройки), и оба документа имеют **отдельные страницы**.
(диалоговое окно и отдельная страница имеют одинаковое содержимое):

- Домен главной панели: `https://<panel-domain>/p/legal/privacy`, `/p/legal/terms`.
- Выделенный домен портала (корневое развертывание): `https://<portal-domain>/legal/privacy`, `/legal/terms`.
- Поддерживает `?lang=xx`; без него используется `Accept-Language` браузера, затем портал по умолчанию.
- Страницы **отрисовываются на сервере** (без JS), что помогает сканеру проверки Google.

**Конфигурация (в любом случае; ссылки выигрывают)**:

1. **Режим контента (рекомендуется, работает «из коробки»)**: редактор портала → Настройки → Общие → Соответствующие документы →
 выберите «Редактировать язык» и вставьте шаблоны §4 в «Содержимое политики конфиденциальности» / «Содержимое условий обслуживания».
 (**Уценка, редактируется для каждого языка**) → после сохранения на карточке входа отображаются ссылки, которые открывают **диалоговое окно**;
 «Отдельную ссылку» редактора можно ввести непосредственно на экране согласия Google.
2. **Режим внешней ссылки**: введите «Политика конфиденциальности URL» / «Условия обслуживания URL» (указывая на страницы, которые вы размещаете) → откроется в новой вкладке.

Контент и ссылки доставляются через `/api/v1/public/portal-config` (`privacy_policy_content` /
`terms_of_service_content` — **карты Markdown для каждого языка** `{"zh-CN":"...","en":"..."}`;
`privacy_policy_url`/`terms_of_service_url` — необязательные внешние ссылки). **Они должны быть доступны без входа в систему** ✓.

### 3.3 Переключатель на панели

`System settings → OAuth`: включите Google и введите идентификатор клиента/секрет клиента (неактивный секрет зашифрован с помощью AES-256-GCM).
Кнопка Google появляется на странице входа в портал только тогда, когда все настроено (`/api/v1/client-auth/oauth/providers` возвращается включенным).

## 4. Политика конфиденциальности/шаблоны условий

> Минимальные шаблоны соответствия, которые можно адаптировать. Перед использованием: замените заполнители `【】`, добавьте/удалите предложения для вашего бизнеса,
> и поручить эксплуатационному органу рассмотреть их. Эти шаблоны не являются юридической консультацией.

### 4.1 Политика конфиденциальности (шаблон)

```markdown
# Privacy Policy

Last updated: [DATE]

[OPERATOR NAME] ("we") operates [PORTAL NAME] ("the Service"). We respect your privacy;
this policy explains what information we collect and how we use it.

## 1. Information we collect
- **Account information**: the email address and password you provide at registration (stored only as a one-way hash);
- **Google sign-in information**: when you choose "Sign in with Google", we receive, with your authorization,
  your Google account email, display name and avatar URL (scopes: openid, email, profile);
- **Service data**: subscription configuration, traffic usage, device IP allow-list, tickets and order records.

## 2. How we use information
Only to: create and sign in to your account, provide the proxy subscription service, bill you and provide support,
and for security controls (sign-in protection, IP allow-list). We **do not** sell your personal information.

## 3. Third-party services
- **Google**: receives account information within the authorized scope only when you actively use "Sign in with Google";
- Payment providers: receive order information only when you initiate a payment.

## 4. Storage and security
Data is stored on our servers; credentials are encrypted at rest and all traffic uses TLS.

## 5. Your rights
You can update your profile/password and end sessions in the portal at any time; to delete your account and all data,
contact [CONTACT EMAIL] and we will process the request within [N] business days.

## 6. Policy changes
Material changes will be announced in the portal notice board.
```

### 4.2 Условия обслуживания (шаблон)

```markdown
# Terms of Service

Last updated: [DATE]

## 1. Service
The Service provides [PROXY / NETWORK ACCESS] to registered users; specific plans are described on the store page.

## 2. Accounts
- You are responsible for the security of your account and password; for accounts created with Google sign-in, credentials are managed by Google;
- You may not lend or transfer your account, or use the Service for any unlawful purpose (including but not limited to:
  unauthorized access to others' systems, distributing illegal content, or circumventing mandatory legal requirements).

## 3. Payment and refunds
Plans are billed [monthly / by usage]; traffic and device limits follow the plan description;
[REFUND POLICY: e.g. full refund within 7 days if unused].

## 4. Changes and termination
We may suspend or terminate the Service due to force majeure or your breach of these terms; deleting an account deletes the related data.

## 5. Disclaimer
The Service is provided "as is"; to the extent permitted by law, we are not liable for indirect losses caused by use of the Service.

## 6. Miscellaneous
These terms are governed by the laws of [JURISDICTION]. Contact: [CONTACT EMAIL].
```

## 5. Контрольный список перед запуском

- [ ] Экран согласия OAuth: имя приложения, логотип, домашняя страница, политика конфиденциальности, условия — все присутствует; домен зарегистрирован
- [ ] Страницы «Политика конфиденциальности» и «Условия» **доступны без входа** (пользовательская страница портала или отдельная статическая страница)
- [ ] Обе ссылки видны на карточке входа/нижнем колонтитуле портала.
- [ ] редирект URI соответствует фактическому домену и имеет вид https.
- [ ] Опубликовать приложение → отправить на проверку; ограничение для тестовых пользователей снимается после одобрения
- [ ] Настройки OAuth панели: включено = true с заполненным идентификатором клиента/секретом (возвраты конечной точки поставщика включены: true)

## 6. Сравнение GitHub

GitHub Приложения OAuth не имеют состояния публикации или потока проверки **нет**: создайте приложение, установите обратный вызов URL, и оно будет работать публично.
Обратите внимание, что приложение OAuth GitHub принимает только **один** обратный вызов авторизации URL (соответствие префикса); для нескольких доменов,
используйте подход с несколькими URI из Google или создайте отдельное приложение для каждого домена портала.
