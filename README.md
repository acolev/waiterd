# waiterd

Шлюз на Fiber с поддержкой backend-прокси, агрегирования и кэша (Redis или in-memory).

## Установка бинаря (релиз GitHub)

Для macOS (arm64):
```bash
curl -L https://github.com/acolev/waiterd/releases/latest/download/waiterd-darwin-arm64 -o /usr/local/bin/waiterd
chmod +x /usr/local/bin/waiterd
waiterd --help
```

Другие сборки доступны по названию файла `waiterd-<goos>-<goarch>`:
- macOS amd64: `waiterd-darwin-amd64`
- Linux amd64: `waiterd-linux-amd64`
- Linux arm64: `waiterd-linux-arm64`

Пример для Linux amd64:
```bash
curl -L https://github.com/acolev/waiterd/releases/latest/download/waiterd-linux-amd64 -o /usr/local/bin/waiterd
chmod +x /usr/local/bin/waiterd
waiterd --help
```

## Конфиг и окружение
- Основной YAML-конфиг: см. примеры `example/config.v1.yaml` и `example/config.v2.yaml`. Скопируй нужный: `cp example/config.v2.yaml config.yaml` (или v1).
- Перекрытия через ENV (см. `internal/config/config.go`):
  - `GATEWAY_ADDR` (по умолчанию `:` → 0.0.0.0:80). Пример: `:8080`.
  - Кэш: `CACHE_DRIVER=memory|redis`.
    - `memory` — в памяти процесса (удобно локально/в тестах, без Redis).
    - `redis` — общий кэш через Redis.
    Для Redis: `CACHE_HOST`, `CACHE_PORT`, `CACHE_DB`, `CACHE_PASSWORD`, `CACHE_TTL`.
  - Таймауты сервера: `GATEWAY_READ_TIMEOUT`, `GATEWAY_WRITE_TIMEOUT`, `GATEWAY_IDLE_TIMEOUT`, `GATEWAY_SHUTDOWN_TIMEOUT`.
  - Инклюды по окружению: `WAITERD_ENV=dev|prod` — подставляет `{env}` в includes (по умолчанию `dev`).
- Запуск с файлом: `waiterd --config config.yaml`.
- Запуск без файла (inline): `waiterd --config "$(cat example/config.v2.yaml)"`.
- `.env` не обязателен: без ENV возьмёт YAML и дефолты (адрес `:`). Кэш включается только если задан `cache.ttl` или `cache_ttl` у endpoint.

## Debug endpoints

`/debug/config` **включён только при `APP_ENV=dev`** (в остальных окружениях маршрут не регистрируется и будет 404).
Это сделано намеренно, чтобы не утекали чувствительные данные из конфига (например, пароль Redis).

## Запуск локально
```bash
cp example/config.v2.yaml config.yaml   # или v1
WAITERD_ENV=dev \                        # если в конфиге есть includes с {env}, подставится .dev/.prod и т.п. (по умолчанию dev)
GATEWAY_ADDR=:8080 \                    # при необходимости
./dist/waiterd --config config.yaml
```

## Сборка из исходников
```bash
go mod download
go build -o dist/waiterd ./cmd
```

## Тесты
```bash
go test ./...
```
