package converter

import (
	"strings"
	"testing"

	bbconfig "github.com/prometheus/blackbox_exporter/config"
	promconfig "github.com/prometheus/common/config"

	monitoringv1alpha1 "github.com/sebastiangaiser/blackbox-exporter-operator/api/v1alpha1"
)

func authorizationSpec(authType string) *monitoringv1alpha1.BlackboxModuleSpec {
	spec := &monitoringv1alpha1.BlackboxModuleSpec{
		Timeout: "5s",
		HTTP: &monitoringv1alpha1.HTTPProbeConfig{
			Method: "GET",
			Authorization: &monitoringv1alpha1.Authorization{
				CredentialsRef: monitoringv1alpha1.SecretKeySelector{Name: "api-token", Key: "token"},
			},
		},
	}
	if authType != "" {
		spec.HTTP.Authorization.Type = authType
	}
	return spec
}

func TestConvertModule_HTTP_Authorization(t *testing.T) {
	secrets := &ResolvedSecrets{AuthorizationCredentials: "s3cr3t-token"}

	mod, err := ConvertModule(authorizationSpec(""), secrets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mod.HTTP.HTTPClientConfig.Authorization == nil {
		t.Fatal("authorization = nil, want configured")
	}
	if mod.HTTP.HTTPClientConfig.Authorization.Type != "Bearer" {
		t.Errorf("type = %q, want %q", mod.HTTP.HTTPClientConfig.Authorization.Type, "Bearer")
	}
	if got := string(mod.HTTP.HTTPClientConfig.Authorization.Credentials); got != "s3cr3t-token" {
		t.Errorf("credentials = %q, want %q", got, "s3cr3t-token")
	}
}

func TestConvertModule_HTTP_AuthorizationCustomType(t *testing.T) {
	secrets := &ResolvedSecrets{AuthorizationCredentials: "abc123"}

	mod, err := ConvertModule(authorizationSpec("Token"), secrets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mod.HTTP.HTTPClientConfig.Authorization.Type != "Token" {
		t.Errorf("type = %q, want %q", mod.HTTP.HTTPClientConfig.Authorization.Type, "Token")
	}
}

func TestConvertModule_HTTP_AuthorizationNoSecrets(t *testing.T) {
	mod, err := ConvertModule(authorizationSpec(""), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mod.HTTP.HTTPClientConfig.Authorization == nil {
		t.Fatal("authorization = nil, want configured")
	}
	if mod.HTTP.HTTPClientConfig.Authorization.Credentials != "" {
		t.Errorf("credentials = %q, want empty", mod.HTTP.HTTPClientConfig.Authorization.Credentials)
	}
}

func TestRenderConfig_AuthorizationCredentials(t *testing.T) {
	secrets := &ResolvedSecrets{AuthorizationCredentials: "s3cr3t-token"}

	mod, err := ConvertModule(authorizationSpec(""), secrets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rendered, err := RenderConfig(map[string]bbconfig.Module{"monitoring-bearer": mod})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := string(rendered)

	if strings.Contains(got, "<secret>") {
		t.Errorf("rendered config contains a redaction placeholder:\n%s", got)
	}
	if !strings.Contains(got, "s3cr3t-token") {
		t.Errorf("rendered config does not contain the resolved credentials:\n%s", got)
	}
	if !strings.Contains(got, "authorization:") {
		t.Errorf("rendered config does not contain an authorization block:\n%s", got)
	}
	if !strings.Contains(got, "type: Bearer") {
		t.Errorf("rendered config does not contain the default type:\n%s", got)
	}
}

func TestRenderConfig_BasicAuthSecret(t *testing.T) {
	spec := &monitoringv1alpha1.BlackboxModuleSpec{
		Timeout: "5s",
		HTTP: &monitoringv1alpha1.HTTPProbeConfig{
			BasicAuth: &monitoringv1alpha1.BasicAuth{
				Username:    "monitor",
				PasswordRef: monitoringv1alpha1.SecretKeySelector{Name: "creds", Key: "password"},
			},
		},
	}
	secrets := &ResolvedSecrets{BasicAuthPassword: "hunter2"}

	mod, err := ConvertModule(spec, secrets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rendered, err := RenderConfig(map[string]bbconfig.Module{"monitoring-basic": mod})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := string(rendered)

	if strings.Contains(got, "<secret>") {
		t.Errorf("rendered config contains a redaction placeholder:\n%s", got)
	}
	if !strings.Contains(got, "hunter2") {
		t.Errorf("rendered config does not contain the resolved password:\n%s", got)
	}
}

func TestRenderConfig_TLSKeySecret(t *testing.T) {
	spec := &monitoringv1alpha1.BlackboxModuleSpec{
		Timeout: "5s",
		HTTP: &monitoringv1alpha1.HTTPProbeConfig{
			TLSConfig: &monitoringv1alpha1.TLSConfig{
				KeyRef: &monitoringv1alpha1.SecretKeySelector{Name: "tls", Key: "key"},
			},
		},
	}
	secrets := &ResolvedSecrets{TLSKey: "-----BEGIN PRIVATE KEY-----"}

	mod, err := ConvertModule(spec, secrets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rendered, err := RenderConfig(map[string]bbconfig.Module{"monitoring-tls": mod})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := string(rendered); strings.Contains(got, "<secret>") {
		t.Errorf("rendered config contains a redaction placeholder:\n%s", got)
	}
}

func TestRenderConfig_RestoresMarshalSecretValue(t *testing.T) {
	if _, err := RenderConfig(map[string]bbconfig.Module{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if promconfig.MarshalSecretValue {
		t.Error("MarshalSecretValue left enabled after RenderConfig returned")
	}
}
