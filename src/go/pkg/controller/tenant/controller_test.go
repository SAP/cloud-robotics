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

package tenant

import (
	"context"
	"testing"

	apps "github.com/SAP/cloud-robotics/src/go/pkg/apis/apps/v1alpha1"
	config "github.com/SAP/cloud-robotics/src/go/pkg/apis/config/v1alpha1"
	registry "github.com/SAP/cloud-robotics/src/go/pkg/apis/registry/v1alpha1"
	cert "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	dns "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	"github.com/stretchr/testify/assert"
	networking "istio.io/client-go/pkg/apis/networking/v1beta1"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func createTestTenant(customTenantDomain bool) *config.Tenant {
	t := config.Tenant{
		ObjectMeta: meta.ObjectMeta{
			Name: "test-tenant",
		},
		Spec: config.TenantSpec{},
	}
	if customTenantDomain {
		t.Spec.TenantDomain = "tenant-domain.local"
	}
	return &t
}

func addTenantCondition(t *config.Tenant, tc config.TenantConditionType, v core.ConditionStatus) *config.Tenant {
	now := meta.Now()
	t.Status.Conditions = append(t.Status.Conditions, config.TenantCondition{
		Type:               tc,
		LastUpdateTime:     now,
		LastTransitionTime: now,
		Status:             v,
		Message:            "Test Condition",
	})
	return t
}

func boolToCondition(b bool) core.ConditionStatus {
	if b == true {
		return core.ConditionTrue
	}
	return core.ConditionFalse
}

func createTestTenantController(initObjs ...client.Object) *Reconciler {
	sc := runtime.NewScheme()
	scheme.AddToScheme(sc)
	config.AddToScheme(sc)
	cert.AddToScheme(sc)
	dns.AddToScheme(sc)
	networking.AddToScheme(sc)
	apps.AddToScheme(sc)
	registry.AddToScheme(sc)
	client := fake.NewClientBuilder().WithScheme(sc).WithObjects(initObjs...).Build()
	r := &Reconciler{client: client, scheme: sc, domain: "domain.local"}
	return r
}

func TestContainsString(t *testing.T) {
	type testCase struct {
		name     string
		s        string
		sList    []string
		expected bool
	}

	tests := []testCase{
		{
			name:     "Empty list",
			s:        "abc",
			sList:    []string{},
			expected: false,
		},
		{
			name:     "Not found",
			s:        "abc",
			sList:    []string{"zdf", "xyz"},
			expected: false,
		},
		{
			name:     "Found One",
			s:        "abc",
			sList:    []string{"zdf", "xyz", "abc"},
			expected: true,
		},
		{
			name:     "Found Two",
			s:        "abc",
			sList:    []string{"abc", "zdf", "xyz"},
			expected: true,
		},
		{
			name:     "Found multiple",
			s:        "abc",
			sList:    []string{"abc", "zdf", "xyz", "abc"},
			expected: true,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				assert.Equal(t, test.expected, containsString(test.sList, test.s))
			})
	}
}

func TestSetCondition(t *testing.T) {
	type testCase struct {
		name      string
		t         *config.Tenant
		condition config.TenantConditionType
		expected  bool
	}

	tests := []testCase{
		{
			name:      "ConditionInitial",
			t:         createTestTenant(true),
			condition: config.TenantConditionNamespace,
			expected:  false,
		},
		{
			name:      "ConditionFalse",
			t:         addTenantCondition(createTestTenant(true), config.TenantConditionNamespace, core.ConditionFalse),
			condition: config.TenantConditionNamespace,
			expected:  false,
		},
		{
			name:      "ConditionTrue",
			t:         addTenantCondition(createTestTenant(true), config.TenantConditionNamespace, core.ConditionTrue),
			condition: config.TenantConditionNamespace,
			expected:  true,
		},
		{
			name:      "ChangeCondition",
			t:         addTenantCondition(createTestTenant(true), config.TenantConditionNamespace, core.ConditionFalse),
			condition: config.TenantConditionNamespace,
			expected:  true,
		},
		{
			name:      "MultipleConditionsOne",
			t:         addTenantCondition(createTestTenant(true), config.TenantConditionDomain, core.ConditionTrue),
			condition: config.TenantConditionNamespace,
			expected:  true,
		},
		{
			name:      "MultipleConditionsTwo",
			t:         addTenantCondition(createTestTenant(true), config.TenantConditionDomain, core.ConditionFalse),
			condition: config.TenantConditionNamespace,
			expected:  true,
		},
	}
	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				setCondition(test.t, test.condition, boolToCondition(test.expected), "Test Condition")
				assert.Equal(t, test.expected, inCondition(test.t, test.condition))
			})
	}
}

func TestReconcileOne(t *testing.T) {
	ctx := context.Background()

	var clientObjects []client.Object

	// Test case: create a new tenant without custom tenant domain
	clientObjects = append(clientObjects, createTestTenant(false))

	c := createTestTenantController(clientObjects...)

	reconcileRequest := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name: "test-tenant",
		},
	}

	c.Reconcile(ctx, reconcileRequest)

	// Expected status of tenant
	var tenantResult config.Tenant
	c.client.Get(ctx, reconcileRequest.NamespacedName, &tenantResult)

	assert.Equal(t, true, inCondition(&tenantResult, config.TenantConditionNamespace))
	assert.Equal(t, true, inCondition(&tenantResult, config.TenantConditionServiceAccount))
	// There is no robot-setup config map in test data
	assert.Equal(t, false, inCondition(&tenantResult, config.TenantConditionRobotSetup))
	// There is no template pull secret in test data
	assert.Equal(t, false, inCondition(&tenantResult, config.TenantConditionPullSecret))
	assert.Equal(t, true, inCondition(&tenantResult, config.TenantConditionPermissions))
	// There is no LoadBalancer service with an IP adress in test data
	assert.Equal(t, false, inCondition(&tenantResult, config.TenantConditionDomain))
	assert.Equal(t, false, inCondition(&tenantResult, config.TenantConditionCertificate))
	assert.Equal(t, false, inCondition(&tenantResult, config.TenantConditionGateway))

}
