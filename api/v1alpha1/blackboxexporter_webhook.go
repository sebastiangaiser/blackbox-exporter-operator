package v1alpha1

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-monitoring-gaiser-bayern-v1alpha1-blackboxexporter,mutating=false,failurePolicy=fail,sideEffects=None,groups=monitoring.gaiser.bayern,resources=blackboxexporters,verbs=create;update,versions=v1alpha1,name=vblackboxexporter.kb.io,admissionReviewVersions=v1

// BlackboxExporterCustomValidator validates BlackboxExporter resources.
type BlackboxExporterCustomValidator struct{}

var _ admission.Validator[*BlackboxExporter] = &BlackboxExporterCustomValidator{}

func (v *BlackboxExporterCustomValidator) ValidateCreate(_ context.Context, obj *BlackboxExporter) (admission.Warnings, error) {
	return validateExporterSpec(&obj.Spec)
}

func (v *BlackboxExporterCustomValidator) ValidateUpdate(_ context.Context, _ *BlackboxExporter, newObj *BlackboxExporter) (admission.Warnings, error) {
	return validateExporterSpec(&newObj.Spec)
}

func (v *BlackboxExporterCustomValidator) ValidateDelete(_ context.Context, _ *BlackboxExporter) (admission.Warnings, error) {
	return nil, nil
}

func validateExporterSpec(spec *BlackboxExporterSpec) (admission.Warnings, error) {
	if spec.Port != 0 && (spec.Port < 1 || spec.Port > 65535) {
		return nil, fmt.Errorf("port must be between 1 and 65535, got %d", spec.Port)
	}

	if spec.Replicas != nil && *spec.Replicas < 0 {
		return nil, fmt.Errorf("replicas must be >= 0, got %d", *spec.Replicas)
	}

	return nil, nil
}
