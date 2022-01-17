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
	"fmt"
	"log"
	"strings"
	"time"

	apps "github.com/SAP/cloud-robotics/src/go/pkg/apis/apps/v1alpha1"
	config "github.com/SAP/cloud-robotics/src/go/pkg/apis/config/v1alpha1"
	registry "github.com/SAP/cloud-robotics/src/go/pkg/apis/registry/v1alpha1"
	"github.com/SAP/cloud-robotics/src/go/pkg/coretools"
	cert "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	dns "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	"github.com/pkg/errors"
	networkingapi "istio.io/api/networking/v1beta1"
	networking "istio.io/client-go/pkg/apis/networking/v1beta1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	finalizer               string = "tenant-controller.config.cloud-robotics.com"
	crSyncerRolebinding     string = "cloud-robotics:cr-syncer:robot-service"
	robotSetupRoleBinding   string = "cloud-robotics:robot-setup-service"
	dnsEntryName            string = "tenant-domain"
	istioLBServiceName      string = "istio-ingressgateway"
	istioLBServiceNamespace string = "istio-system"
	gatewayName             string = "tenant-gateway"
	tenantLabel             string = "cloud-robotics-tenant"
	// Requeue interval when the underlying Tenant is not in a stable state yet.
	requeueFast = 3 * time.Second
	// Requeue interval after the underlying Tenant reached a stable state.
	requeueSlow = 3 * time.Minute
)

// Reconciler for cloud-robotics tenants
type Reconciler struct {
	client                 client.Client
	scheme                 *runtime.Scheme
	domain                 string
	defaultGateway         string
	tenantSpecificGateways bool
}

var _ reconcile.Reconciler = &Reconciler{}

func containsString(s []string, e string) bool {
	for _, x := range s {
		if x == e {
			return true
		}
	}
	return false
}

func stringsDelete(list []string, s string) (res []string) {
	for _, x := range list {
		if x != s {
			res = append(res, x)
		}
	}
	return res
}

// inCondition returns true if the Tenant has a condition of the given
// type in state true.
func inCondition(t *config.Tenant, tc config.TenantConditionType) bool {
	for _, cond := range t.Status.Conditions {
		if cond.Type == tc && cond.Status == core.ConditionTrue {
			return true
		}
	}
	return false
}

// setCondition adds or updates a condition. Existing conditions are detected
// based on the Type field.
func setCondition(t *config.Tenant, tc config.TenantConditionType, v core.ConditionStatus, msg string) {
	now := meta.Now()

	for i, c := range t.Status.Conditions {
		if c.Type != tc {
			continue
		}
		// Update existing condition.
		if c.Status != v || c.Message != msg {
			c.LastUpdateTime = now
		}
		if c.Status != v {
			c.LastTransitionTime = now
		}
		c.Message = msg
		c.Status = v
		t.Status.Conditions[i] = c
		return
	}
	// Condition set for the first time.
	t.Status.Conditions = append(t.Status.Conditions, config.TenantCondition{
		Type:               tc,
		LastUpdateTime:     now,
		LastTransitionTime: now,
		Status:             v,
		Message:            msg,
	})
}

func setLabel(o *meta.ObjectMeta, k, v string) {
	if o.Labels == nil {
		o.Labels = map[string]string{}
	}
	o.Labels[k] = v
}

func setAnnotation(o *meta.ObjectMeta, k, v string) {
	if o.Annotations == nil {
		o.Annotations = map[string]string{}
	}
	o.Annotations[k] = v
}

func Add(mgr manager.Manager, domain, defaultGateway string, tenanSpecificGateways bool) error {

	// Create tenant controller
	r := &Reconciler{client: mgr.GetClient(), scheme: mgr.GetScheme(), domain: domain, defaultGateway: defaultGateway, tenantSpecificGateways: tenanSpecificGateways}
	c, err := controller.New("tenant-controller", mgr, controller.Options{Reconciler: r})

	if err != nil {
		return errors.Wrap(err, "create controller")
	}

	// Watch tenant CRs
	err = c.Watch(&source.Kind{Type: &config.Tenant{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return errors.Wrap(err, "watch Tenant")
	}

	// Watch namespaces
	err = c.Watch(&source.Kind{Type: &core.Namespace{}},
		&handler.Funcs{
			CreateFunc: func(e event.CreateEvent, q workqueue.RateLimitingInterface) {
				r.enqueueFromNamespaceName(e.Object, q)
			},
			DeleteFunc: func(e event.DeleteEvent, q workqueue.RateLimitingInterface) {
				r.enqueueFromNamespaceName(e.Object, q)
			},
		},
	)
	if err != nil {
		return errors.Wrap(err, "watch Namespace")
	}

	// Watch service accounts
	err = c.Watch(&source.Kind{Type: &core.ServiceAccount{}}, &handler.EnqueueRequestForOwner{OwnerType: &config.Tenant{}})
	if err != nil {
		return errors.Wrap(err, "watch ServiceAccount")
	}

	// Watch role bindings
	err = c.Watch(&source.Kind{Type: &rbac.RoleBinding{}}, &handler.EnqueueRequestForOwner{OwnerType: &config.Tenant{}})
	if err != nil {
		return errors.Wrap(err, "watch RoleBinding")
	}

	// Watch secrets
	err = c.Watch(&source.Kind{Type: &core.Secret{}}, &handler.EnqueueRequestForOwner{OwnerType: &config.Tenant{}})
	if err != nil {
		return errors.Wrap(err, "watch Secret")
	}

	// Watch robot-config confimap
	err = c.Watch(&source.Kind{Type: &core.ConfigMap{}},
		&handler.Funcs{
			CreateFunc: func(e event.CreateEvent, q workqueue.RateLimitingInterface) {
				if e.Object.GetName() == coretools.RobotSetupConfigmap && e.Object.GetNamespace() == coretools.RobotConfigCloudNamespace {
					r.enqueueAll(q)
				}
			},
			UpdateFunc: func(e event.UpdateEvent, q workqueue.RateLimitingInterface) {
				if e.ObjectNew.GetName() == coretools.RobotSetupConfigmap && e.ObjectNew.GetNamespace() == coretools.RobotConfigCloudNamespace {
					r.enqueueAll(q)
				}
			},
			DeleteFunc: func(e event.DeleteEvent, q workqueue.RateLimitingInterface) {
				if e.Object.GetName() == coretools.RobotSetupConfigmap && e.Object.GetNamespace() == coretools.RobotConfigCloudNamespace {
					r.enqueueAll(q)
				}
			},
		},
	)
	if err != nil {
		return errors.Wrap(err, "watch Namespace")
	}

	if tenanSpecificGateways {
		// Watch dns entries
		err = c.Watch(&source.Kind{Type: &dns.DNSEntry{}}, &handler.EnqueueRequestForOwner{OwnerType: &config.Tenant{}})
		if err != nil {
			return errors.Wrap(err, "watch DNSEntry")
		}

		// Watch certificates
		err = c.Watch(&source.Kind{Type: &cert.Certificate{}}, &handler.EnqueueRequestForOwner{OwnerType: &config.Tenant{}})
		if err != nil {
			return errors.Wrap(err, "watch Certificate")
		}

		// Watch Istio gateways
		err = c.Watch(&source.Kind{Type: &networking.Gateway{}}, &handler.EnqueueRequestForOwner{OwnerType: &config.Tenant{}})
		if err != nil {
			return errors.Wrap(err, "watch Gateway")
		}
	}

	// Watch robots
	err = c.Watch(&source.Kind{Type: &registry.Robot{}},
		&handler.Funcs{
			CreateFunc: func(e event.CreateEvent, q workqueue.RateLimitingInterface) {
				r.enqueueFromObjectNamespace(e.Object, q)
			},
			DeleteFunc: func(e event.DeleteEvent, q workqueue.RateLimitingInterface) {
				r.enqueueFromObjectNamespace(e.Object, q)
			},
		},
	)
	if err != nil {
		return errors.Wrap(err, "watch Robot")
	}

	return err
}

func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	// Get tenant CR
	var t config.Tenant

	err := r.client.Get(ctx, client.ObjectKey{Name: request.Name}, &t)
	if k8serrors.IsNotFound(err) {
		log.Printf("Tenant %s does not longer exist, skipping reconciliation", request.Name)
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "get CR of tenant %s", request.Name)
	}

	// Check for deleted tenant
	if t.DeletionTimestamp != nil {
		log.Printf("Ensure clean-up of tenant %s", t.Name)
		deleted, err := r.ensureDeleted(ctx, &t)
		if err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "delete tenant %s", t.Name)
		}
		if !deleted {
			return reconcile.Result{RequeueAfter: requeueFast}, nil
		}
		if ok := containsString(t.Finalizers, finalizer); ok {
			log.Printf("Removing finalizer from tenant %s", t.Name)
			t.Finalizers = stringsDelete(t.Finalizers, finalizer)
			if err := r.client.Update(ctx, &t); err != nil {
				return reconcile.Result{}, errors.Wrapf(err, "remove finalizer from tenant %s", t.Name)
			}
		}
		log.Printf("Tenant %s deleted", t.Name)
		return reconcile.Result{}, nil
	}

	// Add finalizer
	if ok := containsString(t.Finalizers, finalizer); !ok {
		log.Printf("Adding finalizer to tenant %s", t.Name)
		t.Finalizers = append(t.Finalizers, finalizer)
		if err := r.client.Update(ctx, &t); err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "add finalizer to tenant %s", t.Name)
		}
	}

	var lastErrorOnEnsure error

	// Ensure tenant namespaces are created
	if err := r.ensureNamespace(ctx, &t); err != nil {
		if _, ok := err.(*coretools.NamespaceDeletionError); ok {
			log.Printf("namespace in deletion: %s", err)
			// Requeue to track deletion progress.
			r.updateTenantStatus(ctx, &t)
			return reconcile.Result{RequeueAfter: requeueFast}, nil
		}
		r.updateTenantStatus(ctx, &t)
		return reconcile.Result{}, errors.Wrap(err, "ensure namespace")
	}

	// Collect namespaces of this tenant
	if err := r.collectTenantNamespaces(ctx, &t); err != nil {
		r.updateTenantStatus(ctx, &t)
		return reconcile.Result{}, errors.Wrap(err, "collect namespaces")
	}

	// Collect robots of this tenant
	if err := r.collectRobots(ctx, &t); err != nil {
		r.updateTenantStatus(ctx, &t)
		return reconcile.Result{}, errors.Wrap(err, "collect robots")
	}

	// Ensure robot / robot setup service accounts are existing
	if err := r.ensureServiceAccount(ctx, &t); err != nil {
		lastErrorOnEnsure = err
		log.Printf("Ensure service account for tenant %s failed: %s", t.Name, err)
	}

	// Ensure permissions for service accounts are correct
	if err := r.ensurePermissions(ctx, &t); err != nil {
		lastErrorOnEnsure = err
		log.Printf("Ensure permissions for tenant %s failed: %s", t.Name, err)
	}

	// Ensure robot-setup configmap
	if err := r.ensureRobotSetupConfig(ctx, &t); err != nil {
		lastErrorOnEnsure = err
		log.Printf("Ensure robot-setup config for tenant %s failed: %s", t.Name, err)
	}

	// Ensure that image pull secret exists in tenant main namespace and is assigned to its default service account
	if err := r.ensurePullSecret(ctx, &t); err != nil {
		lastErrorOnEnsure = err
		if _, ok := err.(*coretools.MissingServiceAccountError); ok {
			log.Printf("Missing default service account for tenant %s. This is expected to occur rarely: %s", t.Name, err)
		} else {
			log.Printf("Ensure pull-secret for tenant %s failed: %s", t.Name, err)
		}
	}

	if r.tenantSpecificGateways {
		// Ensure DNS Entry is correct
		dnsErrors := false
		if err := r.ensureDNS(ctx, &t); err != nil {
			dnsErrors = true
			lastErrorOnEnsure = err
			log.Printf("Ensure DNS of tenant %s failed: %s", t.Name, err)
		}

		certErrors := false
		// Ensure TLS certicate is created
		if !dnsErrors {
			if err := r.ensureCertificate(ctx, &t); err != nil {
				certErrors = true
				lastErrorOnEnsure = err
				log.Printf("Ensure certificate for tenant %s failed: %s", t.Name, err)
			}
		} else {
			setCondition(&t, config.TenantConditionCertificate, core.ConditionFalse, "No DNS record for the tenant created")
		}

		// Ensure Istio Gateway is setup
		if !dnsErrors && !certErrors {
			if err := r.ensureGateway(ctx, &t); err != nil {
				lastErrorOnEnsure = err
				fmt.Printf("Ensure Istio gateway for domain %s on tenant %s failed: %s", t.Status.TenantDomain, t.Name, err)
			}
		} else {
			setCondition(&t, config.TenantConditionGateway, core.ConditionFalse, "No DNS record and/or TLS certificate for the tenant created")
		}
	} else {
		t.Status.TenantDomain = ""
		t.Status.Gateway = r.defaultGateway
	}

	// Update status
	err = r.updateTenantStatus(ctx, &t)
	if err != nil {
		return reconcile.Result{}, err
	}

	if lastErrorOnEnsure != nil {
		return reconcile.Result{}, lastErrorOnEnsure
	}

	return reconcile.Result{RequeueAfter: requeueSlow}, nil
}

func (r *Reconciler) enqueueAll(q workqueue.RateLimitingInterface) {
	var tenantList config.TenantList
	err := r.client.List(context.Background(), &tenantList)
	if err != nil {
		log.Print("Error getting tenants when enqueueFromNamespaceName")
		return
	}
	for _, t := range tenantList.Items {
		q.Add(reconcile.Request{
			NamespacedName: client.ObjectKey{Name: t.Name},
		})

	}
}

func (r *Reconciler) enqueueFromNamespaceName(o client.Object, q workqueue.RateLimitingInterface) {
	// Enqueue namespaces starting with tenant prefix "t-"
	if namespace := o.GetName(); strings.HasPrefix(namespace, coretools.TenantPrefix) {
		split := strings.SplitN(namespace, "-", 2)
		if len(split) > 1 {
			var tenantList config.TenantList
			err := r.client.List(context.Background(), &tenantList)
			if err != nil {
				log.Print("Error getting tenants when enqueueFromNamespaceName")
				return
			}
			for _, t := range tenantList.Items {
				// If there are two tenants where one tenant name acts like a prefix for the other too many reconciles could be triggered
				// A namespace change for tenant "abc" will also trigger reconcile for tenant "abc-xyz"
				if strings.HasPrefix(split[1], t.Name) {
					q.Add(reconcile.Request{
						NamespacedName: client.ObjectKey{Name: t.Name},
					})
				}
			}
		}
	}
}

func (r *Reconciler) enqueueFromObjectNamespace(o client.Object, q workqueue.RateLimitingInterface) {
	q.Add(reconcile.Request{
		NamespacedName: client.ObjectKey{Name: strings.TrimPrefix(o.GetNamespace(), coretools.TenantPrefix)},
	})
}

func (r *Reconciler) getCertificateName(t *config.Tenant) string {
	return fmt.Sprintf("t-%s-tls", t.Name)
}

func (r *Reconciler) getDefaultTenantDomain(t *config.Tenant) string {
	if t.Spec.TenantDomain == "" {
		return fmt.Sprintf("%s.t.%s", t.Name, r.domain)
	}
	return t.Spec.TenantDomain
}

func (r *Reconciler) generateTenantDomain() (*string, error) {
	tenantSubdomain := fmt.Sprintf(".t.%s", r.domain)
	tenantSubdomainLength := len(tenantSubdomain)
	if tenantSubdomainLength > 61 {
		return nil, errors.Errorf("Cannot generate domain name. Tenant subdomain %q is longer than 61 characeters", tenantSubdomain)
	}
	tenantDomain := fmt.Sprintf("%s%s", coretools.RandomString(62-tenantSubdomainLength), tenantSubdomain)
	return &tenantDomain, nil
}

func (r *Reconciler) updateTenantStatus(ctx context.Context, t *config.Tenant) error {
	err := r.client.Status().Update(ctx, t)
	if err != nil {
		log.Printf("Update of tenant status %s failed: %s", t.Name, err)
	}
	return err
}

func (r *Reconciler) ensureNamespace(ctx context.Context, t *config.Tenant) error {
	if t.Name == coretools.DefaultTenantName {
		setCondition(t, config.TenantConditionNamespace, core.ConditionTrue, "Default tenant uses default namespace")
		return nil
	}

	mainNamespace := coretools.TenantMainNamespace(t.Name)
	robotConfigNamespace := coretools.RobotConfigNamespace(mainNamespace)
	tenantNamespaces := []string{mainNamespace, robotConfigNamespace}
	for _, namespace := range tenantNamespaces {
		// Create tenant namespace if it doesn't exist.
		var ns core.Namespace
		ns.Name = namespace
		err := r.client.Get(ctx, client.ObjectKeyFromObject(&ns), &ns)

		if err != nil && !k8serrors.IsNotFound(err) {
			setCondition(t, config.TenantConditionNamespace, core.ConditionFalse, fmt.Sprintf("Getting namespace %s failed: %s", ns.Name, err))
			return errors.Errorf("getting Namespace %q failed: %s", namespace, err)
		}
		if ns.DeletionTimestamp != nil {
			setCondition(t, config.TenantConditionNamespace, core.ConditionFalse, fmt.Sprintf("Namespace %s in deletion", ns.Name))
			return coretools.NewNamespaceDeletionError(fmt.Sprintf("namespace %q was marked for deletion at %s, skipping", namespace, ns.DeletionTimestamp))
		}

		createNamespace := k8serrors.IsNotFound(err)

		setLabel(&t.ObjectMeta, tenantLabel, t.Name)
		controllerutil.SetControllerReference(t, &ns, r.scheme)

		if createNamespace {
			log.Printf("Creating namespace %s for tenant %s", namespace, t.Name)
			err = r.client.Create(ctx, &ns)
		} else {
			err = r.client.Update(ctx, &ns)
		}
		if err != nil {
			setCondition(t, config.TenantConditionNamespace, core.ConditionFalse, fmt.Sprintf("Updating namespace %s failed: %s", ns.Name, err))
			return err
		}
	}
	setCondition(t, config.TenantConditionNamespace, core.ConditionTrue, "Tenant namespaces created")
	return nil
}

func (r *Reconciler) ensureServiceAccount(ctx context.Context, t *config.Tenant) error {
	tenantServiceAccounts := []string{coretools.RobotServiceAccount, coretools.RobotSetupServiceAccount}
	namespace := coretools.RobotConfigNamespace(coretools.TenantMainNamespace(t.Name))
	for _, serviceAccount := range tenantServiceAccounts {
		var sa core.ServiceAccount
		sa.Namespace = namespace
		sa.Name = serviceAccount
		err := r.client.Get(ctx, client.ObjectKeyFromObject(&sa), &sa)

		if err != nil && !k8serrors.IsNotFound(err) {
			setCondition(t, config.TenantConditionServiceAccount, core.ConditionFalse, fmt.Sprintf("Get service account %s failed: %s", sa.Name, err))
			return errors.Wrap(err, "get service account")
		}

		createServiceAccount := k8serrors.IsNotFound(err)
		controllerutil.SetControllerReference(t, &sa, r.scheme)

		if createServiceAccount {
			log.Printf("Creating service account %s for tenant %s", sa.Name, t.Name)
			err = r.client.Create(ctx, &sa)
		} else {
			err = r.client.Update(ctx, &sa)
		}
		if err != nil {
			setCondition(t, config.TenantConditionServiceAccount, core.ConditionFalse, fmt.Sprintf("Updating service account %s failed: %s", sa.Name, err))
			return err
		}
	}
	setCondition(t, config.TenantConditionServiceAccount, core.ConditionTrue, "Tenant service accounts created")
	return nil
}

type roleRefSubjects struct {
	roleRef  rbac.RoleRef
	subjects []rbac.Subject
}

func (r *Reconciler) ensurePermissions(ctx context.Context, t *config.Tenant) error {
	mainNamespace := coretools.TenantMainNamespace(t.Name)
	robotConfigNamespace := coretools.RobotConfigNamespace(mainNamespace)
	// Ensure role bindings are created
	roleBindings := map[types.NamespacedName]roleRefSubjects{
		// cr-syncer RoleBinding for robot-service
		{Name: crSyncerRolebinding, Namespace: mainNamespace}: {
			roleRef: rbac.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "cloud-robotics:cr-syncer",
			},
			subjects: []rbac.Subject{
				{
					Kind:      rbac.ServiceAccountKind,
					Namespace: robotConfigNamespace,
					Name:      coretools.RobotServiceAccount,
				},
			},
		},
		// robot-config RoleBinding for robot-setup-service
		{Name: robotSetupRoleBinding, Namespace: robotConfigNamespace}: {
			roleRef: rbac.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "cloud-robotics:robot-setup:robot-config",
			},
			subjects: []rbac.Subject{
				{
					Kind:      rbac.ServiceAccountKind,
					Namespace: robotConfigNamespace,
					Name:      coretools.RobotSetupServiceAccount,
				},
			},
		},
		// robots RoleBinding for robot-setup-service
		{Name: robotSetupRoleBinding, Namespace: mainNamespace}: {
			roleRef: rbac.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "cloud-robotics:robot-setup:robots",
			},
			subjects: []rbac.Subject{
				{
					Kind:      rbac.ServiceAccountKind,
					Namespace: robotConfigNamespace,
					Name:      coretools.RobotSetupServiceAccount,
				},
			},
		},
	}

	// Ensure role bindings from map
	for namespacedName, roleRefSubjects := range roleBindings {
		var rb rbac.RoleBinding

		rb.Name = namespacedName.Name
		rb.Namespace = namespacedName.Namespace

		err := r.client.Get(ctx, client.ObjectKeyFromObject(&rb), &rb)

		if err != nil && !k8serrors.IsNotFound(err) {
			setCondition(t, config.TenantConditionPermissions, core.ConditionFalse, fmt.Sprintf("Get role binding failed: %s", err))
			return errors.Wrapf(err, "get role binding %s", namespacedName)
		}

		createRoleBinding := k8serrors.IsNotFound(err)

		rb.RoleRef = roleRefSubjects.roleRef
		rb.Subjects = roleRefSubjects.subjects
		controllerutil.SetControllerReference(t, &rb, r.scheme)

		if createRoleBinding {
			log.Printf("Creating role binding %s for tenant %s", rb.Name, t.Name)
			err = r.client.Create(ctx, &rb)
		} else {
			err = r.client.Update(ctx, &rb)
		}
		if err != nil {
			setCondition(t, config.TenantConditionPermissions, core.ConditionFalse, fmt.Sprintf("Update role binding failed: %s", err))
			return errors.Wrapf(err, "create %s role binding", namespacedName)
		}
	}
	setCondition(t, config.TenantConditionPermissions, core.ConditionTrue, "Tenant permissions set")
	return nil
}

func (r *Reconciler) ensurePullSecret(ctx context.Context, t *config.Tenant) error {
	// Copy imagePullSecret from 'default' namespace, since service accounts cannot reference
	// secrets in other namespaces.
	var templateSecret core.Secret
	//TODO: this should be changed when chart assignment controller is running in a different namespace
	err := r.client.Get(ctx, client.ObjectKey{Namespace: "default", Name: coretools.ImagePullSecret}, &templateSecret)
	if k8serrors.IsNotFound(err) {
		setCondition(t, config.TenantConditionPullSecret, core.ConditionFalse, fmt.Sprintf("Secret \"default:%s\" not found", coretools.ImagePullSecret))
		return fmt.Errorf("secret \"default:%s\" not found", coretools.ImagePullSecret)
	} else if err != nil {
		setCondition(t, config.TenantConditionPullSecret, core.ConditionFalse, fmt.Sprintf("getting Secret \"default:%s\" failed: %s", coretools.ImagePullSecret, err))
		return fmt.Errorf("getting Secret \"default:%s\" failed: %s", coretools.ImagePullSecret, err)
	}

	mainNamespace := coretools.TenantMainNamespace(t.Name)
	robotConfigNamespace := coretools.RobotConfigNamespace(mainNamespace)

	namespaces := []string{mainNamespace, robotConfigNamespace}

	for _, namespace := range namespaces {
		// Do not change pull secret of default namespace
		if namespace == meta.NamespaceDefault {
			continue
		}
		var secret core.Secret
		secret.Name = coretools.ImagePullSecret
		secret.Namespace = namespace
		err = r.client.Get(ctx, client.ObjectKeyFromObject(&secret), &secret)

		if err != nil && !k8serrors.IsNotFound(err) {
			setCondition(t, config.TenantConditionPullSecret, core.ConditionFalse, fmt.Sprintf("Get secret %s/%s failed: %s", secret.Namespace, secret.Name, err))
			return errors.Wrapf(err, "get secret %s", secret.Name)
		}

		createSecret := k8serrors.IsNotFound(err)

		controllerutil.SetControllerReference(t, &secret, r.scheme)
		secret.Data = templateSecret.Data
		secret.Type = templateSecret.Type

		if createSecret {
			log.Printf("Creating docker pull secret %s/%s for tenant %s", secret.Namespace, secret.Name, t.Name)
			err = r.client.Create(ctx, &secret)
		} else {
			err = r.client.Update(ctx, &secret)
		}
		if err != nil {
			setCondition(t, config.TenantConditionPullSecret, core.ConditionFalse, fmt.Sprintf("Update secret %s/%s failed: %s", secret.Name, secret.Namespace, err))
			return fmt.Errorf("update Secret \"%s:%s\" failed: %s", namespace, coretools.ImagePullSecret, err)
		}
		// Configure the default service account in the namespace.
		var sa core.ServiceAccount
		err = r.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "default"}, &sa)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				// The Service Account Controller hasn't created the default SA yet.
				setCondition(t, config.TenantConditionPullSecret, core.ConditionFalse, "Missing default service account")
				return coretools.NewMissingServiceAccountError(fmt.Sprintf("ServiceAccount \"%s:default\" not yet created", namespace))
			}
			setCondition(t, config.TenantConditionPullSecret, core.ConditionFalse, fmt.Sprintf("Get ServiceAccount \"%s:default\" failed: %s", namespace, err))
			return fmt.Errorf("get ServiceAccount \"%s:default\" failed: %s", namespace, err)
		}

		// Only add the secret once.
		ips := core.LocalObjectReference{Name: coretools.ImagePullSecret}
		found := false
		for _, s := range sa.ImagePullSecrets {
			if s == ips {
				found = true
				break
			}
		}
		if !found {
			sa.ImagePullSecrets = append(sa.ImagePullSecrets, ips)
		}
		err = r.client.Update(ctx, &sa)
		if err != nil {
			setCondition(t, config.TenantConditionPullSecret, core.ConditionFalse, fmt.Sprintf("Update ServiceAccount \"%s:default\" failed: %s", namespace, err))
			return err
		}
	}
	setCondition(t, config.TenantConditionPullSecret, core.ConditionTrue, "Tenant pull secret created")
	return nil
}

func (r *Reconciler) ensureDNS(ctx context.Context, t *config.Tenant) error {
	mainNamespace := coretools.TenantMainNamespace(t.Name)

	var d dns.DNSEntry
	d.Name = dnsEntryName
	d.Namespace = mainNamespace

	err := r.client.Get(ctx, client.ObjectKeyFromObject(&d), &d)

	if err != nil && !k8serrors.IsNotFound(err) {
		setCondition(t, config.TenantConditionDomain, core.ConditionFalse, fmt.Sprintf("Get DNS entry failed: %s", err))
		return errors.Wrapf(err, "get DNS entry %s", dnsEntryName)
	}

	createDNSEntry := k8serrors.IsNotFound(err)

	// Get IP adress from Istio loadbalancer service
	var iSVC core.Service
	iSVC.Name = istioLBServiceName
	iSVC.Namespace = istioLBServiceNamespace

	err = r.client.Get(ctx, client.ObjectKeyFromObject(&iSVC), &iSVC)
	if err != nil {
		setCondition(t, config.TenantConditionDomain, core.ConditionFalse, fmt.Sprintf("Get Istio loadbalancer service failed: %s", err))
		return errors.Wrap(err, "get Istio loadbalancer service")
	}

	var target string
	for _, i := range iSVC.Status.LoadBalancer.Ingress {
		if i.IP != "" {
			target = i.IP
			break
		}
		if i.Hostname != "" {
			target = i.Hostname
			break
		}
	}
	if target == "" {
		setCondition(t, config.TenantConditionDomain, core.ConditionFalse, "No hostname or IP in Istio loadbalancer service found")
		return errors.New("No hostname or IP in Istio loadbalancer service found")
	}

	tenantDomain := r.getDefaultTenantDomain(t)

	// Tenant domain must not be longer than 64 characters, because this is the maximum length for certificate common name
	// limit to 62 to allow sub domain wildcard *.
	if len(tenantDomain) > 62 {
		if strings.Contains(t.Status.TenantDomain, fmt.Sprintf(".t.%s", r.domain)) {
			tenantDomain = t.Status.TenantDomain
		} else {
			log.Printf("Default tenant domain *.%s has more than 64 characters which is maximum for commonName in TLS certificates. Generating random tenant domain", tenantDomain)
			g, err := r.generateTenantDomain()
			if err != nil {
				setCondition(t, config.TenantConditionDomain, core.ConditionFalse, fmt.Sprintf("Generate tenant domain failed: %s", err))
				return errors.Wrap(err, "generate tenant domain")
			}
			tenantDomain = *g
			log.Printf("Using generated tenant domain %s", tenantDomain)
		}
	}

	d.Spec.DNSName = fmt.Sprintf("*.%s", tenantDomain)
	ttl := int64(600)
	d.Spec.TTL = &ttl
	d.Spec.Targets = []string{target}
	setAnnotation(&d.ObjectMeta, "dns.gardener.cloud/class", "garden")
	controllerutil.SetControllerReference(t, &d, r.scheme)

	if createDNSEntry {
		log.Printf("Creating DNS entry %s for tenant %s", d.Name, t.Name)
		err = r.client.Create(ctx, &d)
	} else {
		err = r.client.Update(ctx, &d)
	}
	if err != nil {
		setCondition(t, config.TenantConditionDomain, core.ConditionFalse, fmt.Sprintf("Update DNS entry failed: %s", err))
		return errors.Wrapf(err, "update %s DNS entry", dnsEntryName)
	}

	t.Status.TenantDomain = tenantDomain

	if d.Status.State == dns.STATE_READY {
		setCondition(t, config.TenantConditionDomain, core.ConditionTrue, fmt.Sprintf("DNS entry for domain *.%s created", t.Status.TenantDomain))
	} else {
		setCondition(t, config.TenantConditionDomain, core.ConditionFalse, fmt.Sprintf("DNS entry for domain *.%s created, but in state %q", t.Status.TenantDomain, d.Status.State))
	}
	return nil
}

func (r *Reconciler) ensureCertificate(ctx context.Context, t *config.Tenant) error {
	tenantDomain := t.Status.TenantDomain
	if tenantDomain == "" {
		setCondition(t, config.TenantConditionCertificate, core.ConditionFalse, "Tenant domain not set yet")
		return errors.Errorf("Not able to create TLS certificate. Tenant domain not set yet")
	}

	var c cert.Certificate
	c.Name = r.getCertificateName(t)
	// TLS secret has to be in the same namespace as Istio ingress controller
	c.Namespace = "istio-system"

	err := r.client.Get(ctx, client.ObjectKeyFromObject(&c), &c)

	if err != nil && !k8serrors.IsNotFound(err) {
		setCondition(t, config.TenantConditionCertificate, core.ConditionFalse, fmt.Sprintf("Get certificate *.%s failed: %s", t.Status.TenantDomain, err))
		return errors.Wrapf(err, "get certificate %s", r.getCertificateName(t))
	}

	createCertificate := k8serrors.IsNotFound(err)

	controllerutil.SetControllerReference(t, &c, r.scheme)
	commonName := fmt.Sprintf("*.%s", tenantDomain)
	c.Spec.CommonName = &commonName
	cn := r.getCertificateName(t)
	c.Spec.SecretName = &cn

	if createCertificate {
		log.Printf("Creating certificate %s for tenant %s", c.Name, t.Name)
		err = r.client.Create(ctx, &c)
	} else {
		err = r.client.Update(ctx, &c)
	}
	if err != nil {
		setCondition(t, config.TenantConditionCertificate, core.ConditionFalse, fmt.Sprintf("Update certificate *.%s failed: %s", t.Status.TenantDomain, err))
		return errors.Wrapf(err, "update %s certificate", r.getCertificateName(t))
	}
	if c.Status.State == cert.StateReady {
		setCondition(t, config.TenantConditionCertificate, core.ConditionTrue, fmt.Sprintf("TLS Certificate for domain *.%s created", t.Status.TenantDomain))
	} else {
		setCondition(t, config.TenantConditionCertificate, core.ConditionFalse, fmt.Sprintf("TLS Certificate for domain *.%s created, but in state %q", t.Status.TenantDomain, c.Status.State))
	}
	return nil
}

func (r *Reconciler) ensureGateway(ctx context.Context, t *config.Tenant) error {
	tenantDomain := t.Status.TenantDomain
	if tenantDomain == "" {
		setCondition(t, config.TenantConditionGateway, core.ConditionFalse, "Tenant domain not set yet")
		return errors.Errorf("Not able to create Istio gateway. Tenant domain not set yet")
	}
	mainNamespace := coretools.TenantMainNamespace(t.Name)

	var g networking.Gateway
	g.Name = gatewayName
	g.Namespace = mainNamespace

	err := r.client.Get(ctx, client.ObjectKeyFromObject(&g), &g)

	if err != nil && !k8serrors.IsNotFound(err) {
		setCondition(t, config.TenantConditionGateway, core.ConditionFalse, fmt.Sprintf("Get Istio gateway %s failed: %s", gatewayName, err))
		return errors.Wrapf(err, "get Istio gateway %s", gatewayName)
	}

	createGateway := k8serrors.IsNotFound(err)

	controllerutil.SetControllerReference(t, &g, r.scheme)
	g.Spec.Selector = map[string]string{
		"app":   "istio-ingressgateway",
		"istio": "ingressgateway",
	}

	g.Spec.Servers = []*networkingapi.Server{
		{
			Hosts: []string{fmt.Sprintf("*.%s", tenantDomain)},
			Port: &networkingapi.Port{
				Name:     "https",
				Number:   443,
				Protocol: "HTTPS",
			},
			Tls: &networkingapi.ServerTLSSettings{
				CipherSuites: []string{
					"ECDHE-RSA-CHACHA20-POLY1305",
					"ECDHE-RSA-AES256-GCM-SHA384",
					"ECDHE-RSA-AES256-SHA",
					"ECDHE-RSA-AES128-GCM-SHA256",
					"ECDHE-RSA-AES128-SHA",
				},
				CredentialName:     r.getCertificateName(t),
				MinProtocolVersion: networkingapi.ServerTLSSettings_TLSV1_2,
				Mode:               networkingapi.ServerTLSSettings_SIMPLE,
			},
		},
		{
			Hosts: []string{fmt.Sprintf("*.%s", tenantDomain)},
			Port: &networkingapi.Port{
				Name:     "http",
				Number:   80,
				Protocol: "HTTP",
			},
			Tls: &networkingapi.ServerTLSSettings{
				HttpsRedirect: true,
			},
		},
	}

	if createGateway {
		log.Printf("Creating Istio gateway %s for tenant %s", g.Name, t.Name)
		err = r.client.Create(ctx, &g)
	} else {
		err = r.client.Update(ctx, &g)
	}
	if err != nil {
		setCondition(t, config.TenantConditionGateway, core.ConditionFalse, fmt.Sprintf("Update Istio gateway %s failed: %s", gatewayName, err))
		return errors.Wrapf(err, "update %s Istio gateway", gatewayName)
	}
	t.Status.Gateway = fmt.Sprintf("%s/%s", g.Namespace, g.Name)
	setCondition(t, config.TenantConditionGateway, core.ConditionTrue, fmt.Sprintf("Istio gateway for domain *.%s created", t.Status.TenantDomain))
	return nil
}

func (r *Reconciler) ensureRobotSetupConfig(ctx context.Context, t *config.Tenant) error {

	var templateConfigMap core.ConfigMap

	err := r.client.Get(ctx, client.ObjectKey{Namespace: coretools.RobotConfigCloudNamespace, Name: coretools.RobotSetupConfigmap}, &templateConfigMap)
	if err != nil {
		setCondition(t, config.TenantConditionRobotSetup, core.ConditionFalse, fmt.Sprintf("Get config map %s/%s failed: %s", coretools.RobotConfigCloudNamespace, coretools.RobotSetupConfigmap, err))
		return errors.Wrapf(err, "get %s/%s configmap", coretools.RobotConfigCloudNamespace, coretools.RobotSetupConfigmap)
	}

	namespace := coretools.RobotConfigNamespace(coretools.TenantMainNamespace(t.Name))
	var configMap core.ConfigMap
	configMap.Name = coretools.RobotSetupConfigmap
	configMap.Namespace = namespace
	err = r.client.Get(ctx, client.ObjectKeyFromObject(&configMap), &configMap)

	if err != nil && !k8serrors.IsNotFound(err) {
		setCondition(t, config.TenantConditionRobotSetup, core.ConditionFalse, fmt.Sprintf("Get config map %s/%s failed: %s", configMap.Name, configMap.Namespace, err))
		return errors.Wrapf(err, "get config map %s", configMap.Name)
	}

	createConfigMap := k8serrors.IsNotFound(err)

	controllerutil.SetControllerReference(t, &configMap, r.scheme)
	configMap.Data = templateConfigMap.Data
	configMap.Data["tenant"] = t.Name
	configMap.Data["tenant_domain"] = t.Status.TenantDomain
	configMap.Data["tenant_main_namespace"] = coretools.TenantMainNamespace(t.Name)

	if createConfigMap {
		log.Printf("Creating config map %s/%s for tenant %s", configMap.Namespace, configMap.Name, t.Name)
		err = r.client.Create(ctx, &configMap)
	} else {
		err = r.client.Update(ctx, &configMap)
	}
	if err != nil {
		setCondition(t, config.TenantConditionRobotSetup, core.ConditionFalse, fmt.Sprintf("Update config map %s/%s failed: %s", configMap.Name, configMap.Namespace, err))
		return fmt.Errorf("update ConfigMap \"%s:%s\" failed: %s", namespace, coretools.RobotSetupConfigmap, err)
	}
	setCondition(t, config.TenantConditionRobotSetup, core.ConditionTrue, "Robot-setup config synchronized")
	return nil
}

func (r *Reconciler) collectTenantNamespaces(ctx context.Context, t *config.Tenant) error {
	mainNamespace := coretools.TenantMainNamespace(t.Name)
	robotConfigNamespace := coretools.RobotConfigNamespace(mainNamespace)
	var namespaces core.NamespaceList
	err := r.client.List(ctx, &namespaces)
	if err != nil {
		return errors.Wrap(err, "list namespaces")
	}

	var appNamespace string
	if t.Name == coretools.DefaultTenantName {
		appNamespace = "app-"
	} else {
		appNamespace = fmt.Sprintf("%s-app-", mainNamespace)
	}

	tenantNamespaces := []string{}
	for _, ns := range namespaces.Items {
		if strings.HasPrefix(ns.Name, appNamespace) ||
			ns.Name == mainNamespace ||
			ns.Name == robotConfigNamespace {
			tenantNamespaces = append(tenantNamespaces, ns.Name)
			// Label all tenant namespaces accordingly
			setLabel(&ns.ObjectMeta, tenantLabel, t.Name)
		}
		err := r.client.Update(ctx, &ns)
		if err != nil {
			return errors.Wrap(err, "label namespace")
		}
	}

	t.Status.TenantNamespaces = tenantNamespaces

	return nil
}

func (r *Reconciler) collectRobots(ctx context.Context, t *config.Tenant) error {
	// Collect robots in tenant namespace
	var robotList registry.RobotList

	err := r.client.List(ctx,
		&robotList,
		client.InNamespace(coretools.TenantMainNamespace(t.Name)),
	)
	if err != nil {
		return errors.Wrap(err, "list robots")
	}

	t.Status.Robots = len(robotList.Items)

	// Collect robot clusters based on their service account secrets
	var secretList core.SecretList

	err = r.client.List(ctx,
		&secretList,
		client.InNamespace(coretools.RobotConfigNamespace(coretools.TenantMainNamespace(t.Name))),
	)
	if err != nil {
		return errors.Wrap(err, "list robot service account tokens")
	}

	var clusters int
	for _, s := range secretList.Items {
		if s.Annotations["kubernetes.io/service-account.name"] == coretools.RobotServiceAccount && strings.HasPrefix(s.Name, "robot-token-") {
			clusters += 1
		}
	}
	t.Status.RobotClusters = clusters

	return nil
}

func (r *Reconciler) ensureDeleted(ctx context.Context, t *config.Tenant) (bool, error) {
	mainNamespace := coretools.TenantMainNamespace(t.Name)

	// Ensure that all approllouts of tenant namespace are deleted
	var a apps.AppRolloutList

	err := r.client.List(ctx, &a, client.InNamespace(mainNamespace))
	if err != nil {
		return false, errors.Wrapf(err, "list AppRollouts of tenant %s", t.Name)
	}

	if len(a.Items) > 0 {
		log.Printf("Deleting %v AppRollouts in order to delete tenant %s", len(a.Items), t.Name)
		err := r.client.DeleteAllOf(ctx, &apps.AppRollout{}, client.InNamespace(mainNamespace))
		if err != nil {
			return false, errors.Wrapf(err, "delete AppRollouts of tenant %s", t.Name)
		}
		return false, nil
	}

	// Check if all chartassignments of tenant namespace are deleted
	var c apps.ChartAssignmentList

	err = r.client.List(ctx, &c, client.InNamespace(mainNamespace))
	if err != nil {
		return false, errors.Wrapf(err, "list ChartAssignments of tenant %s", t.Name)
	}
	if l := len(c.Items); l > 0 {
		log.Printf("Waiting for %v ChartAssignments to be deleted for tenant %s", len(c.Items), t.Name)
		return false, nil
	}

	return true, nil
}
