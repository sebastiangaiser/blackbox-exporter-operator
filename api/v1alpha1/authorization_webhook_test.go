package v1alpha1

import "testing"

func authorizationModule(auth *Authorization) *BlackboxModuleSpec {
	return &BlackboxModuleSpec{
		Timeout: "5s",
		HTTP: &HTTPProbeConfig{
			Method:        "GET",
			Authorization: auth,
		},
	}
}

func TestValidateModuleSpec_AuthorizationValid(t *testing.T) {
	spec := authorizationModule(&Authorization{
		Type:           "Bearer",
		CredentialsRef: SecretKeySelector{Name: "api-token", Key: "token"},
	})
	if _, err := validateModuleSpec(spec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateModuleSpec_AuthorizationEmptyTypeIsValid(t *testing.T) {
	// The CRD defaults the type to Bearer; the webhook must accept it unset.
	spec := authorizationModule(&Authorization{
		CredentialsRef: SecretKeySelector{Name: "api-token", Key: "token"},
	})
	if _, err := validateModuleSpec(spec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateModuleSpec_AuthorizationMissingCredentialsRef(t *testing.T) {
	tests := map[string]*Authorization{
		"missing name": {Type: "Bearer", CredentialsRef: SecretKeySelector{Key: "token"}},
		"missing key":  {Type: "Bearer", CredentialsRef: SecretKeySelector{Name: "api-token"}},
		"empty ref":    {Type: "Bearer"},
	}
	for name, auth := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := validateModuleSpec(authorizationModule(auth)); err == nil {
				t.Fatal("expected error for incomplete credentialsRef")
			}
		})
	}
}

func TestValidateModuleSpec_AuthorizationBasicTypeRejected(t *testing.T) {
	for _, authType := range []string{"basic", "BASIC", " Basic "} {
		t.Run(authType, func(t *testing.T) {
			spec := authorizationModule(&Authorization{
				Type:           authType,
				CredentialsRef: SecretKeySelector{Name: "api-token", Key: "token"},
			})
			if _, err := validateModuleSpec(spec); err == nil {
				t.Fatalf("expected error for authorization type %q", authType)
			}
		})
	}
}

func TestValidateModuleSpec_AuthMutualExclusion(t *testing.T) {
	creds := SecretKeySelector{Name: "api-token", Key: "token"}

	tests := map[string]*HTTPProbeConfig{
		"authorization and basicAuth": {
			Authorization: &Authorization{CredentialsRef: creds},
			BasicAuth:     &BasicAuth{Username: "user", PasswordRef: creds},
		},
		"authorization and oauth2": {
			Authorization: &Authorization{CredentialsRef: creds},
			OAuth2: &OAuth2Config{
				ClientID:        "cid",
				ClientSecretRef: creds,
				TokenURL:        "https://auth.example.com/token",
			},
		},
		"basicAuth and oauth2": {
			BasicAuth: &BasicAuth{Username: "user", PasswordRef: creds},
			OAuth2: &OAuth2Config{
				ClientID:        "cid",
				ClientSecretRef: creds,
				TokenURL:        "https://auth.example.com/token",
			},
		},
	}

	for name, http := range tests {
		t.Run(name, func(t *testing.T) {
			spec := &BlackboxModuleSpec{Timeout: "5s", HTTP: http}
			if _, err := validateModuleSpec(spec); err == nil {
				t.Fatal("expected error for mutually exclusive auth methods")
			}
		})
	}
}

func TestValidateModuleSpec_BasicAuthAllowsEmptyUsername(t *testing.T) {
	spec := &BlackboxModuleSpec{
		Timeout: "5s",
		HTTP: &HTTPProbeConfig{
			BasicAuth: &BasicAuth{
				PasswordRef: SecretKeySelector{Name: "creds", Key: "password"},
			},
		},
	}
	if _, err := validateModuleSpec(spec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateModuleSpec_AuthorizationHeaderInHeadersWarns(t *testing.T) {
	for _, header := range []string{"Authorization", "authorization", "AUTHORIZATION"} {
		t.Run(header, func(t *testing.T) {
			spec := authorizationModule(&Authorization{
				CredentialsRef: SecretKeySelector{Name: "api-token", Key: "token"},
			})
			spec.HTTP.Headers = map[string]string{header: "Bearer hardcoded"}

			warnings, err := validateModuleSpec(spec)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(warnings) == 0 {
				t.Fatal("expected a warning for an Authorization header alongside http.authorization")
			}
		})
	}
}

func TestValidateModuleSpec_HeadersWithoutAuthorizationDoesNotWarn(t *testing.T) {
	spec := authorizationModule(&Authorization{
		CredentialsRef: SecretKeySelector{Name: "api-token", Key: "token"},
	})
	spec.HTTP.Headers = map[string]string{"User-Agent": "BlackboxExporter"}

	warnings, err := validateModuleSpec(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
}

func TestValidateModuleSpec_NonHTTPProberSkipsAuthValidation(t *testing.T) {
	// Auth rules are HTTP-only; a TCP module must not be affected.
	spec := &BlackboxModuleSpec{Timeout: "5s", TCP: &TCPProbeConfig{}}
	if _, err := validateModuleSpec(spec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
