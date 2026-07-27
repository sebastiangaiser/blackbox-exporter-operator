package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BlackboxExporterSpec defines the desired state of BlackboxExporter.
type BlackboxExporterSpec struct {
	// replicas is the number of blackbox-exporter pods.
	// +optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	Replicas *int32 `json:"replicas,omitempty"`

	// image defines the blackbox-exporter container image.
	// +optional
	Image ImageSpec `json:"image,omitempty"`

	// resources defines the CPU/memory requests and limits.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// port is the port the blackbox-exporter listens on.
	// +optional
	// +kubebuilder:default=9115
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port,omitempty"`

	// additionalArgs are extra command-line arguments passed to the blackbox-exporter.
	// --config.file and --config.enable-auto-reload are set automatically.
	// +optional
	AdditionalArgs []string `json:"additionalArgs,omitempty"`

	// serviceMonitor configures the optional ServiceMonitor for the exporter's /metrics endpoint.
	// +optional
	ServiceMonitor ServiceMonitorConfig `json:"serviceMonitor,omitempty"`

	// moduleSelector selects BlackboxModule resources to include in this exporter's configuration.
	// +required
	ModuleSelector ModuleSelector `json:"moduleSelector"`

	// nodeSelector constrains scheduling to nodes with matching labels.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// tolerations allow scheduling on tainted nodes.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// affinity defines scheduling affinity rules.
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// additionalLabels are added to all managed resources.
	// +optional
	AdditionalLabels map[string]string `json:"additionalLabels,omitempty"`

	// additionalAnnotations are added to all managed resources.
	// +optional
	AdditionalAnnotations map[string]string `json:"additionalAnnotations,omitempty"`

	// enableICMP grants the CAP_NET_RAW capability to the blackbox-exporter container,
	// which is required for ICMP probes. This downgrades the security context from the
	// restricted Pod Security Standard to baseline.
	// +optional
	EnableICMP bool `json:"enableICMP,omitempty"`
}

// BlackboxExporterStatus defines the observed state of BlackboxExporter.
type BlackboxExporterStatus struct {
	// conditions represent the current state of the BlackboxExporter resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// moduleCount is the number of modules in the current configuration.
	// +optional
	ModuleCount int32 `json:"moduleCount,omitempty"`

	// readyReplicas is the number of ready blackbox-exporter pods.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// observedGeneration is the last observed .metadata.generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Replicas",type=integer,JSONPath=`.spec.replicas`
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.readyReplicas`
// +kubebuilder:printcolumn:name="Modules",type=integer,JSONPath=`.status.moduleCount`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// BlackboxExporter is the Schema for the blackboxexporters API.
type BlackboxExporter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   BlackboxExporterSpec   `json:"spec"`
	Status BlackboxExporterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BlackboxExporterList contains a list of BlackboxExporter.
type BlackboxExporterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BlackboxExporter `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BlackboxExporter{}, &BlackboxExporterList{})
}
