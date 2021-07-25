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
)

// GitHubWebhookSpec defines the desired state of GitHubWebhook
type GitHubWebhookSpec struct {
	// ID holds the webhook's GitHub ID. This allows edits to be made. The ID is populated when the
	// webhook is created or when the specification matches with an existing webhook. The matching
	// criteria is as follows: events, url, content type,
	ID *int64 `json:"id,omitempty"`
	// Active refers to status of the webhook for event deliveries.
	// https://developer.github.com/webhooks/creating/#active
	// +optional
	Active     bool       `json:"active,omitempty"`
	Repository Repository `json:"repository,omitempty"`
	// Events refers to Github events to subscribe to
	// +optional
	Events []string `json:"events,omitempty"`
	Config Config   `json:"config,omitempty"`
}

type Repository struct {
	Owner string `json:"owner,omitempty"`
	Name  string `json:"name,omitempty"`
}

type Config struct {
	// URL to send webhook payload
	URL string `json:"url,omitempty"`
	// Enum: application/x-www-form-urlencoded (default), application/json
	// +optional
	ContentType string `json:"contentType,omitempty"`
	// GitHub webhook secret
	// https://developer.github.com/webhooks/securing/
	// +optional
	Secret *corev1.SecretKeySelector `json:"secret,omitempty"`
	// +optional
	InsecureSSL bool `json:"insecureSSL,omitempty"`
}

// GitHubWebhookStatus defines the observed state of GitHubWebhook
type GitHubWebhookStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

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
