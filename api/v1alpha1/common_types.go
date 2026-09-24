package v1alpha1

import (
	"github.com/zeiss/pkg/utilx"
	corev1 "k8s.io/api/core/v1"
)

const (
	ZonesNameSpacePrefix = "zones-tenant"
)

const (
	// AnnotationDeletePolicy is the annotation key for the delete policy
	AnnotationDeletePolicy = "zones.zeiss.com/delete-policy"
)

const (
	ConditionTypeSynchronizing = "Sychronizing"
	ConditionTypeSynchronized  = "Synchronized"
	ConditionTypeFailed        = "Failed"
)

const (
	ConditionReasonCreated      = "Created"
	ConditionReasonSynchronized = "Synchronized"
	ConditionReasonFailed       = "Failed"
)

const (
	FinalizerName              = "zones.zeiss.com/finalizer"
	AccountServerFinalizerName = "zones.zeiss.com/account-server-finalizer"
	OwnerAnnotation            = "zones.zeiss.com/owner"
)

type OperationPhase string

const (
	OperationCreating     OperationPhase = "Creating"
	OperationDeleting     OperationPhase = "Deleting"
	OperationError        OperationPhase = "Error"
	OperationFailed       OperationPhase = "Failed"
	OperationSucceeded    OperationPhase = "Succeeded"
	OperationSynchronized OperationPhase = "Synchronized"
	OperationTerminating  OperationPhase = "Terminating"
)

func (os OperationPhase) Completed() bool {
	return utilx.Or(os == OperationFailed, os == OperationSucceeded)
}

func (os OperationPhase) Synchronized() bool {
	return os == OperationSynchronized
}

func (os OperationPhase) Successful() bool {
	return os == OperationSucceeded
}

func (os OperationPhase) Failed() bool {
	return os == OperationFailed
}

func (os OperationPhase) Terminating() bool {
	return os == OperationTerminating
}

func (os OperationPhase) Creating() bool {
	return os == OperationCreating
}

func (os OperationPhase) Deleting() bool {
	return os == OperationDeleting
}

// SecretValueFromSource represents the source of a secret value
type SecretValueFromSource struct {
	// The Secret key to select from.
	SecretKeyRef *corev1.SecretKeySelector `json:"secretKeyRef,omitempty"`
}

// SecretKeyRef points at a key inside a Secret in the tenant namespace.
type SecretKeyRef struct {
	// Name of the Secret.
	// +optional
	Name string `json:"name,omitempty"`
	// Key inside the Secret.
	// +optional
	Key string `json:"key,omitempty"`
}
