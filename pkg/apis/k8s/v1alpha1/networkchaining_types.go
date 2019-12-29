package v1alpha1

import (
        metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// NetworkChainingSpec defines the desired state of NetworkChaining
// +k8s:openapi-gen=true
type NetworkChainingSpec struct {
        ChainType       string        `json:"chainType"`      // Currently only Routing type is supported
        RoutingSpec     RouteSpec     `json:"routingSpec"`     // Spec for Routing type
        // Add other Chanining mechanisms here
}

type RouteSpec struct {
        LeftNetwork      []RoutingNetwork   `json:"leftNetwork"`  // Info on Network on the left side
        RightNetwork     []RoutingNetwork    `json:"rightNetwork"`// Info on Network on the right side
        NetworkChain     string              `json:"networkChain"`// NetworkChain is a comma seprated list with format DeploymentName, middle Network Name, DeploymentName, ...
        Namespace        string              `json:"namespace"`   // Kubernetes namespace
 }

 type RoutingNetwork struct {
         NetworkName     string              `json:"networkName"` // Name of the network
         GatewayIP       string              `json:"gatewayIp"`   // Gateway IP Address
         Subnet          string                          `json:"subnet"`      // Subnet
 }

// NetworkChainingStatus defines the observed state of NetworkChaining
// +k8s:openapi-gen=true
type NetworkChainingStatus struct {
        State string `json:"state"` // Indicates if Network Chain is in "created" state
}


// NetworkChaining is the Schema for the networkchainings API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=networkchainings,scope=Namespaced
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type NetworkChaining struct {
        metav1.TypeMeta   `json:",inline"`
        metav1.ObjectMeta `json:"metadata,omitempty"`

        Spec   NetworkChainingSpec   `json:"spec,omitempty"`
        Status NetworkChainingStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NetworkChainingList contains a list of NetworkChaining
type NetworkChainingList struct {
        metav1.TypeMeta `json:",inline"`
        metav1.ListMeta `json:"metadata,omitempty"`
        Items           []NetworkChaining `json:"items"`
}

func init() {
        SchemeBuilder.Register(&NetworkChaining{}, &NetworkChainingList{})
}



