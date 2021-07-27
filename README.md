# scm-controller

SCM Controller allows management of SCM resources using Kubernetes. Currently only GitHubWebhooks
are supported.

## Create Webhook

> GitHub Enterprise is currently not supported

Create a GitHub personal access token with `admin:repo_hook` scope:
[https://github.com/settings/tokens/new?scopes=write:repo_hook](https://github.com/settings/tokens/new?scopes=admin:repo_hook). Admin is needed for webhook deletion.

Create a Kubernetes Secret containing the personal access token:

```sh
kubectl create namespace scm-controller-system \
  -o yaml \
  --dry-run=client \
    | kubectl apply -f -
kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: github
  namespace: scm-controller-system
stringData:
  token: ghp_EXAMPLE123
EOF
```

Deploy the controller:

```sh
make deploy
```

Create GitHubWebhook with secret:

```sh
kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: github-webhook
stringData:
  secret: c2VjcmV0
---
apiVersion: scm.dippynark.co.uk/v1alpha1
kind: GitHubWebhook
metadata:
  name: test
spec:
  repository:
    owner: example
    name: application
  payloadURL: https://test.example.com
  # Supports json or form
  contentType: json
  secret:
    name: github-webhook
    key: secret
  insecureSSL: false
  events:
  - "*"
  active: true
EOF
```

## Adoption

GitHub does not allow two different webhooks with the same parameters (unless they are created
inactive). For safety we opt to not allow SCM controller to adopt existing webhooks and instead
raise an error if there is a collision.

## TODO

- Add conditions (e.g. readiness) and add as columns
