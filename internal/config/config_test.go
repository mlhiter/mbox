package config

import (
	"strings"
	"testing"
)

func TestLoadTrustedPrincipalHeadersDefaultDisabled(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://mbox:mbox@127.0.0.1:5432/mbox?sslmode=disable")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TrustedPrincipalHeaders.Enabled {
		t.Fatalf("expected trusted principal headers disabled by default: %+v", cfg.TrustedPrincipalHeaders)
	}
	if cfg.TrustedPrincipalHeaders.PrincipalHeader != "X-Mbox-Principal" ||
		cfg.TrustedPrincipalHeaders.PrincipalTypeHeader != "X-Mbox-Principal-Type" {
		t.Fatalf("unexpected trusted principal header defaults: %+v", cfg.TrustedPrincipalHeaders)
	}
}

func TestLoadTrustedPrincipalHeadersRequiresNamesWhenEnabled(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://mbox:mbox@127.0.0.1:5432/mbox?sslmode=disable")
	t.Setenv("MBOX_TRUSTED_PRINCIPAL_HEADERS_ENABLED", "true")
	t.Setenv("MBOX_TRUSTED_PRINCIPAL_HEADER", " ")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "trusted principal header names are required") {
		t.Fatalf("expected trusted principal header validation error, got %v", err)
	}
}

func TestLoadTrustedPrincipalHeadersReadsOverrides(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://mbox:mbox@127.0.0.1:5432/mbox?sslmode=disable")
	t.Setenv("MBOX_TRUSTED_PRINCIPAL_HEADERS_ENABLED", "true")
	t.Setenv("MBOX_TRUSTED_PRINCIPAL_HEADER", "X-Forwarded-User")
	t.Setenv("MBOX_TRUSTED_PRINCIPAL_TYPE_HEADER", "X-Forwarded-Principal-Type")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.TrustedPrincipalHeaders.Enabled ||
		cfg.TrustedPrincipalHeaders.PrincipalHeader != "X-Forwarded-User" ||
		cfg.TrustedPrincipalHeaders.PrincipalTypeHeader != "X-Forwarded-Principal-Type" {
		t.Fatalf("unexpected trusted principal header config: %+v", cfg.TrustedPrincipalHeaders)
	}
}

func TestLoadProjectRBACEnforcementRequiresTrustedPrincipalHeaders(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://mbox:mbox@127.0.0.1:5432/mbox?sslmode=disable")
	t.Setenv("MBOX_PROJECT_RBAC_ENFORCEMENT_ENABLED", "true")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "MBOX_PROJECT_RBAC_ENFORCEMENT_ENABLED requires") {
		t.Fatalf("expected project RBAC dependency validation error, got %v", err)
	}
}

func TestLoadProjectRBACEnforcementReadsEnabledFlag(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://mbox:mbox@127.0.0.1:5432/mbox?sslmode=disable")
	t.Setenv("MBOX_TRUSTED_PRINCIPAL_HEADERS_ENABLED", "true")
	t.Setenv("MBOX_PROJECT_RBAC_ENFORCEMENT_ENABLED", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.ProjectRBAC.EnforcementEnabled {
		t.Fatalf("expected project RBAC enforcement enabled: %+v", cfg.ProjectRBAC)
	}
}
