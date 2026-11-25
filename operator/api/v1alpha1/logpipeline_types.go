/*
Copyright 2026.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// LogPipelineSpec defines the desired state of the Vector log pipeline.
// The operator reconciles this spec into ConfigMaps for the agent and
// egress tiers, and annotates the DaemonSet/Deployments to trigger a
// rolling restart whenever the config changes.
type LogPipelineSpec struct {
	// Agent configures the DaemonSet tier that collects logs from each node.
	// +optional
	Agent AgentSpec `json:"agent,omitempty"`

	// Egress configures the Deployment tier that ships logs to a destination.
	// +required
	Egress EgressSpec `json:"egress"`
}

// AgentSpec configures the agent (DaemonSet) tier.
type AgentSpec struct {
	// ExcludeNamespaces lists namespaces whose logs must not be collected.
	// The pipeline's own namespace (logging) is always excluded implicitly —
	// omitting it from this list does not re-enable it.
	// +optional
	ExcludeNamespaces []string `json:"excludeNamespaces,omitempty"`
}

// EgressSpec configures the egress (Deployment) tier.
type EgressSpec struct {
	// Loki configures the Loki sink.
	Loki LokiSpec `json:"loki"`
}

// LokiSpec configures the Loki push API destination.
type LokiSpec struct {
	// Endpoint is the Loki push API base URL.
	// Example: "http://loki.monitoring.svc.cluster.local:3100"
	// +kubebuilder:validation:MinLength=1
	Endpoint string `json:"endpoint"`

	// ExtraLabels adds static key/value labels to every Loki stream, in
	// addition to the labels set by the pipeline (cluster, namespace, pod,
	// container, node, app, stream, pipeline_tier).
	// +optional
	ExtraLabels map[string]string `json:"extraLabels,omitempty"`
}

// LogPipelineStatus defines the observed state of LogPipeline.
type LogPipelineStatus struct {
	// Conditions represents the latest available observations of the pipeline's state.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the .metadata.generation that was last successfully reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// ConfigHash is the SHA-256 of the last reconciled config payload.
	// It appears as the value of the
	// observability.endurance.io/config-hash annotation on the agent
	// DaemonSet and egress Deployment so that a spec change automatically
	// triggers a rolling restart.
	// +optional
	ConfigHash string `json:"configHash,omitempty"`
}

// Condition type constants used in LogPipelineStatus.
const (
	// ConditionReady is True when all tiers are configured and healthy.
	ConditionReady = "Ready"
	// ConditionAgentConfigured is True when the agent ConfigMap is in sync with the spec.
	ConditionAgentConfigured = "AgentConfigured"
	// ConditionEgressConfigured is True when the egress ConfigMap is in sync with the spec.
	ConditionEgressConfigured = "EgressConfigured"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Config-Hash",type="string",JSONPath=".status.configHash"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// LogPipeline is the Schema for the logpipelines API.
// A single cluster-scoped LogPipeline CR is the declarative source of
// truth for the Vector pipeline configuration; the operator keeps the
// agent and egress ConfigMaps in sync with it.
type LogPipeline struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec LogPipelineSpec `json:"spec"`

	// +optional
	Status LogPipelineStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// LogPipelineList contains a list of LogPipeline
type LogPipelineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []LogPipeline `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &LogPipeline{}, &LogPipelineList{})
		return nil
	})
}
