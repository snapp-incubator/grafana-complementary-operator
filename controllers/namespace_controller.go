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

	grafanav1alpha1 "github.com/grafana-operator/grafana-operator/v4/api/integreatly/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// NamespaceReconciler reconciles a Namespace object
type NamespaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=namespaces/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core,resources=namespaces/finalizers,verbs=update

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

	// getting serviceAccount
	sa := &corev1.ServiceAccount{}
	err := r.Get(ctx, types.NamespacedName{Name: "monitoring-datasource", Namespace: req.Name}, sa)
	if err != nil {
		logger.Error(err, "unable to get ServiceAccount")
		return ctrl.Result{}, err
	}

	// getting secret
	secretName := sa.Secrets[0].Name
	secret := &corev1.Secret{}
	err = r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: req.Name}, secret)
	if err != nil {
		logger.Error(err, "unable to get Secret")
		return ctrl.Result{}, err
	}

	b64token := string(secret.Data["token"])

	//decode b64
	// token = b64dec(b64token)
	grafanaDatasource := &grafanav1alpha1.GrafanaDataSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: "snappcloud-monitoring",
		},
		Spec: grafanav1alpha1.GrafanaDataSourceSpec{
			Name: req.Name + ".yaml",
			Datasources: []grafanav1alpha1.GrafanaDataSourceFields{{
				Access:    "proxy",
				Editable:  false,
				IsDefault: false,
				Name:      req.Name,
				OrgId:     1,
				Type:      "prometheus",
				Url:       "https://thanos-querier-custom.openshift-monitoring.svc.cluster.local:9092",
				Version:   1,
				JsonData: grafanav1alpha1.GrafanaDataSourceJsonData{
					HTTPMethod:      "POST",
					TlsSkipVerify:   true,
					HTTPHeaderName1: "Authorization",
					HTTPHeaderName2: "namespace",
				},
				SecureJsonData: grafanav1alpha1.GrafanaDataSourceSecureJsonData{
					HTTPHeaderValue1: "Bearer " + b64token,
					HTTPHeaderValue2: req.Name,
				},
			}},
		},
	}
	err = r.Create(ctx, grafanaDatasource)
	if err != nil {
		logger.Error(err, "unable to create GrafanaDataSource")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		Complete(r)
}
