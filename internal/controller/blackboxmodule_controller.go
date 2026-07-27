package controller

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	monitoringv1alpha1 "github.com/sebastiangaiser/blackbox-exporter-operator/api/v1alpha1"
	"github.com/sebastiangaiser/blackbox-exporter-operator/internal/converter"
)

// BlackboxModuleReconciler reconciles a BlackboxModule object.
type BlackboxModuleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=monitoring.gaiser.bayern,resources=blackboxmodules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.gaiser.bayern,resources=blackboxmodules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=monitoring.gaiser.bayern,resources=blackboxmodules/finalizers,verbs=update

func (r *BlackboxModuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	module := &monitoringv1alpha1.BlackboxModule{}
	if err := r.Get(ctx, req.NamespacedName, module); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Validate by attempting conversion (without secrets — validation only).
	_, err := converter.ConvertModule(&module.Spec, nil)

	configValid := metav1.ConditionTrue
	reason := "Valid"
	message := "Module configuration is valid"
	if err != nil {
		configValid = metav1.ConditionFalse
		reason = "Invalid"
		message = fmt.Sprintf("Module configuration is invalid: %v", err)
		log.Error(err, "invalid module configuration")
	}

	setCondition(&module.Status.Conditions, metav1.Condition{
		Type:               conditionTypeConfig,
		Status:             configValid,
		ObservedGeneration: module.Generation,
		Reason:             reason,
		Message:            message,
	})

	// Find referencing exporters.
	module.Status.ReferencedByExporters = nil
	var exporters monitoringv1alpha1.BlackboxExporterList
	if err := r.List(ctx, &exporters); err == nil {
		for _, exp := range exporters.Items {
			if moduleMatchesSelector(module, &exp) {
				module.Status.ReferencedByExporters = append(module.Status.ReferencedByExporters, monitoringv1alpha1.NamespacedReference{
					Name:      exp.Name,
					Namespace: exp.Namespace,
				})
			}
		}
	}

	// Set Ready condition based on ConfigValid.
	ready := configValid
	readyReason := reason
	readyMessage := message
	if configValid == metav1.ConditionTrue {
		readyReason = conditionTypeReady
		readyMessage = "Module is ready"
	}
	setCondition(&module.Status.Conditions, metav1.Condition{
		Type:               conditionTypeReady,
		Status:             ready,
		ObservedGeneration: module.Generation,
		Reason:             readyReason,
		Message:            readyMessage,
	})

	module.Status.ObservedGeneration = module.Generation

	if err := r.Status().Update(ctx, module); err != nil {
		return ctrl.Result{}, err
	}

	// Trigger reconciliation of referencing exporters by re-queuing them.
	for _, ref := range module.Status.ReferencedByExporters {
		exp := &monitoringv1alpha1.BlackboxExporter{}
		if err := r.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: ref.Namespace}, exp); err == nil {
			// Touch the exporter annotation to trigger reconciliation.
			if exp.Annotations == nil {
				exp.Annotations = make(map[string]string)
			}
			exp.Annotations["monitoring.gaiser.bayern/module-updated"] = module.ResourceVersion
			if err := r.Update(ctx, exp); err != nil {
				log.Error(err, "failed to trigger exporter reconciliation", "exporter", ref.Name, "namespace", ref.Namespace)
			}
		}
	}

	return ctrl.Result{}, nil
}

func moduleMatchesSelector(module *monitoringv1alpha1.BlackboxModule, exporter *monitoringv1alpha1.BlackboxExporter) bool {
	sel := exporter.Spec.ModuleSelector

	// Check namespace.
	if !sel.NamespaceSelector.Any {
		if len(sel.NamespaceSelector.MatchNames) > 0 {
			found := false
			for _, ns := range sel.NamespaceSelector.MatchNames {
				if module.Namespace == ns {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		} else {
			if module.Namespace != exporter.Namespace {
				return false
			}
		}
	}

	// Check labels.
	for k, v := range sel.MatchLabels {
		if module.Labels[k] != v {
			return false
		}
	}

	return true
}

// SetupWithManager sets up the controller with the Manager.
func (r *BlackboxModuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&monitoringv1alpha1.BlackboxModule{}).
		Named("blackboxmodule").
		Complete(r)
}
