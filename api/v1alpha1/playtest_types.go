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
type PlaytestGroup struct {
	Name  string   `json:"name,omitempty"`
	Users []string `json:"users,omitempty"`
}

// PlaytestSpec defines the desired state of Playtest
type PlaytestSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	DisplayName     string      `json:"displayName,omitempty"`
	Version         string      `json:"version,omitempty"`
	Map             string      `json:"map,omitempty"`
	MinGroups       int         `json:"minGroups,omitempty"`
	PlayersPerGroup int         `json:"playersPerGroup,omitempty"`
	StartTime       metav1.Time `json:"startTime,omitempty"`
	FeedbackURL     string      `json:"feedbackURL,omitempty"`

	// +optional
	UsersToAutoAssign []string `json:"usersToAutoAssign,omitempty"`
	GameServerCmdArgs []string `json:"gameServerCmdArgs,omitempty"`

	Groups []PlaytestGroup `json:"groups,omitempty"`

	// IncludeReadinessProbe is true if the game server should include a readiness probe
	// +kubebuilder:default=false
	IncludeReadinessProbe bool `json:"includeReadinessProbe,omitempty"`

	// DisableGameServers is true if game servers should not be created for this playtest
	// +kubebuilder:default=false
	DisableGameServers bool `json:"disableGameServers,omitempty"`
}

type PlaytestGroupStatus struct {
	Name      string                       `json:"name,omitempty"`
	ServerRef *corev1.LocalObjectReference `json:"serverRef,omitempty"`
	Users     []string                     `json:"users,omitempty"`
	Ready     bool                         `json:"ready,omitempty"`
}

// PlaytestStatus defines the observed state of Playtest
type PlaytestStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Groups []PlaytestGroupStatus `json:"groups,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Playtest is the Schema for the playtests API
type Playtest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PlaytestSpec   `json:"spec,omitempty"`
	Status PlaytestStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PlaytestList contains a list of Playtest
type PlaytestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Playtest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Playtest{}, &PlaytestList{})
}
