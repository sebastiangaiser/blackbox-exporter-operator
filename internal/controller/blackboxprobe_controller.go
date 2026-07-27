package controller

import (
	"context"
	"fmt"

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

	// Build labels for the generated Probe CR.
	probeLabels := map[string]string{
		"app.kubernetes.io/managed-by":      "blackbox-exporter-operator",
		"monitoring.gaiser.bayern/exporter": exporter.Name,
		"monitoring.gaiser.bayern/module":   module.Name,
	}
	for k, v := range probe.Spec.AdditionalLabels {
		probeLabels[k] = v
	}

	// Build targets.
	probeTargets := promv1.ProbeTargets{}

	if len(probe.Spec.Targets) > 0 {
		var targetLabels map[string]string
		if len(probe.Spec.AdditionalLabels) > 0 {
			targetLabels = probe.Spec.AdditionalLabels
		}
		probeTargets.StaticConfig = &promv1.ProbeTargetStaticConfig{
			Targets: probe.Spec.Targets,
			Labels:  targetLabels,
		}
	}

	if probe.Spec.Ingress != nil {
		ingressTarget := &promv1.ProbeTargetIngress{
			Selector: probe.Spec.Ingress.Selector,
		}
		if probe.Spec.Ingress.NamespaceSelector.Any {
			ingressTarget.NamespaceSelector = promv1.NamespaceSelector{Any: true}
		} else if len(probe.Spec.Ingress.NamespaceSelector.MatchNames) > 0 {
			ingressTarget.NamespaceSelector = promv1.NamespaceSelector{
				MatchNames: probe.Spec.Ingress.NamespaceSelector.MatchNames,
			}
		}
		for _, rl := range probe.Spec.Ingress.RelabelConfigs {
			ingressTarget.RelabelConfigs = append(ingressTarget.RelabelConfigs, *convertRelabelConfig(rl))
		}
		probeTargets.Ingress = ingressTarget
	}

	promProbe := &promv1.Probe{
		ObjectMeta: metav1.ObjectMeta{
			Name:      probe.Name,
			Namespace: probe.Namespace,
			Labels:    probeLabels,
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
		rc.Modulus = rl.Modulus
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
