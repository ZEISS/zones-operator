package status

import (
	zonesv1alpha1 "github.com/zeiss/zones-operator/api/v1alpha1"

	"github.com/zeiss/pkg/slices"
	"github.com/zeiss/pkg/utilx"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewCondition ...
func NewCondition(conditionType string, conditionStatus metav1.ConditionStatus, now metav1.Time, reason, message string) metav1.Condition {
	return metav1.Condition{
		Type:               conditionType,
		Status:             conditionStatus,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
	}
}

// SetCondition ...
func SetCondition(condition metav1.Condition, conditions ...metav1.Condition) []metav1.Condition {
	return utilx.IfElse(
		slices.Any(func(cond metav1.Condition) bool {
			return cond.Type == condition.Type
		}, conditions...),
		conditions,
		append(conditions, condition),
	)
}

// SetZonesClusterCondition ...
func SetZonesClusterCondition(obj *zonesv1alpha1.ZonesCluster, condition metav1.Condition) {
	obj.Status.Conditions = SetCondition(condition, obj.Status.Conditions...)
}

// NewZonesClusterFailed creates the provisioning started condition in cluster conditions.
func NewZonesClusterFailed(obj *zonesv1alpha1.ZonesCluster, err error) metav1.Condition {
	return metav1.Condition{
		Type:               zonesv1alpha1.ConditionTypeFailed,
		ObservedGeneration: obj.Generation,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Message:            err.Error(),
		Reason:             zonesv1alpha1.ConditionReasonFailed,
	}
}

// NewZonesClusterSynchronizedCondition ...
func NewZonesClusterSynchronizedCondition(obj *zonesv1alpha1.ZonesCluster) metav1.Condition {
	return metav1.Condition{
		Type:               zonesv1alpha1.ConditionTypeSynchronized,
		ObservedGeneration: obj.Generation,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Message:            "cluster synchronized",
		Reason:             zonesv1alpha1.ConditionReasonSynchronized,
	}
}
