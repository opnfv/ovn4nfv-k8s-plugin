package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// NetworkSpec defines the desired state of Network
// +k8s:openapi-gen=true
type NetworkSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	CniType     string     `json:"cniType"`
	Ipv4Subnets []IpSubnet `json:"ipv4Subnets"`
	Ipv6Subnets []IpSubnet `json:"ipv6Subnets,omitempty"`
	DNS         DnsSpec    `json:"dns,omitempty"`
	Routes      []Route    `json:"routes,omitempty"`
}

type IpSubnet struct {
	Name       string `json:"name"`
	Subnet     string `json:"subnet"`
	Gateway    string `json:"gateway,omitempty"`
	ExcludeIps string `json:"excludeIps,omitempty"`
}

type Route struct {
	Dst string `json:"dst"`
	GW  string `json:"gw,omitempty"`
}

type DnsSpec struct {
	Nameservers []string `json:"nameservers,omitempty"`
	Domain      string   `json:"domain,omitempty"`
	Search      []string `json:"search,omitempty"`
	Options     []string `json:"options,omitempty"`
}

const (
	//Created indicates the status of success
	Created = "Created"
	//CreateInternalError indicates create internal irrecoverable Error
	CreateInternalError = "CreateInternalError"
	//DeleteInternalError indicates delete internal irrecoverable Error
	DeleteInternalError = "DeleteInternalError"
)

// NetworkStatus defines the observed state of Network
// +k8s:openapi-gen=true
type NetworkStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	State string `json:"state"` // Indicates if Network is in "created" state
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Network is the Schema for the networks API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type Network struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkSpec   `json:"spec,omitempty"`
	Status NetworkStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NetworkList contains a list of Network
type NetworkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Network `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Network{}, &NetworkList{})
}
