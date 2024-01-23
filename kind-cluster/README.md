# kind-tekton-argocd

Demo to deploy a local Kind cluster ArgoCD

## Prepare WSL

```bash
pip install ansible ansible-lint kubernetes docker passlib bcrypt
```

or shorter

```bash
pip install -r requirements.txt
```

## Run Playbook

To set up a Kind-Cluster with ArgoCD and Tekton:

```bash
ansible-playbook kind.yml
```

To tear it down afterwards:

```bash
ansible-playbook -e state=absent kind.yml
```

## Port-Forward Argo

```bash
kubectl port-forward svc/argocd-server 8443:443 -n argocd
```
