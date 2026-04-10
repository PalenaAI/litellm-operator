---
layout: home

hero:
  name: LiteLLM Operator
  text: Kubernetes-native AI Gateway Management
  tagline: Deploy and manage production-ready LiteLLM instances with declarative custom resources. Bidirectional config sync lets you use both GitOps and the Admin UI.
  actions:
    - theme: brand
      text: Get Started
      link: /guide/getting-started
    - theme: alt
      text: View on GitHub
      link: https://github.com/PalenaAI/litellm-operator
  image:
    src: /logo.png
    alt: LiteLLM Operator

features:
  - icon: "\U0001F4E6"
    title: Declarative CRDs
    details: Manage LiteLLM instances, organizations, models, teams, users, and API keys as Kubernetes custom resources. Full GitOps compatibility.
  - icon: "\U0001F504"
    title: Bidirectional Config Sync
    details: Changes made via CRDs or the Admin UI are kept in sync. Three policies for unmanaged resources — preserve, prune, or adopt.
  - icon: "\U0001F465"
    title: SSO-Aware Team Management
    details: Three member management modes — CRD authoritative, SSO authoritative, or mixed. Never accidentally destroy SSO-provisioned memberships.
  - icon: "\U0001F511"
    title: Automatic Secret Management
    details: Virtual keys are generated via the LiteLLM API and stored in Kubernetes Secrets with owner references for automatic garbage collection.
  - icon: "\U0001F680"
    title: Production Ready
    details: HPA, PDB, NetworkPolicy, health probes, security contexts, resource limits, and database migrations out of the box.
  - icon: "\U0001F30D"
    title: Multiple Install Methods
    details: OLM bundles for OperatorHub and OpenShift, Helm chart for vanilla Kubernetes, or direct Makefile-based install.
---
