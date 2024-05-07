/*
Copyright 2023.

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// GameServerSpec defines the desired state of GameServer
type GameServerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// DisplayName is the human-readable name of the game server
	DisplayName string `json:"displayName,omitempty"`

	// Version corresponds to the git commit SHA of the desired game version
	Version string `json:"version"`

	// Path to map for server to load
	Map string `json:"map,omitempty"`
}

// GameServerStatus defines the observed state of GameServer
type GameServerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// IP represents the underlying pod's IP
	IP string `json:"ip,omitempty"`

	// Port represents the port on which the underlying Pod is listening for game traffic
	Port int32 `json:"port,omitempty"`

	// NetImguiPort represents the port on which the underlying pod is listening for netimgui traffic
	NetImguiPort int32 `json:"netimguiPort,omitempty"`

	// PodRef refers to the name of the Pod backing the GameServer
	PodRef *corev1.LocalObjectReference `json:"podRef,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:path=gameservers,scope=Namespaced,shortName=gs
//+kubebuilder:printcolumn:name="IP",type=string,JSONPath=`.status.ip`
//+kubebuilder:printcolumn:name="Port",type=integer,JSONPath=`.status.port`
//+kubebuilder:printcolumn:name="Reserved Slots",type=integer,JSONPath=`.status.reservedCount`

// GameServer is the Schema for the gameservers API
type GameServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GameServerSpec   `json:"spec,omitempty"`
	Status GameServerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// GameServerList contains a list of GameServer
type GameServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GameServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GameServer{}, &GameServerList{})
}
