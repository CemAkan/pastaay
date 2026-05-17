<p align="center">
  <img src="../../docs/assets/gitops_header.png" alt="Gitops Header">
</p>

<br>

This directory contains reference architectures for deploying and managing Pastaay Chaos Policies via GitOps.

## Why GitOps for Chaos Engineering?
In enterprise environments, executing chaos strikes manually via CLI is risky and un-auditable. By adopting GitOps:
1. **Auditability:** Every chaos experiment is a Git commit. You know exactly *who* triggered it, *when*, and *why* (via Pull Request reviews).
2. **Autonomous Rollback:** Combined with Pastaay's native `duration` spec, policies are applied by ArgoCD and seamlessly reverted by the Pastaay Operator once the experiment concludes.
3. **Disaster Recovery:** If a cluster goes down, your chaos scenarios remain safely version-controlled in Git.

## Quick Start (ArgoCD)
1. Fork or create a repository for your Kubernetes manifests.
2. Place your `ChaosPolicy` YAML files in a designated path (e.g., `chaos-engineering/production`).
3. Apply the `argocd-application.yaml` to your cluster. ArgoCD will now continuously monitor your Git repository and synchronize the chaos policies directly to the Pastaay Operator.