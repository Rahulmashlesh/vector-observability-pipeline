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

package controller

import (
	"context"
	"crypto/sha256"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	observabilityv1alpha1 "github.com/Rahulmashlesh/vector-observability-pipeline/operator/api/v1alpha1"
)

const (
	// configHashAnnotation is stamped on the agent DaemonSet and egress
	// Deployment pod templates so that a spec change triggers a rolling
	// restart automatically.
	configHashAnnotation = "observability.endurance.io/config-hash"

	// agentConfigMapName is the ConfigMap the agent DaemonSet mounts.
	agentConfigMapName = "vector-agent-config"
	// agentDaemonSetName is the DaemonSet to annotate on config change.
	agentDaemonSetName = "vector-agent"

	// egressConfigMapName is the ConfigMap the egress Deployment mounts.
	egressConfigMapName = "vector-egress-config"
	// egressDeploymentName is the Deployment to annotate on config change.
	egressDeploymentName = "vector-egress"
)

// LogPipelineReconciler reconciles a LogPipeline object.
type LogPipelineReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=observability.endurance.io,resources=logpipelines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=observability.endurance.io,resources=logpipelines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=observability.endurance.io,resources=logpipelines/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=apps,resources=daemonsets;deployments,verbs=get;list;watch;patch

// Reconcile moves the cluster state toward the desired state declared in the
// LogPipeline CR.  It:
//
//  1. Renders the agent and egress Vector configs from the spec.
//  2. Creates or updates the agent and egress ConfigMaps.
//  3. Stamps the resulting config hash onto the agent DaemonSet and
//     egress Deployment pod-template annotations so that any config
//     change triggers an automatic rolling restart.
//  4. Updates the LogPipeline status conditions.
func (r *LogPipelineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// ── 1. Fetch the CR ────────────────────────────────────────────────────
	pipeline := &observabilityv1alpha1.LogPipeline{}
	if err := r.Get(ctx, req.NamespacedName, pipeline); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("Reconciling LogPipeline", "name", pipeline.Name)

	// ── 2. Render configs ──────────────────────────────────────────────────
	agentCfg, err := buildAgentConfig(pipeline)
	if err != nil {
		r.setCondition(pipeline, observabilityv1alpha1.ConditionAgentConfigured,
			metav1.ConditionFalse, "RenderFailed", err.Error())
		_ = r.Status().Update(ctx, pipeline)
		return ctrl.Result{}, err
	}

	egressCfg, err := buildEgressConfig(pipeline)
	if err != nil {
		r.setCondition(pipeline, observabilityv1alpha1.ConditionEgressConfigured,
			metav1.ConditionFalse, "RenderFailed", err.Error())
		_ = r.Status().Update(ctx, pipeline)
		return ctrl.Result{}, err
	}

	// ── 3. Reconcile ConfigMaps ────────────────────────────────────────────
	if err := r.reconcileConfigMap(ctx, agentConfigMapName, pipelineNamespace,
		"agent.yaml", agentCfg, pipeline); err != nil {
		r.setCondition(pipeline, observabilityv1alpha1.ConditionAgentConfigured,
			metav1.ConditionFalse, "ConfigMapFailed", err.Error())
		_ = r.Status().Update(ctx, pipeline)
		return ctrl.Result{}, err
	}
	r.setCondition(pipeline, observabilityv1alpha1.ConditionAgentConfigured,
		metav1.ConditionTrue, "Reconciled", "agent ConfigMap is in sync with spec")

	if err := r.reconcileConfigMap(ctx, egressConfigMapName, pipelineNamespace,
		"egress.yaml", egressCfg, pipeline); err != nil {
		r.setCondition(pipeline, observabilityv1alpha1.ConditionEgressConfigured,
			metav1.ConditionFalse, "ConfigMapFailed", err.Error())
		_ = r.Status().Update(ctx, pipeline)
		return ctrl.Result{}, err
	}
	r.setCondition(pipeline, observabilityv1alpha1.ConditionEgressConfigured,
		metav1.ConditionTrue, "Reconciled", "egress ConfigMap is in sync with spec")

	// ── 4. Stamp config hash on workloads to trigger rolling restarts ───────
	configHash := hashConfigs(agentCfg, egressCfg)

	if err := r.annotateAgentDaemonSet(ctx, configHash); err != nil {
		log.Error(err, "failed to annotate agent DaemonSet")
		// Not fatal: ConfigMaps are correct; next reconcile will retry.
	}
	if err := r.annotateEgressDeployment(ctx, configHash); err != nil {
		log.Error(err, "failed to annotate egress Deployment")
	}

	// ── 5. Update status ───────────────────────────────────────────────────
	r.setCondition(pipeline, observabilityv1alpha1.ConditionReady,
		metav1.ConditionTrue, "Ready", "all tiers configured and in sync")
	pipeline.Status.ObservedGeneration = pipeline.Generation
	pipeline.Status.ConfigHash = configHash[:12] // short hash for readability
	if err := r.Status().Update(ctx, pipeline); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("LogPipeline reconciled", "configHash", configHash[:12])
	return ctrl.Result{}, nil
}

// reconcileConfigMap creates or updates the named ConfigMap so its data key
// matches the rendered config string.
func (r *LogPipelineReconciler) reconcileConfigMap(
	ctx context.Context,
	name, namespace, key, content string,
	owner *observabilityv1alpha1.LogPipeline,
) error {
	cm := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, cm)

	if apierrors.IsNotFound(err) {
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "logpipeline-operator",
					"app.kubernetes.io/part-of":    "vector-observability-pipeline",
				},
				Annotations: map[string]string{
					"observability.endurance.io/managed-by-cr": owner.Name,
				},
			},
			Data: map[string]string{key: content},
		}
		return r.Create(ctx, cm)
	}
	if err != nil {
		return fmt.Errorf("getting ConfigMap %s: %w", name, err)
	}

	// Only patch if content actually changed — avoids spurious workload restarts.
	if cm.Data[key] == content {
		return nil
	}

	patch := client.MergeFrom(cm.DeepCopy())
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	cm.Data[key] = content
	return r.Patch(ctx, cm, patch)
}

// annotateAgentDaemonSet stamps the config hash onto the agent DaemonSet's
// pod template so k8s triggers a rolling restart of the agent pods.
func (r *LogPipelineReconciler) annotateAgentDaemonSet(ctx context.Context, hash string) error {
	ds := &appsv1.DaemonSet{}
	if err := r.Get(ctx, types.NamespacedName{Name: agentDaemonSetName, Namespace: pipelineNamespace}, ds); err != nil {
		return fmt.Errorf("getting agent DaemonSet: %w", err)
	}
	if ds.Spec.Template.Annotations[configHashAnnotation] == hash {
		return nil // already up to date
	}
	patch := client.MergeFrom(ds.DeepCopy())
	if ds.Spec.Template.Annotations == nil {
		ds.Spec.Template.Annotations = make(map[string]string)
	}
	ds.Spec.Template.Annotations[configHashAnnotation] = hash
	return r.Patch(ctx, ds, patch)
}

// annotateEgressDeployment stamps the config hash onto the egress Deployment's
// pod template so k8s triggers a rolling restart of the egress pod.
func (r *LogPipelineReconciler) annotateEgressDeployment(ctx context.Context, hash string) error {
	dep := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: egressDeploymentName, Namespace: pipelineNamespace}, dep); err != nil {
		return fmt.Errorf("getting egress Deployment: %w", err)
	}
	if dep.Spec.Template.Annotations[configHashAnnotation] == hash {
		return nil
	}
	patch := client.MergeFrom(dep.DeepCopy())
	if dep.Spec.Template.Annotations == nil {
		dep.Spec.Template.Annotations = make(map[string]string)
	}
	dep.Spec.Template.Annotations[configHashAnnotation] = hash
	return r.Patch(ctx, dep, patch)
}

// hashConfigs returns the hex SHA-256 of the concatenated config strings.
func hashConfigs(configs ...string) string {
	h := sha256.New()
	for _, c := range configs {
		_, _ = fmt.Fprint(h, c)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// setCondition sets or updates a named condition on the pipeline status.
func (r *LogPipelineReconciler) setCondition(
	pipeline *observabilityv1alpha1.LogPipeline,
	condType string,
	status metav1.ConditionStatus,
	reason, message string,
) {
	meta.SetStatusCondition(&pipeline.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: pipeline.Generation,
	})
}

// SetupWithManager sets up the controller with the Manager.
func (r *LogPipelineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&observabilityv1alpha1.LogPipeline{}).
		Named("logpipeline").
		Complete(r)
}
