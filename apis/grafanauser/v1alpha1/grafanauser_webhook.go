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

package v1alpha1

import (
	"context"
	"fmt"

	"github.com/grafana-tools/sdk"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var grafanauserlog = logf.Log.WithName("grafanauser-resource")

// Get Grafana URL and PassWord as a env.
var grafanaPassword = "xAR6WJKrszFBJsnlHCdoeuA2w2Q10y9E7iJ3J46l3Vpk1yigQl"
var grafanaUsername = "admin"
var grafanaURL = "https://grafana.okd4.teh-1.snappcloud.io"

// Get Grafana URL and PassWord as a env.

func (r *GrafanaUser) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-grafanauser-snappcloud-io-v1alpha1-grafanauser,mutating=true,failurePolicy=fail,sideEffects=None,groups=grafanauser.snappcloud.io,resources=grafanausers,verbs=create;update,versions=v1alpha1,name=mgrafanauser.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Defaulter = &GrafanaUser{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *GrafanaUser) Default() {
	grafanauserlog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-grafanauser-snappcloud-io-v1alpha1-grafanauser,mutating=false,failurePolicy=fail,sideEffects=None,groups=grafanauser.snappcloud.io,resources=grafanausers,verbs=create;update,versions=v1alpha1,name=vgrafanauser.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &GrafanaUser{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *GrafanaUser) ValidateCreate() error {
	grafanauserlog.Info("validate create", "name", r.Name)
	// TODO(user): fill in your validation logic upon object creation.
	var emaillist []string
	emaillist = append(r.Spec.Admin, r.Spec.Edit...)
	emaillist = append(emaillist, r.Spec.View...)
	err := r.ValidateEmailExist(context.Background(), emaillist)
	if err != nil {
		return err
	}
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *GrafanaUser) ValidateUpdate(old runtime.Object) error {
	grafanauserlog.Info("validate update", "name", r.Name)
	var emaillist []string
	emaillist = append(r.Spec.Admin, r.Spec.Edit...)
	emaillist = append(emaillist, r.Spec.View...)
	err := r.ValidateEmailExist(context.Background(), emaillist)
	if err != nil {
		return err
	}
	// TODO(user): fill in your validation logic upon object update.
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *GrafanaUser) ValidateDelete() error {
	grafanauserlog.Info("validate delete", "name", r.Name)
	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}

func (r *GrafanaUser) ValidateEmailExist(ctx context.Context, emails []string) error {
	client, _ := sdk.NewClient(grafanaURL, fmt.Sprintf("%s:%s", grafanaUsername, grafanaPassword), sdk.DefaultHTTPClient)
	grafanalUsers, _ := client.GetAllUsers(ctx)
	var Users []string
	for _, email := range emails {
		for _, grafanauser := range grafanalUsers {
			Grafanau := grafanauser.Email
			if email == Grafanau {
				continue
			} else {
				Users = append(Users, Grafanau)

			}

		}
	}
	if len(Users) == 0 {
		return fmt.Errorf("please make sure all of the user are login")

	}
	return nil
}
