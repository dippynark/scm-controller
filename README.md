# scm-controller

Need access token with `admin:repo_hook`.

```sh
kubebuilder init --domain dippynark.co.uk # --repo github.com/dippynark/scm-controller
kubebuilder create api --group scm --version v1alpha1 --kind GitHubWebhook
```

```sh
kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: github
  namespace: scm-controller-system
stringData:
  # dippynark personal access token with admin:repo_hook scope
  token: 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
EOF
```
