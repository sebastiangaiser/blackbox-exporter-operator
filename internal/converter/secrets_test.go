package converter

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	monitoringv1alpha1 "github.com/sebastiangaiser/blackbox-exporter-operator/api/v1alpha1"
)

func secretsFakeClient(t *testing.T, data map[string]string) *fake.ClientBuilder {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("adding corev1 to scheme: %v", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "api-token", Namespace: "monitoring"},
		Data:       map[string][]byte{},
	}
	for k, v := range data {
		secret.Data[k] = []byte(v)
	}

	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret)
}

func TestResolveModuleSecrets_Authorization(t *testing.T) {
	c := secretsFakeClient(t, map[string]string{"token": "s3cr3t-token"}).Build()
	spec := &monitoringv1alpha1.BlackboxModuleSpec{
		Timeout: "5s",
		HTTP: &monitoringv1alpha1.HTTPProbeConfig{
			Authorization: &monitoringv1alpha1.Authorization{
				CredentialsRef: monitoringv1alpha1.SecretKeySelector{Name: "api-token", Key: "token"},
			},
		},
	}

	resolved, err := ResolveModuleSecrets(context.Background(), c, "monitoring", spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.AuthorizationCredentials != "s3cr3t-token" {
		t.Errorf("AuthorizationCredentials = %q, want %q", resolved.AuthorizationCredentials, "s3cr3t-token")
	}
}

func TestResolveModuleSecrets_AuthorizationMissingKey(t *testing.T) {
	c := secretsFakeClient(t, map[string]string{"other": "value"}).Build()
	spec := &monitoringv1alpha1.BlackboxModuleSpec{
		Timeout: "5s",
		HTTP: &monitoringv1alpha1.HTTPProbeConfig{
			Authorization: &monitoringv1alpha1.Authorization{
				CredentialsRef: monitoringv1alpha1.SecretKeySelector{Name: "api-token", Key: "token"},
			},
		},
	}

	if _, err := ResolveModuleSecrets(context.Background(), c, "monitoring", spec); err == nil {
		t.Fatal("expected error for a missing secret key")
	}
}

func TestResolveModuleSecrets_AuthorizationMissingSecret(t *testing.T) {
	c := secretsFakeClient(t, map[string]string{"token": "value"}).Build()
	spec := &monitoringv1alpha1.BlackboxModuleSpec{
		Timeout: "5s",
		HTTP: &monitoringv1alpha1.HTTPProbeConfig{
			Authorization: &monitoringv1alpha1.Authorization{
				CredentialsRef: monitoringv1alpha1.SecretKeySelector{Name: "absent", Key: "token"},
			},
		},
	}

	if _, err := ResolveModuleSecrets(context.Background(), c, "monitoring", spec); err == nil {
		t.Fatal("expected error for a missing secret")
	}
}

func TestResolveModuleSecrets_NoAuthorization(t *testing.T) {
	c := secretsFakeClient(t, nil).Build()
	spec := &monitoringv1alpha1.BlackboxModuleSpec{
		Timeout: "5s",
		HTTP:    &monitoringv1alpha1.HTTPProbeConfig{Method: "GET"},
	}

	resolved, err := ResolveModuleSecrets(context.Background(), c, "monitoring", spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.AuthorizationCredentials != "" {
		t.Errorf("AuthorizationCredentials = %q, want empty", resolved.AuthorizationCredentials)
	}
}
