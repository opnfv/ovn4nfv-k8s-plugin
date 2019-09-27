package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ProviderNetworkSpec defines the desired state of ProviderNetwork
// +k8s:openapi-gen=true
type ProviderNetworkSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	CniType         string     `json:"cniType"`
	Ipv4Subnets     []IpSubnet `json:"ipv4Subnets"`
	Ipv6Subnets     []IpSubnet `json:"ipv6Subnets,omitempty"`
	DNS             DnsSpec    `json:"dns,omitempty"`
	Routes          []Route    `json:"routes,omitempty"`
	ProviderNetType string     `json:"providerNetType"`
	Vlan            VlanSpec   `json:"vlan"` // For now VLAN is the only supported type
}

type VlanSpec struct {
	VlanId                string   `json:"vlanId"`
	VlanNodeSelector      string   `json:"vlanNodeSelector"`        // "all"/"any"(in which case a node will be randomly selected)/"specific"(see below)
	NodeLabelList         []string `json:"nodeLabelList,omitempty"` // if VlanNodeSelector is value "specific" then this array provides a list of nodes labels
	ProviderInterfaceName string   `json:"providerInterfaceName"`
	LogicalInterfaceName  string   `json:"logicalInterfaceName,omitempty"`
}

// ProviderNetworkStatus defines the observed state of ProviderNetwork
// +k8s:openapi-gen=true
type ProviderNetworkStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	State string `json:"state"` // Indicates if ProviderNetwork is in "created" state
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ProviderNetwork is the Schema for the providernetworks API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type ProviderNetwork struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProviderNetworkSpec   `json:"spec,omitempty"`
	Status ProviderNetworkStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ProviderNetworkList contains a list of ProviderNetwork
type ProviderNetworkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProviderNetwork `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ProviderNetwork{}, &ProviderNetworkList{})
}
