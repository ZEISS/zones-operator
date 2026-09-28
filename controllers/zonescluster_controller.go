package controllers

import (
	"context"
	"fmt"
	"math"
	"time"

	zonesv1alpha1 "github.com/zeiss/zones-operator/api/v1alpha1"
	"github.com/zeiss/zones-operator/pkg/provisioner"
	"github.com/zeiss/zones-operator/pkg/status"

	"github.com/zeiss/pkg/conv"
	"github.com/zeiss/pkg/slices"
	"github.com/zeiss/pkg/utilx"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	EventRecorderLabel = "zones-controller"
)

type EventReason string

const (
	EventReasonClusterCreateFailed EventReason = "CreateFailed"
	EventReasonClusterUpdateFailed EventReason = "UpdateFailed"
	EventReasonClusterDeleteFailed EventReason = "DeleteFailed"
	EventReasonClusterSynchronized EventReason = "Synchronized"
	EventReasonClusterFailed       EventReason = "Failed"
)

// ZonesClusterOperatorReconciler ...
type ZonesClusterOperatorReconciler struct {
	client.Client
	provisioner *provisioner.HelmProvisioner
	Scheme      *runtime.Scheme
	Recorder    record.EventRecorder
}

// NewZonesClusterOperatorReconciler ...
func NewZonesClusterOperatorReconciler(mgr ctrl.Manager) *ZonesClusterOperatorReconciler {
	return &ZonesClusterOperatorReconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		provisioner: provisioner.NewHelmProvisioner(provisioner.WithRestConfig(mgr.GetConfig())),
		Recorder:    mgr.GetEventRecorderFor(EventRecorderLabel),
	}
}

// +kubebuilder:rbac:groups=zones.zeiss.com,resources=zonesclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=zones.zeiss.com,resources=zonesclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=zones.zeiss.com,resources=zonesclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=resourcequotas;limitranges,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets;configmaps;serviceaccounts;services;endpoints;persistentvolumeclaims;pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=apps,resources=statefulsets;deployments;replicasets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings;clusterroles;clusterrolebindings,verbs=get;list;watch;create;update;patch;delete;bind;escalate
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses;networkpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=tlsroutes;referencegrants,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete

// Reconcile ...
func (r *ZonesClusterOperatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	cluster := &zonesv1alpha1.ZonesCluster{}
	if err := r.Get(ctx, req.NamespacedName, cluster); err != nil {
		// Request object not found, could have been deleted after reconcile request.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !cluster.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, cluster)
	}

	if cluster.Spec.Paused {
		return r.reconcilePaused(ctx, cluster)
	}

	// get latest version of the cluster
	if err := r.Get(ctx, req.NamespacedName, cluster); err != nil {
		return reconcile.Result{}, err
	}

	status.SetZonesClusterCondition(cluster, status.NewZonesClusterPending(cluster))
	cluster.Status.Phase = zonesv1alpha1.OperationCreating

	if err := r.Client.Status().Update(ctx, cluster); err != nil {
		return r.ManageError(ctx, cluster, err)
	}

	if err := r.reconcileResources(ctx, cluster); err != nil {
		return r.ManageError(ctx, cluster, err)
	}

	return r.ManageSuccess(ctx, cluster)
}

func (r *ZonesClusterOperatorReconciler) reconcilePaused(ctx context.Context, sk *zonesv1alpha1.ZonesCluster) (ctrl.Result, error) {
	if sk.Status.ControlPaused {
		return ctrl.Result{}, nil
	}

	if sk.Spec.Paused {
		sk.Status.ControlPaused = true
		return ctrl.Result{}, r.Status().Update(ctx, sk)
	}

	return ctrl.Result{}, nil
}

func (r *ZonesClusterOperatorReconciler) reconcileResources(ctx context.Context, cluster *zonesv1alpha1.ZonesCluster) error {
	if !controllerutil.ContainsFinalizer(cluster, zonesv1alpha1.FinalizerName) {
		controllerutil.AddFinalizer(cluster, zonesv1alpha1.FinalizerName)
		if err := r.Update(ctx, cluster); err != nil {
			return fmt.Errorf("updating cluster %q: %w", cluster.Name, err)
		}
	}

	if err := r.reconcileNamespace(ctx, cluster); err != nil {
		return err
	}

	if err := r.reconcileVCluster(ctx, cluster); err != nil {
		return err
	}

	return nil
}

func (r *ZonesClusterOperatorReconciler) reconcileDelete(ctx context.Context, cluster *zonesv1alpha1.ZonesCluster) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(cluster, zonesv1alpha1.FinalizerName) {
		return ctrl.Result{}, nil
	}

	if !r.IsDeleting(cluster) {
		status.SetZonesClusterCondition(cluster, status.NewZonesClusterDeleting(cluster))
		cluster.Status.Phase = zonesv1alpha1.OperationDeleting

		if err := r.Client.Status().Update(ctx, cluster); err != nil {
			return ctrl.Result{}, err
		}
	}

	namespaceName := cluster.Spec.Namespace
	req := provisioner.Request{
		ReleaseName: cluster.Spec.Name,
		Namespace:   cluster.Spec.Namespace,
	}

	if err := r.provisioner.Uninstall(ctx, req); err != nil {
		return ctrl.Result{}, fmt.Errorf("uninstalling vCluster release %q: %w", req.ReleaseName, err)
	}

	namespace := corev1.Namespace{Name: namespaceName}
	if err := r.Delete(ctx, &namespace); err != nil && !errors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("deleting namespace %q: %w", namespaceName, err)
	}

	controllerutil.RemoveFinalizer(cluster, zonesv1alpha1.FinalizerName)
	if err := r.Update(ctx, cluster); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating cluster %q: %w", cluster.Name, err)
	}

	log.Info("zone deleted", "zone", cluster.Name, "namespace", namespaceName)

	return ctrl.Result{}, nil
}

// reconcileNamespace reconciles the namespace for the cluster.
func (r *ZonesClusterOperatorReconciler) reconcileNamespace(ctx context.Context, cluster *zonesv1alpha1.ZonesCluster) error {
	namespace := &corev1.Namespace{}
	name := types.NamespacedName{Name: cluster.Spec.Namespace}
	err := r.Get(ctx, name, namespace)
	if err == nil {
		return nil
	}

	if !errors.IsNotFound(err) {
		return err
	}

	if err := controllerutil.SetControllerReference(cluster, namespace, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference: %w", err)
	}

	namespace = &corev1.Namespace{
		Name: cluster.Spec.Namespace,
	}

	if err := r.Create(ctx, namespace); err != nil {
		return fmt.Errorf("creating namespace: %w", err)
	}

	return nil
}

// reconcileVCluster reconciles the vcluster workload for the given operator.
func (r *ZonesClusterOperatorReconciler) reconcileVCluster(ctx context.Context, cluster *zonesv1alpha1.ZonesCluster) error {
	req := provisioner.Request{
		ReleaseName:  cluster.Spec.Name,
		Namespace:    cluster.Spec.Namespace,
		ChartVersion: provisioner.DefaultChartVersion,
		RepoURL:      provisioner.DefaultChartRef,
	}

	if err := r.provisioner.Install(ctx, req); err != nil {
		return fmt.Errorf("installing vcluster: %w", err)
	}

	ready, _, err := r.vclusterReady(ctx, cluster, cluster.Spec.Namespace)
	if err != nil {
		return fmt.Errorf("vcluster not ready: %w", err)
	}

	if !ready {

	}

	return err
}

// vclusterReady returns true if the vcluster workload is ready and the kubeconfig secret is present.
func (r *ZonesClusterOperatorReconciler) vclusterReady(ctx context.Context, operator *zonesv1alpha1.ZonesCluster, namespace string) (bool, string, error) {
	workloadReady, reason, err := r.vClusterDeploymentReady(ctx, operator.Name, namespace)
	if err != nil {
		return false, "", err
	}
	if !workloadReady {
		return false, reason, nil
	}

	secret := &corev1.Secret{}
	secretName := operator.KubeconfigSecretName()
	err = r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespace}, secret)

	if errors.IsNotFound(err) {
		return false, fmt.Sprintf("waiting for kubeconfig secret %s/%s", namespace, secretName), nil
	}

	if err != nil {
		return false, "", err
	}

	if _, ok := secret.Data[zonesv1alpha1.KubeconfigSecretKey]; !ok {
		return false, fmt.Sprintf("kubeconfig secret %s/%s missing key %q", namespace, secretName, zonesv1alpha1.KubeconfigSecretKey), nil
	}

	return true, "", nil
}

// vClusterDeploymentReady returns true if the vcluster deployment is ready.
func (r *ZonesClusterOperatorReconciler) vClusterDeploymentReady(ctx context.Context, name, namespace string) (bool, string, error) {
	key := types.NamespacedName{Name: name, Namespace: namespace}

	statefulSet := &appsv1.StatefulSet{}
	err := r.Get(ctx, key, statefulSet)
	if err == nil {
		desired := int32(1)
		if statefulSet.Spec.Replicas != nil {
			desired = *statefulSet.Spec.Replicas
		}
		if desired > 0 && statefulSet.Status.ReadyReplicas >= desired {
			return true, "", nil
		}
		return false, fmt.Sprintf("waiting for statefulset %s/%s: %d/%d replicas ready",
			namespace, name, statefulSet.Status.ReadyReplicas, desired), nil
	}

	if !errors.IsNotFound(err) {
		return false, "", err
	}

	deployment := &appsv1.Deployment{}
	err = r.Get(ctx, key, deployment)
	if err == nil {
		desired := int32(1)
		if deployment.Spec.Replicas != nil {
			desired = *deployment.Spec.Replicas
		}
		if desired > 0 && deployment.Status.ReadyReplicas >= desired {
			return true, "", nil
		}
		return false, fmt.Sprintf("waiting for deployment %s/%s: %d/%d replicas ready",
			namespace, name, deployment.Status.ReadyReplicas, desired), nil
	}
	if !errors.IsNotFound(err) {
		return false, "", err
	}

	return false, fmt.Sprintf("waiting for vCluster workload %s/%s to appear", namespace, name), nil
}

// IsAccepted ...
func (r *ZonesClusterOperatorReconciler) IsAccepted(obj *zonesv1alpha1.ZonesCluster) bool {
	return obj.Status.Phase == zonesv1alpha1.OperationAccepted
}

// IsCreating ...
func (r *ZonesClusterOperatorReconciler) IsCreating(obj *zonesv1alpha1.ZonesCluster) bool {
	return utilx.Or(obj.Status.Conditions == nil, slices.Len(0, obj.Status.Conditions...))
}

// IsDeleting ...
func (r *ZonesClusterOperatorReconciler) IsDeleting(obj *zonesv1alpha1.ZonesCluster) bool {
	return obj.Status.Phase == zonesv1alpha1.OperationDeleting
}

// IsSynchronized ...
func (r *ZonesClusterOperatorReconciler) IsSynchronized(obj *zonesv1alpha1.ZonesCluster) bool {
	return obj.Status.Phase == zonesv1alpha1.OperationSynchronized
}

// IsPaused ...
func (r *ZonesClusterOperatorReconciler) IsPaused(obj *zonesv1alpha1.ZonesCluster) bool {
	return obj.Status.ControlPaused
}

// ManageError ...
func (r *ZonesClusterOperatorReconciler) ManageError(ctx context.Context, obj *zonesv1alpha1.ZonesCluster, err error) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Error(err, "reconciling cluster", "operator", obj.Name)

	if errors.IsNotFound(err) {
		return ctrl.Result{Requeue: true}, nil
	}

	status.SetZonesClusterCondition(obj, status.NewZonesClusterFailed(obj, err))
	obj.Status.Phase = zonesv1alpha1.OperationFailed

	if err := r.Client.Status().Update(ctx, obj); err != nil {
		return ctrl.Result{Requeue: true, RequeueAfter: time.Second}, err
	}

	r.Recorder.Event(obj, corev1.EventTypeWarning, conv.String(EventReasonClusterFailed), "cluster synchronization failed")

	var retryInterval time.Duration

	return reconcile.Result{
		RequeueAfter: time.Duration(math.Min(float64(retryInterval.Nanoseconds()*2), float64(time.Hour.Nanoseconds()*6))),
		Requeue:      true,
	}, nil
}

// ManageSuccess ...
func (r *ZonesClusterOperatorReconciler) ManageSuccess(ctx context.Context, cluster *zonesv1alpha1.ZonesCluster) (ctrl.Result, error) {
	if r.IsSynchronized(cluster) {
		return ctrl.Result{}, nil
	}

	cluster.Status.Phase = zonesv1alpha1.OperationSynchronized
	cluster.Status.LastUpdate = metav1.Now()
	status.SetZonesClusterCondition(cluster, status.NewZonesClusterSynchronizedCondition(cluster))

	err := r.Status().Update(ctx, cluster)
	if err != nil {
		return ctrl.Result{Requeue: true, RequeueAfter: time.Second}, err
	}

	r.Recorder.Event(cluster, corev1.EventTypeNormal, conv.String(EventReasonClusterSynchronized), "cluster synchronized")

	return ctrl.Result{}, nil
}

// IsControlPaused ...
func (r *ZonesClusterOperatorReconciler) IsControlPaused(obj *zonesv1alpha1.ZonesCluster) bool {
	return obj.Status.ControlPaused
}

// SetupWithManager sets up the controller with the Manager.
func (r *ZonesClusterOperatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&zonesv1alpha1.ZonesCluster{}).
		Owns(&corev1.Secret{}).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.LabelChangedPredicate{})).
		Complete(r)
}
