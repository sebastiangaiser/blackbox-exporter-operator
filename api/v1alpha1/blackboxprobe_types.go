package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BlackboxProbeSpec defines the desired state of BlackboxProbe.
type BlackboxProbeSpec struct {
	// exporterRef references the BlackboxExporter instance to use.
	// +required
	ExporterRef NamespacedReference `json:"exporterRef"`

	// moduleRef references the BlackboxModule to use for probing.
	// +required
	ModuleRef NamespacedReference `json:"moduleRef"`

	// targets is a list of static endpoints to probe (URLs or host:port).
	// At least one of targets or ingress must be set.
	// +optional
	Targets []string `json:"targets,omitempty"`

	// ingress configures target discovery from Ingress resources.
	// The operator configures a target for each host of matching Ingress objects.
	// At least one of targets or ingress must be set.
	// +optional
	Ingress *IngressTargetConfig `json:"ingress,omitempty"`

	// interval is the scrape interval for the generated Probe CR.
	// +optional
	// +kubebuilder:default="60s"
	Interval string `json:"interval,omitempty"`

	// scrapeTimeout is the scrape timeout. Must be <= interval.
	// +optional
	// +kubebuilder:default="10s"
	ScrapeTimeout string `json:"scrapeTimeout,omitempty"`

	// additionalLabels are added to the generated Probe CR and propagate to scraped metrics.
	// +optional
	AdditionalLabels map[string]string `json:"additionalLabels,omitempty"`

	// metricRelabelings are applied to the generated Probe CR.
	// +optional
	MetricRelabelings []RelabelConfig `json:"metricRelabelings,omitempty"`
}

// BlackboxProbeStatus defines the observed state of BlackboxProbe.
type BlackboxProbeStatus struct {
	// conditions represent the current state of the BlackboxProbe resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// targetCount is the number of configured targets.
	// +optional
	TargetCount int32 `json:"targetCount,omitempty"`

	// probeRef is a reference to the generated prometheus-operator Probe CR.
	// +optional
	ProbeRef *NamespacedReference `json:"probeRef,omitempty"`

	// observedGeneration is the last observed .metadata.generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Exporter",type=string,JSONPath=`.spec.exporterRef.name`
// +kubebuilder:printcolumn:name="Module",type=string,JSONPath=`.spec.moduleRef.name`
// +kubebuilder:printcolumn:name="Targets",type=integer,JSONPath=`.status.targetCount`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// BlackboxProbe is the Schema for the blackboxprobes API.
type BlackboxProbe struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   BlackboxProbeSpec   `json:"spec"`
	Status BlackboxProbeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BlackboxProbeList contains a list of BlackboxProbe.
type BlackboxProbeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BlackboxProbe `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BlackboxProbe{}, &BlackboxProbeList{})
}
