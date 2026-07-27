package v1alpha1

import (
	"testing"

	"k8s.io/utils/ptr"
)

func TestValidateModuleSpec_NoProber(t *testing.T) {
	spec := &BlackboxModuleSpec{Timeout: "5s"}
	_, err := validateModuleSpec(spec)
	if err == nil {
		t.Fatal("expected error for no prober")
	}
}

func TestValidateModuleSpec_MultipleProbers(t *testing.T) {
	spec := &BlackboxModuleSpec{
		Timeout: "5s",
		HTTP:    &HTTPProbeConfig{},
		TCP:     &TCPProbeConfig{},
	}
	_, err := validateModuleSpec(spec)
	if err == nil {
		t.Fatal("expected error for multiple probers")
	}
}

func TestValidateModuleSpec_ValidHTTP(t *testing.T) {
	spec := &BlackboxModuleSpec{
		Timeout: "5s",
		HTTP:    &HTTPProbeConfig{Method: "GET"},
	}
	_, err := validateModuleSpec(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateModuleSpec_InvalidTimeout(t *testing.T) {
	spec := &BlackboxModuleSpec{
		Timeout: "not-a-duration",
		HTTP:    &HTTPProbeConfig{},
	}
	_, err := validateModuleSpec(spec)
	if err == nil {
		t.Fatal("expected error for invalid timeout")
	}
}

func TestValidateModuleSpec_DNSMissingQueryName(t *testing.T) {
	spec := &BlackboxModuleSpec{
		Timeout: "5s",
		DNS:     &DNSProbeConfig{},
	}
	_, err := validateModuleSpec(spec)
	if err == nil {
		t.Fatal("expected error for missing dns.queryName")
	}
}

func TestValidateModuleSpec_ValidDNS(t *testing.T) {
	spec := &BlackboxModuleSpec{
		Timeout: "5s",
		DNS:     &DNSProbeConfig{QueryName: "example.com"},
	}
	_, err := validateModuleSpec(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateProbeSpec_Valid(t *testing.T) {
	spec := &BlackboxProbeSpec{
		ExporterRef:   NamespacedReference{Name: "main"},
		ModuleRef:     NamespacedReference{Name: "http-2xx"},
		Targets:       []string{"https://example.com"},
		Interval:      "60s",
		ScrapeTimeout: "10s",
	}
	_, err := validateProbeSpec(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateProbeSpec_MissingExporterRef(t *testing.T) {
	spec := &BlackboxProbeSpec{
		ModuleRef: NamespacedReference{Name: "http-2xx"},
		Targets:   []string{"https://example.com"},
	}
	_, err := validateProbeSpec(spec)
	if err == nil {
		t.Fatal("expected error for missing exporterRef.name")
	}
}

func TestValidateProbeSpec_MissingModuleRef(t *testing.T) {
	spec := &BlackboxProbeSpec{
		ExporterRef: NamespacedReference{Name: "main"},
		Targets:     []string{"https://example.com"},
	}
	_, err := validateProbeSpec(spec)
	if err == nil {
		t.Fatal("expected error for missing moduleRef.name")
	}
}

func TestValidateProbeSpec_NoTargetsNoIngress(t *testing.T) {
	spec := &BlackboxProbeSpec{
		ExporterRef: NamespacedReference{Name: "main"},
		ModuleRef:   NamespacedReference{Name: "http-2xx"},
	}
	_, err := validateProbeSpec(spec)
	if err == nil {
		t.Fatal("expected error when neither targets nor ingress is set")
	}
}

func TestValidateProbeSpec_IngressOnly(t *testing.T) {
	spec := &BlackboxProbeSpec{
		ExporterRef: NamespacedReference{Name: "main"},
		ModuleRef:   NamespacedReference{Name: "http-2xx"},
		Ingress: &IngressTargetConfig{
			NamespaceSelector: NamespaceSelector{Any: true},
		},
		Interval: "60s",
	}
	_, err := validateProbeSpec(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateProbeSpec_BothTargetsAndIngress(t *testing.T) {
	spec := &BlackboxProbeSpec{
		ExporterRef: NamespacedReference{Name: "main"},
		ModuleRef:   NamespacedReference{Name: "http-2xx"},
		Targets:     []string{"https://example.com"},
		Ingress: &IngressTargetConfig{
			NamespaceSelector: NamespaceSelector{Any: true},
		},
		Interval: "60s",
	}
	_, err := validateProbeSpec(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateProbeSpec_TimeoutExceedsInterval(t *testing.T) {
	spec := &BlackboxProbeSpec{
		ExporterRef:   NamespacedReference{Name: "main"},
		ModuleRef:     NamespacedReference{Name: "http-2xx"},
		Targets:       []string{"https://example.com"},
		Interval:      "10s",
		ScrapeTimeout: "30s",
	}
	_, err := validateProbeSpec(spec)
	if err == nil {
		t.Fatal("expected error for scrapeTimeout > interval")
	}
}

func TestValidateProbeSpec_InvalidInterval(t *testing.T) {
	spec := &BlackboxProbeSpec{
		ExporterRef: NamespacedReference{Name: "main"},
		ModuleRef:   NamespacedReference{Name: "http-2xx"},
		Targets:     []string{"https://example.com"},
		Interval:    "bad",
	}
	_, err := validateProbeSpec(spec)
	if err == nil {
		t.Fatal("expected error for invalid interval")
	}
}

func TestValidateExporterSpec_Valid(t *testing.T) {
	spec := &BlackboxExporterSpec{
		Replicas: ptr.To(int32(2)),
		Port:     9115,
	}
	_, err := validateExporterSpec(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateExporterSpec_InvalidPort(t *testing.T) {
	spec := &BlackboxExporterSpec{Port: 99999}
	_, err := validateExporterSpec(spec)
	if err == nil {
		t.Fatal("expected error for invalid port")
	}
}

func TestValidateExporterSpec_NegativeReplicas(t *testing.T) {
	spec := &BlackboxExporterSpec{Replicas: ptr.To(int32(-1))}
	_, err := validateExporterSpec(spec)
	if err == nil {
		t.Fatal("expected error for negative replicas")
	}
}
