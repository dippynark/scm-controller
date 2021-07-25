# scm-controller

SCM Controller allows management of SCM resources using Kubernetes. Currently only GitHubWebhooks
are supported.

## Install

Create personal access token with `admin:repo_hook` scope:
[https://github.com/settings/tokens/new?scopes=write:repo_hook](https://github.com/settings/tokens/new?scopes=admin:repo_hook). Admin is needed for webhook deletion.

Create Kubernetes Secret containing the personal access token and deploy the controller.

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
make deploy
```

## Example

An example can be found [here](config/samples/scm_v1alpha1_githubwebhook.yaml).

## Adoption

GitHub does not allow two different webhooks with the same parameters (unless they are created
diabled). For safety we opt to not allow SCM controller to adopt existing webhooks and instead raise
an error if there is a collision.

## TODO

- Add conditions (e.g. readiness) and add as columns
- Raise detailed event if a matching webhook already exists (and other similar errors)
- If ID is set there is no need to list all hooks, just get the specified one by ID
