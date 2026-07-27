package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BlackboxModuleSpec defines the desired state of BlackboxModule.
// Exactly one prober configuration must be set. The prober type is inferred
// from which section is present.
type BlackboxModuleSpec struct {
	// timeout is the probe timeout.
	// +optional
	// +kubebuilder:default="5s"
	Timeout string `json:"timeout,omitempty"`

	// http configures an HTTP prober.
	// +optional
	HTTP *HTTPProbeConfig `json:"http,omitempty"`

	// tcp configures a TCP prober.
	// +optional
	TCP *TCPProbeConfig `json:"tcp,omitempty"`

	// dns configures a DNS prober.
	// +optional
	DNS *DNSProbeConfig `json:"dns,omitempty"`

	// icmp configures an ICMP prober.
	// Requires CAP_NET_RAW which is not available under the restricted Pod Security Standard.
	// +optional
	ICMP *ICMPProbeConfig `json:"icmp,omitempty"`

	// grpc configures a gRPC prober.
	// +optional
	GRPC *GRPCProbeConfig `json:"grpc,omitempty"`

	// unix configures a Unix socket prober.
	// +optional
	Unix *UnixProbeConfig `json:"unix,omitempty"`

	// websocket configures a WebSocket prober.
	// +optional
	Websocket *WebsocketProbeConfig `json:"websocket,omitempty"`
}

// HTTPProbeConfig configures an HTTP probe.
type HTTPProbeConfig struct {
	// method is the HTTP method to use.
	// +optional
	// +kubebuilder:default="GET"
	Method string `json:"method,omitempty"`

	// headers are additional HTTP headers to send.
	// +optional
	Headers map[string]string `json:"headers,omitempty"`

	// body is the request body for POST/PUT requests.
	// +optional
	Body string `json:"body,omitempty"`

	// validStatusCodes is a list of accepted HTTP status codes. Defaults to 2xx.
	// +optional
	ValidStatusCodes []int `json:"validStatusCodes,omitempty"`

	// validHTTPVersions is a list of accepted HTTP versions.
	// +optional
	ValidHTTPVersions []string `json:"validHTTPVersions,omitempty"`

	// followRedirects controls whether to follow HTTP redirects.
	// +optional
	// +kubebuilder:default=true
	FollowRedirects *bool `json:"followRedirects,omitempty"`

	// failIfSSL fails the probe if the response is served over SSL.
	// +optional
	FailIfSSL bool `json:"failIfSSL,omitempty"`

	// failIfNotSSL fails the probe if the response is not served over SSL.
	// +optional
	FailIfNotSSL bool `json:"failIfNotSSL,omitempty"`

	// failIfBodyMatchesRegexp fails the probe if the body matches any of these regexps.
	// +optional
	FailIfBodyMatchesRegexp []string `json:"failIfBodyMatchesRegexp,omitempty"`

	// failIfBodyNotMatchesRegexp fails the probe if the body does not match any of these regexps.
	// +optional
	FailIfBodyNotMatchesRegexp []string `json:"failIfBodyNotMatchesRegexp,omitempty"`

	// failIfHeaderMatchesRegexp fails the probe if a header matches.
	// +optional
	FailIfHeaderMatchesRegexp []HeaderMatchConfig `json:"failIfHeaderMatchesRegexp,omitempty"`

	// failIfHeaderNotMatchesRegexp fails the probe if a header does not match.
	// +optional
	FailIfHeaderNotMatchesRegexp []HeaderMatchConfig `json:"failIfHeaderNotMatchesRegexp,omitempty"`

	// preferredIPProtocol is the preferred IP protocol version.
	// +optional
	// +kubebuilder:validation:Enum=ip4;ip6
	PreferredIPProtocol string `json:"preferredIPProtocol,omitempty"`

	// tlsConfig configures TLS for the HTTP request.
	// +optional
	TLSConfig *TLSConfig `json:"tlsConfig,omitempty"`

	// basicAuth configures HTTP basic authentication.
	// +optional
	BasicAuth *BasicAuth `json:"basicAuth,omitempty"`

	// oauth2 configures OAuth2 client credentials.
	// +optional
	OAuth2 *OAuth2Config `json:"oauth2,omitempty"`

	// proxyURL is the HTTP proxy to use.
	// +optional
	ProxyURL string `json:"proxyURL,omitempty"`

	// bodySizeLimit limits the response body size in bytes. 0 means no limit.
	// +optional
	BodySizeLimit int64 `json:"bodySizeLimit,omitempty"`
}

// HeaderMatchConfig configures HTTP header matching.
type HeaderMatchConfig struct {
	// header is the HTTP header name.
	// +required
	Header string `json:"header"`

	// regexp is the regular expression to match against.
	// +required
	Regexp string `json:"regexp"`

	// allowMissing allows the header to be absent.
	// +optional
	AllowMissing bool `json:"allowMissing,omitempty"`
}

// TCPProbeConfig configures a TCP probe.
type TCPProbeConfig struct {
	// queryResponse defines the expected query-response pairs.
	// +optional
	QueryResponse []QueryResponseConfig `json:"queryResponse,omitempty"`

	// tls enables TLS for the TCP connection.
	// +optional
	TLS bool `json:"tls,omitempty"`

	// tlsConfig configures TLS settings.
	// +optional
	TLSConfig *TLSConfig `json:"tlsConfig,omitempty"`

	// preferredIPProtocol is the preferred IP protocol version.
	// +optional
	// +kubebuilder:validation:Enum=ip4;ip6
	PreferredIPProtocol string `json:"preferredIPProtocol,omitempty"`
}

// QueryResponseConfig defines a TCP/Unix query-response pair.
type QueryResponseConfig struct {
	// send is the string to send.
	// +optional
	Send string `json:"send,omitempty"`

	// expect is the regular expression to match the response against.
	// +optional
	Expect string `json:"expect,omitempty"`

	// startTLS upgrades the connection to TLS after this exchange.
	// +optional
	StartTLS bool `json:"startTLS,omitempty"`
}

// DNSProbeConfig configures a DNS probe.
type DNSProbeConfig struct {
	// queryName is the DNS name to query.
	// +required
	QueryName string `json:"queryName"`

	// queryType is the DNS query type.
	// +optional
	// +kubebuilder:default="A"
	// +kubebuilder:validation:Enum=A;AAAA;CNAME;MX;NS;SOA;TXT;SRV;PTR
	QueryType string `json:"queryType,omitempty"`

	// queryClass is the DNS query class.
	// +optional
	// +kubebuilder:default="IN"
	QueryClass string `json:"queryClass,omitempty"`

	// recursionDesired sets the RD flag in DNS queries.
	// +optional
	// +kubebuilder:default=true
	RecursionDesired *bool `json:"recursionDesired,omitempty"`

	// validRcodes is a list of acceptable DNS response codes.
	// +optional
	ValidRcodes []string `json:"validRcodes,omitempty"`

	// validateAnswer defines validation rules for the answer section.
	// +optional
	ValidateAnswer *DNSRRValidator `json:"validateAnswer,omitempty"`

	// validateAuthority defines validation rules for the authority section.
	// +optional
	ValidateAuthority *DNSRRValidator `json:"validateAuthority,omitempty"`

	// validateAdditional defines validation rules for the additional section.
	// +optional
	ValidateAdditional *DNSRRValidator `json:"validateAdditional,omitempty"`

	// transportProtocol is the DNS transport protocol.
	// +optional
	// +kubebuilder:default="udp"
	// +kubebuilder:validation:Enum=udp;tcp
	TransportProtocol string `json:"transportProtocol,omitempty"`

	// dnsOverTLS enables DNS over TLS.
	// +optional
	DNSOverTLS bool `json:"dnsOverTLS,omitempty"`

	// preferredIPProtocol is the preferred IP protocol version.
	// +optional
	// +kubebuilder:validation:Enum=ip4;ip6
	PreferredIPProtocol string `json:"preferredIPProtocol,omitempty"`
}

// DNSRRValidator defines validation rules for DNS resource record sections.
type DNSRRValidator struct {
	// failIfMatchesRegexp fails the probe if any entry matches these regexps.
	// +optional
	FailIfMatchesRegexp []string `json:"failIfMatchesRegexp,omitempty"`

	// failIfNotMatchesRegexp fails the probe if no entry matches these regexps.
	// +optional
	FailIfNotMatchesRegexp []string `json:"failIfNotMatchesRegexp,omitempty"`
}

// ICMPProbeConfig configures an ICMP probe.
type ICMPProbeConfig struct {
	// preferredIPProtocol is the preferred IP protocol version.
	// +optional
	// +kubebuilder:default="ip4"
	// +kubebuilder:validation:Enum=ip4;ip6
	PreferredIPProtocol string `json:"preferredIPProtocol,omitempty"`

	// payloadSize is the ICMP payload size in bytes.
	// +optional
	PayloadSize int `json:"payloadSize,omitempty"`

	// dontFragment sets the DF bit in the IP header.
	// +optional
	DontFragment bool `json:"dontFragment,omitempty"`

	// ttl sets the IP Time To Live.
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=255
	TTL int `json:"ttl,omitempty"`

	// sourceIPAddress is the source IP address for ICMP packets.
	// +optional
	SourceIPAddress string `json:"sourceIPAddress,omitempty"`
}

// GRPCProbeConfig configures a gRPC probe.
type GRPCProbeConfig struct {
	// service is the gRPC service name to check.
	// +optional
	Service string `json:"service,omitempty"`

	// tls enables TLS for the gRPC connection.
	// +optional
	TLS bool `json:"tls,omitempty"`

	// tlsConfig configures TLS settings.
	// +optional
	TLSConfig *TLSConfig `json:"tlsConfig,omitempty"`

	// preferredIPProtocol is the preferred IP protocol version.
	// +optional
	// +kubebuilder:validation:Enum=ip4;ip6
	PreferredIPProtocol string `json:"preferredIPProtocol,omitempty"`
}

// UnixProbeConfig configures a Unix socket probe.
type UnixProbeConfig struct {
	// queryResponse defines the expected query-response pairs.
	// +optional
	QueryResponse []QueryResponseConfig `json:"queryResponse,omitempty"`
}

// WebsocketProbeConfig configures a WebSocket probe.
type WebsocketProbeConfig struct {
	// headers are additional HTTP headers for the WebSocket handshake.
	// +optional
	Headers map[string]string `json:"headers,omitempty"`
}

// BlackboxModuleStatus defines the observed state of BlackboxModule.
type BlackboxModuleStatus struct {
	// conditions represent the current state of the BlackboxModule resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// referencedByExporters lists the BlackboxExporters that include this module.
	// +optional
	ReferencedByExporters []NamespacedReference `json:"referencedByExporters,omitempty"`

	// observedGeneration is the last observed .metadata.generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Prober",type=string,JSONPath=`.status.prober`,priority=0
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// BlackboxModule is the Schema for the blackboxmodules API.
type BlackboxModule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   BlackboxModuleSpec   `json:"spec"`
	Status BlackboxModuleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BlackboxModuleList contains a list of BlackboxModule.
type BlackboxModuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BlackboxModule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BlackboxModule{}, &BlackboxModuleList{})
}
