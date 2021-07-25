/*
Copyright 2021.

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

package controllers

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/google/go-github/v31/github"
	"golang.org/x/oauth2"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/dippynark/scm-controller/api/v1alpha1"
	scmv1alpha1 "github.com/dippynark/scm-controller/api/v1alpha1"
)

const (
	githubTokenEnvVar = "GITHUB_TOKEN"
)

// GitHubWebhookReconciler reconciles a GitHubWebhook object
type GitHubWebhookReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=scm.dippynark.co.uk,resources=githubwebhooks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=scm.dippynark.co.uk,resources=githubwebhooks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=scm.dippynark.co.uk,resources=githubwebhooks/finalizers,verbs=update

// Reconcile GitHubWebhook
func (r *GitHubWebhookReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, rerr error) {
	log := log.Log.WithValues(namespaceLogName, req.Namespace, gitHubWebhookLogName, req.Name)

	// Fetch the GitHubWebhook instance
	gitHubWebhook := &v1alpha1.GitHubWebhook{}
	if err := r.Client.Get(ctx, req.NamespacedName, gitHubWebhook); err != nil {
		if k8serrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Initialise patch helper
	patchHelper, err := patch.NewHelper(gitHubWebhook, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	// Always attempt to patch the GitHubWebhook object and status after each reconciliation
	defer func() {
		// r.reconcilePhase(gitHubWebhook)

		if err := patchHelper.Patch(ctx, gitHubWebhook); err != nil {
			log.Error(err, "failed to patch GitHubWebhook")
			if rerr == nil {
				rerr = err
			}
		}
	}()

	// https://github.com/google/go-github#authentication
	githubToken := os.Getenv(githubTokenEnvVar)
	fmt.Print(githubToken)
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubToken},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	// List hooks across all pages
	hooks := []*github.Hook{}
	nextPage := 1
	for nextPage > 0 {
		listOptions := &github.ListOptions{Page: nextPage}
		newHooks, response, err := client.Repositories.ListHooks(ctx,
			gitHubWebhook.Spec.Repository.Owner,
			gitHubWebhook.Spec.Repository.Name,
			listOptions)
		if err != nil {
			return ctrl.Result{}, err
		}
		hooks = append(hooks, newHooks...)
		nextPage = response.NextPage
	}

	for _, hook := range hooks {
		fmt.Printf("%#v\n", *hook)
	}

	gitHubHookExists, hook := gitHubHookExists(gitHubWebhook, hooks)
	if !gitHubHookExists {
		events := gitHubWebhook.Spec.Events
		// TODO: use defaulting webhook
		if gitHubWebhook.Spec.Events == nil {
			events = []string{"*"}
		}
		hook = &github.Hook{
			Active: newTrue(),
			Events: events,
			Config: map[string]interface{}{
				"url":          gitHubWebhook.Spec.PayloadURL,
				"content_type": gitHubWebhook.Spec.ContentType,
				// TODO: implement secret
				// "secret":       secret,
				"insecure_ssl": boolToInt(gitHubWebhook.Spec.InsecureSSL),
			},
		}
		hook, _, err = client.Repositories.CreateHook(ctx, gitHubWebhook.Spec.Repository.Owner, gitHubWebhook.Spec.Repository.Name, hook)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// Webhook exists so save ID
	id := hook.GetID()
	gitHubWebhook.Spec.ID = &id

	// At this point the webhook has been created in GitHub (or has matched with an existing one) and
	// we have set the correponding ID in the spec. We now need to work out whether to edit the
	// webhook in GitHub by comparing each field
	editWebhook, hook, err := gitHubHookNeedsEdit(gitHubWebhook, hook)
	if err != nil {
		return ctrl.Result{}, err
	}

	if editWebhook {
		_, _, err := client.Repositories.EditHook(ctx, gitHubWebhook.Spec.Repository.Owner, gitHubWebhook.Spec.Repository.Name, *gitHubWebhook.Spec.ID, hook)
		if err != nil {
			return ctrl.Result{}, err
		}
		log.Info("Webhook updated successfully!")
		// TODO: return hook and check values have actually changed
	}

	return ctrl.Result{}, nil
}

func gitHubHookNeedsEdit(gitHubWebhook *v1alpha1.GitHubWebhook, hook *github.Hook) (bool, *github.Hook, error) {
	editWebhook := false

	// Check whether to update active
	if gitHubWebhook.Spec.Active != hook.GetActive() {
		fmt.Println("Active differs!")
		hook.Active = &gitHubWebhook.Spec.Active
		editWebhook = true
	}
	// Check update to events
outer:
	for _, desiredEvent := range gitHubWebhook.Spec.Events {
		for _, actualEvent := range hook.Events {
			// Check whether an element is missing from the 'opposite' list. This allows for duplicate entries
			// TODO: Should we allow duplicate entries?
			if !stringInSlice(desiredEvent, hook.Events) || !stringInSlice(actualEvent, gitHubWebhook.Spec.Events) {
				hook.Events = gitHubWebhook.Spec.Events
				editWebhook = true
				break outer
			}
		}
	}
	// Check update to URL
	if _, ok := hook.Config["url"]; ok {
		if url, ok := hook.Config["url"].(string); ok {
			if gitHubWebhook.Spec.PayloadURL != url {
				hook.Config["url"] = gitHubWebhook.Spec.PayloadURL
				editWebhook = true
			}
		} else {
			return false, hook, errors.New("failed to type cast URL parameter")
		}
	} else {
		hook.Config["url"] = gitHubWebhook.Spec.PayloadURL
		editWebhook = true
	}
	// Check update to content type
	if _, ok := hook.Config["content_type"]; ok {
		if contentType, ok := hook.Config["content_type"].(string); ok {
			if gitHubWebhook.Spec.ContentType != contentType {
				hook.Config["content_type"] = gitHubWebhook.Spec.ContentType
				editWebhook = true
			}
		} else {
			return false, hook, errors.New("failed to type cast content type parameter")
		}
	} else {
		hook.Config["content_type"] = gitHubWebhook.Spec.ContentType
		editWebhook = true
	}
	// Check update to insecure SSL
	if _, ok := hook.Config["insecure_ssl"]; ok {
		if insecureSSL, ok := hook.Config["insecure_ssl"].(string); ok {
			if gitHubWebhook.Spec.InsecureSSL != stringToBool(insecureSSL) {
				hook.Config["insecure_ssl"] = boolToInt(gitHubWebhook.Spec.InsecureSSL)
				editWebhook = true
			}
		} else {
			return false, hook, errors.New("failed to type cast insecure SSL parameter")
		}
	} else {
		hook.Config["insecure_ssl"] = boolToInt(gitHubWebhook.Spec.InsecureSSL)
		editWebhook = true
	}

	return editWebhook, hook, nil
}

func gitHubHookExists(gitHubWebhook *v1alpha1.GitHubWebhook, hooks []*github.Hook) (bool, *github.Hook) {

	if gitHubWebhook.Spec.ID == nil {
		for _, hook := range hooks {
			// Check events match
			for _, desiredEvent := range gitHubWebhook.Spec.Events {
				for _, actualEvent := range hook.Events {
					// Check whether an element is missing from the 'opposite' list. This allows for duplicate entries
					// TODO: Should we allow duplicate entries?
					if !stringInSlice(desiredEvent, hook.Events) || !stringInSlice(actualEvent, gitHubWebhook.Spec.Events) {
						return false, nil
					}
				}
			}
			// Check whether URL matches
			if url, ok := hook.Config["url"]; ok {
				if gitHubWebhook.Spec.PayloadURL != url {
					continue
				}
			} else {
				continue
			}
			// Check whether content type matches
			if contentType, ok := hook.Config["content_type"]; ok {
				if gitHubWebhook.Spec.ContentType != contentType {
					continue
				}
			} else {
				continue
			}
			// Check whether insecure SSL matches
			if insecureSSL, ok := hook.Config["insecure_ssl"]; ok {
				if insecureSSL, ok := insecureSSL.(string); ok {
					if gitHubWebhook.Spec.InsecureSSL != stringToBool(insecureSSL) {
						continue
					}
				} else {
					continue
				}
			} else {
				continue
			}

			return true, hook
		}
	} else {
		for _, hook := range hooks {
			if *gitHubWebhook.Spec.ID == hook.GetID() {
				return true, hook
			}
		}
	}

	return false, nil
}

func stringInSlice(s string, list []string) bool {
	for _, l := range list {
		if s == l {
			return true
		}
	}
	return false
}

func stringToBool(s string) bool {
	return s == "1"
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func newTrue() *bool {
	r := true
	return &r
}

// SetupWithManager sets up the controller with the Manager.
func (r *GitHubWebhookReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&scmv1alpha1.GitHubWebhook{}).
		Complete(r)
}
