// Copyright 2021 The Cloud Robotics Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Tenant struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TenantSpec   `json:"spec"`
	Status TenantStatus `json:"status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type TenantList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Tenant `json:"items"`
}

type TenantSpec struct {
	TenantDomain string `json:"tenantDomain,omitempty"`
}

type TenantStatus struct {
	Robots           int               `json:"robots"`
	RobotClusters    int               `json:"robotClusters"`
	Gateway          string            `json:"gateway,omitempty"`
	TenantDomain     string            `json:"tenantDomain,omitempty"`
	TenantNamespaces []string          `json:"tenantNamespaces,omitempty"`
	Conditions       []TenantCondition `json:"conditions,omitempty"`
}

type TenantCondition struct {
	Type               TenantConditionType    `json:"type"`
	Status             corev1.ConditionStatus `json:"status"`
	LastUpdateTime     metav1.Time            `json:"lastUpdateTime,omitempty"`
	LastTransitionTime metav1.Time            `json:"lastTransitionTime,omitempty"`
	Message            string                 `json:"message,omitempty"`
}

type TenantConditionType string

const (
	TenantConditionNamespace      TenantConditionType = "Namespace"
	TenantConditionDomain         TenantConditionType = "Domain"
	TenantConditionCertificate    TenantConditionType = "Certificate"
	TenantConditionGateway        TenantConditionType = "Gateway"
	TenantConditionServiceAccount TenantConditionType = "ServiceAccount"
	TenantConditionPermissions    TenantConditionType = "Permissions"
	TenantConditionPullSecret     TenantConditionType = "PullSecret"
	TenantConditionRobotSetup     TenantConditionType = "RobotSetupConfig"
)
