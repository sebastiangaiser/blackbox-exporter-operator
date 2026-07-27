package v1alpha1

import (
	"context"
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-monitoring-gaiser-bayern-v1alpha1-blackboxprobe,mutating=false,failurePolicy=fail,sideEffects=None,groups=monitoring.gaiser.bayern,resources=blackboxprobes,verbs=create;update,versions=v1alpha1,name=vblackboxprobe.kb.io,admissionReviewVersions=v1

// BlackboxProbeCustomValidator validates BlackboxProbe resources.
type BlackboxProbeCustomValidator struct{}

var _ admission.Validator[*BlackboxProbe] = &BlackboxProbeCustomValidator{}

func (v *BlackboxProbeCustomValidator) ValidateCreate(_ context.Context, obj *BlackboxProbe) (admission.Warnings, error) {
	return validateProbeSpec(&obj.Spec)
}

func (v *BlackboxProbeCustomValidator) ValidateUpdate(_ context.Context, _ *BlackboxProbe, newObj *BlackboxProbe) (admission.Warnings, error) {
	return validateProbeSpec(&newObj.Spec)
}

func (v *BlackboxProbeCustomValidator) ValidateDelete(_ context.Context, _ *BlackboxProbe) (admission.Warnings, error) {
	return nil, nil
}

func validateProbeSpec(spec *BlackboxProbeSpec) (admission.Warnings, error) {
	if spec.ExporterRef.Name == "" {
		return nil, fmt.Errorf("exporterRef.name is required")
	}

	if spec.ModuleRef.Name == "" {
		return nil, fmt.Errorf("moduleRef.name is required")
	}

	if len(spec.Targets) == 0 && spec.Ingress == nil {
		return nil, fmt.Errorf("at least one of targets or ingress must be set")
	}

	var intervalDuration time.Duration
	if spec.Interval != "" {
		d, err := time.ParseDuration(spec.Interval)
		if err != nil {
			return nil, fmt.Errorf("invalid interval %q: %w", spec.Interval, err)
		}
		intervalDuration = d
	}

	if spec.ScrapeTimeout != "" {
		d, err := time.ParseDuration(spec.ScrapeTimeout)
		if err != nil {
			return nil, fmt.Errorf("invalid scrapeTimeout %q: %w", spec.ScrapeTimeout, err)
		}
		if intervalDuration > 0 && d > intervalDuration {
			return nil, fmt.Errorf("scrapeTimeout (%s) must be <= interval (%s)", spec.ScrapeTimeout, spec.Interval)
		}
	}

	return nil, nil
}
