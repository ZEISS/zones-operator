package v1alpha1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterRessources represents a cluster resource.
type ClusterRessources struct {
	// CPU quota (Kubernetes quantity, e.g. "4").
	// +optional
	CPU string `json:"cpu,omitempty"`
	// Memory quota (e.g. "8Gi").
	// +optional
	Memory string `json:"memory,omitempty"`
	// Storage quota (e.g. "20Gi").
	// +optional
	Storage string `json:"storage,omitempty"`
}

// ClusterConfig represents the configuration of a cluster.
type ClusterConfig struct {
	// Version is the version of the cluster.
	// +optional
	Version string `json:"version,omitempty"`
	// KubernetesVersion is the Kubernetes version inside the virtual cluster.
	// +optional
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`
	// ValuesOverrides are raw vCluster chart values overrides (escape hatch).
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Type=object
	// +optional
	ValuesOverrides *apiextensionsv1.JSON `json:"valuesOverrides,omitempty"`
}

// ZonesClusterSpec defines the desired state of a Zones cluster.
type ZonesClusterSpec struct {
	// Name is the name of the cluster.
	Name string `json:"name"`
	// Namespace is the namespace of the cluster.
	Namespace string `json:"namespace"`
	// Resources defines the desired resources for the cluster.
	Resources *ClusterRessources `json:"resources,omitempty"`
	// Config defines the configuration of the cluster.
	// +optional
	Config *ClusterConfig `json:"config,omitempty"`
	// PreventDeletion is a flag that indicates if the  should be locked to prevent deletion.
	// +kubebuilder:default=false
	PreventDeletion bool `json:"prevent_deletion,omitempty"`
	// Paused is a flag that indicates if the  is paused.
	// +kubebuilder:default=false
	Paused bool `json:"paused,omitempty"`
}

// ZonesClusterStatus defines the observed state of a Zones cluster.
type ZonesClusterStatus struct {
	// Conditions is an array of conditions that the operator is currently in.
	Conditions []metav1.Condition `json:"conditions,omitempty" optional:"true"`
	// Phase is the current phase of the operator.
	//
	// +kubebuilder:validation:Enum={None,Pending,Creating,Synchronized,Failed,Deleting}
	Phase OperationPhase `json:"phase"`
	// ControlPaused is a flag that indicates if the operator is paused.
	ControlPaused bool `json:"controlPaused,omitempty" optional:"true"`
	// LastUpdate is the timestamp of the last update.
	LastUpdate metav1.Time `json:"lastUpdate,omitempty"`
	// KubeconfigSecretRef points at the Secret (in the target namespace) holding
	// the tenant kubeconfig.
	// +optional
	KubeconfigSecretRef *SecretKeyRef `json:"kubeconfigSecretRef,omitempty"`
}

// +genclient
// +genclient:nonNamespaced
// +genreconciler
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=zone
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ZonesCluster is the Schema for the zonesclusters API.
type ZonesCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ZonesClusterSpec   `json:"spec,omitempty"`
	Status ZonesClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ZonesClusterList contains a list of ZonesCluster resources.
type ZonesClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ZonesCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ZonesCluster{}, &ZonesClusterList{})
}
