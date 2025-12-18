package logger

import (
	"log"
	"os"
	"path/filepath"
)

func Setup(env string) func() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)

	if env != "prod" {
		log.SetOutput(os.Stdout)
		return func() {}
	}

	logDir := "logs"
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		log.Printf("[logger] failed to create log dir, fallback to stdout: %v", err)
		log.SetOutput(os.Stdout)
		return func() {}
	}

	logPath := filepath.Join(logDir, "waiterd.log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("[logger] failed to open log file, fallback to stdout: %v", err)
		log.SetOutput(os.Stdout)
		return func() {}
	}

	log.SetOutput(f)
	// возвращаем функцию, которая закроет файл при завершении
	return func() {
		_ = f.Close()
	}
}
