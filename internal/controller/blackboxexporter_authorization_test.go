package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	monitoringv1alpha1 "github.com/sebastiangaiser/blackbox-exporter-operator/api/v1alpha1"
)

var _ = Describe("BlackboxExporter Controller with authorization", func() {
	const (
		bearerExporterName = "bearer-exporter"
		bearerModuleName   = "http-bearer"
		bearerSecretName   = "api-credentials"
		bearerToken        = "s3cr3t-bearer-token"
		bearerNamespace    = "default"
		selectorLabel      = "auth-exporter"
	)

	ctx := context.Background()
	exporterKey := types.NamespacedName{Name: bearerExporterName, Namespace: bearerNamespace}

	BeforeEach(func() {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: bearerSecretName, Namespace: bearerNamespace},
			Type:       corev1.SecretTypeOpaque,
			Data:       map[string][]byte{"token": []byte(bearerToken)},
		}
		err := k8sClient.Get(ctx, types.NamespacedName{Name: bearerSecretName, Namespace: bearerNamespace}, &corev1.Secret{})
		if err != nil {
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		}

		module := &monitoringv1alpha1.BlackboxModule{
			ObjectMeta: metav1.ObjectMeta{
				Name:      bearerModuleName,
				Namespace: bearerNamespace,
				Labels:    map[string]string{selectorLabel: bearerExporterName},
			},
			Spec: monitoringv1alpha1.BlackboxModuleSpec{
				Timeout: "5s",
				HTTP: &monitoringv1alpha1.HTTPProbeConfig{
					Method: "GET",
					Authorization: &monitoringv1alpha1.Authorization{
						Type: "Bearer",
						CredentialsRef: monitoringv1alpha1.SecretKeySelector{
							Name: bearerSecretName,
							Key:  "token",
						},
					},
				},
			},
		}
		err = k8sClient.Get(ctx, types.NamespacedName{Name: bearerModuleName, Namespace: bearerNamespace}, &monitoringv1alpha1.BlackboxModule{})
		if err != nil {
			Expect(k8sClient.Create(ctx, module)).To(Succeed())
		}

		exporter := &monitoringv1alpha1.BlackboxExporter{
			ObjectMeta: metav1.ObjectMeta{Name: bearerExporterName, Namespace: bearerNamespace},
			Spec: monitoringv1alpha1.BlackboxExporterSpec{
				Replicas: ptr.To(int32(1)),
				Port:     9115,
				ModuleSelector: monitoringv1alpha1.ModuleSelector{
					NamespaceSelector: monitoringv1alpha1.NamespaceSelector{
						MatchNames: []string{bearerNamespace},
					},
					MatchLabels: map[string]string{selectorLabel: bearerExporterName},
				},
			},
		}
		err = k8sClient.Get(ctx, exporterKey, &monitoringv1alpha1.BlackboxExporter{})
		if err != nil {
			Expect(k8sClient.Create(ctx, exporter)).To(Succeed())
		}
	})

	AfterEach(func() {
		exporter := &monitoringv1alpha1.BlackboxExporter{}
		if err := k8sClient.Get(ctx, exporterKey, exporter); err == nil {
			Expect(k8sClient.Delete(ctx, exporter)).To(Succeed())
		}
		module := &monitoringv1alpha1.BlackboxModule{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: bearerModuleName, Namespace: bearerNamespace}, module); err == nil {
			Expect(k8sClient.Delete(ctx, module)).To(Succeed())
		}
		secret := &corev1.Secret{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: bearerSecretName, Namespace: bearerNamespace}, secret); err == nil {
			Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
		}
	})

	It("renders the resolved bearer token into the exporter config", func() {
		reconciler := &BlackboxExporterReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: exporterKey})
		Expect(err).NotTo(HaveOccurred())

		cm := &corev1.ConfigMap{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name: resourceName(bearerExporterName), Namespace: bearerNamespace,
		}, cm)).To(Succeed())

		config := cm.Data["blackbox.yml"]
		Expect(config).To(ContainSubstring("authorization:"))
		Expect(config).To(ContainSubstring("type: Bearer"))
		Expect(config).To(ContainSubstring(bearerToken))
		// The redaction placeholder would make the exporter send "<secret>".
		Expect(config).NotTo(ContainSubstring("<secret>"))
	})
})
