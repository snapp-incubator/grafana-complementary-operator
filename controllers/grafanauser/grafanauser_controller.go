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

package grafanauser

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/grafana-tools/sdk"
	grafanauserv1alpha1 "github.com/snapp-cab/grafana-complementary-operator/apis/grafana/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

const (
	teamLabel = "snappcloud.io/team"
)

// Get Grafana URL and PassWord as a env.
var grafanaPassword = "xAR6WJKrszFBJsnlHCdoeuA2w2Q10y9E7iJ3J46l3Vpk1yigQl"
var grafanaUsername = "admin"
var grafanaURL = "https://grafana.okd4.teh-1.snappcloud.io"

// GrafanaReconciler reconciles a Grafana object
type GrafanaUserReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=grafana.snappcloud.io,resources=grafanausers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=grafana.snappcloud.io,resources=grafanausers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=grafana.snappcloud.io,resources=grafanausers/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=user.openshift.io,resources=*,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Grafana object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.9.2/pkg/reconcile
func (r *GrafanaUserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	reqLogger := log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	// Getting namespace
	ns := &corev1.Namespace{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: req.Namespace}, ns)
	if err != nil {
		log.Error(err, "Failed to get namespace")
		return ctrl.Result{}, err
	}
	// Ignore namespaces which does not have team label
	org, ok := ns.Labels[teamLabel]
	if !ok {
		reqLogger.Info("Namespace does not have team label. Ignoring", "namespace", ns.Name, "team name ", org)
		return ctrl.Result{}, nil
	}
	//Connecting to the Grafana API
	grafanaclient, err := sdk.NewClient(grafanaURL, fmt.Sprintf("%s:%s", grafanaUsername, grafanaPassword), sdk.DefaultHTTPClient)
	if err != nil {
		reqLogger.Error(err, "Unable to create Grafana client")
		return ctrl.Result{}, err
	}
	//Retrieving the Organization Info
	retrievedOrg, err := grafanaclient.GetOrgByOrgName(ctx, org)
	if err != nil {
		if strings.Contains(err.Error(), "Organization not found") {
			reqLogger.Error(err, "Unable to get organization")
			return ctrl.Result{}, err
		}
	}
	reqLogger.Info("Reconciling grafana")
	grafana := &grafanauserv1alpha1.GrafanaUser{}
	err = r.Client.Get(context.TODO(), req.NamespacedName, grafana)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	} else {
		log.Info("grafana_org is found and orgName is : " + org)

	}

	_, err = r.AddUsersToGrafanaOrgByEmail(ctx, req, org, grafanaclient, retrievedOrg, grafana.Spec.Admin, "admin")
	if err != nil {
		return ctrl.Result{}, err
	}

	_, err = r.AddUsersToGrafanaOrgByEmail(ctx, req, org, grafanaclient, retrievedOrg, grafana.Spec.Edit, "editor")
	if err != nil {
		return ctrl.Result{}, err
	}
	_, err = r.AddUsersToGrafanaOrgByEmail(ctx, req, org, grafanaclient, retrievedOrg, grafana.Spec.View, "viewer")
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}
func (r *GrafanaUserReconciler) AddUsersToGrafanaOrgByEmail(ctx context.Context, req ctrl.Request, org string, client *sdk.Client, retrievedOrg sdk.Org, emails []string, role string) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	reqLogger := log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	orgID := retrievedOrg.ID
	orgName := retrievedOrg.Name
	getallUser, _ := client.GetAllUsers(ctx)
	getuserOrg, _ := client.GetOrgUsers(ctx, orgID)
	for _, email := range emails {
		var orguserfound bool
		for _, orguser := range getuserOrg {
			UserOrg := orguser.Email
			if email == UserOrg {
				UserRole := orguser.Role
				UserID := orguser.ID
				orguserfound = true
				if role != UserRole {
					reqLogger.Info(email)
					changerole := sdk.UserRole{LoginOrEmail: email, Role: role}
					_, err := client.UpdateOrgUser(ctx, changerole, UserID, orgID)
					if err != nil {
						return ctrl.Result{}, err
					} else {
						reqLogger.Info(orguser.Email, "is already in", orgName, "but the user role change to", role)
						reqLogger.Info(role)
						s1 := strconv.FormatInt(int64(UserID), 10)
						reqLogger.Info(s1)
					}
				} else {
					reqLogger.Info(orguser.Email, "is already in", orgName, "and the role hasn't changed ")
				}
				break
			}
		}
		if orguserfound {
			continue
		}
		for _, user := range getallUser {
			UserEmail := user.Email
			if email == UserEmail {
				newuser := sdk.UserRole{LoginOrEmail: email, Role: role}
				_, err := client.AddOrgUser(ctx, newuser, orgID)
				if err != nil {
					return ctrl.Result{}, err
				} else {
					log.Info(UserEmail, "is added to", orgName)
				}
				break
			}
		}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GrafanaUserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&grafanauserv1alpha1.GrafanaUser{}).
		Complete(r)
}
