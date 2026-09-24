package controllers

import (
	"context"
	"math"
	"time"

	zonesv1alpha1 "github.com/zeiss/zones-operator/api/v1alpha1"
	"github.com/zeiss/zones-operator/pkg/status"

	"github.com/zeiss/pkg/conv"
	"github.com/zeiss/pkg/slices"
	"github.com/zeiss/pkg/utilx"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
)

const (
	EventRecorderLabel = "natz-controller"
)

type EventReason string

const (
	EventReasonOperatorCreateFailed          EventReason = "OperatorCreateFailed"
	EventReasonOperatorUpdateFailed          EventReason = "OperatorUpdateFailed"
	EventReasonOperatorDeleteFailed          EventReason = "OperatorDeleteFailed"
	EventReasonOperatorSecretCreateSucceeded EventReason = "OperatorSecretCreateSucceeded"
	EventReasonOperatorSecretCreateFailed    EventReason = "OperatorSecretCreateFailed"
	EventReasonOperatorSynchronized          EventReason = "OperatorSynchronized"
	EventReasonOperatorFailed                EventReason = "OperatorFailed"
	EventReasonOperatorSynchronizeFailed     EventReason = "OperatorSynchronizeFailed"
)

// ZonesClusterOperatorReconciler ...
type ZonesClusterOperatorReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// NewZonesClusterOperatorReconciler ...
func NewZonesClusterOperatorReconciler(mgr ctrl.Manager) *ZonesClusterOperatorReconciler {
	return &ZonesClusterOperatorReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor(EventRecorderLabel),
	}
}

//+kubebuilder:rbac:groups=natz.zeiss.com,resources=natsoperators,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=natz.zeiss.com,resources=natsoperators/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=natz.zeiss.com,resources=natsoperators/finalizers,verbs=update
//+kubebuilder:rbac:groups=,resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile ...
// nolint:gocyclo
func (r *ZonesClusterOperatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	operator := &zonesv1alpha1.ZonesCluster{}
	if err := r.Get(ctx, req.NamespacedName, operator); err != nil {
		// Request object not found, could have been deleted after reconcile request.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !operator.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, operator)
	}

	if operator.Spec.Paused {
		return r.reconcilePaused(ctx, operator)
	}

	// get latest version of the account
	if err := r.Get(ctx, req.NamespacedName, operator); err != nil {
		return reconcile.Result{}, err
	}

	err := r.reconcileResources(ctx, operator)
	if err != nil {
		return r.ManageError(ctx, operator, err)
	}

	return r.ManageSuccess(ctx, operator)
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

func (r *ZonesClusterOperatorReconciler) reconcileResources(ctx context.Context, operator *zonesv1alpha1.ZonesCluster) error {
	return r.reconcileOperator(ctx, operator)
}

func (r *ZonesClusterOperatorReconciler) reconcileOperator(ctx context.Context, obj *zonesv1alpha1.ZonesCluster) error {
	return nil
}

func (r *ZonesClusterOperatorReconciler) reconcileDelete(ctx context.Context, operator *zonesv1alpha1.ZonesCluster) (ctrl.Result, error) {
	return ctrl.Result{Requeue: true}, nil
}

// IsCreating ...
func (r *ZonesClusterOperatorReconciler) IsCreating(obj *zonesv1alpha1.ZonesCluster) bool {
	return utilx.Or(obj.Status.Conditions == nil, slices.Len(0, obj.Status.Conditions...))
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

	r.Recorder.Event(obj, corev1.EventTypeWarning, conv.String(EventReasonOperatorSynchronizeFailed), "operator synchronization failed")

	var retryInterval time.Duration

	return reconcile.Result{
		RequeueAfter: time.Duration(math.Min(float64(retryInterval.Nanoseconds()*2), float64(time.Hour.Nanoseconds()*6))),
		Requeue:      true,
	}, nil
}

// ManageSuccess ...
func (r *ZonesClusterOperatorReconciler) ManageSuccess(ctx context.Context, obj *zonesv1alpha1.ZonesCluster) (ctrl.Result, error) {
	obj.Status.Phase = zonesv1alpha1.OperationSynchronized
	obj.Status.LastUpdate = metav1.Now()
	status.SetZonesClusterCondition(obj, status.NewZonesClusterSynchronizedCondition(obj))

	err := r.Status().Update(ctx, obj)
	if err != nil {
		return ctrl.Result{Requeue: true, RequeueAfter: time.Second}, err
	}

	r.Recorder.Event(obj, corev1.EventTypeNormal, conv.String(EventReasonOperatorSynchronized), "operator synchronized")

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
