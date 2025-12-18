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

## Сборка из исходников
```bash
go mod download
go build -o dist/waiterd ./cmd
```

## Тесты
```bash
go test ./...
```

