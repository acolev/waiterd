package config

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Version   string     `yaml:"version" env-default:"v1"`
	Gateway   Gateway    `yaml:"gateway"`
	Cache     Cache      `yaml:"cache"`
	Services  []Service  `yaml:"services"`
	Endpoints []Endpoint `yaml:"endpoints"`
	Includes  []string   `yaml:"includes"`
}

type Gateway struct {
	Address            string `yaml:"address"              env:"GATEWAY_ADDR"              env-default:":"`
	Timeout            string `yaml:"timeout"              env:"GATEWAY_TIMEOUT"           env-default:"30s"`
	ReadTimeoutSec     int    `yaml:"read_timeout_sec"     env:"GATEWAY_READ_TIMEOUT"      env-default:"15"`
	WriteTimeoutSec    int    `yaml:"write_timeout_sec"    env:"GATEWAY_WRITE_TIMEOUT"     env-default:"15"`
	IdleTimeoutSec     int    `yaml:"idle_timeout_sec"     env:"GATEWAY_IDLE_TIMEOUT"      env-default:"60"`
	ShutdownTimeoutSec int    `yaml:"shutdown_timeout_sec" env:"GATEWAY_SHUTDOWN_TIMEOUT"  env-default:"15"`
}

type Cache struct {
	Driver string `yaml:"driver"    env:"CACHE_DRIVER"    env-default:"memory"`
	Host   string `yaml:"host"      env:"CACHE_HOST"      env-default:"localhost"`
	Port   int    `yaml:"port"      env:"CACHE_PORT"      env-default:"6379"`
	Db     int    `yaml:"db"        env:"CACHE_DB"        env-default:"0"`
	Pass   string `yaml:"password"  env:"CACHE_PASSWORD"  env-default:""`
	TTL    string `yaml:"ttl" env:"CACHE_TTL" env-default:""`
}

type Service struct {
	Name      string `yaml:"name"`
	ProxyURL  string `yaml:"proxy_url"`
	Timeout   string `yaml:"timeout"`
	Transport string `yaml:"transport,omitempty"`
}

type Endpoint struct {
	Path            string            `yaml:"path"`
	Method          string            `yaml:"method"`
	Backend         *Backend          `yaml:"backend,omitempty"`
	Calls           []AggCall         `yaml:"calls,omitempty"`
	ResponseMapping map[string]string `yaml:"response_mapping,omitempty"`
	AuthRequired    bool              `yaml:"auth_required,omitempty"`
	FailOnError     *bool             `yaml:"fail_on_error,omitempty"`
	CacheTTL        string            `yaml:"cache_ttl,omitempty"`
	Middlewares     []string          `yaml:"middlewares,omitempty"`
}

type Backend struct {
	Service string `yaml:"service"`
	Path    string `yaml:"path"`
	Method  string `yaml:"method"`
}

type AggCall struct {
	Name    string            `yaml:"name"`    // ключ в итоговом JSON
	Service string            `yaml:"service"` // имя сервиса
	Path    string            `yaml:"path"`
	Method  string            `yaml:"method"`
	Mapping map[string]string `yaml:"mapping,omitempty"` // { "title": "title", "body": "body" }
}

type FinalConfig struct {
	Gateway   Gateway
	Cache     Cache
	Services  []Service
	Endpoints []Endpoint
}

func Load(pathOrContent string) (*Config, error) {
	var cfg Config

	// Если указанный путь существует как файл — читаем его
	if fi, err := os.Stat(pathOrContent); err == nil && !fi.IsDir() {
		if err := cleanenv.ReadConfig(pathOrContent, &cfg); err != nil {
			return nil, fmt.Errorf("read config %q: %w", pathOrContent, err)
		}
	} else {
		// иначе считаем, что передана сама YAML-конфигурация в строке
		// признаками inline-конфига считаем наличие перевода строки или ключевых YAML-полей
		maybeContent := pathOrContent
		if strings.Contains(maybeContent, "\n") || strings.Contains(maybeContent, "gateway:") || strings.Contains(maybeContent, "services:") {
			if err := yaml.Unmarshal([]byte(maybeContent), &cfg); err != nil {
				return nil, fmt.Errorf("parse config content: %w", err)
			}
		} else {
			// если это не файл и не явный YAML по признакам — пробуем всё-таки прочитать как файл по относительному пути
			abs := pathOrContent
			if !filepath.IsAbs(abs) {
				abs = filepath.Join(".", abs)
			}
			if err := cleanenv.ReadConfig(abs, &cfg); err != nil {
				return nil, fmt.Errorf("read config %q: %w", pathOrContent, err)
			}
		}
	}

	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return nil, fmt.Errorf("read env: %w", err)
	}

	return &cfg, nil
}

func Build(configPath string) (*FinalConfig, error) {
	raw, err := Load(configPath)
	if err != nil {
		return nil, err
	}

	// исходно — то, что есть в самом файле
	services := append([]Service{}, raw.Services...)
	endpoints := append([]Endpoint{}, raw.Endpoints...)

	// добавляем из includes, если указаны
	if len(raw.Includes) > 0 && raw.Version != "v1" {
		incServices, incEndpoints, err := loadIncludes(configPath, raw.Includes)
		if err != nil {
			return nil, err
		}
		services = append(services, incServices...)
		endpoints = append(endpoints, incEndpoints...)
	}

	return &FinalConfig{
		Gateway:   raw.Gateway,
		Cache:     raw.Cache,
		Services:  services,
		Endpoints: endpoints,
	}, nil
}

func loadIncludes(mainPath string, patterns []string) ([]Service, []Endpoint, error) {
	baseDir := filepath.Dir(mainPath)

	env := os.Getenv("WAITERD_ENV")
	if env == "" {
		env = "dev"
	}

	var allServices []Service
	var allEndpoints []Endpoint

	for _, pat := range patterns {
		// подставляем {env}
		pat = strings.ReplaceAll(pat, "{env}", env)

		glob := pat
		if !filepath.IsAbs(glob) {
			glob = filepath.Join(baseDir, glob)
		}

		matches, err := filepath.Glob(glob)
		if err != nil {
			return nil, nil, fmt.Errorf("glob %q: %w", pat, err)
		}

		for _, file := range matches {
			data, err := os.ReadFile(file)
			if err != nil {
				return nil, nil, fmt.Errorf("read included %q: %w", file, err)
			}

			var partial struct {
				Services  []Service  `yaml:"services"`
				Endpoints []Endpoint `yaml:"endpoints"`
			}

			if err := yaml.Unmarshal(data, &partial); err != nil {
				return nil, nil, fmt.Errorf("parse included %q: %w", file, err)
			}

			if len(partial.Services) > 0 {
				allServices = append(allServices, partial.Services...)
			}
			if len(partial.Endpoints) > 0 {
				allEndpoints = append(allEndpoints, partial.Endpoints...)
			}
		}
	}

	return allServices, allEndpoints, nil
}

// Pretty возвращает YAML-представление FinalConfig для простого логирования.
func (fc *FinalConfig) Pretty() (string, error) {
	b, err := yaml.Marshal(fc)
	if err != nil {
		return "", fmt.Errorf("marshal config: %w", err)
	}
	return string(b), nil
}
