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
	"net/http"

	config "github.com/SAP/cloud-robotics/src/go/pkg/apis/config/v1alpha1"
	"github.com/SAP/cloud-robotics/src/go/pkg/coretools"
	"github.com/pkg/errors"
	admissionv1 "k8s.io/api/admission/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// tenantValidator validates Tenants
type tenantValidator struct {
	client                 client.Client
	decoder                *admission.Decoder
	tenantSpecificGateways bool
}

func NewTenantValidationWebhook(client client.Client, tenantSpecificGateways bool) *admission.Webhook {
	return &admission.Webhook{Handler: &tenantValidator{client: client, tenantSpecificGateways: tenantSpecificGateways}}
}

// Handle checks if the given auctioneer is valid
func (v *tenantValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	var tenant config.Tenant
	err := v.decoder.Decode(req, &tenant)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	var tenantList config.TenantList
	err = v.client.List(ctx, &tenantList)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Check if there are no other tenants if a default tenant should be created/updated
	if tenant.Name == coretools.DefaultTenantName && req.Operation != admissionv1.Delete {
		for _, t := range tenantList.Items {
			if t.Name != coretools.DefaultTenantName {
				return admission.Errored(http.StatusBadRequest, errors.New("Delete all other tenants before creating the default tenant"))
			}
		}
	}

	// Check if there is no default tenant if a tenant should be created/updated
	if tenant.Name != coretools.DefaultTenantName && req.Operation != admissionv1.Delete {
		for _, t := range tenantList.Items {
			if t.Name == coretools.DefaultTenantName {
				return admission.Errored(http.StatusBadRequest, errors.New("Delete default tenant before creating other tenants"))
			}
		}
	}

	if !v.tenantSpecificGateways && tenant.Spec.TenantDomain != "" {
		return admission.Errored(http.StatusBadRequest, errors.New("TenantDomain must be empty when tenant specific gateways are disabled"))
	}

	return admission.Allowed("")
}

// InjectDecoder injects the decoder.
func (v *tenantValidator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}
