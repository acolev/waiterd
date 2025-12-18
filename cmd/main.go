package main

import (
	"github.com/joho/godotenv"
	"log"
	"waiterd/internal/config"
	"waiterd/pkg/cfg"
	"waiterd/pkg/logger"
)

func main() {
	_ = godotenv.Load()

	env := cfg.String("WAITERD_ENV", "dev")

	cleanup := logger.Setup(env)
	defer cleanup()

	configPath := cfg.String("WAITERD_CONFIG", "config.yaml")

	conf, err := config.Build(configPath)
	if err != nil {
		log.Fatalf("failed to build config: %v", err)
	}

	// Печатаем полный конфиг в человеко-читаемом YAML-формате
	if pretty, err := conf.Pretty(); err == nil {
		log.Printf("Starting waiterd with config:\n%s", pretty)
	} else {
		log.Println("Starting waiterd with config:", conf)
	}

}
