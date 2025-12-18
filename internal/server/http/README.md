# HTTP cache notes

## Что кешируется

### Proxy (backend)
- Кешируется ответ upstream как структура: **status + безопасный allow-list заголовков + body**.
- По умолчанию кешируем только **GET/HEAD**. Для POST/PUT/PATCH/DELETE кеш выключен, чтобы избежать коллизий (ключ не включает тело).

### Aggregate
- Кешируется финальный JSON-ответ агрегации.

## Ключ кеша

Сейчас ключ: `METHOD:OriginalURL`.
- Для GET/HEAD этого достаточно.
- Для небезопасных методов (POST/PUT/...) ключ должен включать хэш тела/заголовков — пока мы это не делаем и просто не кешируем такие запросы.

## Заголовки

В proxy-режиме на cache-hit мы восстанавливаем allow-list:
- Content-Type
- Cache-Control
- ETag
- Last-Modified
- Vary
- Content-Language

`Set-Cookie` умышленно не кешируем (может быть опасно для разных пользователей).

## Форвардинг заголовков (aggregation)

В режиме aggregate мы прокидываем в downstream вызовы часть заголовков из входящего запроса:
- Authorization
- X-Request-Id
- Accept
- Content-Type
- User-Agent
- X-Forwarded-For
- X-Real-IP

Это приближает поведение к реальному API-gateway.

## Auth / middlewares

Поля `auth_required` и `middlewares: ["auth"]` уже есть в модели конфига, но как механизм пока **не реализованы**.
Сейчас они рассматриваются как «задел на будущее», чтобы подключить middleware-цепочку на уровне router/endpoint.
