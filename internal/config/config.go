package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	ListenAddr               string
	DatabaseURL              string
	RuntimeControllerEnabled bool
	RuntimeReconcileInterval time.Duration
	KubeconfigPath           string
	KubeContext              string
	AgentSandboxWarmPool     string
}

func Load() (Config, error) {
	cfg := Config{
		ListenAddr:               envDefault("MBOX_LISTEN_ADDR", "127.0.0.1:8080"),
		DatabaseURL:              os.Getenv("DATABASE_URL"),
		RuntimeControllerEnabled: envBool("MBOX_RUNTIME_CONTROLLER_ENABLED", false),
		RuntimeReconcileInterval: envDuration("MBOX_RUNTIME_RECONCILE_INTERVAL", 5*time.Second),
		KubeconfigPath:           os.Getenv("MBOX_KUBECONFIG"),
		KubeContext:              os.Getenv("MBOX_KUBE_CONTEXT"),
		AgentSandboxWarmPool:     os.Getenv("MBOX_AGENT_SANDBOX_WARM_POOL"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	return cfg, nil
}

func envDefault(name string, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	return value
}

func envBool(name string, fallback bool) bool {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDuration(name string, fallback time.Duration) time.Duration {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
