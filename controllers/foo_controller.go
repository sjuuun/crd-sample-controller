/*

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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	samplecontrollerv1alpha1 "sample/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// FooReconciler reconciles a Foo object
type FooReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=samplecontroller.k8s.io,resources=foos,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=samplecontroller.k8s.io,resources=foos/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=samplecontroller.k8s.io,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=samplecontroller.k8s.io,resources=deployments/status,verbs=get;update;patch

func (r *FooReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("foo", req.NamespacedName)

	// your logic here

	// Get the Foo resource with namespace/name
	var foo samplecontrollerv1alpha1.Foo
	if err := r.Get(ctx, req.NamespacedName, &foo); err != nil {
		log.Error(err, "unable to fetch Foo")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, err
	}

	// DeploymentName is needed
	deploymentName := foo.Spec.DeploymentName
	if deploymentName == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		log.V(1).Info("deployment name must be specified", "foo")
		return ctrl.Result{}, nil
	}

	// Function for making deployments
	constructDeployforFoo := func(foo *samplecontrollerv1alpha1.Foo) (*appsv1.Deployment, error) {
		name := foo.Spec.DeploymentName
		labels := map[string]string{
			"app":        "nginx",
			"controller": foo.Name,
		}
		groupVersion := schema.GroupVersionKind{
			Group:   "samplecontroller",
			Version: "v1alpha1",
			Kind:    "Foo",
		}

		dep := appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: foo.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(foo, groupVersion),
				},
				Labels: map[string]string{
					"app": "foo",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: labels,
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: labels,
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							corev1.Container{
								Image: "nginx:latest",
								Name:  "nginx",
							},
						},
					},
				},
			},
		}

		return &dep, nil
	}

	// Get Deployment with Namespace and DeploymentName
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: foo.Namespace,
		Name:      foo.Spec.DeploymentName,
	}, &appsv1.Deployment{}); err != nil {
		log.Error(err, "There is no such deployment")

		// create deployment here
		dep, err := constructDeployforFoo(&foo)
		if err != nil {
			log.Error(err, "unable to construct deployment from template")
			// don't bother requeuing until we get a change to the spec
			return ctrl.Result{}, nil
		}

		// ...and create it on the cluster
		if err := r.Create(ctx, dep); err != nil {
			log.Error(err, "unable to create Deployment for Foo", "Deployment", dep)
			return ctrl.Result{}, err
		}

		log.V(1).Info("created Deployment for Foo run", "Deployment", dep)
	}

	//if foo.Spec.Replicas != nil && *foo.Spec.Replicas != *deployment.Spec.Replicas {
	//	log.V(4).Infof("Foo %s replicas: %d, deployment replicas: %d", name, *foo.Spec.Replicas, *deployment.Spec.Replicas)
	//}

	return ctrl.Result{}, nil
}

func (r *FooReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&samplecontrollerv1alpha1.Foo{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
