package converter

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	monitoringv1alpha1 "github.com/sebastiangaiser/blackbox-exporter-operator/api/v1alpha1"
)

// ResolvedSecrets holds secret values resolved from Kubernetes Secrets.
type ResolvedSecrets struct {
	// BasicAuth password.
	BasicAuthPassword string
	// OAuth2 client secret.
	OAuth2ClientSecret string
	// TLS CA certificate PEM.
	TLSCA string
	// TLS client certificate PEM.
	TLSCert string
	// TLS client key PEM.
	TLSKey string
}

// ResolveModuleSecrets reads all SecretKeySelector references from a BlackboxModuleSpec
// and returns the resolved values. The namespace is the namespace of the BlackboxModule
// (secrets must be in the same namespace).
func ResolveModuleSecrets(ctx context.Context, c client.Reader, namespace string, spec *monitoringv1alpha1.BlackboxModuleSpec) (*ResolvedSecrets, error) {
	resolved := &ResolvedSecrets{}

	if spec.HTTP == nil {
		return resolved, nil
	}

	if spec.HTTP.BasicAuth != nil {
		val, err := resolveSecretKey(ctx, c, namespace, &spec.HTTP.BasicAuth.PasswordRef)
		if err != nil {
			return nil, fmt.Errorf("resolving basicAuth.passwordRef: %w", err)
		}
		resolved.BasicAuthPassword = val
	}

	if spec.HTTP.OAuth2 != nil {
		val, err := resolveSecretKey(ctx, c, namespace, &spec.HTTP.OAuth2.ClientSecretRef)
		if err != nil {
			return nil, fmt.Errorf("resolving oauth2.clientSecretRef: %w", err)
		}
		resolved.OAuth2ClientSecret = val
	}

	tlsConfig := spec.HTTP.TLSConfig
	if tlsConfig == nil {
		// Check TCP, gRPC TLS configs too.
		if spec.TCP != nil {
			tlsConfig = spec.TCP.TLSConfig
		} else if spec.GRPC != nil {
			tlsConfig = spec.GRPC.TLSConfig
		}
	}

	if tlsConfig != nil {
		if tlsConfig.CARef != nil {
			val, err := resolveSecretKey(ctx, c, namespace, tlsConfig.CARef)
			if err != nil {
				return nil, fmt.Errorf("resolving tlsConfig.caRef: %w", err)
			}
			resolved.TLSCA = val
		}
		if tlsConfig.CertRef != nil {
			val, err := resolveSecretKey(ctx, c, namespace, tlsConfig.CertRef)
			if err != nil {
				return nil, fmt.Errorf("resolving tlsConfig.certRef: %w", err)
			}
			resolved.TLSCert = val
		}
		if tlsConfig.KeyRef != nil {
			val, err := resolveSecretKey(ctx, c, namespace, tlsConfig.KeyRef)
			if err != nil {
				return nil, fmt.Errorf("resolving tlsConfig.keyRef: %w", err)
			}
			resolved.TLSKey = val
		}
	}

	return resolved, nil
}

func resolveSecretKey(ctx context.Context, c client.Reader, namespace string, ref *monitoringv1alpha1.SecretKeySelector) (string, error) {
	secret := &corev1.Secret{}
	if err := c.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: namespace}, secret); err != nil {
		return "", fmt.Errorf("secret %s/%s: %w", namespace, ref.Name, err)
	}
	val, ok := secret.Data[ref.Key]
	if !ok {
		return "", fmt.Errorf("key %q not found in secret %s/%s", ref.Key, namespace, ref.Name)
	}
	return string(val), nil
}
