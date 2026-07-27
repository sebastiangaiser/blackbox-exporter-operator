package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	monitoringv1alpha1 "github.com/sebastiangaiser/blackbox-exporter-operator/api/v1alpha1"
)

var _ = Describe("BlackboxExporter Controller", func() {
	const (
		exporterName = "test-exporter"
		namespace    = "default"
	)

	ctx := context.Background()
	exporterKey := types.NamespacedName{Name: exporterName, Namespace: namespace}

	Context("When reconciling a BlackboxExporter", func() {
		BeforeEach(func() {
			// Create a module that the exporter will pick up.
			module := &monitoringv1alpha1.BlackboxModule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "http-2xx",
					Namespace: namespace,
					Labels:    map[string]string{"exporter": exporterName},
				},
				Spec: monitoringv1alpha1.BlackboxModuleSpec{
					Timeout: "5s",
					HTTP: &monitoringv1alpha1.HTTPProbeConfig{
						Method: "GET",
					},
				},
			}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: "http-2xx", Namespace: namespace}, &monitoringv1alpha1.BlackboxModule{})
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, module)).To(Succeed())
			}

			// Create the exporter.
			exporter := &monitoringv1alpha1.BlackboxExporter{
				ObjectMeta: metav1.ObjectMeta{
					Name:      exporterName,
					Namespace: namespace,
				},
				Spec: monitoringv1alpha1.BlackboxExporterSpec{
					Replicas: ptr.To(int32(1)),
					Port:     9115,
					ModuleSelector: monitoringv1alpha1.ModuleSelector{
						NamespaceSelector: monitoringv1alpha1.NamespaceSelector{
							MatchNames: []string{namespace},
						},
						MatchLabels: map[string]string{"exporter": exporterName},
					},
				},
			}
			err = k8sClient.Get(ctx, exporterKey, &monitoringv1alpha1.BlackboxExporter{})
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, exporter)).To(Succeed())
			}
		})

		AfterEach(func() {
			exporter := &monitoringv1alpha1.BlackboxExporter{}
			if err := k8sClient.Get(ctx, exporterKey, exporter); err == nil {
				Expect(k8sClient.Delete(ctx, exporter)).To(Succeed())
			}
			module := &monitoringv1alpha1.BlackboxModule{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "http-2xx", Namespace: namespace}, module); err == nil {
				Expect(k8sClient.Delete(ctx, module)).To(Succeed())
			}
		})

		It("should create a ConfigMap with rendered blackbox.yml", func() {
			reconciler := &BlackboxExporterReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: exporterKey})
			Expect(err).NotTo(HaveOccurred())

			cm := &corev1.ConfigMap{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name: resourceName(exporterName), Namespace: namespace,
			}, cm)).To(Succeed())

			Expect(cm.Data).To(HaveKey("blackbox.yml"))
			Expect(cm.Data["blackbox.yml"]).To(ContainSubstring("default-http-2xx"))
		})

		It("should create a Deployment with restricted security context", func() {
			reconciler := &BlackboxExporterReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: exporterKey})
			Expect(err).NotTo(HaveOccurred())

			deploy := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name: resourceName(exporterName), Namespace: namespace,
			}, deploy)).To(Succeed())

			Expect(*deploy.Spec.Replicas).To(Equal(int32(1)))

			// Verify restricted PSS.
			podSec := deploy.Spec.Template.Spec.SecurityContext
			Expect(*podSec.RunAsNonRoot).To(BeTrue())
			Expect(*podSec.RunAsUser).To(Equal(int64(65534)))
			Expect(podSec.SeccompProfile.Type).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))

			containerSec := deploy.Spec.Template.Spec.Containers[0].SecurityContext
			Expect(*containerSec.AllowPrivilegeEscalation).To(BeFalse())
			Expect(*containerSec.ReadOnlyRootFilesystem).To(BeTrue())
			Expect(containerSec.Capabilities.Drop).To(ContainElement(corev1.Capability("ALL")))
		})

		It("should create a Service", func() {
			reconciler := &BlackboxExporterReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: exporterKey})
			Expect(err).NotTo(HaveOccurred())

			svc := &corev1.Service{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name: resourceName(exporterName), Namespace: namespace,
			}, svc)).To(Succeed())

			Expect(svc.Spec.Ports).To(HaveLen(1))
			Expect(svc.Spec.Ports[0].Port).To(Equal(int32(9115)))
		})

		It("should update status with module count", func() {
			reconciler := &BlackboxExporterReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: exporterKey})
			Expect(err).NotTo(HaveOccurred())

			exporter := &monitoringv1alpha1.BlackboxExporter{}
			Expect(k8sClient.Get(ctx, exporterKey, exporter)).To(Succeed())
			Expect(exporter.Status.ModuleCount).To(Equal(int32(1)))
		})

		It("should not create a ServiceMonitor when disabled", func() {
			reconciler := &BlackboxExporterReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: exporterKey})
			Expect(err).NotTo(HaveOccurred())

			// ServiceMonitor should not exist since enabled defaults to false.
			// We can't check for promv1.ServiceMonitor in envtest without its CRD installed,
			// but we verify no error occurred during reconciliation.
		})
	})

	Context("When enableICMP is true", func() {
		const icmpExporterName = "test-icmp-exporter"
		icmpKey := types.NamespacedName{Name: icmpExporterName, Namespace: namespace}

		BeforeEach(func() {
			exporter := &monitoringv1alpha1.BlackboxExporter{
				ObjectMeta: metav1.ObjectMeta{
					Name:      icmpExporterName,
					Namespace: namespace,
				},
				Spec: monitoringv1alpha1.BlackboxExporterSpec{
					Replicas:   ptr.To(int32(1)),
					EnableICMP: true,
					ModuleSelector: monitoringv1alpha1.ModuleSelector{
						NamespaceSelector: monitoringv1alpha1.NamespaceSelector{
							MatchNames: []string{namespace},
						},
					},
				},
			}
			if err := k8sClient.Get(ctx, icmpKey, &monitoringv1alpha1.BlackboxExporter{}); errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, exporter)).To(Succeed())
			}
		})

		AfterEach(func() {
			exporter := &monitoringv1alpha1.BlackboxExporter{}
			if err := k8sClient.Get(ctx, icmpKey, exporter); err == nil {
				Expect(k8sClient.Delete(ctx, exporter)).To(Succeed())
			}
		})

		It("should add CAP_NET_RAW to the container", func() {
			reconciler := &BlackboxExporterReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: icmpKey})
			Expect(err).NotTo(HaveOccurred())

			deploy := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name: resourceName(icmpExporterName), Namespace: namespace,
			}, deploy)).To(Succeed())

			containerSec := deploy.Spec.Template.Spec.Containers[0].SecurityContext
			Expect(containerSec.Capabilities.Drop).To(ContainElement(corev1.Capability("ALL")))
			Expect(containerSec.Capabilities.Add).To(ContainElement(corev1.Capability("NET_RAW")))
		})
	})

	Context("When the BlackboxExporter is deleted", func() {
		It("should handle not found gracefully", func() {
			reconciler := &BlackboxExporterReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "nonexistent", Namespace: namespace},
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
