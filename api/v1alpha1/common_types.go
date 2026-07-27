package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IngressTargetConfig configures target discovery from Ingress resources.
type IngressTargetConfig struct {
	// selector selects Ingress objects by label.
	// +optional
	Selector metav1.LabelSelector `json:"selector,omitempty"`

	// namespaceSelector selects namespaces to discover Ingress objects from.
	// +optional
	NamespaceSelector NamespaceSelector `json:"namespaceSelector,omitempty"`

	// relabelConfigs are applied to the discovered Ingress targets before scraping.
	// Available labels: __tmp_prometheus_ingress_address, __tmp_prometheus_job_name.
	// +optional
	RelabelConfigs []RelabelConfig `json:"relabelConfigs,omitempty"`
}

// NamespacedReference is a reference to a resource in a specific namespace.
type NamespacedReference struct {
	// name of the referenced resource.
	// +required
	Name string `json:"name"`

	// namespace of the referenced resource.
	// Defaults to the namespace of the referencing resource.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// NamespaceSelector selects namespaces by name or match-all.
type NamespaceSelector struct {
	// any selects all namespaces.
	// +optional
	Any bool `json:"any,omitempty"`

	// matchNames is a list of namespace names to select.
	// +optional
	MatchNames []string `json:"matchNames,omitempty"`
}

// ModuleSelector selects BlackboxModule resources across namespaces.
type ModuleSelector struct {
	// namespaceSelector selects namespaces to look for BlackboxModules.
	// +required
	NamespaceSelector NamespaceSelector `json:"namespaceSelector"`

	// matchLabels selects BlackboxModules by label.
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
}

// SecretKeySelector references a key in a Kubernetes Secret.
type SecretKeySelector struct {
	// name of the Secret.
	// +required
	Name string `json:"name"`

	// key within the Secret.
	// +required
	Key string `json:"key"`
}

// TLSConfig configures TLS settings for probers.
type TLSConfig struct {
	// insecureSkipVerify disables TLS certificate verification.
	// +optional
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`

	// caRef references a Secret containing the CA certificate.
	// +optional
	CARef *SecretKeySelector `json:"caRef,omitempty"`

	// certRef references a Secret containing the client certificate.
	// +optional
	CertRef *SecretKeySelector `json:"certRef,omitempty"`

	// keyRef references a Secret containing the client key.
	// +optional
	KeyRef *SecretKeySelector `json:"keyRef,omitempty"`

	// serverName specifies the hostname for SNI.
	// +optional
	ServerName string `json:"serverName,omitempty"`

	// minVersion sets the minimum TLS version.
	// +optional
	// +kubebuilder:validation:Enum=TLS10;TLS11;TLS12;TLS13
	MinVersion string `json:"minVersion,omitempty"`

	// maxVersion sets the maximum TLS version.
	// +optional
	// +kubebuilder:validation:Enum=TLS10;TLS11;TLS12;TLS13
	MaxVersion string `json:"maxVersion,omitempty"`
}

// BasicAuth configures HTTP basic authentication.
type BasicAuth struct {
	// username for basic auth.
	// +required
	Username string `json:"username"`

	// passwordRef references a Secret key containing the password.
	// +required
	PasswordRef SecretKeySelector `json:"passwordRef"`
}

// OAuth2Config configures OAuth2 client credentials.
type OAuth2Config struct {
	// clientID is the OAuth2 client ID.
	// +required
	ClientID string `json:"clientID"`

	// clientSecretRef references a Secret key containing the client secret.
	// +required
	ClientSecretRef SecretKeySelector `json:"clientSecretRef"`

	// tokenURL is the OAuth2 token endpoint.
	// +required
	TokenURL string `json:"tokenURL"`

	// scopes is a list of OAuth2 scopes.
	// +optional
	Scopes []string `json:"scopes,omitempty"`
}

// ServiceMonitorConfig configures the optional ServiceMonitor.
type ServiceMonitorConfig struct {
	// enabled controls whether a ServiceMonitor is created.
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// interval is the scrape interval.
	// +optional
	// +kubebuilder:default="30s"
	Interval string `json:"interval,omitempty"`

	// labels are additional labels added to the ServiceMonitor.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

const (
	// DefaultBlackboxExporterRepository is the default container image repository.
	DefaultBlackboxExporterRepository = "quay.io/prometheus/blackbox-exporter"
	// DefaultBlackboxExporterTag is the default container image tag.
	// renovate: datasource=docker depName=quay.io/prometheus/blackbox-exporter
	DefaultBlackboxExporterTag = "v0.28.0"
)

// ImageSpec defines the container image for the blackbox-exporter.
type ImageSpec struct {
	// repository is the container image repository.
	// +optional
	// +kubebuilder:default="quay.io/prometheus/blackbox-exporter"
	Repository string `json:"repository,omitempty"`

	// tag is the container image tag.
	// +optional
	// +kubebuilder:default="v0.28.0"
	Tag string `json:"tag,omitempty"`

	// pullPolicy is the image pull policy.
	// +optional
	// +kubebuilder:default="IfNotPresent"
	// +kubebuilder:validation:Enum=Always;Never;IfNotPresent
	PullPolicy string `json:"pullPolicy,omitempty"`
}

// RelabelConfig defines metric relabeling configuration.
type RelabelConfig struct {
	// sourceLabels is a list of label names to use as input.
	// +optional
	SourceLabels []string `json:"sourceLabels,omitempty"`

	// separator is the separator used to concatenate source labels.
	// +optional
	// +kubebuilder:default=";"
	Separator string `json:"separator,omitempty"`

	// targetLabel is the label to write the result to.
	// +optional
	TargetLabel string `json:"targetLabel,omitempty"`

	// regex is the regular expression to match against.
	// +optional
	// +kubebuilder:default="(.*)"
	Regex string `json:"regex,omitempty"`

	// replacement is the replacement value for regex matches.
	// +optional
	// +kubebuilder:default="$1"
	Replacement string `json:"replacement,omitempty"`

	// action is the relabeling action to perform.
	// +optional
	// +kubebuilder:default="replace"
	// +kubebuilder:validation:Enum=replace;keep;drop;hashmod;labelmap;labeldrop;labelkeep
	Action string `json:"action,omitempty"`

	// modulus is used with hashmod action.
	// +optional
	Modulus uint64 `json:"modulus,omitempty"`
}
