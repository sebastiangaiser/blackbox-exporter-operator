package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	monitoringv1alpha1 "github.com/sebastiangaiser/blackbox-exporter-operator/api/v1alpha1"
)

var _ = Describe("BlackboxProbe Controller", func() {
	const namespace = "default"
	ctx := context.Background()

	Context("When reconciling a BlackboxProbe with valid refs", func() {
		const (
			probeName    = "test-probe"
			exporterName = "probe-test-exporter"
			moduleName   = "probe-test-module"
		)

		probeKey := types.NamespacedName{Name: probeName, Namespace: namespace}

		BeforeEach(func() {
			// Create exporter.
			exporter := &monitoringv1alpha1.BlackboxExporter{
				ObjectMeta: metav1.ObjectMeta{Name: exporterName, Namespace: namespace},
				Spec: monitoringv1alpha1.BlackboxExporterSpec{
					Replicas: ptr.To(int32(1)),
					Port:     9115,
					ModuleSelector: monitoringv1alpha1.ModuleSelector{
						NamespaceSelector: monitoringv1alpha1.NamespaceSelector{Any: true},
					},
				},
			}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: exporterName, Namespace: namespace}, &monitoringv1alpha1.BlackboxExporter{}); errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, exporter)).To(Succeed())
			}

			// Create module.
			module := &monitoringv1alpha1.BlackboxModule{
				ObjectMeta: metav1.ObjectMeta{Name: moduleName, Namespace: namespace},
				Spec: monitoringv1alpha1.BlackboxModuleSpec{
					Timeout: "5s",
					HTTP:    &monitoringv1alpha1.HTTPProbeConfig{Method: "GET"},
				},
			}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: moduleName, Namespace: namespace}, &monitoringv1alpha1.BlackboxModule{}); errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, module)).To(Succeed())
			}

			// Create probe.
			probe := &monitoringv1alpha1.BlackboxProbe{
				ObjectMeta: metav1.ObjectMeta{Name: probeName, Namespace: namespace},
				Spec: monitoringv1alpha1.BlackboxProbeSpec{
					ExporterRef:  monitoringv1alpha1.NamespacedReference{Name: exporterName},
					ModuleRef:    monitoringv1alpha1.NamespacedReference{Name: moduleName},
					Targets:      []string{"https://example.com", "https://prometheus.io"},
					TargetLabels: map[string]string{"tenant": "acme"},
					Interval:     "30s",
					ProbeMetadata: &monitoringv1alpha1.EmbeddedMetadata{
						Labels: map[string]string{"team": "platform"},
					},
				},
			}
			if err := k8sClient.Get(ctx, probeKey, &monitoringv1alpha1.BlackboxProbe{}); errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, probe)).To(Succeed())
			}
		})

		AfterEach(func() {
			probe := &monitoringv1alpha1.BlackboxProbe{}
			if err := k8sClient.Get(ctx, probeKey, probe); err == nil {
				Expect(k8sClient.Delete(ctx, probe)).To(Succeed())
			}
			exporter := &monitoringv1alpha1.BlackboxExporter{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: exporterName, Namespace: namespace}, exporter); err == nil {
				Expect(k8sClient.Delete(ctx, exporter)).To(Succeed())
			}
			module := &monitoringv1alpha1.BlackboxModule{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: moduleName, Namespace: namespace}, module); err == nil {
				Expect(k8sClient.Delete(ctx, module)).To(Succeed())
			}
		})

		It("should set Ready condition and update target count", func() {
			reconciler := &BlackboxProbeReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: probeKey})
			Expect(err).NotTo(HaveOccurred())

			probe := &monitoringv1alpha1.BlackboxProbe{}
			Expect(k8sClient.Get(ctx, probeKey, probe)).To(Succeed())
			Expect(probe.Status.TargetCount).To(Equal(int32(2)))

			ready := findCondition(probe.Status.Conditions, "Ready")
			Expect(ready).NotTo(BeNil())
			Expect(ready.Status).To(Equal(metav1.ConditionTrue))
			Expect(ready.Reason).To(Equal("ProbeCreated"))
		})

		It("should set probeRef in status", func() {
			reconciler := &BlackboxProbeReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: probeKey})
			Expect(err).NotTo(HaveOccurred())

			probe := &monitoringv1alpha1.BlackboxProbe{}
			Expect(k8sClient.Get(ctx, probeKey, probe)).To(Succeed())
			Expect(probe.Status.ProbeRef).NotTo(BeNil())
			Expect(probe.Status.ProbeRef.Name).To(Equal(probeName))
			Expect(probe.Status.ProbeRef.Namespace).To(Equal(namespace))
		})

		It("should keep probeMetadata labels off the scraped series", func() {
			reconciler := &BlackboxProbeReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: probeKey})
			Expect(err).NotTo(HaveOccurred())

			generated := &promv1.Probe{}
			Expect(k8sClient.Get(ctx, probeKey, generated)).To(Succeed())

			Expect(generated.Labels).To(HaveKeyWithValue("team", "platform"))
			Expect(generated.Labels).NotTo(HaveKey("tenant"))

			Expect(generated.Spec.Targets.StaticConfig).NotTo(BeNil())
			Expect(generated.Spec.Targets.StaticConfig.Labels).To(HaveKeyWithValue("tenant", "acme"))
			Expect(generated.Spec.Targets.StaticConfig.Labels).NotTo(HaveKey("team"))
		})

		It("should apply deprecated additionalLabels to both object and series", func() {
			probe := &monitoringv1alpha1.BlackboxProbe{}
			Expect(k8sClient.Get(ctx, probeKey, probe)).To(Succeed())
			//nolint:staticcheck // the deprecated field is exactly what this test covers
			probe.Spec.AdditionalLabels = map[string]string{"env": "production", "tenant": "legacy"}
			Expect(k8sClient.Update(ctx, probe)).To(Succeed())

			reconciler := &BlackboxProbeReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: probeKey})
			Expect(err).NotTo(HaveOccurred())

			generated := &promv1.Probe{}
			Expect(k8sClient.Get(ctx, probeKey, generated)).To(Succeed())

			// additionalLabels still reach both destinations, as in v0.1.x.
			Expect(generated.Labels).To(HaveKeyWithValue("env", "production"))
			Expect(generated.Spec.Targets.StaticConfig.Labels).To(HaveKeyWithValue("env", "production"))

			// The newer fields win where they overlap.
			Expect(generated.Spec.Targets.StaticConfig.Labels).To(HaveKeyWithValue("tenant", "acme"))
		})

		It("should not let probeMetadata override reserved labels", func() {
			probe := &monitoringv1alpha1.BlackboxProbe{}
			Expect(k8sClient.Get(ctx, probeKey, probe)).To(Succeed())
			probe.Spec.ProbeMetadata.Labels["app.kubernetes.io/managed-by"] = "somebody-else"
			probe.Spec.ProbeMetadata.Labels["monitoring.gaiser.bayern/exporter"] = "wrong"
			Expect(k8sClient.Update(ctx, probe)).To(Succeed())

			reconciler := &BlackboxProbeReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: probeKey})
			Expect(err).NotTo(HaveOccurred())

			generated := &promv1.Probe{}
			Expect(k8sClient.Get(ctx, probeKey, generated)).To(Succeed())
			Expect(generated.Labels).To(HaveKeyWithValue("app.kubernetes.io/managed-by", "blackbox-exporter-operator"))
			Expect(generated.Labels).To(HaveKeyWithValue("monitoring.gaiser.bayern/exporter", exporterName))
		})
	})

	Context("When exporterRef does not exist", func() {
		const probeName = "test-probe-missing-exporter"
		probeKey := types.NamespacedName{Name: probeName, Namespace: namespace}

		BeforeEach(func() {
			probe := &monitoringv1alpha1.BlackboxProbe{
				ObjectMeta: metav1.ObjectMeta{Name: probeName, Namespace: namespace},
				Spec: monitoringv1alpha1.BlackboxProbeSpec{
					ExporterRef: monitoringv1alpha1.NamespacedReference{Name: "nonexistent-exporter"},
					ModuleRef:   monitoringv1alpha1.NamespacedReference{Name: "nonexistent-module"},
					Targets:     []string{"https://example.com"},
				},
			}
			if err := k8sClient.Get(ctx, probeKey, &monitoringv1alpha1.BlackboxProbe{}); errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, probe)).To(Succeed())
			}
		})

		AfterEach(func() {
			probe := &monitoringv1alpha1.BlackboxProbe{}
			if err := k8sClient.Get(ctx, probeKey, probe); err == nil {
				Expect(k8sClient.Delete(ctx, probe)).To(Succeed())
			}
		})

		It("should set Ready to False with ExporterNotFound reason", func() {
			reconciler := &BlackboxProbeReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: probeKey})
			Expect(err).To(HaveOccurred())

			probe := &monitoringv1alpha1.BlackboxProbe{}
			Expect(k8sClient.Get(ctx, probeKey, probe)).To(Succeed())

			ready := findCondition(probe.Status.Conditions, "Ready")
			Expect(ready).NotTo(BeNil())
			Expect(ready.Status).To(Equal(metav1.ConditionFalse))
			Expect(ready.Reason).To(Equal("ExporterNotFound"))
		})
	})

	Context("When reconciling a BlackboxProbe with ingress discovery", func() {
		const (
			probeName    = "test-probe-ingress"
			exporterName = "ingress-test-exporter"
			moduleName   = "ingress-test-module"
		)

		probeKey := types.NamespacedName{Name: probeName, Namespace: namespace}

		BeforeEach(func() {
			exporter := &monitoringv1alpha1.BlackboxExporter{
				ObjectMeta: metav1.ObjectMeta{Name: exporterName, Namespace: namespace},
				Spec: monitoringv1alpha1.BlackboxExporterSpec{
					Replicas: ptr.To(int32(1)),
					Port:     9115,
					ModuleSelector: monitoringv1alpha1.ModuleSelector{
						NamespaceSelector: monitoringv1alpha1.NamespaceSelector{Any: true},
					},
				},
			}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: exporterName, Namespace: namespace}, &monitoringv1alpha1.BlackboxExporter{}); errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, exporter)).To(Succeed())
			}

			module := &monitoringv1alpha1.BlackboxModule{
				ObjectMeta: metav1.ObjectMeta{Name: moduleName, Namespace: namespace},
				Spec: monitoringv1alpha1.BlackboxModuleSpec{
					Timeout: "5s",
					HTTP:    &monitoringv1alpha1.HTTPProbeConfig{Method: "GET"},
				},
			}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: moduleName, Namespace: namespace}, &monitoringv1alpha1.BlackboxModule{}); errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, module)).To(Succeed())
			}

			probe := &monitoringv1alpha1.BlackboxProbe{
				ObjectMeta: metav1.ObjectMeta{Name: probeName, Namespace: namespace},
				Spec: monitoringv1alpha1.BlackboxProbeSpec{
					ExporterRef: monitoringv1alpha1.NamespacedReference{Name: exporterName},
					ModuleRef:   monitoringv1alpha1.NamespacedReference{Name: moduleName},
					Ingress: &monitoringv1alpha1.IngressTargetConfig{
						Selector:          metav1.LabelSelector{MatchLabels: map[string]string{"monitoring": "true"}},
						NamespaceSelector: monitoringv1alpha1.NamespaceSelector{Any: true},
						Labels:            map[string]string{"source": "ingress-discovery", "env": "production"},
						RelabelConfigs: []monitoringv1alpha1.RelabelConfig{
							{SourceLabels: []string{"__tmp_prometheus_job_name"}, TargetLabel: "job", Action: "replace"},
						},
					},
				},
			}
			if err := k8sClient.Get(ctx, probeKey, &monitoringv1alpha1.BlackboxProbe{}); errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, probe)).To(Succeed())
			}
		})

		AfterEach(func() {
			probe := &monitoringv1alpha1.BlackboxProbe{}
			if err := k8sClient.Get(ctx, probeKey, probe); err == nil {
				Expect(k8sClient.Delete(ctx, probe)).To(Succeed())
			}
			exporter := &monitoringv1alpha1.BlackboxExporter{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: exporterName, Namespace: namespace}, exporter); err == nil {
				Expect(k8sClient.Delete(ctx, exporter)).To(Succeed())
			}
			module := &monitoringv1alpha1.BlackboxModule{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: moduleName, Namespace: namespace}, module); err == nil {
				Expect(k8sClient.Delete(ctx, module)).To(Succeed())
			}
		})

		It("should persist ingress labels as replace relabelings on the Probe CR", func() {
			reconciler := &BlackboxProbeReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: probeKey})
			Expect(err).NotTo(HaveOccurred())

			generated := &promv1.Probe{}
			Expect(k8sClient.Get(ctx, probeKey, generated)).To(Succeed())
			Expect(generated.Spec.Targets.Ingress).NotTo(BeNil())

			rcs := generated.Spec.Targets.Ingress.RelabelConfigs
			Expect(rcs).To(HaveLen(3))

			// Label-derived relabelings come first, sorted by key, then the user's own.
			Expect(rcs[0].TargetLabel).To(Equal("env"))
			Expect(rcs[0].Replacement).To(HaveValue(Equal("production")))
			Expect(rcs[0].Action).To(Equal("replace"))
			Expect(rcs[1].TargetLabel).To(Equal("source"))
			Expect(rcs[1].Replacement).To(HaveValue(Equal("ingress-discovery")))
			Expect(rcs[2].TargetLabel).To(Equal("job"))
			Expect(rcs[2].SourceLabels).To(Equal([]promv1.LabelName{"__tmp_prometheus_job_name"}))
		})

		It("should keep ingress labels off the generated Probe object", func() {
			reconciler := &BlackboxProbeReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: probeKey})
			Expect(err).NotTo(HaveOccurred())

			generated := &promv1.Probe{}
			Expect(k8sClient.Get(ctx, probeKey, generated)).To(Succeed())
			Expect(generated.Labels).NotTo(HaveKey("source"))
			Expect(generated.Labels).NotTo(HaveKey("env"))
			Expect(generated.Spec.Targets.StaticConfig).To(BeNil())
		})

		It("should report a target count of zero for ingress-only probes", func() {
			reconciler := &BlackboxProbeReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: probeKey})
			Expect(err).NotTo(HaveOccurred())

			probe := &monitoringv1alpha1.BlackboxProbe{}
			Expect(k8sClient.Get(ctx, probeKey, probe)).To(Succeed())
			// targetCount only counts static targets; ingress targets are resolved by
			// prometheus-operator, not by this controller.
			Expect(probe.Status.TargetCount).To(Equal(int32(0)))
		})
	})

	Context("When the probe is deleted", func() {
		It("should handle not found gracefully", func() {
			reconciler := &BlackboxProbeReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "nonexistent", Namespace: namespace},
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
