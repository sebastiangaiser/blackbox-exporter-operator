package converter

import (
	"testing"

	bbconfig "github.com/prometheus/blackbox_exporter/config"

	monitoringv1alpha1 "github.com/sebastiangaiser/blackbox-exporter-operator/api/v1alpha1"
)

func TestModuleName(t *testing.T) {
	tests := []struct {
		namespace string
		name      string
		want      string
	}{
		{"monitoring", "http-2xx", "monitoring-http-2xx"},
		{"default", "tcp-connect", "default-tcp-connect"},
	}
	for _, tt := range tests {
		got := ModuleName(tt.namespace, tt.name)
		if got != tt.want {
			t.Errorf("ModuleName(%q, %q) = %q, want %q", tt.namespace, tt.name, got, tt.want)
		}
	}
}

func TestConvertModule_NoProber(t *testing.T) {
	spec := &monitoringv1alpha1.BlackboxModuleSpec{
		Timeout: "5s",
	}
	_, err := ConvertModule(spec, nil)
	if err == nil {
		t.Fatal("expected error for empty prober, got nil")
	}
}

func TestConvertModule_InvalidTimeout(t *testing.T) {
	spec := &monitoringv1alpha1.BlackboxModuleSpec{
		Timeout: "not-a-duration",
		HTTP:    &monitoringv1alpha1.HTTPProbeConfig{},
	}
	_, err := ConvertModule(spec, nil)
	if err == nil {
		t.Fatal("expected error for invalid timeout, got nil")
	}
}

func TestConvertModule_HTTP(t *testing.T) {
	followRedirects := true
	spec := &monitoringv1alpha1.BlackboxModuleSpec{
		Timeout: "10s",
		HTTP: &monitoringv1alpha1.HTTPProbeConfig{
			Method:              "POST",
			Body:                `{"health":true}`,
			ValidStatusCodes:    []int{200, 201},
			ValidHTTPVersions:   []string{"HTTP/1.1"},
			FollowRedirects:     &followRedirects,
			FailIfNotSSL:        true,
			PreferredIPProtocol: "ip4",
			Headers:             map[string]string{"Content-Type": "application/json"},
		},
	}
	mod, err := ConvertModule(spec, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mod.Prober != "http" {
		t.Errorf("prober = %q, want %q", mod.Prober, "http")
	}
	if mod.Timeout.String() != "10s" {
		t.Errorf("timeout = %v, want 10s", mod.Timeout)
	}
	if mod.HTTP.Method != "POST" {
		t.Errorf("method = %q, want %q", mod.HTTP.Method, "POST")
	}
	if mod.HTTP.Body != `{"health":true}` {
		t.Errorf("body = %q, want %q", mod.HTTP.Body, `{"health":true}`)
	}
	if len(mod.HTTP.ValidStatusCodes) != 2 {
		t.Errorf("validStatusCodes len = %d, want 2", len(mod.HTTP.ValidStatusCodes))
	}
	if !mod.HTTP.FailIfNotSSL {
		t.Error("failIfNotSSL = false, want true")
	}
	if mod.HTTP.IPProtocol != "ip4" {
		t.Errorf("preferredIPProtocol = %q, want %q", mod.HTTP.IPProtocol, "ip4")
	}
	if mod.HTTP.Headers["Content-Type"] != "application/json" {
		t.Errorf("Content-Type header = %q, want %q", mod.HTTP.Headers["Content-Type"], "application/json")
	}
}

func TestConvertModule_HTTP_Defaults(t *testing.T) {
	spec := &monitoringv1alpha1.BlackboxModuleSpec{
		Timeout: "5s",
		HTTP:    &monitoringv1alpha1.HTTPProbeConfig{},
	}
	mod, err := ConvertModule(spec, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mod.Prober != "http" {
		t.Errorf("prober = %q, want %q", mod.Prober, "http")
	}
	// Default should follow redirects (from upstream DefaultHTTPProbe).
	if !mod.HTTP.HTTPClientConfig.FollowRedirects {
		t.Error("followRedirects should default to true")
	}
}

func TestConvertModule_HTTP_BodyMatchRegexp(t *testing.T) {
	spec := &monitoringv1alpha1.BlackboxModuleSpec{
		Timeout: "5s",
		HTTP: &monitoringv1alpha1.HTTPProbeConfig{
			FailIfBodyMatchesRegexp:    []string{"error", "fail"},
			FailIfBodyNotMatchesRegexp: []string{"ok"},
		},
	}
	mod, err := ConvertModule(spec, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mod.HTTP.FailIfBodyMatchesRegexp) != 2 {
		t.Errorf("failIfBodyMatchesRegexp len = %d, want 2", len(mod.HTTP.FailIfBodyMatchesRegexp))
	}
	if len(mod.HTTP.FailIfBodyNotMatchesRegexp) != 1 {
		t.Errorf("failIfBodyNotMatchesRegexp len = %d, want 1", len(mod.HTTP.FailIfBodyNotMatchesRegexp))
	}
}

func TestConvertModule_HTTP_HeaderMatch(t *testing.T) {
	spec := &monitoringv1alpha1.BlackboxModuleSpec{
		Timeout: "5s",
		HTTP: &monitoringv1alpha1.HTTPProbeConfig{
			FailIfHeaderMatchesRegexp: []monitoringv1alpha1.HeaderMatchConfig{
				{Header: "X-Custom", Regexp: "bad.*", AllowMissing: true},
			},
		},
	}
	mod, err := ConvertModule(spec, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mod.HTTP.FailIfHeaderMatchesRegexp) != 1 {
		t.Fatalf("failIfHeaderMatchesRegexp len = %d, want 1", len(mod.HTTP.FailIfHeaderMatchesRegexp))
	}
	hm := mod.HTTP.FailIfHeaderMatchesRegexp[0]
	if hm.Header != "X-Custom" {
		t.Errorf("header = %q, want %q", hm.Header, "X-Custom")
	}
	if !hm.AllowMissing {
		t.Error("allowMissing = false, want true")
	}
}

func TestConvertModule_HTTP_ProxyURL(t *testing.T) {
	spec := &monitoringv1alpha1.BlackboxModuleSpec{
		Timeout: "5s",
		HTTP: &monitoringv1alpha1.HTTPProbeConfig{
			ProxyURL: "http://proxy.example.com:8080",
		},
	}
	mod, err := ConvertModule(spec, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mod.HTTP.HTTPClientConfig.ProxyURL.String() != "http://proxy.example.com:8080" {
		t.Errorf("proxyURL = %q, want %q", mod.HTTP.HTTPClientConfig.ProxyURL.String(), "http://proxy.example.com:8080")
	}
}

func TestConvertModule_TCP(t *testing.T) {
	spec := &monitoringv1alpha1.BlackboxModuleSpec{
		Timeout: "5s",
		TCP: &monitoringv1alpha1.TCPProbeConfig{
			PreferredIPProtocol: "ip4",
			TLS:                 true,
			QueryResponse: []monitoringv1alpha1.QueryResponseConfig{
				{Expect: "^SSH-2.0-"},
			},
		},
	}
	mod, err := ConvertModule(spec, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mod.Prober != "tcp" {
		t.Errorf("prober = %q, want %q", mod.Prober, "tcp")
	}
	if !mod.TCP.TLS {
		t.Error("tls = false, want true")
	}
	if len(mod.TCP.QueryResponse) != 1 {
		t.Fatalf("queryResponse len = %d, want 1", len(mod.TCP.QueryResponse))
	}
	if mod.TCP.QueryResponse[0].Expect.String() != "^SSH-2.0-" {
		t.Errorf("expect = %q, want %q", mod.TCP.QueryResponse[0].Expect.String(), "^SSH-2.0-")
	}
}

func TestConvertModule_DNS(t *testing.T) {
	recursion := false
	spec := &monitoringv1alpha1.BlackboxModuleSpec{
		Timeout: "5s",
		DNS: &monitoringv1alpha1.DNSProbeConfig{
			QueryName:           "example.com",
			QueryType:           "A",
			TransportProtocol:   "udp",
			RecursionDesired:    &recursion,
			ValidRcodes:         []string{"NOERROR"},
			PreferredIPProtocol: "ip4",
			ValidateAnswer: &monitoringv1alpha1.DNSRRValidator{
				FailIfNotMatchesRegexp: []string{"1\\.2\\.3\\.4"},
			},
		},
	}
	mod, err := ConvertModule(spec, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mod.Prober != "dns" {
		t.Errorf("prober = %q, want %q", mod.Prober, "dns")
	}
	if mod.DNS.QueryName != "example.com" {
		t.Errorf("queryName = %q, want %q", mod.DNS.QueryName, "example.com")
	}
	if mod.DNS.QueryType != "A" {
		t.Errorf("queryType = %q, want %q", mod.DNS.QueryType, "A")
	}
	if mod.DNS.Recursion {
		t.Error("recursion = true, want false")
	}
	if len(mod.DNS.ValidateAnswer.FailIfNotMatchesRegexp) != 1 {
		t.Error("validateAnswer.failIfNotMatchesRegexp should have 1 entry")
	}
}

func TestConvertModule_ICMP(t *testing.T) {
	spec := &monitoringv1alpha1.BlackboxModuleSpec{
		Timeout: "5s",
		ICMP: &monitoringv1alpha1.ICMPProbeConfig{
			PreferredIPProtocol: "ip4",
			PayloadSize:         64,
			DontFragment:        true,
			TTL:                 128,
		},
	}
	mod, err := ConvertModule(spec, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mod.Prober != "icmp" {
		t.Errorf("prober = %q, want %q", mod.Prober, "icmp")
	}
	if mod.ICMP.PayloadSize != 64 {
		t.Errorf("payloadSize = %d, want 64", mod.ICMP.PayloadSize)
	}
	if !mod.ICMP.DontFragment {
		t.Error("dontFragment = false, want true")
	}
	if mod.ICMP.TTL != 128 {
		t.Errorf("ttl = %d, want 128", mod.ICMP.TTL)
	}
}

func TestConvertModule_GRPC(t *testing.T) {
	spec := &monitoringv1alpha1.BlackboxModuleSpec{
		Timeout: "5s",
		GRPC: &monitoringv1alpha1.GRPCProbeConfig{
			Service:             "grpc.health.v1.Health",
			TLS:                 true,
			PreferredIPProtocol: "ip6",
		},
	}
	mod, err := ConvertModule(spec, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mod.Prober != "grpc" {
		t.Errorf("prober = %q, want %q", mod.Prober, "grpc")
	}
	if mod.GRPC.Service != "grpc.health.v1.Health" {
		t.Errorf("service = %q, want %q", mod.GRPC.Service, "grpc.health.v1.Health")
	}
	if !mod.GRPC.TLS {
		t.Error("tls = false, want true")
	}
}

func TestConvertModule_Unix(t *testing.T) {
	spec := &monitoringv1alpha1.BlackboxModuleSpec{
		Timeout: "5s",
		Unix: &monitoringv1alpha1.UnixProbeConfig{
			QueryResponse: []monitoringv1alpha1.QueryResponseConfig{
				{Send: "hello", Expect: "world"},
			},
		},
	}
	mod, err := ConvertModule(spec, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Unix uses "tcp" prober upstream.
	if mod.Prober != "tcp" {
		t.Errorf("prober = %q, want %q", mod.Prober, "tcp")
	}
	if len(mod.Unix.QueryResponse) != 1 {
		t.Fatalf("queryResponse len = %d, want 1", len(mod.Unix.QueryResponse))
	}
}

func TestConvertModule_TLSConfig(t *testing.T) {
	spec := &monitoringv1alpha1.BlackboxModuleSpec{
		Timeout: "5s",
		HTTP: &monitoringv1alpha1.HTTPProbeConfig{
			TLSConfig: &monitoringv1alpha1.TLSConfig{
				InsecureSkipVerify: true,
				ServerName:         "example.com",
				MinVersion:         "TLS12",
				MaxVersion:         "TLS13",
			},
		},
	}
	mod, err := ConvertModule(spec, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mod.HTTP.HTTPClientConfig.TLSConfig.InsecureSkipVerify {
		t.Error("insecureSkipVerify = false, want true")
	}
	if mod.HTTP.HTTPClientConfig.TLSConfig.ServerName != "example.com" {
		t.Errorf("serverName = %q, want %q", mod.HTTP.HTTPClientConfig.TLSConfig.ServerName, "example.com")
	}
	if mod.HTTP.HTTPClientConfig.TLSConfig.MinVersion != 0x0303 {
		t.Errorf("minVersion = %x, want 0x0303", mod.HTTP.HTTPClientConfig.TLSConfig.MinVersion)
	}
	if mod.HTTP.HTTPClientConfig.TLSConfig.MaxVersion != 0x0304 {
		t.Errorf("maxVersion = %x, want 0x0304", mod.HTTP.HTTPClientConfig.TLSConfig.MaxVersion)
	}
}

func TestRenderConfig(t *testing.T) {
	spec := &monitoringv1alpha1.BlackboxModuleSpec{
		Timeout: "5s",
		HTTP: &monitoringv1alpha1.HTTPProbeConfig{
			Method: "GET",
		},
	}
	mod, err := ConvertModule(spec, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bbModules := map[string]bbconfig.Module{
		"monitoring-http-2xx": mod,
	}

	rendered, err := RenderConfig(bbModules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	yaml := string(rendered)
	if len(yaml) == 0 {
		t.Fatal("rendered config is empty")
	}
	// Should contain the module name.
	if !contains(yaml, "monitoring-http-2xx") {
		t.Errorf("rendered config does not contain module name, got:\n%s", yaml)
	}
	// Should contain the prober.
	if !contains(yaml, "http") {
		t.Errorf("rendered config does not contain prober, got:\n%s", yaml)
	}
}

func TestRenderConfig_Empty(t *testing.T) {
	rendered, err := RenderConfig(map[string]bbconfig.Module{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rendered) == 0 {
		t.Fatal("rendered config is empty for empty modules")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
