package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuild_ConfigV1(t *testing.T) {
	t.Setenv("APP_ENV", "dev")

	cfgPath := filepath.Join(repoRoot(t), "example", "config.v1.yaml")
	conf, err := Build(cfgPath)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	if conf.Gateway.Address != ":8080" {
		t.Fatalf("unexpected gateway address: %q", conf.Gateway.Address)
	}

	expectedServices := 2 // test + jsonp
	if len(conf.Services) != expectedServices {
		t.Fatalf("expected %d services, got %d", expectedServices, len(conf.Services))
	}

	expectedEndpoints := 2
	if len(conf.Endpoints) != expectedEndpoints {
		t.Fatalf("expected %d endpoints, got %d", expectedEndpoints, len(conf.Endpoints))
	}
}

func TestBuild_ConfigV2_WithIncludes(t *testing.T) {
	t.Setenv("APP_ENV", "dev")

	cfgPath := filepath.Join(repoRoot(t), "example", "config.v2.yaml")
	conf, err := Build(cfgPath)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	if conf.Gateway.Address != ":8080" {
		t.Fatalf("unexpected gateway address: %q", conf.Gateway.Address)
	}

	expectedServices := []string{"test", "jsonp"}
	if len(conf.Services) != len(expectedServices) {
		t.Fatalf("expected %d services, got %d", len(expectedServices), len(conf.Services))
	}
	for _, name := range expectedServices {
		if !serviceExists(conf.Services, name) {
			t.Fatalf("expected service %q to be loaded from includes", name)
		}
	}

	expectedEndpoints := []string{"/api/test", "/posts/{id}"}
	if len(conf.Endpoints) != len(expectedEndpoints) {
		t.Fatalf("expected %d endpoints, got %d", len(expectedEndpoints), len(conf.Endpoints))
	}
	for _, path := range expectedEndpoints {
		if !endpointExists(conf.Endpoints, path) {
			t.Fatalf("expected endpoint %q to be loaded from includes", path)
		}
	}
}

func serviceExists(services []Service, name string) bool {
	for _, s := range services {
		if s.Name == name {
			return true
		}
	}
	return false
}

func endpointExists(endpoints []Endpoint, path string) bool {
	for _, e := range endpoints {
		if e.Path == path {
			return true
		}
	}
	return false
}

func repoRoot(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}

		parent := filepath.Dir(wd)
		if parent == wd {
			t.Fatal("go.mod not found from test cwd upward")
		}
		wd = parent
	}
}
