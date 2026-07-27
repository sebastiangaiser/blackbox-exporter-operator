package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	monitoringv1alpha1 "github.com/sebastiangaiser/blackbox-exporter-operator/api/v1alpha1"
)

var _ = Describe("BlackboxModule Controller", func() {
	const namespace = "default"
	ctx := context.Background()

	Context("When reconciling a valid module", func() {
		const moduleName = "test-valid-module"
		moduleKey := types.NamespacedName{Name: moduleName, Namespace: namespace}

		BeforeEach(func() {
			module := &monitoringv1alpha1.BlackboxModule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      moduleName,
					Namespace: namespace,
				},
				Spec: monitoringv1alpha1.BlackboxModuleSpec{
					Timeout: "5s",
					HTTP: &monitoringv1alpha1.HTTPProbeConfig{
						Method: "GET",
					},
				},
			}
			err := k8sClient.Get(ctx, moduleKey, &monitoringv1alpha1.BlackboxModule{})
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, module)).To(Succeed())
			}
		})

		AfterEach(func() {
			module := &monitoringv1alpha1.BlackboxModule{}
			if err := k8sClient.Get(ctx, moduleKey, module); err == nil {
				Expect(k8sClient.Delete(ctx, module)).To(Succeed())
			}
		})

		It("should set ConfigValid condition to True", func() {
			reconciler := &BlackboxModuleReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: moduleKey})
			Expect(err).NotTo(HaveOccurred())

			module := &monitoringv1alpha1.BlackboxModule{}
			Expect(k8sClient.Get(ctx, moduleKey, module)).To(Succeed())

			configValid := findCondition(module.Status.Conditions, "ConfigValid")
			Expect(configValid).NotTo(BeNil())
			Expect(configValid.Status).To(Equal(metav1.ConditionTrue))
		})

		It("should set Ready condition to True", func() {
			reconciler := &BlackboxModuleReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: moduleKey})
			Expect(err).NotTo(HaveOccurred())

			module := &monitoringv1alpha1.BlackboxModule{}
			Expect(k8sClient.Get(ctx, moduleKey, module)).To(Succeed())

			ready := findCondition(module.Status.Conditions, "Ready")
			Expect(ready).NotTo(BeNil())
			Expect(ready.Status).To(Equal(metav1.ConditionTrue))
		})
	})

	Context("When reconciling an invalid module", func() {
		const moduleName = "test-invalid-module"
		moduleKey := types.NamespacedName{Name: moduleName, Namespace: namespace}

		BeforeEach(func() {
			module := &monitoringv1alpha1.BlackboxModule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      moduleName,
					Namespace: namespace,
				},
				Spec: monitoringv1alpha1.BlackboxModuleSpec{
					Timeout: "5s",
					// No prober configured — invalid.
				},
			}
			err := k8sClient.Get(ctx, moduleKey, &monitoringv1alpha1.BlackboxModule{})
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, module)).To(Succeed())
			}
		})

		AfterEach(func() {
			module := &monitoringv1alpha1.BlackboxModule{}
			if err := k8sClient.Get(ctx, moduleKey, module); err == nil {
				Expect(k8sClient.Delete(ctx, module)).To(Succeed())
			}
		})

		It("should set ConfigValid condition to False", func() {
			reconciler := &BlackboxModuleReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: moduleKey})
			Expect(err).NotTo(HaveOccurred())

			module := &monitoringv1alpha1.BlackboxModule{}
			Expect(k8sClient.Get(ctx, moduleKey, module)).To(Succeed())

			configValid := findCondition(module.Status.Conditions, "ConfigValid")
			Expect(configValid).NotTo(BeNil())
			Expect(configValid.Status).To(Equal(metav1.ConditionFalse))
			Expect(configValid.Reason).To(Equal("Invalid"))
		})
	})

	Context("When the module is deleted", func() {
		It("should handle not found gracefully", func() {
			reconciler := &BlackboxModuleReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "nonexistent", Namespace: namespace},
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

func findCondition(conditions []metav1.Condition, condType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}
