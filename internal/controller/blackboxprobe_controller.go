package controller

import (
	"context"
	"fmt"
	"maps"
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	monitoringv1alpha1 "github.com/sebastiangaiser/blackbox-exporter-operator/api/v1alpha1"
	"github.com/sebastiangaiser/blackbox-exporter-operator/internal/converter"
)

// BlackboxProbeReconciler reconciles a BlackboxProbe object.
type BlackboxProbeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=monitoring.gaiser.bayern,resources=blackboxprobes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.gaiser.bayern,resources=blackboxprobes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=monitoring.gaiser.bayern,resources=blackboxprobes/finalizers,verbs=update
// +kubebuilder:rbac:groups=monitoring.gaiser.bayern,resources=blackboxexporters,verbs=get;list;watch
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=probes,verbs=get;list;watch;create;update;patch;delete

func (r *BlackboxProbeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	probe := &monitoringv1alpha1.BlackboxProbe{}
	if err := r.Get(ctx, req.NamespacedName, probe); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Resolve exporterRef.
	exporterNS := probe.Namespace
	if probe.Spec.ExporterRef.Namespace != "" {
		exporterNS = probe.Spec.ExporterRef.Namespace
	}
	exporter := &monitoringv1alpha1.BlackboxExporter{}
	if err := r.Get(ctx, types.NamespacedName{Name: probe.Spec.ExporterRef.Name, Namespace: exporterNS}, exporter); err != nil {
		log.Error(err, "failed to resolve exporterRef")
		setCondition(&probe.Status.Conditions, metav1.Condition{
			Type:               conditionTypeReady,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: probe.Generation,
			Reason:             "ExporterNotFound",
			Message:            fmt.Sprintf("BlackboxExporter %s/%s not found", exporterNS, probe.Spec.ExporterRef.Name),
		})
		probe.Status.ObservedGeneration = probe.Generation
		_ = r.Status().Update(ctx, probe)
		return ctrl.Result{}, err
	}

	// Resolve moduleRef.
	moduleNS := probe.Namespace
	if probe.Spec.ModuleRef.Namespace != "" {
		moduleNS = probe.Spec.ModuleRef.Namespace
	}
	module := &monitoringv1alpha1.BlackboxModule{}
	if err := r.Get(ctx, types.NamespacedName{Name: probe.Spec.ModuleRef.Name, Namespace: moduleNS}, module); err != nil {
		log.Error(err, "failed to resolve moduleRef")
		setCondition(&probe.Status.Conditions, metav1.Condition{
			Type:               conditionTypeReady,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: probe.Generation,
			Reason:             "ModuleNotFound",
			Message:            fmt.Sprintf("BlackboxModule %s/%s not found", moduleNS, probe.Spec.ModuleRef.Name),
		})
		probe.Status.ObservedGeneration = probe.Generation
		_ = r.Status().Update(ctx, probe)
		return ctrl.Result{}, err
	}

	// Build the prometheus-operator Probe CR.
	moduleName := converter.ModuleName(module.Namespace, module.Name)
	exporterPort := int32(9115)
	if exporter.Spec.Port != 0 {
		exporterPort = exporter.Spec.Port
	}
	proberURL := fmt.Sprintf("%s.%s.svc.cluster.local:%d",
		resourceName(exporter.Name), exporter.Namespace, exporterPort)

	interval := promv1.Duration("60s")
	if probe.Spec.Interval != "" {
		interval = promv1.Duration(probe.Spec.Interval)
	}
	scrapeTimeout := promv1.Duration("10s")
	if probe.Spec.ScrapeTimeout != "" {
		scrapeTimeout = promv1.Duration(probe.Spec.ScrapeTimeout)
	}

	probeLabels, probeAnnotations := buildProbeMetadata(probe, exporter.Name, module.Name)
	probeTargets := buildProbeTargets(probe)

	promProbe := &promv1.Probe{
		ObjectMeta: metav1.ObjectMeta{
			Name:        probe.Name,
			Namespace:   probe.Namespace,
			Labels:      probeLabels,
			Annotations: probeAnnotations,
		},
		Spec: promv1.ProbeSpec{
			ProberSpec: promv1.ProberSpec{
				URL:    proberURL,
				Scheme: schemePtr("http"),
				Path:   "/probe",
			},
			Module:        moduleName,
			Interval:      interval,
			ScrapeTimeout: scrapeTimeout,
			Targets:       probeTargets,
		},
	}

	// Convert metric relabelings.
	for _, rl := range probe.Spec.MetricRelabelings {
		rc := convertRelabelConfig(rl)
		promProbe.Spec.MetricRelabelConfigs = append(promProbe.Spec.MetricRelabelConfigs, *rc)
	}

	if err := controllerutil.SetControllerReference(probe, promProbe, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	// Create or update the Probe CR.
	existing := &promv1.Probe{}
	err := r.Get(ctx, types.NamespacedName{Name: promProbe.Name, Namespace: promProbe.Namespace}, existing)
	if apierrors.IsNotFound(err) {
		if err := r.Create(ctx, promProbe); err != nil {
			return ctrl.Result{}, err
		}
	} else if err != nil {
		return ctrl.Result{}, err
	} else {
		promProbe.SetResourceVersion(existing.GetResourceVersion())
		if err := r.Update(ctx, promProbe); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Update status.
	probe.Status.TargetCount = int32(len(probe.Spec.Targets))
	probe.Status.ProbeRef = &monitoringv1alpha1.NamespacedReference{
		Name:      promProbe.Name,
		Namespace: promProbe.Namespace,
	}
	probe.Status.ObservedGeneration = probe.Generation
	setCondition(&probe.Status.Conditions, metav1.Condition{
		Type:               conditionTypeReady,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: probe.Generation,
		Reason:             "ProbeCreated",
		Message:            "prometheus-operator Probe CR created",
	})

	if err := r.Status().Update(ctx, probe); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// deprecatedAdditionalLabels reads spec.additionalLabels. The field is deprecated in
// favour of probeMetadata and targetLabels, but is still honoured so that resources
// created before those fields existed keep behaving the same. Reading it is confined to
// this helper to keep the deprecation warning in one place.
//
//nolint:staticcheck // reading the deprecated field is the point of this helper
func deprecatedAdditionalLabels(probe *monitoringv1alpha1.BlackboxProbe) map[string]string {
	return probe.Spec.AdditionalLabels
}

// buildProbeMetadata renders labels and annotations for the generated Probe CR.
// The deprecated additionalLabels form the base, probeMetadata refines it, and the
// reserved labels are applied last so that neither can override the operator's own
// bookkeeping.
func buildProbeMetadata(probe *monitoringv1alpha1.BlackboxProbe, exporterName, moduleName string) (map[string]string, map[string]string) {
	labels := map[string]string{}
	var annotations map[string]string

	maps.Copy(labels, deprecatedAdditionalLabels(probe))

	if probe.Spec.ProbeMetadata != nil {
		maps.Copy(labels, probe.Spec.ProbeMetadata.Labels)
		annotations = maps.Clone(probe.Spec.ProbeMetadata.Annotations)
	}

	labels["app.kubernetes.io/managed-by"] = "blackbox-exporter-operator"
	labels["monitoring.gaiser.bayern/exporter"] = exporterName
	labels["monitoring.gaiser.bayern/module"] = moduleName

	return labels, annotations
}

// buildProbeTargets renders the target configuration for the generated Probe CR.
func buildProbeTargets(probe *monitoringv1alpha1.BlackboxProbe) promv1.ProbeTargets {
	targets := promv1.ProbeTargets{}

	if len(probe.Spec.Targets) > 0 {
		// The deprecated additionalLabels form the base, targetLabels refine it.
		additional := deprecatedAdditionalLabels(probe)
		var labels map[string]string
		if len(additional) > 0 || len(probe.Spec.TargetLabels) > 0 {
			labels = make(map[string]string, len(additional)+len(probe.Spec.TargetLabels))
			maps.Copy(labels, additional)
			maps.Copy(labels, probe.Spec.TargetLabels)
		}
		targets.StaticConfig = &promv1.ProbeTargetStaticConfig{
			Targets: slices.Clone(probe.Spec.Targets),
			Labels:  labels,
		}
	}

	if probe.Spec.Ingress == nil {
		return targets
	}

	ingress := &promv1.ProbeTargetIngress{Selector: probe.Spec.Ingress.Selector}
	if probe.Spec.Ingress.NamespaceSelector.Any {
		ingress.NamespaceSelector = promv1.NamespaceSelector{Any: true}
	} else if len(probe.Spec.Ingress.NamespaceSelector.MatchNames) > 0 {
		ingress.NamespaceSelector = promv1.NamespaceSelector{
			MatchNames: probe.Spec.Ingress.NamespaceSelector.MatchNames,
		}
	}

	// Ingress targets have no label field upstream, so labels become replace
	// relabelings. Keys are sorted to keep the generated spec stable.
	for _, k := range slices.Sorted(maps.Keys(probe.Spec.Ingress.Labels)) {
		value := probe.Spec.Ingress.Labels[k]
		ingress.RelabelConfigs = append(ingress.RelabelConfigs, promv1.RelabelConfig{
			TargetLabel: k,
			Replacement: &value,
			Action:      "replace",
		})
	}
	for _, rl := range probe.Spec.Ingress.RelabelConfigs {
		ingress.RelabelConfigs = append(ingress.RelabelConfigs, *convertRelabelConfig(rl))
	}
	targets.Ingress = ingress

	return targets
}

func convertRelabelConfig(rl monitoringv1alpha1.RelabelConfig) *promv1.RelabelConfig {
	rc := &promv1.RelabelConfig{}

	for _, sl := range rl.SourceLabels {
		rc.SourceLabels = append(rc.SourceLabels, promv1.LabelName(sl))
	}
	if rl.Separator != "" {
		rc.Separator = &rl.Separator
	}
	if rl.TargetLabel != "" {
		rc.TargetLabel = rl.TargetLabel
	}
	if rl.Regex != "" {
		rc.Regex = rl.Regex
	}
	if rl.Replacement != "" {
		rc.Replacement = &rl.Replacement
	}
	if rl.Action != "" {
		rc.Action = rl.Action
	}
	if rl.Modulus != 0 {
		rc.Modulus = int64(rl.Modulus)
	}

	return rc
}

func schemePtr(s string) *promv1.Scheme {
	scheme := promv1.Scheme(s)
	return &scheme
}

// SetupWithManager sets up the controller with the Manager.
func (r *BlackboxProbeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&monitoringv1alpha1.BlackboxProbe{}).
		Owns(&promv1.Probe{}).
		Named("blackboxprobe").
		Complete(r)
}
