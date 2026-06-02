package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ServerVersion            string
	ListenAddr               string
	DatabaseURL              string
	APIToken                 string
	RuntimeControllerEnabled bool
	RuntimeAccessEnabled     bool
	RuntimeReconcileInterval time.Duration
	KubeconfigPath           string
	KubeContext              string
	AgentSandboxWarmPool     string
	ArtifactContentBackend   string
	ArtifactContentDir       string
	ArtifactContentS3        ArtifactContentS3Config
	TrustedPrincipalHeaders  TrustedPrincipalHeaderConfig
	ProjectRBAC              ProjectRBACConfig
}

type ArtifactContentS3Config struct {
	Endpoint        string
	Region          string
	Bucket          string
	Prefix          string
	AccessKeyID     string
	SecretAccessKey string
	ForcePathStyle  bool
}

type TrustedPrincipalHeaderConfig struct {
	Enabled             bool
	PrincipalHeader     string
	PrincipalTypeHeader string
}

type ProjectRBACConfig struct {
	EnforcementEnabled bool
}

func Load() (Config, error) {
	cfg := Config{
		ServerVersion:            envDefault("MBOX_SERVER_VERSION", "0.1.0-dev"),
		ListenAddr:               envDefault("MBOX_LISTEN_ADDR", "127.0.0.1:18080"),
		DatabaseURL:              os.Getenv("DATABASE_URL"),
		APIToken:                 strings.TrimSpace(os.Getenv("MBOX_API_TOKEN")),
		RuntimeControllerEnabled: envBool("MBOX_RUNTIME_CONTROLLER_ENABLED", false),
		RuntimeAccessEnabled:     envBool("MBOX_RUNTIME_ACCESS_ENABLED", false),
		RuntimeReconcileInterval: envDuration("MBOX_RUNTIME_RECONCILE_INTERVAL", 5*time.Second),
		KubeconfigPath:           os.Getenv("MBOX_KUBECONFIG"),
		KubeContext:              os.Getenv("MBOX_KUBE_CONTEXT"),
		AgentSandboxWarmPool:     os.Getenv("MBOX_AGENT_SANDBOX_WARM_POOL"),
		ArtifactContentBackend:   envDefault("MBOX_ARTIFACT_CONTENT_BACKEND", "postgres"),
		ArtifactContentDir:       os.Getenv("MBOX_ARTIFACT_CONTENT_DIR"),
		TrustedPrincipalHeaders: TrustedPrincipalHeaderConfig{
			Enabled:             envBool("MBOX_TRUSTED_PRINCIPAL_HEADERS_ENABLED", false),
			PrincipalHeader:     envDefaultTrimmed("MBOX_TRUSTED_PRINCIPAL_HEADER", "X-Mbox-Principal"),
			PrincipalTypeHeader: envDefaultTrimmed("MBOX_TRUSTED_PRINCIPAL_TYPE_HEADER", "X-Mbox-Principal-Type"),
		},
		ProjectRBAC: ProjectRBACConfig{
			EnforcementEnabled: envBool("MBOX_PROJECT_RBAC_ENFORCEMENT_ENABLED", false),
		},
		ArtifactContentS3: ArtifactContentS3Config{
			Endpoint:        strings.TrimSpace(os.Getenv("MBOX_ARTIFACT_CONTENT_S3_ENDPOINT")),
			Region:          envDefault("MBOX_ARTIFACT_CONTENT_S3_REGION", "us-east-1"),
			Bucket:          strings.TrimSpace(os.Getenv("MBOX_ARTIFACT_CONTENT_S3_BUCKET")),
			Prefix:          strings.TrimSpace(os.Getenv("MBOX_ARTIFACT_CONTENT_S3_PREFIX")),
			AccessKeyID:     strings.TrimSpace(os.Getenv("MBOX_ARTIFACT_CONTENT_S3_ACCESS_KEY_ID")),
			SecretAccessKey: strings.TrimSpace(os.Getenv("MBOX_ARTIFACT_CONTENT_S3_SECRET_ACCESS_KEY")),
			ForcePathStyle:  envBool("MBOX_ARTIFACT_CONTENT_S3_FORCE_PATH_STYLE", true),
		},
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	cfg.TrustedPrincipalHeaders.PrincipalHeader = strings.TrimSpace(cfg.TrustedPrincipalHeaders.PrincipalHeader)
	cfg.TrustedPrincipalHeaders.PrincipalTypeHeader = strings.TrimSpace(cfg.TrustedPrincipalHeaders.PrincipalTypeHeader)
	if cfg.TrustedPrincipalHeaders.Enabled &&
		(cfg.TrustedPrincipalHeaders.PrincipalHeader == "" || cfg.TrustedPrincipalHeaders.PrincipalTypeHeader == "") {
		return Config{}, fmt.Errorf("trusted principal header names are required when MBOX_TRUSTED_PRINCIPAL_HEADERS_ENABLED=true")
	}
	if cfg.ProjectRBAC.EnforcementEnabled && !cfg.TrustedPrincipalHeaders.Enabled {
		return Config{}, fmt.Errorf("MBOX_PROJECT_RBAC_ENFORCEMENT_ENABLED requires MBOX_TRUSTED_PRINCIPAL_HEADERS_ENABLED=true")
	}
	cfg.ArtifactContentBackend = strings.ToLower(strings.TrimSpace(cfg.ArtifactContentBackend))
	switch cfg.ArtifactContentBackend {
	case "postgres":
	case "filesystem":
		if strings.TrimSpace(cfg.ArtifactContentDir) == "" {
			cfg.ArtifactContentDir = ".mbox/artifacts"
		}
	case "s3":
		if cfg.ArtifactContentS3.Endpoint == "" {
			return Config{}, fmt.Errorf("MBOX_ARTIFACT_CONTENT_S3_ENDPOINT is required when MBOX_ARTIFACT_CONTENT_BACKEND=s3")
		}
		if cfg.ArtifactContentS3.Bucket == "" {
			return Config{}, fmt.Errorf("MBOX_ARTIFACT_CONTENT_S3_BUCKET is required when MBOX_ARTIFACT_CONTENT_BACKEND=s3")
		}
		if cfg.ArtifactContentS3.AccessKeyID == "" || cfg.ArtifactContentS3.SecretAccessKey == "" {
			return Config{}, fmt.Errorf("MBOX_ARTIFACT_CONTENT_S3_ACCESS_KEY_ID and MBOX_ARTIFACT_CONTENT_S3_SECRET_ACCESS_KEY are required when MBOX_ARTIFACT_CONTENT_BACKEND=s3")
		}
	default:
		return Config{}, fmt.Errorf("MBOX_ARTIFACT_CONTENT_BACKEND must be postgres, filesystem, or s3")
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

func envDefaultTrimmed(name string, fallback string) string {
	value, ok := os.LookupEnv(name)
	if !ok {
		return fallback
	}
	return strings.TrimSpace(value)
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
