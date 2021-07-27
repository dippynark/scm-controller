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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

const (
	// GitHubWebhookFinalizer allows reconciliation to clean up resources associated with a
	// GitHubWebhook before removing it from the API Server
	GitHubWebhookFinalizer = "githubwebhook.scm.dippynark.co.uk"
)

// GitHubWebhookSpec defines the desired state of GitHubWebhook
type GitHubWebhookSpec struct {
	// ID holds the webhook's GitHub ID. The ID is populated when the webhook is created.
	ID         *int64     `json:"id,omitempty"`
	Repository Repository `json:"repository,omitempty"`
	// URL to send webhook payload
	PayloadURL string `json:"payloadURL,omitempty"`
	// Enum: application/x-www-form-urlencoded (default), application/json
	// +optional
	ContentType string `json:"contentType,omitempty"`
	// GitHub webhook secret
	// https://developer.github.com/webhooks/securing/
	// +optional
	Secret *corev1.SecretKeySelector `json:"secret,omitempty"`
	// Active refers to status of the webhook for event deliveries.
	// https://developer.github.com/webhooks/creating/#active
	// +optional
	// +optional
	InsecureSSL bool `json:"insecureSSL,omitempty"`
	// Events refers to Github events to subscribe to
	// +optional
	Events []string `json:"events,omitempty"`
	Active bool     `json:"active,omitempty"`
}

type Repository struct {
	Owner string `json:"owner,omitempty"`
	Name  string `json:"name,omitempty"`
}

// GitHubWebhookStatus defines the observed state of GitHubWebhook
type GitHubWebhookStatus struct {
	LastObserveredUpdateTime *metav1.Time `json:"lastObservedUpdateTime,omitempty"`
	LastObservedSecretHash   *string      `json:"lastObservedSecretHash,omitempty"`

	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// Phase represents the current phase of GitHubWebhookPhase actuation
	// +optional
	Phase GitHubWebhookPhase `json:"phase,omitempty"`

	// Ready indicates when the GitHubWebhook is ready
	Ready bool `json:"ready,omitempty"`

	// FailureMessage will be set in the event that there is a terminal problem reconciling the
	// GitHubWebhook and will contain a more verbose string suitable for logging and human
	// consumption
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`
}

// GitHubWebhookPhase describes the state of a KubernetesMachine
type GitHubWebhookPhase string

// These are the valid phases of GitHubWebhook
const (
	GitHubWebhookPhaseCreating GitHubWebhookPhase = "Creating"
	GitHubWebhookPhaseEditing  GitHubWebhookPhase = "Editing"
	GitHubWebhookPhaseReady    GitHubWebhookPhase = "Ready"
	GitHubWebhookPhaseDeleting GitHubWebhookPhase = "Deleting"
	GitHubWebhookPhaseFailed   GitHubWebhookPhase = "Failed"
)

type GitHubWebhookConditionType string

const (
	ReadyGitHubWebhookConditionType GitHubWebhookConditionType = "Ready"
)

// SetFailureMessage sets the KubernetesMachine error message.
func (g *GitHubWebhookStatus) SetFailureMessage(v error) {
	g.FailureMessage = pointer.StringPtr(v.Error())
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="id",type="string",JSONPath=".spec.id",description="GitHub ID"
// +kubebuilder:printcolumn:name="phase",type="string",JSONPath=".status.phase",description="GitHubWebhook phase"
// +kubebuilder:printcolumn:name="payload url",type="string",JSONPath=".spec.payloadURL",description="Payload URL"
// +kubebuilder:printcolumn:name="age",type="date",JSONPath=".metadata.creationTimestamp"

// GitHubWebhook is the Schema for the githubwebhooks API
type GitHubWebhook struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GitHubWebhookSpec   `json:"spec,omitempty"`
	Status GitHubWebhookStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// GitHubWebhookList contains a list of GitHubWebhook
type GitHubWebhookList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GitHubWebhook `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GitHubWebhook{}, &GitHubWebhookList{})
}

// Backup
// APIToken refers to a K8s secret containing github api token
// +optional
// APIToken *corev1.SecretKeySelector `json:"apiToken,omitempty"`
// GitHub base URL (for GitHub Enterprise)
// +optional
// GithubBaseURL string `json:"githubBaseURL,omitempty" protobuf:"bytes,11,opt,name=githubBaseURL"`
// GitHub upload URL (for GitHub Enterprise)
// +optional
// GithubUploadURL string `json:"githubUploadURL,omitempty" protobuf:"bytes,12,opt,name=githubUploadURL"`
