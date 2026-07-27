package v1alpha1

import (
	"context"
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-monitoring-gaiser-bayern-v1alpha1-blackboxmodule,mutating=false,failurePolicy=fail,sideEffects=None,groups=monitoring.gaiser.bayern,resources=blackboxmodules,verbs=create;update,versions=v1alpha1,name=vblackboxmodule.kb.io,admissionReviewVersions=v1

// BlackboxModuleCustomValidator validates BlackboxModule resources.
type BlackboxModuleCustomValidator struct{}

var _ admission.Validator[*BlackboxModule] = &BlackboxModuleCustomValidator{}

func (v *BlackboxModuleCustomValidator) ValidateCreate(_ context.Context, obj *BlackboxModule) (admission.Warnings, error) {
	return validateModuleSpec(&obj.Spec)
}

func (v *BlackboxModuleCustomValidator) ValidateUpdate(_ context.Context, _ *BlackboxModule, newObj *BlackboxModule) (admission.Warnings, error) {
	return validateModuleSpec(&newObj.Spec)
}

func (v *BlackboxModuleCustomValidator) ValidateDelete(_ context.Context, _ *BlackboxModule) (admission.Warnings, error) {
	return nil, nil
}

func validateModuleSpec(spec *BlackboxModuleSpec) (admission.Warnings, error) {
	count := 0
	if spec.HTTP != nil {
		count++
	}
	if spec.TCP != nil {
		count++
	}
	if spec.DNS != nil {
		count++
	}
	if spec.ICMP != nil {
		count++
	}
	if spec.GRPC != nil {
		count++
	}
	if spec.Unix != nil {
		count++
	}
	if spec.Websocket != nil {
		count++
	}

	if count == 0 {
		return nil, fmt.Errorf("exactly one prober must be configured, got none")
	}
	if count > 1 {
		return nil, fmt.Errorf("exactly one prober must be configured, got %d", count)
	}

	if spec.Timeout != "" {
		if _, err := time.ParseDuration(spec.Timeout); err != nil {
			return nil, fmt.Errorf("invalid timeout %q: %w", spec.Timeout, err)
		}
	}

	if spec.DNS != nil && spec.DNS.QueryName == "" {
		return nil, fmt.Errorf("dns.queryName is required")
	}

	return nil, nil
}
