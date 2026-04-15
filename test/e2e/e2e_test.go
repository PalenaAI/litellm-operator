/*
Copyright 2026 bitkaio LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/PalenaAI/litellm-operator/test/utils"
)

// namespace where the project is deployed in
const namespace = "litellm-operator-system"

// serviceAccountName created for the project
const serviceAccountName = "litellm-operator-controller-manager"

// metricsServiceName is the name of the metrics service of the project
const metricsServiceName = "litellm-operator-controller-manager-metrics-service"

// metricsRoleBindingName is the name of the RBAC that will be created to allow get the metrics data
const metricsRoleBindingName = "litellm-operator-metrics-binding"

// Full-stack E2E test constants
const (
	testNamespace    = "e2e-fullstack"
	instanceName     = "e2e-litellm"
	orgName          = "e2e-org"
	modelName        = "e2e-model"
	teamName         = "e2e-team"
	orgTeamName      = "e2e-org-team"
	userName         = "e2e-user"
	customerName     = "e2e-customer"
	customerID       = "e2e-customer-42"
	virtualKeyName   = "e2e-vk"
	guardrailName    = "e2e-guardrail"
	guardrailAlias   = "e2e-pii-detector"
	guardrailEnvVar  = "GUARDRAIL_E2E_PII_DETECTOR_API_KEY"
	litellmTestImage = "ghcr.io/berriai/litellm:main-latest"
)

const postgresYAML = `
apiVersion: v1
kind: Pod
metadata:
  name: postgres
  namespace: e2e-fullstack
  labels:
    app: postgres
spec:
  containers:
  - name: postgres
    image: postgres:16-alpine
    env:
    - name: POSTGRES_USER
      value: litellm
    - name: POSTGRES_PASSWORD
      value: litellm
    - name: POSTGRES_DB
      value: litellm
    ports:
    - containerPort: 5432
    readinessProbe:
      exec:
        command: ["pg_isready", "-U", "litellm"]
      initialDelaySeconds: 5
      periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: postgres
  namespace: e2e-fullstack
spec:
  selector:
    app: postgres
  ports:
  - port: 5432
    targetPort: 5432
`

const dbSecretYAML = `
apiVersion: v1
kind: Secret
metadata:
  name: e2e-db-credentials
  namespace: e2e-fullstack
type: Opaque
stringData:
  DATABASE_URL: "postgresql://litellm:litellm@postgres.e2e-fullstack.svc:5432/litellm"
`

const fakeAPIKeySecretYAML = `
apiVersion: v1
kind: Secret
metadata:
  name: e2e-fake-api-key
  namespace: e2e-fullstack
type: Opaque
stringData:
  OPENAI_API_KEY: "sk-fake-e2e-test-key-not-real"
`

const instanceYAML = `
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMInstance
metadata:
  name: e2e-litellm
  namespace: e2e-fullstack
spec:
  image:
    repository: ghcr.io/berriai/litellm
    tag: main-latest
    pullPolicy: Always
  replicas: 1
  masterKey:
    autoGenerate: true
  database:
    external:
      connectionSecretRef:
        name: e2e-db-credentials
        key: DATABASE_URL
  service:
    type: ClusterIP
    port: 4000
`

const organizationYAML = `
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMOrganization
metadata:
  name: e2e-org
  namespace: e2e-fullstack
spec:
  instanceRef:
    name: e2e-litellm
  organizationAlias: e2e-test-org
  models:
    - e2e-gpt-4o
  maxBudget: 1000
  budgetDuration: "30d"
`

const orgTeamYAML = `
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMTeam
metadata:
  name: e2e-org-team
  namespace: e2e-fullstack
spec:
  instanceRef:
    name: e2e-litellm
  organizationRef:
    name: e2e-org
  teamAlias: e2e-org-scoped-team
  models:
    - e2e-gpt-4o
  memberManagement: crd
`

const modelYAML = `
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMModel
metadata:
  name: e2e-model
  namespace: e2e-fullstack
spec:
  instanceRef:
    name: e2e-litellm
  modelName: e2e-gpt-4o
  litellmParams:
    model: openai/gpt-4o
    apiKeySecretRef:
      name: e2e-fake-api-key
      key: OPENAI_API_KEY
`

const teamYAML = `
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMTeam
metadata:
  name: e2e-team
  namespace: e2e-fullstack
spec:
  instanceRef:
    name: e2e-litellm
  teamAlias: e2e-test-team
  models:
    - e2e-gpt-4o
  memberManagement: crd
`

const userYAML = `
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMUser
metadata:
  name: e2e-user
  namespace: e2e-fullstack
spec:
  instanceRef:
    name: e2e-litellm
  userId: e2e-test@example.com
  userEmail: e2e-test@example.com
  userRole: internal_user
`

const customerYAML = `
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMCustomer
metadata:
  name: e2e-customer
  namespace: e2e-fullstack
spec:
  instanceRef:
    name: e2e-litellm
  customerId: e2e-customer-42
  alias: e2e-test-customer
  maxBudget: 100
  budgetDuration: "30d"
  tpmLimit: 50000
  rpmLimit: 500
  models:
    - e2e-gpt-4o
  metadata:
    source: e2e
    tier: test
`

const virtualKeyYAML = `
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMVirtualKey
metadata:
  name: e2e-vk
  namespace: e2e-fullstack
spec:
  instanceRef:
    name: e2e-litellm
  keyAlias: e2e-test-key
  teamRef:
    name: e2e-team
  models:
    - e2e-gpt-4o
  keySecretName: e2e-vk-api-key
`

// guardrailYAML reuses the existing e2e-fake-api-key Secret (OPENAI_API_KEY)
// as a stand-in for a provider API key. The controller only validates that the
// Secret and key exist — no outbound network call is made to the guardrail
// provider during e2e, so a fake key is sufficient.
const guardrailYAML = `
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMGuardrail
metadata:
  name: e2e-guardrail
  namespace: e2e-fullstack
spec:
  instanceRef:
    name: e2e-litellm
  guardrailName: e2e-pii-detector
  provider: aporia
  mode: pre_call
  apiBase: https://gr-prd-dc.aporia.com
  apiKeySecretRef:
    name: e2e-fake-api-key
    key: OPENAI_API_KEY
  defaultOn: false
`

var _ = Describe("Manager", Ordered, ContinueOnFailure, func() {
	var controllerPodName string

	// Before running the tests, set up the environment by creating the namespace,
	// enforce the restricted security policy to the namespace, installing CRDs,
	// and deploying the controller.
	BeforeAll(func() {
		By("cleaning up stale cluster-scoped resources from previous runs")
		cmd := exec.Command("kubectl", "delete", "clusterrolebinding", metricsRoleBindingName, "--ignore-not-found")
		_, _ = utils.Run(cmd)

		By("creating manager namespace")
		cmd = exec.Command("kubectl", "create", "ns", namespace)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")

		By("labeling the namespace to enforce the restricted security policy")
		cmd = exec.Command("kubectl", "label", "--overwrite", "ns", namespace,
			"pod-security.kubernetes.io/enforce=restricted")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with restricted policy")

		By("installing CRDs")
		cmd = exec.Command("make", "install")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")

		By("deploying the controller-manager")
		cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectImage))
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager")
	})

	// After all tests have been executed, clean up by undeploying the controller, uninstalling CRDs,
	// and deleting the namespace.
	AfterAll(func() {
		By("cleaning up the curl pod for metrics")
		cmd := exec.Command("kubectl", "delete", "pod", "curl-metrics", "-n", namespace)
		_, _ = utils.Run(cmd)

		By("cleaning up cluster-scoped resources")
		cmd = exec.Command("kubectl", "delete", "clusterrolebinding", metricsRoleBindingName, "--ignore-not-found")
		_, _ = utils.Run(cmd)

		By("undeploying the controller-manager")
		cmd = exec.Command("make", "undeploy")
		_, _ = utils.Run(cmd)

		By("uninstalling CRDs")
		cmd = exec.Command("make", "uninstall")
		_, _ = utils.Run(cmd)

		By("removing manager namespace")
		cmd = exec.Command("kubectl", "delete", "ns", namespace)
		_, _ = utils.Run(cmd)
	})

	// After each test, check for failures and collect logs, events,
	// and pod descriptions for debugging.
	AfterEach(func() {
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			By("Fetching controller manager pod logs")
			cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
			controllerLogs, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Controller logs:\n %s", controllerLogs)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Controller logs: %s", err)
			}

			By("Fetching Kubernetes events")
			cmd = exec.Command("kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
			eventsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Kubernetes events:\n%s", eventsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Kubernetes events: %s", err)
			}

			By("Fetching curl-metrics logs")
			cmd = exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
			metricsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Metrics logs:\n %s", metricsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get curl-metrics logs: %s", err)
			}

			By("Fetching controller manager pod description")
			cmd = exec.Command("kubectl", "describe", "pod", controllerPodName, "-n", namespace)
			podDescription, err := utils.Run(cmd)
			if err == nil {
				fmt.Println("Pod description:\n", podDescription)
			} else {
				fmt.Println("Failed to describe controller pod")
			}
		}
	})

	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	Context("Manager", func() {
		It("should run successfully", func() {
			By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func(g Gomega) {
				// Get the name of the controller-manager pod
				cmd := exec.Command("kubectl", "get",
					"pods", "-l", "control-plane=controller-manager",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve controller-manager pod information")
				podNames := utils.GetNonEmptyLines(podOutput)
				g.Expect(podNames).To(HaveLen(1), "expected 1 controller pod running")
				controllerPodName = podNames[0]
				g.Expect(controllerPodName).To(ContainSubstring("controller-manager"))

				// Validate the pod's status
				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"), "Incorrect controller-manager pod status")
			}
			Eventually(verifyControllerUp).Should(Succeed())
		})

		It("should ensure the metrics endpoint is serving metrics", func() {
			By("creating a ClusterRoleBinding for the service account to allow access to metrics")
			cmd := exec.Command("kubectl", "create", "clusterrolebinding", metricsRoleBindingName,
				"--clusterrole=litellm-operator-metrics-reader",
				fmt.Sprintf("--serviceaccount=%s:%s", namespace, serviceAccountName),
			)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create ClusterRoleBinding")

			By("validating that the metrics service is available")
			cmd = exec.Command("kubectl", "get", "service", metricsServiceName, "-n", namespace)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Metrics service should exist")

			By("getting the service account token")
			token, err := serviceAccountToken()
			Expect(err).NotTo(HaveOccurred())
			Expect(token).NotTo(BeEmpty())

			By("waiting for the metrics endpoint to be ready")
			verifyMetricsEndpointReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "endpoints", metricsServiceName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("8443"), "Metrics endpoint is not ready")
			}
			Eventually(verifyMetricsEndpointReady).Should(Succeed())

			By("verifying that the controller manager is serving the metrics server")
			verifyMetricsServerStarted := func(g Gomega) {
				cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("controller-runtime.metrics\tServing metrics server"),
					"Metrics server not yet started")
			}
			Eventually(verifyMetricsServerStarted).Should(Succeed())

			By("creating the curl-metrics pod to access the metrics endpoint")
			cmd = exec.Command("kubectl", "run", "curl-metrics", "--restart=Never",
				"--namespace", namespace,
				"--image=curlimages/curl:latest",
				"--overrides",
				fmt.Sprintf(`{
					"spec": {
						"containers": [{
							"name": "curl",
							"image": "curlimages/curl:latest",
							"command": ["/bin/sh", "-c"],
							"args": ["curl -v -k -H 'Authorization: Bearer %s' https://%s.%s.svc.cluster.local:8443/metrics"],
							"securityContext": {
								"allowPrivilegeEscalation": false,
								"capabilities": {
									"drop": ["ALL"]
								},
								"runAsNonRoot": true,
								"runAsUser": 1000,
								"seccompProfile": {
									"type": "RuntimeDefault"
								}
							}
						}],
						"serviceAccount": "%s"
					}
				}`, token, metricsServiceName, namespace, serviceAccountName))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create curl-metrics pod")

			By("waiting for the curl-metrics pod to complete.")
			verifyCurlUp := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pods", "curl-metrics",
					"-o", "jsonpath={.status.phase}",
					"-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Succeeded"), "curl pod in wrong status")
			}
			Eventually(verifyCurlUp, 5*time.Minute).Should(Succeed())

			By("getting the metrics by checking curl-metrics logs")
			metricsOutput := getMetricsOutput()
			Expect(metricsOutput).To(ContainSubstring(
				"controller_runtime_reconcile_total",
			))
		})

		// +kubebuilder:scaffold:e2e-webhooks-checks
	})

	Context("Full Stack", Ordered, func() {
		BeforeAll(func() {
			By("creating test namespace")
			cmd := exec.Command("kubectl", "create", "ns", testNamespace)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterAll(func() {
			By("deleting test namespace")
			cmd := exec.Command("kubectl", "delete", "ns", testNamespace, "--ignore-not-found", "--timeout=120s")
			_, _ = utils.Run(cmd)
		})

		AfterEach(func() {
			if CurrentSpecReport().Failed() {
				By("Collecting debug info from test namespace")
				cmd := exec.Command("kubectl", "logs", "-l", "app.kubernetes.io/name=litellm",
					"-n", testNamespace, "--tail=100")
				output, err := utils.Run(cmd)
				if err == nil {
					_, _ = fmt.Fprintf(GinkgoWriter, "LiteLLM pod logs:\n%s", output)
				}

				cmd = exec.Command("kubectl", "get", "events", "-n", testNamespace,
					"--sort-by=.lastTimestamp")
				output, err = utils.Run(cmd)
				if err == nil {
					_, _ = fmt.Fprintf(GinkgoWriter, "Test namespace events:\n%s", output)
				}

				kinds := []string{
					"litellminstance", "litellmorganization", "litellmmodel",
					"litellmteam", "litellmuser", "litellmcustomer", "litellmvirtualkey",
					"litellmguardrail",
				}
				for _, kind := range kinds {
					cmd = exec.Command("kubectl", "get", kind, "-n", testNamespace, "-o", "yaml")
					output, err = utils.Run(cmd)
					if err == nil {
						_, _ = fmt.Fprintf(GinkgoWriter, "%s resources:\n%s", kind, output)
					}
				}

				cmd = exec.Command("kubectl", "describe", "pods", "-n", testNamespace)
				output, err = utils.Run(cmd)
				if err == nil {
					_, _ = fmt.Fprintf(GinkgoWriter, "Pod descriptions:\n%s", output)
				}
			}
		})

		It("should deploy PostgreSQL and wait for it to be ready", func() {
			By("deploying PostgreSQL pod and service")
			applyYAML(postgresYAML)

			By("waiting for PostgreSQL to be ready")
			cmd := exec.Command("kubectl", "wait", "--for=condition=Ready", "pod/postgres",
				"-n", testNamespace, "--timeout=120s")
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "PostgreSQL pod did not become ready")
		})

		It("should create database and API key secrets", func() {
			applyYAML(dbSecretYAML)
			applyYAML(fakeAPIKeySecretYAML)
		})

		It("should create a LiteLLMInstance and wait for it to become Ready", func() {
			By("applying the LiteLLMInstance CR")
			applyYAML(instanceYAML)

			By("waiting for the LiteLLM deployment pod to be ready")
			verifyDeploymentReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "deployment", instanceName,
					"-n", testNamespace, "-o", "jsonpath={.status.readyReplicas}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(strings.TrimSpace(output)).To(Equal("1"), "expected 1 ready replica")
			}
			Eventually(verifyDeploymentReady, 10*time.Minute, 10*time.Second).Should(Succeed())

			By("waiting for the LiteLLMInstance Ready condition")
			verifyInstanceReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "litellminstance", instanceName,
					"-n", testNamespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"))
			}
			Eventually(verifyInstanceReady, 2*time.Minute, 10*time.Second).Should(Succeed())
		})

		It("should have created the expected Kubernetes resources", func() {
			By("verifying Deployment exists")
			cmd := exec.Command("kubectl", "get", "deployment", instanceName, "-n", testNamespace)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("verifying Service exists")
			cmd = exec.Command("kubectl", "get", "service", instanceName, "-n", testNamespace)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("verifying ConfigMap exists")
			cmd = exec.Command("kubectl", "get", "configmap", instanceName+"-config", "-n", testNamespace)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("verifying auto-generated master key Secret exists")
			cmd = exec.Command("kubectl", "get", "secret", instanceName+"-master-key", "-n", testNamespace)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should create a LiteLLMModel and wait for Synced", func() {
			By("ensuring the LiteLLM instance is ready before creating the model")
			verifyInstanceReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "litellminstance", instanceName,
					"-n", testNamespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"))
			}
			Eventually(verifyInstanceReady, 10*time.Minute, 10*time.Second).Should(Succeed())

			By("applying the LiteLLMModel CR")
			applyYAML(modelYAML)

			By("waiting for the model to be synced")
			verifyModelSynced := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "litellmmodel", modelName,
					"-n", testNamespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Synced')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"))
			}
			Eventually(verifyModelSynced, 2*time.Minute, 5*time.Second).Should(Succeed())

			By("verifying the model has a LiteLLM ID")
			cmd := exec.Command("kubectl", "get", "litellmmodel", modelName,
				"-n", testNamespace, "-o", "jsonpath={.status.litellmModelId}")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).NotTo(BeEmpty(), "litellmModelId should be set after sync")
		})

		It("should create a LiteLLMOrganization and wait for Synced", func() {
			By("applying the LiteLLMOrganization CR")
			applyYAML(organizationYAML)

			By("waiting for the organization to be synced")
			verifyOrgSynced := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "litellmorganization", orgName,
					"-n", testNamespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Synced')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"))
			}
			Eventually(verifyOrgSynced, 2*time.Minute, 5*time.Second).Should(Succeed())

			By("verifying the organization has a LiteLLM ID")
			cmd := exec.Command("kubectl", "get", "litellmorganization", orgName,
				"-n", testNamespace, "-o", "jsonpath={.status.litellmOrganizationId}")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).NotTo(BeEmpty(), "litellmOrganizationId should be set after sync")
		})

		It("should create a LiteLLMTeam scoped to an organization", func() {
			By("applying the org-scoped LiteLLMTeam CR")
			applyYAML(orgTeamYAML)

			By("waiting for the org-scoped team to be synced")
			verifyOrgTeamSynced := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "litellmteam", orgTeamName,
					"-n", testNamespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Synced')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"))
			}
			Eventually(verifyOrgTeamSynced, 2*time.Minute, 5*time.Second).Should(Succeed())

			By("verifying the org-scoped team has a LiteLLM ID")
			cmd := exec.Command("kubectl", "get", "litellmteam", orgTeamName,
				"-n", testNamespace, "-o", "jsonpath={.status.litellmTeamId}")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).NotTo(BeEmpty(), "litellmTeamId should be set for org-scoped team")
		})

		It("should create a LiteLLMTeam and wait for Synced", func() {
			By("applying the LiteLLMTeam CR")
			applyYAML(teamYAML)

			By("waiting for the team to be synced")
			verifyTeamSynced := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "litellmteam", teamName,
					"-n", testNamespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Synced')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"))
			}
			Eventually(verifyTeamSynced, 2*time.Minute, 5*time.Second).Should(Succeed())

			By("verifying the team has a LiteLLM ID")
			cmd := exec.Command("kubectl", "get", "litellmteam", teamName,
				"-n", testNamespace, "-o", "jsonpath={.status.litellmTeamId}")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).NotTo(BeEmpty(), "litellmTeamId should be set after sync")
		})

		It("should create a LiteLLMUser and wait for Synced", func() {
			By("applying the LiteLLMUser CR")
			applyYAML(userYAML)

			By("waiting for the user to be synced")
			verifyUserSynced := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "litellmuser", userName,
					"-n", testNamespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Synced')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"))
			}
			Eventually(verifyUserSynced, 2*time.Minute, 5*time.Second).Should(Succeed())

			By("verifying the user has a LiteLLM ID")
			cmd := exec.Command("kubectl", "get", "litellmuser", userName,
				"-n", testNamespace, "-o", "jsonpath={.status.litellmUserId}")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).NotTo(BeEmpty(), "litellmUserId should be set after sync")
		})

		It("should create a LiteLLMCustomer and wait for Synced", func() {
			By("applying the LiteLLMCustomer CR")
			applyYAML(customerYAML)

			By("waiting for the customer to be synced")
			verifyCustomerSynced := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "litellmcustomer", customerName,
					"-n", testNamespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Synced')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"))
			}
			Eventually(verifyCustomerSynced, 2*time.Minute, 5*time.Second).Should(Succeed())

			By("verifying the customer status.synced flag is set")
			cmd := exec.Command("kubectl", "get", "litellmcustomer", customerName,
				"-n", testNamespace, "-o", "jsonpath={.status.synced}")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(strings.TrimSpace(output)).To(Equal("true"), "status.synced should be true after sync")
		})

		It("should create a LiteLLMVirtualKey and wait for Synced", func() {
			By("applying the LiteLLMVirtualKey CR")
			applyYAML(virtualKeyYAML)

			By("waiting for the virtual key to be synced")
			verifyVKSynced := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "litellmvirtualkey", virtualKeyName,
					"-n", testNamespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Synced')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"))
			}
			Eventually(verifyVKSynced, 2*time.Minute, 5*time.Second).Should(Succeed())

			By("verifying the API key Secret was created")
			cmd := exec.Command("kubectl", "get", "secret", "e2e-vk-api-key",
				"-n", testNamespace, "-o", "jsonpath={.data.api_key}")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).NotTo(BeEmpty(), "api_key should be populated in the Secret")
		})

		It("should create a LiteLLMGuardrail and wait for Ready", func() {
			By("applying the LiteLLMGuardrail CR")
			applyYAML(guardrailYAML)

			By("waiting for the guardrail Ready condition")
			verifyGuardrailReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "litellmguardrail", guardrailName,
					"-n", testNamespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"))
			}
			Eventually(verifyGuardrailReady, 2*time.Minute, 5*time.Second).Should(Succeed())

			By("verifying status.configured is true")
			cmd := exec.Command("kubectl", "get", "litellmguardrail", guardrailName,
				"-n", testNamespace, "-o", "jsonpath={.status.configured}")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(strings.TrimSpace(output)).To(Equal("true"),
				"status.configured should be true after guardrail validation")
		})

		It("should render the guardrail into the ConfigMap and inject the env var", func() {
			By("waiting for the guardrail entry to appear in the instance ConfigMap")
			verifyConfigMap := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "configmap", instanceName+"-config",
					"-n", testNamespace, "-o", `jsonpath={.data.proxy_server_config\.yaml}`)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("guardrail_name: "+guardrailAlias),
					"rendered config should contain the guardrail_name")
				g.Expect(output).To(ContainSubstring("guardrail: aporia"),
					"rendered config should contain the provider")
				g.Expect(output).To(ContainSubstring("mode: pre_call"),
					"rendered config should contain the execution mode")
				g.Expect(output).To(ContainSubstring("os.environ/"+guardrailEnvVar),
					"rendered config should reference the api_key via os.environ/…")
			}
			Eventually(verifyConfigMap, 2*time.Minute, 5*time.Second).Should(Succeed())

			By("verifying the guardrail env var is injected into the Deployment")
			verifyDeploymentEnvVar := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "deployment", instanceName,
					"-n", testNamespace,
					"-o", fmt.Sprintf(
						"jsonpath={.spec.template.spec.containers[0].env[?(@.name=='%s')].valueFrom.secretKeyRef.name}",
						guardrailEnvVar))
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(strings.TrimSpace(output)).To(Equal("e2e-fake-api-key"),
					"env var should be backed by a secretKeyRef pointing at the fake-api-key Secret")
			}
			Eventually(verifyDeploymentEnvVar, 2*time.Minute, 5*time.Second).Should(Succeed())
		})

		It("should delete VirtualKey and verify cleanup", func() {
			By("deleting the LiteLLMVirtualKey")
			cmd := exec.Command("kubectl", "delete", "litellmvirtualkey", virtualKeyName,
				"-n", testNamespace, "--timeout=60s")
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("verifying the VirtualKey is gone")
			verifyVKDeleted := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "litellmvirtualkey", virtualKeyName,
					"-n", testNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).To(HaveOccurred())
			}
			Eventually(verifyVKDeleted, 1*time.Minute, 5*time.Second).Should(Succeed())

			By("verifying the owned API key Secret is garbage collected")
			verifySecretDeleted := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "secret", "e2e-vk-api-key",
					"-n", testNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).To(HaveOccurred())
			}
			Eventually(verifySecretDeleted, 1*time.Minute, 5*time.Second).Should(Succeed())
		})

		It("should delete Guardrail and verify it is stripped from the ConfigMap", func() {
			By("deleting the LiteLLMGuardrail")
			cmd := exec.Command("kubectl", "delete", "litellmguardrail", guardrailName,
				"-n", testNamespace, "--timeout=60s")
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("verifying the Guardrail CR is gone")
			verifyDeleted := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "litellmguardrail", guardrailName,
					"-n", testNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).To(HaveOccurred())
			}
			Eventually(verifyDeleted, 1*time.Minute, 5*time.Second).Should(Succeed())

			By("verifying the guardrail entry is removed from the ConfigMap")
			verifyConfigMapCleaned := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "configmap", instanceName+"-config",
					"-n", testNamespace, "-o", `jsonpath={.data.proxy_server_config\.yaml}`)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).NotTo(ContainSubstring("guardrail_name: "+guardrailAlias),
					"guardrail entry should be removed from rendered config")
				g.Expect(output).NotTo(ContainSubstring("os.environ/"+guardrailEnvVar),
					"guardrail env var reference should be removed from rendered config")
			}
			Eventually(verifyConfigMapCleaned, 2*time.Minute, 5*time.Second).Should(Succeed())

			By("verifying the guardrail env var is removed from the Deployment")
			verifyDeploymentEnvVarGone := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "deployment", instanceName,
					"-n", testNamespace,
					"-o", fmt.Sprintf(
						"jsonpath={.spec.template.spec.containers[0].env[?(@.name=='%s')].name}",
						guardrailEnvVar))
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(strings.TrimSpace(output)).To(BeEmpty(),
					"guardrail env var should no longer be present on the Deployment")
			}
			Eventually(verifyDeploymentEnvVarGone, 2*time.Minute, 5*time.Second).Should(Succeed())
		})

		It("should delete Customer and verify cleanup", func() {
			cmd := exec.Command("kubectl", "delete", "litellmcustomer", customerName,
				"-n", testNamespace, "--timeout=60s")
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			verifyDeleted := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "litellmcustomer", customerName,
					"-n", testNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).To(HaveOccurred())
			}
			Eventually(verifyDeleted, 1*time.Minute, 5*time.Second).Should(Succeed())
		})

		It("should delete User and verify cleanup", func() {
			cmd := exec.Command("kubectl", "delete", "litellmuser", userName,
				"-n", testNamespace, "--timeout=60s")
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			verifyDeleted := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "litellmuser", userName,
					"-n", testNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).To(HaveOccurred())
			}
			Eventually(verifyDeleted, 1*time.Minute, 5*time.Second).Should(Succeed())
		})

		It("should delete Team and verify cleanup", func() {
			cmd := exec.Command("kubectl", "delete", "litellmteam", teamName,
				"-n", testNamespace, "--timeout=60s")
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			verifyDeleted := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "litellmteam", teamName,
					"-n", testNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).To(HaveOccurred())
			}
			Eventually(verifyDeleted, 1*time.Minute, 5*time.Second).Should(Succeed())
		})

		It("should delete org-scoped Team and verify cleanup", func() {
			cmd := exec.Command("kubectl", "delete", "litellmteam", orgTeamName,
				"-n", testNamespace, "--timeout=60s")
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			verifyDeleted := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "litellmteam", orgTeamName,
					"-n", testNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).To(HaveOccurred())
			}
			Eventually(verifyDeleted, 1*time.Minute, 5*time.Second).Should(Succeed())
		})

		It("should delete Organization and verify cleanup", func() {
			cmd := exec.Command("kubectl", "delete", "litellmorganization", orgName,
				"-n", testNamespace, "--timeout=60s")
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			verifyDeleted := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "litellmorganization", orgName,
					"-n", testNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).To(HaveOccurred())
			}
			Eventually(verifyDeleted, 1*time.Minute, 5*time.Second).Should(Succeed())
		})

		It("should delete Model and verify cleanup", func() {
			cmd := exec.Command("kubectl", "delete", "litellmmodel", modelName,
				"-n", testNamespace, "--timeout=60s")
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			verifyDeleted := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "litellmmodel", modelName,
					"-n", testNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).To(HaveOccurred())
			}
			Eventually(verifyDeleted, 1*time.Minute, 5*time.Second).Should(Succeed())
		})

		It("should delete the LiteLLMInstance and verify owned resources are cleaned up", func() {
			By("deleting the LiteLLMInstance")
			cmd := exec.Command("kubectl", "delete", "litellminstance", instanceName,
				"-n", testNamespace, "--timeout=120s")
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("verifying owned resources are garbage collected")
			verifyCleanup := func(g Gomega) {
				// Deployment should be gone
				cmd := exec.Command("kubectl", "get", "deployment", instanceName, "-n", testNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).To(HaveOccurred())

				// Service should be gone
				cmd = exec.Command("kubectl", "get", "service", instanceName, "-n", testNamespace)
				_, err = utils.Run(cmd)
				g.Expect(err).To(HaveOccurred())

				// ConfigMap should be gone
				cmd = exec.Command("kubectl", "get", "configmap", instanceName+"-config", "-n", testNamespace)
				_, err = utils.Run(cmd)
				g.Expect(err).To(HaveOccurred())

				// Master key Secret should be gone
				cmd = exec.Command("kubectl", "get", "secret", instanceName+"-master-key", "-n", testNamespace)
				_, err = utils.Run(cmd)
				g.Expect(err).To(HaveOccurred())
			}
			Eventually(verifyCleanup, 2*time.Minute, 5*time.Second).Should(Succeed())
		})
	})
})

// applyYAML applies a YAML manifest via kubectl stdin.
func applyYAML(yaml string) {
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(yaml)
	_, err := utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to apply YAML")
}

// serviceAccountToken returns a token for the specified service account in the given namespace.
// It uses the Kubernetes TokenRequest API to generate a token by directly sending a request
// and parsing the resulting token from the API response.
func serviceAccountToken() (string, error) {
	const tokenRequestRawString = `{
		"apiVersion": "authentication.k8s.io/v1",
		"kind": "TokenRequest"
	}`

	// Temporary file to store the token request
	secretName := fmt.Sprintf("%s-token-request", serviceAccountName)
	tokenRequestFile := filepath.Join("/tmp", secretName)
	err := os.WriteFile(tokenRequestFile, []byte(tokenRequestRawString), os.FileMode(0o644))
	if err != nil {
		return "", err
	}

	var out string
	verifyTokenCreation := func(g Gomega) {
		// Execute kubectl command to create the token
		cmd := exec.Command("kubectl", "create", "--raw", fmt.Sprintf(
			"/api/v1/namespaces/%s/serviceaccounts/%s/token",
			namespace,
			serviceAccountName,
		), "-f", tokenRequestFile)

		output, err := cmd.CombinedOutput()
		g.Expect(err).NotTo(HaveOccurred())

		// Parse the JSON output to extract the token
		var token tokenRequest
		err = json.Unmarshal(output, &token)
		g.Expect(err).NotTo(HaveOccurred())

		out = token.Status.Token
	}
	Eventually(verifyTokenCreation).Should(Succeed())

	return out, err
}

// getMetricsOutput retrieves and returns the logs from the curl pod used to access the metrics endpoint.
func getMetricsOutput() string {
	By("getting the curl-metrics logs")
	cmd := exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
	metricsOutput, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to retrieve logs from curl pod")
	Expect(metricsOutput).To(ContainSubstring("< HTTP/1.1 200 OK"))
	return metricsOutput
}

// tokenRequest is a simplified representation of the Kubernetes TokenRequest API response,
// containing only the token field that we need to extract.
type tokenRequest struct {
	Status struct {
		Token string `json:"token"`
	} `json:"status"`
}
