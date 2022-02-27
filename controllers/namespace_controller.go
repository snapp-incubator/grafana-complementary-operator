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

	grafanav1alpha1 "github.com/grafana-operator/grafana-operator/v4/api/integreatly/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	baseNs        = "snappcloud-monitoring"
	baseSa        = "monitoring-datasource"
	prometheusURL = "https://thanos-querier-custom.openshift-monitoring.svc.cluster.local:9092"
)

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

	// getting serviceAccount
	sa := &corev1.ServiceAccount{}
	err := r.Get(ctx, types.NamespacedName{Name: baseSa, Namespace: req.Name}, sa)
	if err != nil {
		logger.Error(err, "unable to get ServiceAccount")
		return ctrl.Result{}, err
	}

	// getting serviceaccount token
	secret := &corev1.Secret{}
	var token string
	for _, ref := range sa.Secrets {
		// get secret
		err = r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: req.Name}, secret)
		if err != nil {
			logger.Error(err, "unable to get Secret")
			return ctrl.Result{}, err
		}

		// check if secret is a token for the serviceaccount
		if secret.Type != corev1.SecretTypeServiceAccountToken {
			continue
		}
		name := secret.Annotations[corev1.ServiceAccountNameKey]
		uid := secret.Annotations[corev1.ServiceAccountUIDKey]
		tokenData := secret.Data[corev1.ServiceAccountTokenKey]
		if name == ref.Name && uid == string(ref.UID) && len(tokenData) > 0 {
			// found token, the first token found is used
			token = string(tokenData)
			break
		}
	}
	// if no token found
	if token == "" {
		logger.Error(fmt.Errorf("did not found service account token for service account %q", sa.Name), "")
		return ctrl.Result{}, err
	}

	grafanaDatasource := &grafanav1alpha1.GrafanaDataSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: baseNs,
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
