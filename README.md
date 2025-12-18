# waiterd

Шлюз на Fiber с поддержкой backend-прокси, агрегирования и кэша (Redis/memory).

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
- Основной конфиг: YAML (v1/v2). Примеры: `example/config.v1.yaml`, `example/config.v2.yaml`. Скопируй нужный вариант рядом с бинарём, например `cp example/config.v2.yaml config.yaml`.
- ENV переменные перекрывают YAML (см. `internal/config/config.go`). Основные:
  - `GATEWAY_ADDR` — адрес слушателя (по умолчанию `:` → 0.0.0.0:80 в Fiber). Можно задать `:8080`.
  - `CACHE_DRIVER` — `memory` или `redis` (по умолчанию memory/выключен). Для Redis также: `CACHE_HOST`, `CACHE_PORT`, `CACHE_DB`, `CACHE_PASSWORD`, `CACHE_TTL`.
  - `GATEWAY_READ_TIMEOUT`, `GATEWAY_WRITE_TIMEOUT`, `GATEWAY_IDLE_TIMEOUT`, `GATEWAY_SHUTDOWN_TIMEOUT` — таймауты сервера.
- Если не хочешь копировать пример: можно запускать с `--config` inline (поддерживается `config.Build`): `waiterd --config="$(cat example/config.v2.yaml)"` или указать путь к файлу.
- Без `.env`/ENV всё равно стартует: возьмёт значения из YAML и дефолты (адрес `:`, cache TTL 0 → кеш отключён).

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
