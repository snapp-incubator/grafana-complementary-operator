/*
Copyright 2022.

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

package controllers

import (
	"context"
	"fmt"
	"os"
	"reflect"

	grafanav1alpha1 "github.com/grafana-operator/grafana-operator/v4/api/integreatly/v1alpha1"
	"github.com/grafana-tools/sdk"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	baseNs            = "snappcloud-monitoring"
	baseSa            = "monitoring-datasource"
	prometheusURL     = "https://thanos-querier-custom.openshift-monitoring.svc.cluster.local:9092"
	nsMonitoringLabel = "monitoring.snappcloud.io/grafana-datasource"
	teamLabel         = "snappcloud.io/team"
	// grafanaURL        = "https://grafana.snappgroup.teh-1.snappcloud.io"
	// grafanaUsername = "admin"
	// grafanaPassword   = "xAR6WJKrszFBJsnlHCdoeuA2w2Q10y9E7iJ3J46l3Vpk1yigQl"
)

// Get Grafana URL and PassWord as a env.
var grafanaPassword = os.Getenv("GRAFANA_PASSWORD")
var grafanaUsername = os.Getenv("GRAFANA_USERNAME")
var grafanaURL = os.Getenv("GRAFANA_URL")

// NamespaceReconciler reconciles a Namespace object
type NamespaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=namespaces/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core,resources=namespaces/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

//+kubebuilder:rbac:groups=integreatly.org,resources=grafanadatasources,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Namespace object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *NamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	ns := &corev1.Namespace{}
	err := r.Get(ctx, req.NamespacedName, ns)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			logger.Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		logger.Error(err, "Failed to get Namespace")
		return ctrl.Result{}, err
	}

	// Ignore namespaces which does not have special label
	if _, ok := ns.Labels[nsMonitoringLabel]; !ok {
		logger.Info("Namespace does not have monitoring label. Ignoring", "namespace", ns.Name)
		return ctrl.Result{}, nil
	}

	// Ignore namespaces which does not have team label
	team, ok := ns.Labels[teamLabel]
	if !ok {
		logger.Info("Namespace does not have team label. Ignoring", "namespace", ns.Name, "team name ", team)
		return ctrl.Result{}, nil
	}

	logger.Info("Reconciling Namespace", "Namespace.Name", req.NamespacedName, "Team", team)

	// Getting serviceAccount
	logger.Info("Getting serviceAccount", "serviceAccount.Name", baseSa, "Namespace.Name", req.NamespacedName)
	sa := &corev1.ServiceAccount{}
	err = r.Get(ctx, types.NamespacedName{Name: baseSa, Namespace: req.Name}, sa)
	if err != nil {
		logger.Error(err, "Unable to get ServiceAccount")
		return ctrl.Result{}, err
	}

	// Getting serviceaccount token
	secret := &corev1.Secret{}
	var token string
	for _, ref := range sa.Secrets {

		logger.Info("Getting secret", "secret.Name", ref.Name, "Namespace.Name", req.Name)
		// get secret
		err = r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: req.Name}, secret)
		if err != nil {
			logger.Error(err, "Unable to get Secret")
			return ctrl.Result{}, err
		}

		// Check if secret is a token for the serviceaccount
		if secret.Type != corev1.SecretTypeServiceAccountToken {
			continue
		}
		name := secret.Annotations[corev1.ServiceAccountNameKey]
		uid := secret.Annotations[corev1.ServiceAccountUIDKey]
		tokenData := secret.Data[corev1.ServiceAccountTokenKey]
		//tmp
		logger.Info("Token data", "token", string(tokenData))
		logger.Info("Token meta", "name", name, "uid", uid, "ref.Name", ref.Name, "ref.UID", ref.UID)
		if name == sa.Name && uid == string(sa.UID) && len(tokenData) > 0 {
			// found token, the first token found is used
			token = string(tokenData)
			logger.Info("Found token", "token", token)
			break
		}

	}
	// if no token found
	if token == "" {
		logger.Error(fmt.Errorf("did not found service account token for service account %q", sa.Name), "")
		return ctrl.Result{}, err
	}
	gfDs, err := r.generateGfDataSource(ctx, req.Name, team, token, ns)
	if err != nil {
		logger.Error(err, "Error generating grafanaDatasource manifest")
		return ctrl.Result{}, err
	}

	// Check if grafanaDatasource does not exist and create a new one
	found := &grafanav1alpha1.GrafanaDataSource{}
	err = r.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: baseNs}, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating grafana datasource", "grafanaDatasource.Name", gfDs.Name)
		err = r.Create(ctx, gfDs)
		if err != nil {
			logger.Error(err, "Unable to create GrafanaDataSource")
			return ctrl.Result{}, err
		}
		// grafanaDatasource created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		logger.Error(err, "Failed to get grafanaDatasource")
		return ctrl.Result{}, err
	}

	// If GrafanaDatasource already exist, check if it is deeply equal with desrired state
	if !reflect.DeepEqual(gfDs.Spec, found.Spec) {
		logger.Info("Updating grafanaDatasource", "grafanaDatasource.Namespace", found.Namespace, "grafanaDatasource.Name", found.Name)
		found.Spec = gfDs.Spec
		err := r.Update(ctx, found)
		if err != nil {
			logger.Error(err, "Failed to update grafanaDatasource", "grafanaDatasource.Namespace", found.Namespace, "grafanaDatasource.Name", found.Name)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		Owns(&grafanav1alpha1.GrafanaDataSource{}).
		Complete(r)
}

func (r *NamespaceReconciler) generateGfDataSource(ctx context.Context, name, team, token string, nsOwner *corev1.Namespace) (*grafanav1alpha1.GrafanaDataSource, error) {
	logger := log.FromContext(ctx)

	// Connecting to the Grafana API
	client, _ := sdk.NewClient(grafanaURL, fmt.Sprintf("%s:%s", grafanaUsername, grafanaPassword), sdk.DefaultHTTPClient)

	// Retrieving the Organization Info
	retrievedOrg, err := client.GetOrgByOrgName(ctx, team)
	if err != nil {
		logger.Error(err, "Failed to retrieve the organization", "Team name:", team, "Namespace", name)
	}
	if retrievedOrg.Name != team {
		logger.Error(err, "Got wrong org:", "got", retrievedOrg.Name, "expected", team)
	}
	// Generating the datasource
	logger.Info("Start creating Datasource", "Team name:", team, "Team ID:", retrievedOrg.ID, "Namespace", name)
	grafanaDatasource := &grafanav1alpha1.GrafanaDataSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: baseNs,
		},
		Spec: grafanav1alpha1.GrafanaDataSourceSpec{
			Name: name + ".yaml",
			Datasources: []grafanav1alpha1.GrafanaDataSourceFields{{
				Access:    "proxy",
				Editable:  false,
				IsDefault: false,
				Name:      name,
				OrgId:     int(retrievedOrg.ID),
				Type:      "prometheus",
				Url:       prometheusURL,
				Version:   1,
				JsonData: grafanav1alpha1.GrafanaDataSourceJsonData{
					HTTPMethod:      "POST",
					TlsSkipVerify:   true,
					HTTPHeaderName1: "Authorization",
					HTTPHeaderName2: "namespace",
				},
				SecureJsonData: grafanav1alpha1.GrafanaDataSourceSecureJsonData{
					HTTPHeaderValue1: "Bearer " + token,
					HTTPHeaderValue2: name,
				},
			}},
		},
	}

	// Set target Namespace as the owner of GrafanaDatasource in another Namespace
	err = ctrl.SetControllerReference(nsOwner, grafanaDatasource, r.Scheme)
	if err != nil {
		return nil, err
	}

	return grafanaDatasource, nil
}
