package converter

import (
	"fmt"
	"net/url"
	"time"

	bbconfig "github.com/prometheus/blackbox_exporter/config"
	promconfig "github.com/prometheus/common/config"
	"go.yaml.in/yaml/v3"

	monitoringv1alpha1 "github.com/sebastiangaiser/blackbox-exporter-operator/api/v1alpha1"
)

// Upstream blackbox-exporter prober identifiers.
const (
	proberHTTP = "http"
	proberTCP  = "tcp"
	proberDNS  = "dns"
	proberICMP = "icmp"
	proberGRPC = "grpc"
)

// ModuleName returns the unique module name for a BlackboxModule in the rendered blackbox.yml.
func ModuleName(namespace, name string) string {
	return fmt.Sprintf("%s-%s", namespace, name)
}

// ConvertModule converts a BlackboxModuleSpec into an upstream blackbox-exporter config.Module.
// If secrets is non-nil, resolved secret values are injected into the config.
func ConvertModule(spec *monitoringv1alpha1.BlackboxModuleSpec, secrets *ResolvedSecrets) (bbconfig.Module, error) {
	module := bbconfig.Module{}

	timeout, err := time.ParseDuration(spec.Timeout)
	if err != nil {
		return module, fmt.Errorf("invalid timeout %q: %w", spec.Timeout, err)
	}
	module.Timeout = timeout

	switch {
	case spec.HTTP != nil:
		module.Prober = proberHTTP
		module.HTTP = convertHTTPProbe(spec.HTTP, secrets)
	case spec.TCP != nil:
		module.Prober = proberTCP
		module.TCP = convertTCPProbe(spec.TCP, secrets)
	case spec.DNS != nil:
		module.Prober = proberDNS
		module.DNS = convertDNSProbe(spec.DNS)
	case spec.ICMP != nil:
		module.Prober = proberICMP
		module.ICMP = convertICMPProbe(spec.ICMP)
	case spec.GRPC != nil:
		module.Prober = proberGRPC
		module.GRPC = convertGRPCProbe(spec.GRPC, secrets)
	case spec.Unix != nil:
		module.Prober = proberTCP // unix uses tcp prober upstream
		module.Unix = convertUnixProbe(spec.Unix)
	default:
		return module, fmt.Errorf("no prober configuration set")
	}

	return module, nil
}

// RenderConfig renders a map of modules into a blackbox.yml YAML byte slice.
func RenderConfig(modules map[string]bbconfig.Module) ([]byte, error) {
	cfg := bbconfig.Config{
		Modules: modules,
	}
	return yaml.Marshal(cfg)
}

func convertHTTPProbe(spec *monitoringv1alpha1.HTTPProbeConfig, secrets *ResolvedSecrets) bbconfig.HTTPProbe {
	probe := bbconfig.DefaultHTTPProbe

	if spec.Method != "" {
		probe.Method = spec.Method
	}
	if spec.Headers != nil {
		probe.Headers = spec.Headers
	}
	if spec.Body != "" {
		probe.Body = spec.Body
	}
	if len(spec.ValidStatusCodes) > 0 {
		probe.ValidStatusCodes = spec.ValidStatusCodes
	}
	if len(spec.ValidHTTPVersions) > 0 {
		probe.ValidHTTPVersions = spec.ValidHTTPVersions
	}
	if spec.FollowRedirects != nil {
		probe.HTTPClientConfig.FollowRedirects = *spec.FollowRedirects
	}
	probe.FailIfSSL = spec.FailIfSSL
	probe.FailIfNotSSL = spec.FailIfNotSSL

	if len(spec.FailIfBodyMatchesRegexp) > 0 {
		for _, r := range spec.FailIfBodyMatchesRegexp {
			probe.FailIfBodyMatchesRegexp = append(probe.FailIfBodyMatchesRegexp, bbconfig.MustNewRegexp(r))
		}
	}
	if len(spec.FailIfBodyNotMatchesRegexp) > 0 {
		for _, r := range spec.FailIfBodyNotMatchesRegexp {
			probe.FailIfBodyNotMatchesRegexp = append(probe.FailIfBodyNotMatchesRegexp, bbconfig.MustNewRegexp(r))
		}
	}
	if len(spec.FailIfHeaderMatchesRegexp) > 0 {
		for _, hm := range spec.FailIfHeaderMatchesRegexp {
			probe.FailIfHeaderMatchesRegexp = append(probe.FailIfHeaderMatchesRegexp, bbconfig.HeaderMatch{
				Header:       hm.Header,
				Regexp:       bbconfig.MustNewRegexp(hm.Regexp),
				AllowMissing: hm.AllowMissing,
			})
		}
	}
	if len(spec.FailIfHeaderNotMatchesRegexp) > 0 {
		for _, hm := range spec.FailIfHeaderNotMatchesRegexp {
			probe.FailIfHeaderNotMatchesRegexp = append(probe.FailIfHeaderNotMatchesRegexp, bbconfig.HeaderMatch{
				Header:       hm.Header,
				Regexp:       bbconfig.MustNewRegexp(hm.Regexp),
				AllowMissing: hm.AllowMissing,
			})
		}
	}

	if spec.PreferredIPProtocol != "" {
		probe.IPProtocol = spec.PreferredIPProtocol
	}

	if spec.TLSConfig != nil {
		probe.HTTPClientConfig.TLSConfig = convertTLSConfig(spec.TLSConfig, secrets)
	}

	if spec.ProxyURL != "" {
		if parsed, err := url.Parse(spec.ProxyURL); err == nil {
			probe.HTTPClientConfig.ProxyURL = promconfig.URL{URL: parsed}
		}
	}

	if spec.BasicAuth != nil {
		probe.HTTPClientConfig.BasicAuth = &promconfig.BasicAuth{
			Username: spec.BasicAuth.Username,
		}
		if secrets != nil && secrets.BasicAuthPassword != "" {
			probe.HTTPClientConfig.BasicAuth.Password = promconfig.Secret(secrets.BasicAuthPassword)
		}
	}

	if spec.OAuth2 != nil {
		probe.HTTPClientConfig.OAuth2 = &promconfig.OAuth2{
			ClientID: spec.OAuth2.ClientID,
			TokenURL: spec.OAuth2.TokenURL,
			Scopes:   spec.OAuth2.Scopes,
		}
		if secrets != nil && secrets.OAuth2ClientSecret != "" {
			probe.HTTPClientConfig.OAuth2.ClientSecret = promconfig.Secret(secrets.OAuth2ClientSecret)
		}
	}

	return probe
}

func convertTCPProbe(spec *monitoringv1alpha1.TCPProbeConfig, secrets *ResolvedSecrets) bbconfig.TCPProbe {
	probe := bbconfig.DefaultTCPProbe

	probe.TLS = spec.TLS
	if spec.PreferredIPProtocol != "" {
		probe.IPProtocol = spec.PreferredIPProtocol
	}
	if spec.TLSConfig != nil {
		probe.TLSConfig = convertTLSConfig(spec.TLSConfig, secrets)
	}

	for _, qr := range spec.QueryResponse {
		entry := bbconfig.QueryResponse{
			Send:     qr.Send,
			StartTLS: qr.StartTLS,
		}
		if qr.Expect != "" {
			entry.Expect = bbconfig.MustNewRegexp(qr.Expect)
		}
		probe.QueryResponse = append(probe.QueryResponse, entry)
	}

	return probe
}

func convertDNSProbe(spec *monitoringv1alpha1.DNSProbeConfig) bbconfig.DNSProbe {
	probe := bbconfig.DefaultDNSProbe

	probe.QueryName = spec.QueryName
	if spec.QueryType != "" {
		probe.QueryType = spec.QueryType
	}
	if spec.QueryClass != "" {
		probe.QueryClass = spec.QueryClass
	}
	if spec.RecursionDesired != nil {
		probe.Recursion = *spec.RecursionDesired
	}
	if len(spec.ValidRcodes) > 0 {
		probe.ValidRcodes = spec.ValidRcodes
	}
	if spec.TransportProtocol != "" {
		probe.TransportProtocol = spec.TransportProtocol
	}
	probe.DNSOverTLS = spec.DNSOverTLS
	if spec.PreferredIPProtocol != "" {
		probe.IPProtocol = spec.PreferredIPProtocol
	}

	if spec.ValidateAnswer != nil {
		probe.ValidateAnswer = convertDNSRRValidator(spec.ValidateAnswer)
	}
	if spec.ValidateAuthority != nil {
		probe.ValidateAuthority = convertDNSRRValidator(spec.ValidateAuthority)
	}
	if spec.ValidateAdditional != nil {
		probe.ValidateAdditional = convertDNSRRValidator(spec.ValidateAdditional)
	}

	return probe
}

func convertICMPProbe(spec *monitoringv1alpha1.ICMPProbeConfig) bbconfig.ICMPProbe {
	probe := bbconfig.DefaultICMPProbe

	if spec.PreferredIPProtocol != "" {
		probe.IPProtocol = spec.PreferredIPProtocol
	}
	if spec.PayloadSize > 0 {
		probe.PayloadSize = spec.PayloadSize
	}
	probe.DontFragment = spec.DontFragment
	if spec.TTL > 0 {
		probe.TTL = spec.TTL
	}
	if spec.SourceIPAddress != "" {
		probe.SourceIPAddress = spec.SourceIPAddress
	}

	return probe
}

func convertGRPCProbe(spec *monitoringv1alpha1.GRPCProbeConfig, secrets *ResolvedSecrets) bbconfig.GRPCProbe {
	probe := bbconfig.DefaultGRPCProbe

	probe.Service = spec.Service
	probe.TLS = spec.TLS
	if spec.PreferredIPProtocol != "" {
		probe.PreferredIPProtocol = spec.PreferredIPProtocol
	}
	if spec.TLSConfig != nil {
		probe.TLSConfig = convertTLSConfig(spec.TLSConfig, secrets)
	}

	return probe
}

func convertUnixProbe(spec *monitoringv1alpha1.UnixProbeConfig) bbconfig.UnixProbe {
	probe := bbconfig.UnixProbe{}

	for _, qr := range spec.QueryResponse {
		entry := bbconfig.QueryResponse{
			Send:     qr.Send,
			StartTLS: qr.StartTLS,
		}
		if qr.Expect != "" {
			entry.Expect = bbconfig.MustNewRegexp(qr.Expect)
		}
		probe.QueryResponse = append(probe.QueryResponse, entry)
	}

	return probe
}

var tlsVersionMap = map[string]promconfig.TLSVersion{
	"TLS10": promconfig.TLSVersion(0x0301),
	"TLS11": promconfig.TLSVersion(0x0302),
	"TLS12": promconfig.TLSVersion(0x0303),
	"TLS13": promconfig.TLSVersion(0x0304),
}

func convertTLSConfig(spec *monitoringv1alpha1.TLSConfig, secrets *ResolvedSecrets) promconfig.TLSConfig {
	tlsCfg := promconfig.TLSConfig{
		InsecureSkipVerify: spec.InsecureSkipVerify,
		ServerName:         spec.ServerName,
	}
	if v, ok := tlsVersionMap[spec.MinVersion]; ok {
		tlsCfg.MinVersion = v
	}
	if v, ok := tlsVersionMap[spec.MaxVersion]; ok {
		tlsCfg.MaxVersion = v
	}
	if secrets != nil {
		if secrets.TLSCA != "" {
			tlsCfg.CA = secrets.TLSCA
		}
		if secrets.TLSCert != "" {
			tlsCfg.Cert = secrets.TLSCert
		}
		if secrets.TLSKey != "" {
			tlsCfg.Key = promconfig.Secret(secrets.TLSKey)
		}
	}
	return tlsCfg
}

func convertDNSRRValidator(spec *monitoringv1alpha1.DNSRRValidator) bbconfig.DNSRRValidator {
	return bbconfig.DNSRRValidator{
		FailIfMatchesRegexp:    spec.FailIfMatchesRegexp,
		FailIfNotMatchesRegexp: spec.FailIfNotMatchesRegexp,
	}
}
