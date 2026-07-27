//go:build e2e
// +build e2e

package e2e

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sebastiangaiser/blackbox-exporter-operator/test/utils"
)

const namespace = "blackbox-exporter-operator-system"

var _ = Describe("Operator", Ordered, func() {
	BeforeAll(func() {
		By("creating operator namespace")
		cmd := exec.Command("kubectl", "create", "ns", namespace)
		_, _ = utils.Run(cmd)

		By("labeling the namespace to enforce restricted security policy")
		cmd = exec.Command("kubectl", "label", "--overwrite", "ns", namespace,
			"pod-security.kubernetes.io/enforce=restricted")
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred())

		By("installing CRDs")
		cmd = exec.Command("make", "install")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred())

		By("installing prometheus-operator CRDs")
		cmd = exec.Command("kubectl", "apply", "-f", "config/crd/external/")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred())

		By("deploying the operator via Helm")
		cmd = exec.Command("helm", "upgrade", "--install",
			"blackbox-exporter-operator",
			"charts/blackbox-exporter-operator",
			"--namespace", namespace,
			"--set", fmt.Sprintf("image.repository=%s", "localhost/blackbox-exporter-operator"),
			"--set", "image.tag=e2e",
			"--set", "image.pullPolicy=Never",
			"--wait", "--timeout", "120s",
		)
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		By("cleaning up test resources")
		cmd := exec.Command("kubectl", "delete", "-f", "examples/basic/", "--ignore-not-found")
		_, _ = utils.Run(cmd)

		By("uninstalling the operator")
		cmd = exec.Command("helm", "uninstall", "blackbox-exporter-operator", "--namespace", namespace)
		_, _ = utils.Run(cmd)

		By("uninstalling CRDs")
		cmd = exec.Command("make", "uninstall")
		_, _ = utils.Run(cmd)

		By("removing namespace")
		cmd = exec.Command("kubectl", "delete", "ns", namespace)
		_, _ = utils.Run(cmd)
	})

	AfterEach(func() {
		if CurrentSpecReport().Failed() {
			By("Fetching operator pod logs")
			cmd := exec.Command("kubectl", "logs", "-l",
				"app.kubernetes.io/name=blackbox-exporter-operator",
				"-n", namespace, "--tail=50")
			logs, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Operator logs:\n%s\n", logs)
			}

			By("Fetching events")
			cmd = exec.Command("kubectl", "get", "events", "-n", "monitoring", "--sort-by=.lastTimestamp")
			events, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Events:\n%s\n", events)
			}
		}
	})

	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(2 * time.Second)

	Context("Operator deployment", func() {
		It("should have the operator pod running", func() {
			verifyOperatorRunning := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pods",
					"-l", "app.kubernetes.io/name=blackbox-exporter-operator",
					"-n", namespace,
					"-o", "jsonpath={.items[0].status.phase}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"))
			}
			Eventually(verifyOperatorRunning).Should(Succeed())
		})
	})

	Context("Basic example", func() {
		BeforeAll(func() {
			By("creating monitoring namespace")
			cmd := exec.Command("kubectl", "create", "ns", "monitoring")
			_, _ = utils.Run(cmd)

			By("applying basic examples")
			cmd = exec.Command("kubectl", "apply", "-f", "examples/basic/")
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should create the blackbox-exporter Deployment", func() {
			verifyDeployment := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "deployment",
					"main-blackbox-exporter", "-n", "monitoring",
					"-o", "jsonpath={.status.readyReplicas}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("1"))
			}
			Eventually(verifyDeployment).Should(Succeed())
		})

		It("should create a Service", func() {
			cmd := exec.Command("kubectl", "get", "service",
				"main-blackbox-exporter", "-n", "monitoring",
				"-o", "jsonpath={.spec.ports[0].port}")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal("9115"))
		})

		It("should render the ConfigMap with the module", func() {
			verifyConfigMap := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "configmap",
					"main-blackbox-exporter", "-n", "monitoring",
					"-o", "jsonpath={.data.blackbox\\.yml}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("monitoring-http-2xx"))
				g.Expect(output).To(ContainSubstring("prober: http"))
			}
			Eventually(verifyConfigMap).Should(Succeed())
		})

		It("should create a prometheus-operator Probe CR", func() {
			verifyProbe := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "probe.monitoring.coreos.com",
					"public-websites", "-n", "monitoring",
					"-o", "jsonpath={.spec.module}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("monitoring-http-2xx"))
			}
			Eventually(verifyProbe).Should(Succeed())
		})

		It("should set the correct prober URL on the Probe CR", func() {
			cmd := exec.Command("kubectl", "get", "probe.monitoring.coreos.com",
				"public-websites", "-n", "monitoring",
				"-o", "jsonpath={.spec.prober.url}")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal("main-blackbox-exporter.monitoring.svc.cluster.local:9115"))
		})

		It("should set targets on the Probe CR", func() {
			cmd := exec.Command("kubectl", "get", "probe.monitoring.coreos.com",
				"public-websites", "-n", "monitoring",
				"-o", "jsonpath={.spec.targets.staticConfig.static}")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring("example.com"))
			Expect(output).To(ContainSubstring("prometheus.io"))
		})

		It("should update BlackboxExporter status", func() {
			verifyStatus := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "blackboxexporter",
					"main", "-n", "monitoring",
					"-o", "jsonpath={.status.moduleCount}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("1"))
			}
			Eventually(verifyStatus).Should(Succeed())
		})

		It("should update BlackboxProbe status with target count", func() {
			verifyStatus := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "blackboxprobe",
					"public-websites", "-n", "monitoring",
					"-o", "jsonpath={.status.targetCount}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("2"))
			}
			Eventually(verifyStatus).Should(Succeed())
		})

		It("should successfully probe a target", func() {
			verifyProbe := func(g Gomega) {
				cmd := exec.Command("kubectl", "exec", "-n", "monitoring",
					"deploy/main-blackbox-exporter", "--",
					"wget", "-qO-",
					"http://localhost:9115/probe?module=monitoring-http-2xx&target=https://example.com")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("probe_success 1"))
			}
			Eventually(verifyProbe).Should(Succeed())
		})
	})

	Context("Webhook validation", func() {
		It("should reject a BlackboxModule with no prober", func() {
			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = stringReader(`
apiVersion: monitoring.gaiser.bayern/v1alpha1
kind: BlackboxModule
metadata:
  name: invalid-no-prober
  namespace: monitoring
spec:
  timeout: 5s
`)
			_, err := utils.Run(cmd)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("exactly one prober must be configured"))
		})

		It("should reject a BlackboxProbe with scrapeTimeout > interval", func() {
			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = stringReader(`
apiVersion: monitoring.gaiser.bayern/v1alpha1
kind: BlackboxProbe
metadata:
  name: invalid-timeout
  namespace: monitoring
spec:
  exporterRef:
    name: main
  moduleRef:
    name: http-2xx
  targets:
    - https://example.com
  interval: 10s
  scrapeTimeout: 30s
`)
			_, err := utils.Run(cmd)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("scrapeTimeout"))
		})
	})
})

func stringReader(s string) io.Reader {
	return strings.NewReader(s)
}
