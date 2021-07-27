module github.com/dippynark/scm-controller

go 1.16

require (
	github.com/go-logr/logr v0.4.0
	github.com/google/go-github/v31 v31.0.0
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.13.0
	github.com/pkg/errors v0.9.1
	golang.org/x/oauth2 v0.0.0-20210615190721-d04028783cf1
	k8s.io/api v0.21.2
	k8s.io/apimachinery v0.21.2
	k8s.io/client-go v0.21.2
	k8s.io/utils v0.0.0-20210527160623-6fdb442a123b
	sigs.k8s.io/cluster-api v0.4.0
	sigs.k8s.io/controller-runtime v0.9.1
)
