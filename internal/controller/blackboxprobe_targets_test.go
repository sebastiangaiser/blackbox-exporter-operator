package controller

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	monitoringv1alpha1 "github.com/sebastiangaiser/blackbox-exporter-operator/api/v1alpha1"
)

func probeWithIngress(ingress *monitoringv1alpha1.IngressTargetConfig) *monitoringv1alpha1.BlackboxProbe {
	return &monitoringv1alpha1.BlackboxProbe{
		Spec: monitoringv1alpha1.BlackboxProbeSpec{Ingress: ingress},
	}
}

func TestBuildProbeTargets_IngressLabelsBecomeSortedReplaceRelabelings(t *testing.T) {
	// Deliberately not in alphabetical order, to pin down that the output is sorted
	// rather than dependent on Go's randomized map iteration.
	probe := probeWithIngress(&monitoringv1alpha1.IngressTargetConfig{
		Labels: map[string]string{
			"tenant": "acme",
			"env":    "production",
			"source": "ingress-discovery",
		},
	})

	got := buildProbeTargets(probe)
	if got.Ingress == nil {
		t.Fatal("expected ingress targets to be set")
	}

	want := []promv1.RelabelConfig{
		{TargetLabel: "env", Replacement: ptr.To("production"), Action: "replace"},
		{TargetLabel: "source", Replacement: ptr.To("ingress-discovery"), Action: "replace"},
		{TargetLabel: "tenant", Replacement: ptr.To("acme"), Action: "replace"},
	}
	if !reflect.DeepEqual(got.Ingress.RelabelConfigs, want) {
		t.Fatalf("relabelings mismatch:\n got: %+v\nwant: %+v", got.Ingress.RelabelConfigs, want)
	}
}

func TestBuildProbeTargets_IngressLabelsRepeatable(t *testing.T) {
	// Map iteration order changes between runs; the rendered spec must not, or every
	// reconcile would produce a diff and re-update the Probe CR.
	probe := probeWithIngress(&monitoringv1alpha1.IngressTargetConfig{
		Labels: map[string]string{"a": "1", "b": "2", "c": "3", "d": "4", "e": "5"},
	})

	first := buildProbeTargets(probe).Ingress.RelabelConfigs
	for i := range 20 {
		again := buildProbeTargets(probe).Ingress.RelabelConfigs
		if !reflect.DeepEqual(first, again) {
			t.Fatalf("run %d produced a different order:\n first: %+v\n again: %+v", i, first, again)
		}
	}
}

func TestBuildProbeTargets_IngressLabelsPrecedeUserRelabelConfigs(t *testing.T) {
	probe := probeWithIngress(&monitoringv1alpha1.IngressTargetConfig{
		Labels: map[string]string{"source": "ingress-discovery"},
		RelabelConfigs: []monitoringv1alpha1.RelabelConfig{
			{SourceLabels: []string{"__tmp_prometheus_job_name"}, TargetLabel: "job", Action: "replace"},
		},
	})

	got := buildProbeTargets(probe).Ingress.RelabelConfigs
	if len(got) != 2 {
		t.Fatalf("expected 2 relabelings, got %d: %+v", len(got), got)
	}
	if got[0].TargetLabel != "source" {
		t.Errorf("expected the label-derived relabeling first, got %q", got[0].TargetLabel)
	}
	if got[1].TargetLabel != "job" {
		t.Errorf("expected the user relabeling second, got %q", got[1].TargetLabel)
	}
}

func TestBuildProbeTargets_IngressWithoutLabels(t *testing.T) {
	probe := probeWithIngress(&monitoringv1alpha1.IngressTargetConfig{
		Selector: metav1.LabelSelector{MatchLabels: map[string]string{"monitoring": "true"}},
	})

	got := buildProbeTargets(probe)
	if got.Ingress == nil {
		t.Fatal("expected ingress targets to be set")
	}
	if len(got.Ingress.RelabelConfigs) != 0 {
		t.Errorf("expected no synthetic relabelings, got %+v", got.Ingress.RelabelConfigs)
	}
	if !reflect.DeepEqual(got.Ingress.Selector.MatchLabels, map[string]string{"monitoring": "true"}) {
		t.Errorf("selector not carried over: %+v", got.Ingress.Selector)
	}
}

func TestBuildProbeTargets_IngressNamespaceSelector(t *testing.T) {
	tests := []struct {
		name string
		in   monitoringv1alpha1.NamespaceSelector
		want promv1.NamespaceSelector
	}{
		{
			name: "any",
			in:   monitoringv1alpha1.NamespaceSelector{Any: true},
			want: promv1.NamespaceSelector{Any: true},
		},
		{
			name: "matchNames",
			in:   monitoringv1alpha1.NamespaceSelector{MatchNames: []string{"team-a", "team-b"}},
			want: promv1.NamespaceSelector{MatchNames: []string{"team-a", "team-b"}},
		},
		{
			// any wins; matchNames is redundant once every namespace is selected.
			name: "any takes precedence over matchNames",
			in:   monitoringv1alpha1.NamespaceSelector{Any: true, MatchNames: []string{"team-a"}},
			want: promv1.NamespaceSelector{Any: true},
		},
		{
			name: "unset",
			in:   monitoringv1alpha1.NamespaceSelector{},
			want: promv1.NamespaceSelector{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			probe := probeWithIngress(&monitoringv1alpha1.IngressTargetConfig{NamespaceSelector: tt.in})
			got := buildProbeTargets(probe).Ingress.NamespaceSelector
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestBuildProbeTargets_StaticAndIngressCoexist(t *testing.T) {
	probe := &monitoringv1alpha1.BlackboxProbe{
		Spec: monitoringv1alpha1.BlackboxProbeSpec{
			Targets:      []string{"https://example.com"},
			TargetLabels: map[string]string{"tenant": "acme"},
			Ingress: &monitoringv1alpha1.IngressTargetConfig{
				Labels: map[string]string{"source": "ingress-discovery"},
			},
		},
	}

	got := buildProbeTargets(probe)
	if got.StaticConfig == nil {
		t.Fatal("expected static config to be set")
	}
	if got.Ingress == nil {
		t.Fatal("expected ingress config to be set")
	}

	// The two label sets stay on their own target source.
	if !reflect.DeepEqual(got.StaticConfig.Labels, map[string]string{"tenant": "acme"}) {
		t.Errorf("static labels: got %+v", got.StaticConfig.Labels)
	}
	if len(got.Ingress.RelabelConfigs) != 1 || got.Ingress.RelabelConfigs[0].TargetLabel != "source" {
		t.Errorf("ingress relabelings: got %+v", got.Ingress.RelabelConfigs)
	}
}

func TestBuildProbeTargets_NoIngress(t *testing.T) {
	probe := &monitoringv1alpha1.BlackboxProbe{
		Spec: monitoringv1alpha1.BlackboxProbeSpec{Targets: []string{"https://example.com"}},
	}

	got := buildProbeTargets(probe)
	if got.Ingress != nil {
		t.Errorf("expected no ingress config, got %+v", got.Ingress)
	}
	if got.StaticConfig == nil || len(got.StaticConfig.Targets) != 1 {
		t.Fatalf("expected one static target, got %+v", got.StaticConfig)
	}
	if got.StaticConfig.Labels != nil {
		t.Errorf("expected nil labels when neither targetLabels nor additionalLabels is set, got %+v", got.StaticConfig.Labels)
	}
}

func TestConvertRelabelConfig(t *testing.T) {
	in := monitoringv1alpha1.RelabelConfig{
		SourceLabels: []string{"__tmp_prometheus_ingress_address", "__tmp_prometheus_job_name"},
		Separator:    ";",
		TargetLabel:  "instance",
		Regex:        "(.*)",
		Replacement:  "$1",
		Action:       "replace",
		Modulus:      8,
	}

	got := convertRelabelConfig(in)
	want := &promv1.RelabelConfig{
		SourceLabels: []promv1.LabelName{"__tmp_prometheus_ingress_address", "__tmp_prometheus_job_name"},
		Separator:    ptr.To(";"),
		TargetLabel:  "instance",
		Regex:        "(.*)",
		Replacement:  ptr.To("$1"),
		Action:       "replace",
		Modulus:      8,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestConvertRelabelConfig_EmptyFieldsStayUnset(t *testing.T) {
	got := convertRelabelConfig(monitoringv1alpha1.RelabelConfig{})

	if got.SourceLabels != nil {
		t.Errorf("sourceLabels: got %+v, want nil", got.SourceLabels)
	}
	if got.Separator != nil {
		t.Errorf("separator: got %q, want nil", *got.Separator)
	}
	if got.Replacement != nil {
		t.Errorf("replacement: got %q, want nil", *got.Replacement)
	}
	if got.TargetLabel != "" || got.Regex != "" || got.Action != "" || got.Modulus != 0 {
		t.Errorf("expected zero values, got %+v", got)
	}
}
