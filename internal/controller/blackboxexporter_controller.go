package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	bbconfig "github.com/prometheus/blackbox_exporter/config"

	monitoringv1alpha1 "github.com/sebastiangaiser/blackbox-exporter-operator/api/v1alpha1"
	"github.com/sebastiangaiser/blackbox-exporter-operator/internal/converter"
)

const (
	configFileName      = "blackbox.yml"
	configMountPath     = "/etc/blackbox_exporter"
	conditionTypeReady  = "Ready"
	conditionTypeConfig = "ConfigValid"
)

// BlackboxExporterReconciler reconciles a BlackboxExporter object.
type BlackboxExporterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=monitoring.gaiser.bayern,resources=blackboxexporters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.gaiser.bayern,resources=blackboxexporters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=monitoring.gaiser.bayern,resources=blackboxexporters/finalizers,verbs=update
// +kubebuilder:rbac:groups=monitoring.gaiser.bayern,resources=blackboxmodules,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;list;watch;create;update;patch;delete

func (r *BlackboxExporterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	exporter := &monitoringv1alpha1.BlackboxExporter{}
	if err := r.Get(ctx, req.NamespacedName, exporter); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Collect modules matching the selector.
	modules, err := r.collectModules(ctx, exporter)
	if err != nil {
		log.Error(err, "failed to collect modules")
		return ctrl.Result{}, err
	}

	// Convert modules to upstream config, resolving secrets.
	bbModules := make(map[string]bbconfig.Module, len(modules))
	for _, mod := range modules {
		name := converter.ModuleName(mod.Namespace, mod.Name)
		secrets, err := converter.ResolveModuleSecrets(ctx, r.Client, mod.Namespace, &mod.Spec)
		if err != nil {
			log.Error(err, "failed to resolve secrets for module", "module", mod.Name, "namespace", mod.Namespace)
			secrets = &converter.ResolvedSecrets{}
		}
		converted, err := converter.ConvertModule(&mod.Spec, secrets)
		if err != nil {
			log.Error(err, "failed to convert module", "module", mod.Name, "namespace", mod.Namespace)
			continue
		}
		bbModules[name] = converted
	}

	// Render blackbox.yml.
	configYAML, err := converter.RenderConfig(bbModules)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to render config: %w", err)
	}

	// Reconcile owned resources.
	if err := r.reconcileConfigMap(ctx, exporter, configYAML); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.reconcileDeployment(ctx, exporter); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.reconcileService(ctx, exporter); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.reconcileServiceMonitor(ctx, exporter); err != nil {
		return ctrl.Result{}, err
	}

	// Update status.
	exporter.Status.ModuleCount = int32(len(bbModules))
	exporter.Status.ObservedGeneration = exporter.Generation

	// Get deployment readiness.
	deploy := &appsv1.Deployment{}
	deployName := resourceName(exporter.Name)
	if err := r.Get(ctx, types.NamespacedName{Name: deployName, Namespace: exporter.Namespace}, deploy); err == nil {
		exporter.Status.ReadyReplicas = deploy.Status.ReadyReplicas
	}

	// Set Ready condition.
	ready := metav1.ConditionFalse
	reason := "Reconciling"
	message := "Reconciliation in progress"
	if exporter.Status.ReadyReplicas > 0 {
		ready = metav1.ConditionTrue
		reason = "DeploymentAvailable"
		message = "Deployment has minimum availability"
	}
	setCondition(&exporter.Status.Conditions, metav1.Condition{
		Type:               conditionTypeReady,
		Status:             ready,
		ObservedGeneration: exporter.Generation,
		Reason:             reason,
		Message:            message,
	})
	setCondition(&exporter.Status.Conditions, metav1.Condition{
		Type:               conditionTypeConfig,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: exporter.Generation,
		Reason:             "ConfigRendered",
		Message:            fmt.Sprintf("Configuration rendered from %d modules", len(bbModules)),
	})

	if err := r.Status().Update(ctx, exporter); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *BlackboxExporterReconciler) collectModules(ctx context.Context, exporter *monitoringv1alpha1.BlackboxExporter) ([]monitoringv1alpha1.BlackboxModule, error) {
	var allModules monitoringv1alpha1.BlackboxModuleList
	if err := r.List(ctx, &allModules); err != nil {
		return nil, fmt.Errorf("failed to list BlackboxModules: %w", err)
	}

	sel := exporter.Spec.ModuleSelector
	matched := make([]monitoringv1alpha1.BlackboxModule, 0, len(allModules.Items))

	for _, mod := range allModules.Items {
		// Check namespace.
		if !sel.NamespaceSelector.Any {
			if len(sel.NamespaceSelector.MatchNames) > 0 {
				found := false
				for _, ns := range sel.NamespaceSelector.MatchNames {
					if mod.Namespace == ns {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			} else {
				// No namespace selector configured, only match same namespace.
				if mod.Namespace != exporter.Namespace {
					continue
				}
			}
		}

		// Check labels.
		if len(sel.MatchLabels) > 0 {
			match := true
			for k, v := range sel.MatchLabels {
				if mod.Labels[k] != v {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}

		matched = append(matched, mod)
	}

	return matched, nil
}

func (r *BlackboxExporterReconciler) reconcileConfigMap(ctx context.Context, exporter *monitoringv1alpha1.BlackboxExporter, configData []byte) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceName(exporter.Name),
			Namespace: exporter.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, cm, func() error {
		cm.Labels = commonLabels(exporter)
		cm.Data = map[string]string{
			configFileName: string(configData),
		}
		return controllerutil.SetControllerReference(exporter, cm, r.Scheme)
	})
	return err
}

func (r *BlackboxExporterReconciler) reconcileDeployment(ctx context.Context, exporter *monitoringv1alpha1.BlackboxExporter) error {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceName(exporter.Name),
			Namespace: exporter.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, deploy, func() error {
		replicas := int32(1)
		if exporter.Spec.Replicas != nil {
			replicas = *exporter.Spec.Replicas
		}

		port := int32(9115)
		if exporter.Spec.Port != 0 {
			port = exporter.Spec.Port
		}

		repo := exporter.Spec.Image.Repository
		if repo == "" {
			repo = monitoringv1alpha1.DefaultBlackboxExporterRepository
		}
		tag := exporter.Spec.Image.Tag
		if tag == "" {
			tag = monitoringv1alpha1.DefaultBlackboxExporterTag
		}
		image := fmt.Sprintf("%s:%s", repo, tag)

		args := make([]string, 0, 2+len(exporter.Spec.AdditionalArgs))
		args = append(args,
			fmt.Sprintf("--config.file=%s/%s", configMountPath, configFileName),
			"--config.enable-auto-reload",
		)
		args = append(args, exporter.Spec.AdditionalArgs...)

		labels := commonLabels(exporter)
		deploy.Labels = labels
		deploy.Spec.Replicas = &replicas
		deploy.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: selectorLabels(exporter),
		}
		deploy.Spec.Template.Labels = labels
		deploy.Spec.Template.Spec = corev1.PodSpec{
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: ptr.To(true),
				RunAsUser:    ptr.To(int64(65534)),
				RunAsGroup:   ptr.To(int64(65534)),
				FSGroup:      ptr.To(int64(65534)),
				SeccompProfile: &corev1.SeccompProfile{
					Type: corev1.SeccompProfileTypeRuntimeDefault,
				},
			},
			NodeSelector: exporter.Spec.NodeSelector,
			Tolerations:  exporter.Spec.Tolerations,
			Affinity:     exporter.Spec.Affinity,
			Containers: []corev1.Container{
				{
					Name:            "blackbox-exporter",
					Image:           image,
					ImagePullPolicy: corev1.PullPolicy(exporter.Spec.Image.PullPolicy),
					Args:            args,
					Ports: []corev1.ContainerPort{
						{
							Name:          "http",
							ContainerPort: port,
							Protocol:      corev1.ProtocolTCP,
						},
					},
					Resources:       exporter.Spec.Resources,
					SecurityContext: containerSecurityContext(exporter.Spec.EnableICMP),
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "config",
							MountPath: configMountPath,
							ReadOnly:  true,
						},
					},
					LivenessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/-/healthy",
								Port: intstr.FromInt32(port),
							},
						},
					},
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/-/healthy",
								Port: intstr.FromInt32(port),
							},
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "config",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: resourceName(exporter.Name),
							},
						},
					},
				},
			},
		}
		return controllerutil.SetControllerReference(exporter, deploy, r.Scheme)
	})
	return err
}

func (r *BlackboxExporterReconciler) reconcileService(ctx context.Context, exporter *monitoringv1alpha1.BlackboxExporter) error {
	port := int32(9115)
	if exporter.Spec.Port != 0 {
		port = exporter.Spec.Port
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceName(exporter.Name),
			Namespace: exporter.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		svc.Labels = commonLabels(exporter)
		svc.Spec.Selector = selectorLabels(exporter)
		svc.Spec.Ports = []corev1.ServicePort{
			{
				Name:       "http",
				Port:       port,
				TargetPort: intstr.FromString("http"),
				Protocol:   corev1.ProtocolTCP,
			},
		}
		return controllerutil.SetControllerReference(exporter, svc, r.Scheme)
	})
	return err
}

func (r *BlackboxExporterReconciler) reconcileServiceMonitor(ctx context.Context, exporter *monitoringv1alpha1.BlackboxExporter) error {
	name := resourceName(exporter.Name)
	smKey := types.NamespacedName{Name: name, Namespace: exporter.Namespace}

	if !exporter.Spec.ServiceMonitor.Enabled {
		// Delete ServiceMonitor if it exists but is no longer wanted.
		existing := &promv1.ServiceMonitor{}
		if err := r.Get(ctx, smKey, existing); err == nil {
			return r.Delete(ctx, existing)
		}
		return nil
	}

	interval := "30s"
	if exporter.Spec.ServiceMonitor.Interval != "" {
		interval = exporter.Spec.ServiceMonitor.Interval
	}

	sm := &promv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: exporter.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, sm, func() error {
		labels := commonLabels(exporter)
		for k, v := range exporter.Spec.ServiceMonitor.Labels {
			labels[k] = v
		}
		sm.Labels = labels
		sm.Spec = promv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: selectorLabels(exporter),
			},
			Endpoints: []promv1.Endpoint{
				{
					Port:     "http",
					Path:     "/metrics",
					Interval: promv1.Duration(interval),
				},
			},
		}
		return controllerutil.SetControllerReference(exporter, sm, r.Scheme)
	})
	return err
}

// SetupWithManager sets up the controller with the Manager.
func (r *BlackboxExporterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&monitoringv1alpha1.BlackboxExporter{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&promv1.ServiceMonitor{}).
		Named("blackboxexporter").
		Complete(r)
}

func resourceName(exporterName string) string {
	return fmt.Sprintf("%s-blackbox-exporter", exporterName)
}

func commonLabels(exporter *monitoringv1alpha1.BlackboxExporter) map[string]string {
	labels := map[string]string{
		"app.kubernetes.io/name":       "blackbox-exporter",
		"app.kubernetes.io/instance":   exporter.Name,
		"app.kubernetes.io/managed-by": "blackbox-exporter-operator",
		"app.kubernetes.io/component":  "exporter",
	}
	for k, v := range exporter.Spec.AdditionalLabels {
		labels[k] = v
	}
	return labels
}

func selectorLabels(exporter *monitoringv1alpha1.BlackboxExporter) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":     "blackbox-exporter",
		"app.kubernetes.io/instance": exporter.Name,
	}
}

func containerSecurityContext(enableICMP bool) *corev1.SecurityContext {
	sc := &corev1.SecurityContext{
		AllowPrivilegeEscalation: ptr.To(false),
		ReadOnlyRootFilesystem:   ptr.To(true),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
	}
	if enableICMP {
		sc.Capabilities.Add = []corev1.Capability{"NET_RAW"}
	}
	return sc
}

func setCondition(conditions *[]metav1.Condition, condition metav1.Condition) {
	now := metav1.Now()
	for i, c := range *conditions {
		if c.Type == condition.Type {
			if c.Status != condition.Status {
				condition.LastTransitionTime = now
			} else {
				condition.LastTransitionTime = c.LastTransitionTime
			}
			(*conditions)[i] = condition
			return
		}
	}
	condition.LastTransitionTime = now
	*conditions = append(*conditions, condition)
}
