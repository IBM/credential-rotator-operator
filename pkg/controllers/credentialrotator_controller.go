/*
Copyright 2021.

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
	"strconv"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/IBM/platform-services-go-sdk/resourcecontrollerv2"

	securityv1alpha1 "github.com/IBM/credential-rotator-operator/api/v1alpha1"
	"github.com/IBM/credential-rotator-operator/pkg/ibmcloudclient"
)

// CredentialRotatorReconciler reconciles a CredentialRotator object
type CredentialRotatorReconciler struct {
	client.Client
	Recorder record.EventRecorder
	Scheme   *runtime.Scheme
}

//+kubebuilder:rbac:groups=security.example.com,resources=credentialrotators,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=security.example.com,resources=credentialrotators/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=security.example.com,resources=credentialrotators/finalizers,verbs=update

// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events/status,verbs=get;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// This implementation creates a new resource key in the IBM Cloud.
// This key contains the API key as part of the credentials for accessing
// a service, for example a Cloudant DB. The credentials are stored in a
// Kubernetes secret which an application uses to access the backend service.
// After creating the new key, the app instances will be restarted to load the
// new credentials. The previous resource key will then be deleted.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.2/pkg/reconcile
func (r *CredentialRotatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrllog.FromContext(ctx)
	logger.Info("== Reconciling CredentialRotator")

	// Fetch the CredentialRotator instance
	instance := &securityv1alpha1.CredentialRotator{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request - return and don't requeue:
			return r.doNotRequeue()
		}
		// Error reading the object - requeue the request
		return r.requeueOnErr(err)
	}

	// If no phase set, default to pending (the initial phase)
	if instance.Status.Phase == "" {
		instance.Status.Phase = securityv1alpha1.PhasePending
	}

	// The different operations are broken down into phases:
	// PENDING -> CREATING -> NOTIFYING -> DELETING -> DONE
	// A phase seperates operations as follows :
	// 1. Creating resource key and creating/updating secret to store credentials
	// 2. Notifying app PODs of change to credentials by restarting them
	// 3. Deleting previous resource key which is replaced with newly created key
	switch instance.Status.Phase {

	case securityv1alpha1.PhasePending:
		logger.Info("Phase: PENDING")
		r.Recorder.Event(instance, "Normal", "PhaseChange", securityv1alpha1.PhasePending)
		instance.Status.Phase = securityv1alpha1.PhaseCreating

	case securityv1alpha1.PhaseCreating:
		logger.Info("Phase: CREATING")
		r.Recorder.Event(instance, "Normal", "PhaseChange", securityv1alpha1.PhaseCreating)
		icClient, err := ibmcloudclient.NewClient(instance.Spec.UserAPIKey)
		if err != nil {
			logger.Error(err, "Resource key creation failure")
			// Error creating resource key. Wait until it is fixed.
			return r.requeueOnErr(err)
		}
		resourceKey, err := icClient.CreateResourceKeyForServiceInstance(instance.Spec.ServiceGUID)
		if err != nil {
			logger.Error(err, "Resource key creation failure")
			// Error creating resource key. Wait until it is fixed.
			return r.requeueOnErr(err)
		}
		logger.Info("Resource key created", "ID", resourceKey.GUID)

		// If secret exists, first get the resource key ID and set it
		// to 'instance.Status.PreviousResourceKeyID' so that the
		// previous key can be removed in 'DELETING' phase. Then
		// update the secret with new key created in 'CREATING' phase.
		// Otherwise create secret adding the resource key created.
		secret := newSecretObject(*resourceKey.ID, *resourceKey.Credentials.Apikey, instance.Spec.ServiceURL, instance.Spec.AppNameSpace)
		found := &corev1.Secret{}
		err = r.Get(ctx, types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, found)
		if err != nil && errors.IsNotFound(err) { //Create secret
			err = r.Create(ctx, secret)
			if err != nil {
				// Error creating secret. Wait until it is fixed.
				return r.requeueOnErr(err)
			}
			logger.Info("Secret created", "Name", "Namespace", secret.Name, secret.Namespace)
		} else if err != nil {
			// Error getting secret. Wait until it is fixed.
			return r.requeueOnErr(err)
		} else { //Update secret
			instance.Status.PreviousResourceKeyID = string(found.Data["resourceKeyID"])
			updateSecretObject(found, resourceKey)
			err = r.Update(ctx, found)
			if err != nil {
				// Error updating secret. Wait until it is fixed.
				return r.requeueOnErr(err)
			}
			logger.Info("Secret updated", "Name", "Namespace", secret.Name, secret.Namespace)
		}
		instance.Status.Phase = securityv1alpha1.PhaseNotifying

	case securityv1alpha1.PhaseNotifying:
		logger.Info("Phase: Notifying")
		r.Recorder.Event(instance, "Normal", "PhaseChange", securityv1alpha1.PhaseNotifying)

		found := &appsv1.Deployment{}
		err = r.Get(ctx, types.NamespacedName{Name: instance.Spec.AppName, Namespace: instance.Spec.AppNameSpace}, found)
		if err != nil && errors.IsNotFound(err) { //Create secret
			logger.Info("No Deployment found", "Deployment", instance.Spec.AppName,
				"Namespace", instance.Spec.AppNameSpace)
		} else if err != nil {
			return r.requeueOnErr(err)
		} else { //Restart
			patch := []byte(fmt.Sprintf(`{"spec":{"template":{"metadata":{"labels":{"credentials-rotator-redeloyed":"%v"}}}}}`, time.Now().Unix()))
			err = r.Patch(ctx, found, client.RawPatch(types.StrategicMergePatchType, patch))
			if err != nil {
				// Error updating deployment. Wait until it is fixed.
				return r.requeueOnErr(err)
			}
			logger.Info("Deployment updated and restarted", "Deployment", instance.Spec.AppName,
				"Namespace", instance.Spec.AppNameSpace)
		}
		instance.Status.Phase = securityv1alpha1.PhaseDeleting

	case securityv1alpha1.PhaseDeleting:
		logger.Info("Phase: Deleting")
		r.Recorder.Event(instance, "Normal", "PhaseChange", securityv1alpha1.PhaseDeleting)

		icClient, err := ibmcloudclient.NewClient(instance.Spec.UserAPIKey)
		if err != nil {
			logger.Error(err, "Resource key creation failure")
			// Error creating resource key. Wait until it is fixed.
			return r.requeueOnErr(err)
		}
		deleteResourceKeyID := instance.Status.PreviousResourceKeyID
		if deleteResourceKeyID != "" {
			err := icClient.DeleteResourceKey(deleteResourceKeyID)
			if err != nil {
				logger.Error(err, "Resource key deletion failure")
				// Error deleting resource key. Wait until it is fixed.
				return r.requeueOnErr(err)
			}
			logger.Info("Previous resource key deleted", "ID", deleteResourceKeyID)
		} else {
			logger.Info("No previous resource key to delete")
		}
		instance.Status.Phase = securityv1alpha1.PhaseDone

	case securityv1alpha1.PhaseDone:
		logger.Info("Phase: DONE")
		r.Recorder.Event(instance, "Normal", "PhaseChange", securityv1alpha1.PhaseDone)
		return r.doNotRequeue()

	default:
		logger.Info("NOP")
		return r.doNotRequeue()
	}

	// Update the CredentialRotator instance, setting the status to the respective phase
	err = r.Status().Update(ctx, instance)
	if err != nil {
		return r.requeueOnErr(err)
	}

	return r.requeue()
}

// SetupWithManager sets up the controller with the Manager.
func (r *CredentialRotatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&securityv1alpha1.CredentialRotator{}).
		Complete(r)
}

// newSecretObject constructs a kubernetes Secret object
// to store credemtial information for accessing a service. The
// secret data entry contains the credentials.
//
// The following labels are used within each secret:
//
//    "owner"          - owner of the secret, currently "credential-rotator-controller".
//    "name"           - name of the release, currently "cloudant".
//
func newSecretObject(resourceKeyID, resourceKeyAPIKey, serviceURL, namespace string) *corev1.Secret {
	// apply labels
	var lbs = make(map[string]string)
	lbs["name"] = "cloudant"
	lbs["owner"] = "credential-rotator-controller"

	var immutable bool = false

	// create and return secret object.
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cloudant",
			Namespace: namespace,
			Labels:    lbs,
		},
		Type: "example/credential-rotator-controller",
		Data: map[string][]byte{"url": []byte(serviceURL), "iamApiKey": []byte(resourceKeyAPIKey),
			"resourceKeyID": []byte(resourceKeyID)},
		Immutable: &immutable,
	}
}

// updateSecretObject updates Kubernetes secret object with new
// credential information
func updateSecretObject(secret *corev1.Secret, resourceKey *resourcecontrollerv2.ResourceKey) {
	secret.Data["iamApiKey"] = []byte(*resourceKey.Credentials.Apikey)
	secret.Data["resourceKeyID"] = []byte(*resourceKey.ID)
	secret.ObjectMeta.Labels["modifiedAt"] = strconv.Itoa(int(time.Now().Unix()))
}

// doNotRequeue Finished processing. No need to put back on the reconcile queue.
func (r *CredentialRotatorReconciler) doNotRequeue() (reconcile.Result, error) {
	return ctrl.Result{}, nil
}

// requeue Not finished processing. Put back on reconcile queue and continue.
func (r *CredentialRotatorReconciler) requeue() (reconcile.Result, error) {
	return ctrl.Result{Requeue: true}, nil
}

// requeueOnErr Failed while processing. Put back on reconcile queue and try again.
func (r *CredentialRotatorReconciler) requeueOnErr(err error) (reconcile.Result, error) {
	return ctrl.Result{}, err
}
