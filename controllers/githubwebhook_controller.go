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
	"crypto/sha256"
	"fmt"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/go-github/v31/github"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	scmv1alpha1 "github.com/dippynark/scm-controller/api/v1alpha1"
)

const (
	namespaceLogName                 = "namespace"
	gitHubWebhookLogName             = "githubwebhook"
	gitHubWebhookSecretRequeuePeriod = 120 * time.Second
)

// GitHubWebhookReconciler reconciles a GitHubWebhook object
type GitHubWebhookReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	GitHubClient *github.Client
	Recorder     record.EventRecorder
}

//+kubebuilder:rbac:groups=scm.dippynark.co.uk,resources=githubwebhooks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=scm.dippynark.co.uk,resources=githubwebhooks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=scm.dippynark.co.uk,resources=githubwebhooks/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile GitHubWebhook
func (r *GitHubWebhookReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, rerr error) {
	log := log.Log.WithValues(namespaceLogName, req.Namespace, gitHubWebhookLogName, req.Name)

	// Fetch the GitHubWebhook instance
	gitHubWebhook := &scmv1alpha1.GitHubWebhook{}
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
		reconcilePhase(gitHubWebhook)

		if err := patchHelper.Patch(ctx, gitHubWebhook); err != nil {
			log.Error(err, "failed to patch GitHubWebhook")
			if rerr == nil {
				rerr = err
			}
		}

		log.Info("GitHub webhook successfully reconciled!")
	}()

	if !gitHubWebhook.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, log, gitHubWebhook)
	}

	return r.reconcileNormal(ctx, log, gitHubWebhook)
}

func (r *GitHubWebhookReconciler) reconcileNormal(ctx context.Context, log logr.Logger, gitHubWebhook *scmv1alpha1.GitHubWebhook) (ctrl.Result, error) {

	if gitHubWebhook.Spec.Suspend {
		return ctrl.Result{}, nil
	}

	// Default status
	gitHubWebhook.Status.Repository = fmt.Sprintf("%s/%s", gitHubWebhook.Spec.Repository.Owner, gitHubWebhook.Spec.Repository.Name)
	gitHubWebhook.Status.FailureMessage = nil
	gitHubWebhook.Status.Ready = false
	gitHubWebhook.Status.Conditions = setCondition(gitHubWebhook.Status.Conditions, scmv1alpha1.ReadyGitHubWebhookConditionType, v1.ConditionUnknown, "ReconciliationStarted", "")

	// Add finalizer
	controllerutil.AddFinalizer(gitHubWebhook, scmv1alpha1.GitHubWebhookFinalizer)

	// Retrieve webhook secret
	webhookSecret := ""
	if gitHubWebhook.Spec.Secret != nil {
		webhookSecretObject := &corev1.Secret{}
		err := r.Client.Get(ctx, types.NamespacedName{
			Namespace: gitHubWebhook.Namespace,
			Name:      gitHubWebhook.Spec.Secret.Name,
		}, webhookSecretObject)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				if gitHubWebhook.Spec.Secret.Optional == nil || !*gitHubWebhook.Spec.Secret.Optional {
					err = fmt.Errorf("failed to find secret: %s/%s",
						gitHubWebhook.Namespace,
						gitHubWebhook.Spec.Secret.Name)
					gitHubWebhook.Status.SetFailureMessage(err)
					return ctrl.Result{}, err
				}
			} else {
				gitHubWebhook.Status.SetFailureMessage(err)
				return ctrl.Result{}, err
			}
		} else {
			webhookSecretBytes, ok := webhookSecretObject.Data[gitHubWebhook.Spec.Secret.Key]
			// If the key does not exist and it is not optional then return an error
			if !ok && (gitHubWebhook.Spec.Secret.Optional == nil || !*gitHubWebhook.Spec.Secret.Optional) {
				gitHubWebhook.Status.SetFailureMessage(err)
				err = fmt.Errorf("failed to find secret key: \"%s\"", gitHubWebhook.Spec.Secret.Key)
				return ctrl.Result{}, err
			}
			if ok {
				webhookSecret = string(webhookSecretBytes)
			}
		}
	}

	// Attempt to find webhook by ID
	var hook *github.Hook
	if gitHubWebhook.Spec.ID != nil {
		var err error
		var resp *github.Response
		hook, resp, err = r.GitHubClient.Repositories.GetHook(ctx, gitHubWebhook.Spec.Repository.Owner, gitHubWebhook.Spec.Repository.Name, *gitHubWebhook.Spec.ID)
		if err != nil {
			// We do not care if the webhook is not found
			if resp.StatusCode != 404 {
				gitHubWebhook.Status.SetFailureMessage(err)
				return ctrl.Result{}, err
			}
			// Hook doesn't exist anymore so clear ID field
			gitHubWebhook.Spec.ID = nil
		}
	}

	// If we can not find an existing webhook then create a new one
	if hook == nil {
		// Defaulting webhook should ensure Events are not nil
		events := gitHubWebhook.Spec.Events
		hook = &github.Hook{
			Active: &gitHubWebhook.Spec.Active,
			Events: events,
			Config: map[string]interface{}{
				"url":          gitHubWebhook.Spec.PayloadURL,
				"content_type": gitHubWebhook.Spec.ContentType,
				"secret":       webhookSecret,
				"insecure_ssl": boolToInt(gitHubWebhook.Spec.InsecureSSL),
			},
		}
		var err error
		var resp *github.Response
		hook, resp, err = r.GitHubClient.Repositories.CreateHook(ctx, gitHubWebhook.Spec.Repository.Owner, gitHubWebhook.Spec.Repository.Name, hook)
		if err != nil {
			response := github.CheckResponse(resp.Response)
			errorResponse, ok := response.(*github.ErrorResponse)
			if ok {
				err = errors.New(errorResponse.Message)
				for _, responseError := range errorResponse.Errors {
					err = errors.Errorf("%s: %s", err, responseError.Message)
				}
			}
			r.Recorder.Event(gitHubWebhook, "Warning", "CreateFailed", err.Error())
			gitHubWebhook.Status.SetFailureMessage(err)
			return ctrl.Result{}, err
		}
		r.Recorder.Event(gitHubWebhook, "Normal", "Create", "Webhook created")

		// Set status immediately to prevent editing newly created webhooks

		// Set last update time
		createdAt := v1.NewTime(hook.GetCreatedAt())
		gitHubWebhook.Status.LastObserveredUpdateTime = &createdAt
		// Set webhook secret hash
		sum := sha256.Sum256([]byte(webhookSecret))
		webhookSecretHash := fmt.Sprintf("%x", sum)
		gitHubWebhook.Status.LastObservedSecretHash = &webhookSecretHash

		log.Info("GitHub webhook successfully created!")
	}

	// Webhook created or found so save ID
	id := hook.GetID()
	if gitHubWebhook.Spec.ID == nil || *gitHubWebhook.Spec.ID != id {
		gitHubWebhook.Spec.ID = &id
		// Rely on generated event to continue reconciliation
		return ctrl.Result{}, nil
	}

	// At this point the webhook has been created in GitHub and we have set the correponding ID in the
	// spec. We now need to work out whether to edit the webhook in GitHub by comparing the desired
	// fields against the actual fields
	editWebhook, hook, err := gitHubHookNeedsEdit(log, gitHubWebhook, hook, webhookSecret)
	if err != nil {
		gitHubWebhook.Status.SetFailureMessage(err)
		return ctrl.Result{}, err
	}

	if editWebhook {
		var err error
		hook, _, err = r.GitHubClient.Repositories.EditHook(ctx, gitHubWebhook.Spec.Repository.Owner, gitHubWebhook.Spec.Repository.Name, *gitHubWebhook.Spec.ID, hook)
		if err != nil {
			r.Recorder.Event(gitHubWebhook, "Warning", "EditFailed", err.Error())
			gitHubWebhook.Status.SetFailureMessage(err)
			return ctrl.Result{}, err
		}
		updatedAt := v1.NewTime(hook.GetUpdatedAt())
		gitHubWebhook.Status.LastObserveredUpdateTime = &updatedAt
		// TODO: return hook and check values have actually changed
		r.Recorder.Event(gitHubWebhook, "Normal", "Edit", "Webhook edited")
		log.Info("GitHub webhook successfully edited!")
	}

	// Set readiness and condtions
	gitHubWebhook.Status.Ready = true
	gitHubWebhook.Status.Conditions = setCondition(gitHubWebhook.Status.Conditions, scmv1alpha1.ReadyGitHubWebhookConditionType, v1.ConditionTrue, "ReconciliationSucceeded", "")

	if gitHubWebhook.Spec.Secret != nil {
		// We reconcile regularly if a secret is referenced to ensure external drift is reconciled
		return reconcile.Result{RequeueAfter: gitHubWebhookSecretRequeuePeriod}, nil
	}

	return ctrl.Result{}, nil
}

func (r *GitHubWebhookReconciler) reconcileDelete(ctx context.Context, log logr.Logger, gitHubWebhook *scmv1alpha1.GitHubWebhook) (ctrl.Result, error) {

	if !gitHubWebhook.Spec.Suspend && gitHubWebhook.Spec.ID != nil {
		resp, err := r.GitHubClient.Repositories.DeleteHook(ctx,
			gitHubWebhook.Spec.Repository.Owner,
			gitHubWebhook.Spec.Repository.Name,
			*gitHubWebhook.Spec.ID)
		if err != nil {
			// We do not care if the webhook was not found
			if resp.StatusCode != 404 {
				r.Recorder.Event(gitHubWebhook, "Warning", "DeleteFailed", err.Error())
				gitHubWebhook.Status.SetFailureMessage(err)
				return ctrl.Result{}, err
			}
		}
		log.Info("GitHub webhook successfully deleted!")
	}

	controllerutil.RemoveFinalizer(gitHubWebhook, scmv1alpha1.GitHubWebhookFinalizer)

	return ctrl.Result{}, nil
}

func reconcilePhase(gitHubWebhook *scmv1alpha1.GitHubWebhook) {
	if gitHubWebhook.Spec.ID == nil {
		gitHubWebhook.Status.Phase = scmv1alpha1.GitHubWebhookPhaseCreating
	}

	if gitHubWebhook.Spec.ID != nil {
		gitHubWebhook.Status.Phase = scmv1alpha1.GitHubWebhookPhaseEditing
	}

	if gitHubWebhook.Spec.ID != nil && gitHubWebhook.Status.Ready {
		gitHubWebhook.Status.Phase = scmv1alpha1.GitHubWebhookPhaseReady
	}

	if !gitHubWebhook.DeletionTimestamp.IsZero() {
		gitHubWebhook.Status.Phase = scmv1alpha1.GitHubWebhookPhaseDeleting
	}

	if gitHubWebhook.Status.FailureMessage != nil {
		gitHubWebhook.Status.Phase = scmv1alpha1.GitHubWebhookPhaseFailed
	}
}

func gitHubHookNeedsEdit(log logr.Logger, gitHubWebhook *scmv1alpha1.GitHubWebhook, hook *github.Hook, webhookSecret string) (bool, *github.Hook, error) {
	editWebhook := false

	// Record the last update time to determine whether to overwrite external webhook secret
	updatedAt := v1.NewTime(hook.GetUpdatedAt())
	defer func() {
		gitHubWebhook.Status.LastObserveredUpdateTime = &updatedAt
	}()

	// Calculate webhook secret hash to determine whether to overwrite external webhook secret
	webhookSecretHash := ""
	if webhookSecret != "" {
		sum := sha256.Sum256([]byte(webhookSecret))
		webhookSecretHash = fmt.Sprintf("%x", sum)
	}
	defer func() {
		gitHubWebhook.Status.LastObservedSecretHash = &webhookSecretHash
	}()

	// Check whether to update active
	if gitHubWebhook.Spec.Active != hook.GetActive() {
		hook.Active = &gitHubWebhook.Spec.Active
		editWebhook = true
		log.Info("Active needs to be edited")
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
				log.Info("Events need to be edited")
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
				log.Info("Payload URL needs to be edited")
			}
		} else {
			return false, hook, errors.New("failed to type cast URL parameter")
		}
	} else {
		hook.Config["url"] = gitHubWebhook.Spec.PayloadURL
		editWebhook = true
		log.Info("Payload URL needs to be edited")
	}
	// Check update to content type
	if contentType, ok := hook.Config["content_type"]; ok {
		if contentType, ok := contentType.(string); ok {
			if gitHubWebhook.Spec.ContentType != contentType {
				hook.Config["content_type"] = gitHubWebhook.Spec.ContentType
				editWebhook = true
				log.Info("Content type needs to be edited")
			}
		} else {
			return false, hook, errors.New("failed to type cast content type parameter")
		}
	} else {
		hook.Config["content_type"] = gitHubWebhook.Spec.ContentType
		editWebhook = true
		log.Info("Content type needs to be edited")
	}
	// Check update to insecure SSL
	if _, ok := hook.Config["insecure_ssl"]; ok {
		if insecureSSL, ok := hook.Config["insecure_ssl"].(string); ok {
			insecureSSLInteger, err := strconv.Atoi(insecureSSL)
			if err != nil {
				return false, hook, errors.Wrap(err, "failed to convert insecure_ssl string to an integer")
			}
			if gitHubWebhook.Spec.InsecureSSL != intToBool(insecureSSLInteger) {
				hook.Config["insecure_ssl"] = boolToInt(gitHubWebhook.Spec.InsecureSSL)
				editWebhook = true
				log.Info("Insecure SSL needs to be edited")
			}
		} else {
			return false, hook, errors.New("failed to type cast insecure SSL parameter")
		}
	} else {
		hook.Config["insecure_ssl"] = boolToInt(gitHubWebhook.Spec.InsecureSSL)
		editWebhook = true
		log.Info("Insecure SSL needs to be edited")
	}
	// Check update to secret. The webhook secret is returned as `********` so we make sure it is
	// always set in case we need to make an edit
	hook.Config["secret"] = webhookSecret
	if _, ok := hook.Config["secret"]; ok {
		if secret, ok := hook.Config["secret"].(string); ok {
			// We do a length 0 comparison, a local hash comparison and check whether the external webhook
			// has been updated. Checking whether last update time and secret hash status fields are nil
			// is used when restoring GitHubWebhook resources that have already been created but now have
			// a blank status
			if (len(webhookSecret) == 0 && len(secret) > 0) ||
				(len(secret) == 0 && len(webhookSecret) > 0) ||
				gitHubWebhook.Status.LastObserveredUpdateTime == nil ||
				(gitHubWebhook.Status.LastObserveredUpdateTime != nil &&
					gitHubWebhook.Status.LastObserveredUpdateTime.Before(&updatedAt)) ||
				gitHubWebhook.Status.LastObservedSecretHash == nil ||
				(gitHubWebhook.Status.LastObservedSecretHash != nil &&
					*gitHubWebhook.Status.LastObservedSecretHash != webhookSecretHash) {
				editWebhook = true
				log.Info("Secret needs to be edited")
			}
		} else {
			return false, hook, errors.New("failed to type cast secret parameter")
		}
	} else {
		editWebhook = true
		log.Info("Secret needs to be edited")
	}

	return editWebhook, hook, nil
}

func stringInSlice(s string, list []string) bool {
	for _, l := range list {
		if s == l {
			return true
		}
	}
	return false
}

func intToBool(i int) bool {
	return i != 0
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// SetupWithManager sets up the controller with the Manager.
func (r *GitHubWebhookReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&scmv1alpha1.GitHubWebhook{}).
		Complete(r)
}

// setCondition appends or updates an existing GitHubWebhook condition of the given type with the
// given status value. Note that this function will not append to the conditions list if the new
// condition's status is false (because going from nothing to false is meaningless); it can,
// however, update the status condition to false
func setCondition(list []metav1.Condition, conditionType scmv1alpha1.GitHubWebhookConditionType, status v1.ConditionStatus, reason, message string) []metav1.Condition {
	for i := range list {
		if list[i].Type == string(conditionType) {
			list[i].Status = status
			list[i].LastTransitionTime = metav1.Now()
			list[i].Reason = reason
			list[i].Message = message
			return list
		}
	}
	// A condition with that type doesn't exist in the list.
	if status != v1.ConditionFalse {
		return append(list, *newCondition(conditionType, status, reason, message))
	}
	return list
}

func newCondition(conditionType scmv1alpha1.GitHubWebhookConditionType, status v1.ConditionStatus, reason, message string) *metav1.Condition {
	return &metav1.Condition{
		Type:               string(conditionType),
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}
