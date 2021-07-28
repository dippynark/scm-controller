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

package v1alpha1

import (
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var (
	githubwebhooklog = logf.Log.WithName("githubwebhook-resource")
	// https://docs.github.com/en/developers/webhooks-and-events/webhooks/webhook-events-and-payloads
	allowedEvents = []string{
		"*",
		"check_run",
		"check_suite",
		"code_scanning_alert",
		"commit_comment",
		"content_reference",
		"create",
		"delete",
		"deploy_key",
		"deployment",
		"deployment_status",
		"discussion",
		"discussion_comment",
		"fork",
		"github_app_authorization",
		"gollum",
		"installation",
		"installation_repositories",
		"issue_comment",
		"issues",
		"label",
		"marketplace_purchase",
		"member",
		"membership",
		"meta",
		"milestone",
		"organization",
		"org_block",
		"package",
		"page_build",
		"ping",
		"project_card",
		"project_column",
		"project",
		"public",
		"pull_request",
		"pull_request_review",
		"pull_request_review_comment",
		"push",
		"release",
		"repository_dispatch",
		"repository",
		"repository_import",
		"repository_vulnerability_alert",
		"secret_scanning_alert",
		"security_advisory",
		"sponsorship",
		"star",
		"status",
		"team",
		"team_add",
		"watch",
		"workflow_dispatch",
		"workflow_run",
	}
)

func (r *GitHubWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-scm-dippynark-co-uk-v1alpha1-githubwebhook,mutating=true,failurePolicy=fail,sideEffects=None,groups=scm.dippynark.co.uk,resources=githubwebhooks,verbs=create;update,versions=v1alpha1,name=mgithubwebhook.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Defaulter = &GitHubWebhook{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *GitHubWebhook) Default() {
	githubwebhooklog.Info("default", "name", r.Name)

	if len(r.Spec.Events) == 0 {
		r.Spec.Events = append(r.Spec.Events, "*")
	}
}

//+kubebuilder:webhook:path=/validate-scm-dippynark-co-uk-v1alpha1-githubwebhook,mutating=false,failurePolicy=fail,sideEffects=None,groups=scm.dippynark.co.uk,resources=githubwebhooks,verbs=create;update;delete,versions=v1alpha1,name=vgithubwebhook.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &GitHubWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *GitHubWebhook) ValidateCreate() error {
	githubwebhooklog.Info("validate create", "name", r.Name)

	return r.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *GitHubWebhook) ValidateUpdate(old runtime.Object) error {
	githubwebhooklog.Info("validate update", "name", r.Name)

	return r.validate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *GitHubWebhook) ValidateDelete() error {
	githubwebhooklog.Info("validate delete", "name", r.Name)

	return r.validate()
}

func (r *GitHubWebhook) validate() error {
outer:
	for _, event := range r.Spec.Events {
		// Verify "*" is not alongside other events
		if event == "*" {
			if len(r.Spec.Events) > 1 {
				return errors.New("event \"*\" only makes sense on its own")
			}
		}

		// Verify event is allowed
		for _, allowedEvent := range allowedEvents {
			if event == allowedEvent {
				continue outer
			}
		}
		return fmt.Errorf("event \"%s\" is not allowed: %v", event, allowedEvents)
	}

	return nil
}
