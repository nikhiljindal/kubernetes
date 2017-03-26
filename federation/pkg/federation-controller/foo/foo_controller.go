/*
Copyright 2016 The Kubernetes Authors.

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

package foo

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pkgruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	clientv1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/flowcontrol"
	federationapi "k8s.io/kubernetes/federation/apis/federation/v1beta1"
	federationclientset "k8s.io/kubernetes/federation/client/clientset_generated/federation_clientset"
	"k8s.io/kubernetes/federation/pkg/federation-controller/util"
	"k8s.io/kubernetes/federation/pkg/federation-controller/util/deletionhelper"
	"k8s.io/kubernetes/federation/pkg/federation-controller/util/eventsink"
	"k8s.io/kubernetes/pkg/api"
	apiv1 "k8s.io/kubernetes/pkg/api/v1"
	kubeclientset "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	"k8s.io/kubernetes/pkg/controller"

	"github.com/golang/glog"
)

const (
	allClustersKey = "ALL_CLUSTERS"
	ControllerName = "foos"
)

var (
	RequiredResources = []schema.GroupVersionResource{apiv1.SchemeGroupVersion.WithResource("foos")}
)

type FooController struct {
	// For triggering single foo reconciliation. This is used when there is an
	// add/update/delete operation on a foo in either federated API server or
	// in some member of the federation.
	fooDeliverer *util.DelayingDeliverer

	// For triggering all foos reconciliation. This is used when
	// a new cluster becomes available.
	clusterDeliverer *util.DelayingDeliverer

	// Contains foos present in members of federation.
	fooFederatedInformer util.FederatedInformer
	// For updating members of federation.
	federatedUpdater util.FederatedUpdater
	// Definitions of foos that should be federated.
	fooInformerStore cache.Store
	// Informer controller for foos that should be federated.
	fooInformerController cache.Controller

	// Client to federated api server.
	federatedApiClient federationclientset.Interface

	// Backoff manager for foos
	fooBackoff *flowcontrol.Backoff

	// For events
	eventRecorder record.EventRecorder

	// Finalizers
	deletionHelper *deletionhelper.DeletionHelper

	fooReviewDelay        time.Duration
	clusterAvailableDelay time.Duration
	smallDelay            time.Duration
	updateTimeout         time.Duration
}

// NewFooController returns a new foo controller
func NewFooController(client federationclientset.Interface) *FooController {
	broadcaster := record.NewBroadcaster()
	broadcaster.StartRecordingToSink(eventsink.NewFederatedEventSink(client))
	recorder := broadcaster.NewRecorder(api.Scheme, clientv1.EventSource{Component: "federated-foos-controller"})

	foocontroller := &FooController{
		federatedApiClient:    client,
		fooReviewDelay:        time.Second * 10,
		clusterAvailableDelay: time.Second * 20,
		smallDelay:            time.Second * 3,
		updateTimeout:         time.Second * 30,
		fooBackoff:            flowcontrol.NewBackOff(5*time.Second, time.Minute),
		eventRecorder:         recorder,
	}

	// Build delivereres for triggering reconciliations.
	foocontroller.fooDeliverer = util.NewDelayingDeliverer()
	foocontroller.clusterDeliverer = util.NewDelayingDeliverer()

	// Start informer on federated API servers on foos that should be federated.
	foocontroller.fooInformerStore, foocontroller.fooInformerController = cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (pkgruntime.Object, error) {
				return client.Core().Foos(metav1.NamespaceAll).List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return client.Core().Foos(metav1.NamespaceAll).Watch(options)
			},
		},
		&apiv1.Foo{},
		controller.NoResyncPeriodFunc(),
		util.NewTriggerOnAllChanges(func(obj pkgruntime.Object) { foocontroller.deliverFooObj(obj, 0, false) }))

	// Federated informer on foos in members of federation.
	foocontroller.fooFederatedInformer = util.NewFederatedInformer(
		client,
		func(cluster *federationapi.Cluster, targetClient kubeclientset.Interface) (cache.Store, cache.Controller) {
			return cache.NewInformer(
				&cache.ListWatch{
					ListFunc: func(options metav1.ListOptions) (pkgruntime.Object, error) {
						return targetClient.Core().Foos(metav1.NamespaceAll).List(options)
					},
					WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
						return targetClient.Core().Foos(metav1.NamespaceAll).Watch(options)
					},
				},
				&apiv1.Foo{},
				controller.NoResyncPeriodFunc(),
				// Trigger reconciliation whenever something in federated cluster is changed. In most cases it
				// would be just confirmation that some foo operation succeeded.
				util.NewTriggerOnAllChanges(
					func(obj pkgruntime.Object) {
						foocontroller.deliverFooObj(obj, foocontroller.fooReviewDelay, false)
					},
				))
		},

		&util.ClusterLifecycleHandlerFuncs{
			ClusterAvailable: func(cluster *federationapi.Cluster) {
				// When new cluster becomes available process all the foos again.
				foocontroller.clusterDeliverer.DeliverAt(allClustersKey, nil, time.Now().Add(foocontroller.clusterAvailableDelay))
			},
		},
	)

	// Federated updater along with Create/Update/Delete operations.
	foocontroller.federatedUpdater = util.NewFederatedUpdater(foocontroller.fooFederatedInformer,
		func(client kubeclientset.Interface, obj pkgruntime.Object) error {
			foo := obj.(*apiv1.Foo)
			_, err := client.Core().Foos(foo.Namespace).Create(foo)
			return err
		},
		func(client kubeclientset.Interface, obj pkgruntime.Object) error {
			foo := obj.(*apiv1.Foo)
			_, err := client.Core().Foos(foo.Namespace).Update(foo)
			return err
		},
		func(client kubeclientset.Interface, obj pkgruntime.Object) error {
			foo := obj.(*apiv1.Foo)
			err := client.Core().Foos(foo.Namespace).Delete(foo.Name, &metav1.DeleteOptions{})
			return err
		})

	foocontroller.deletionHelper = deletionhelper.NewDeletionHelper(
		foocontroller.hasFinalizerFunc,
		foocontroller.removeFinalizerFunc,
		foocontroller.addFinalizerFunc,
		// objNameFunc
		func(obj pkgruntime.Object) string {
			foo := obj.(*apiv1.Foo)
			return foo.Name
		},
		foocontroller.updateTimeout,
		foocontroller.eventRecorder,
		foocontroller.fooFederatedInformer,
		foocontroller.federatedUpdater,
	)

	return foocontroller
}

// hasFinalizerFunc returns true if the given object has the given finalizer in its ObjectMeta.
func (foocontroller *FooController) hasFinalizerFunc(obj pkgruntime.Object, finalizer string) bool {
	foo := obj.(*apiv1.Foo)
	for i := range foo.ObjectMeta.Finalizers {
		if string(foo.ObjectMeta.Finalizers[i]) == finalizer {
			return true
		}
	}
	return false
}

// removeFinalizerFunc removes the finalizer from the given objects ObjectMeta. Assumes that the given object is a foo.
func (foocontroller *FooController) removeFinalizerFunc(obj pkgruntime.Object, finalizer string) (pkgruntime.Object, error) {
	foo := obj.(*apiv1.Foo)
	newFinalizers := []string{}
	hasFinalizer := false
	for i := range foo.ObjectMeta.Finalizers {
		if string(foo.ObjectMeta.Finalizers[i]) != finalizer {
			newFinalizers = append(newFinalizers, foo.ObjectMeta.Finalizers[i])
		} else {
			hasFinalizer = true
		}
	}
	if !hasFinalizer {
		// Nothing to do.
		return obj, nil
	}
	foo.ObjectMeta.Finalizers = newFinalizers
	foo, err := foocontroller.federatedApiClient.Core().Foos(foo.Namespace).Update(foo)
	if err != nil {
		return nil, fmt.Errorf("failed to remove finalizer %s from foo %s: %v", finalizer, foo.Name, err)
	}
	return foo, nil
}

// addFinalizerFunc adds the given finalizer to the given objects ObjectMeta. Assumes that the given object is a foo.
func (foocontroller *FooController) addFinalizerFunc(obj pkgruntime.Object, finalizers []string) (pkgruntime.Object, error) {
	foo := obj.(*apiv1.Foo)
	foo.ObjectMeta.Finalizers = append(foo.ObjectMeta.Finalizers, finalizers...)
	foo, err := foocontroller.federatedApiClient.Core().Foos(foo.Namespace).Update(foo)
	if err != nil {
		return nil, fmt.Errorf("failed to add finalizers %v to foo %s: %v", finalizers, foo.Name, err)
	}
	return foo, nil
}

func (foocontroller *FooController) Run(stopChan <-chan struct{}) {
	go foocontroller.fooInformerController.Run(stopChan)
	foocontroller.fooFederatedInformer.Start()
	go func() {
		<-stopChan
		foocontroller.fooFederatedInformer.Stop()
	}()
	foocontroller.fooDeliverer.StartWithHandler(func(item *util.DelayingDelivererItem) {
		foo := item.Value.(*types.NamespacedName)
		foocontroller.reconcileFoo(*foo)
	})
	foocontroller.clusterDeliverer.StartWithHandler(func(_ *util.DelayingDelivererItem) {
		foocontroller.reconcileFoosOnClusterChange()
	})
	util.StartBackoffGC(foocontroller.fooBackoff, stopChan)
}

func (foocontroller *FooController) deliverFooObj(obj interface{}, delay time.Duration, failed bool) {
	foo := obj.(*apiv1.Foo)
	foocontroller.deliverFoo(types.NamespacedName{Namespace: foo.Namespace, Name: foo.Name}, delay, failed)
}

// Adds backoff to delay if this delivery is related to some failure. Resets backoff if there was no failure.
func (foocontroller *FooController) deliverFoo(foo types.NamespacedName, delay time.Duration, failed bool) {
	key := foo.String()
	if failed {
		foocontroller.fooBackoff.Next(key, time.Now())
		delay = delay + foocontroller.fooBackoff.Get(key)
	} else {
		foocontroller.fooBackoff.Reset(key)
	}
	foocontroller.fooDeliverer.DeliverAfter(key, &foo, delay)
}

// Check whether all data stores are in sync. False is returned if any of the informer/stores is not yet
// synced with the corresponding api server.
func (foocontroller *FooController) isSynced() bool {
	if !foocontroller.fooFederatedInformer.ClustersSynced() {
		glog.V(2).Infof("Cluster list not synced")
		return false
	}
	clusters, err := foocontroller.fooFederatedInformer.GetReadyClusters()
	if err != nil {
		glog.Errorf("Failed to get ready clusters: %v", err)
		return false
	}
	if !foocontroller.fooFederatedInformer.GetTargetStore().ClustersSynced(clusters) {
		return false
	}
	return true
}

// The function triggers reconciliation of all federated foos.
func (foocontroller *FooController) reconcileFoosOnClusterChange() {
	if !foocontroller.isSynced() {
		glog.V(4).Infof("Foo controller not synced")
		foocontroller.clusterDeliverer.DeliverAt(allClustersKey, nil, time.Now().Add(foocontroller.clusterAvailableDelay))
	}
	for _, obj := range foocontroller.fooInformerStore.List() {
		foo := obj.(*apiv1.Foo)
		foocontroller.deliverFoo(types.NamespacedName{Namespace: foo.Namespace, Name: foo.Name},
			foocontroller.smallDelay, false)
	}
}

func (foocontroller *FooController) reconcileFoo(nsFoo types.NamespacedName) {
	if !foocontroller.isSynced() {
		glog.V(4).Infof("Configmap controller not synced")
		foocontroller.deliverFoo(nsFoo, foocontroller.clusterAvailableDelay, false)
		return
	}

	key := nsFoo.String()
	baseFooObj, exist, err := foocontroller.fooInformerStore.GetByKey(key)
	if err != nil {
		glog.Errorf("Failed to query main foo store for %v: %v", key, err)
		foocontroller.deliverFoo(nsFoo, 0, true)
		return
	}

	if !exist {
		// Not federated foo, ignoring.
		glog.V(8).Infof("Skipping not federated foo: %s", key)
		return
	}
	obj, err := api.Scheme.DeepCopy(baseFooObj)
	fooObj, ok := obj.(*apiv1.Foo)
	if err != nil || !ok {
		glog.Errorf("Error in retrieving obj from store: %v, %v", ok, err)
		return
	}

	// Check if deletion has been requested.
	if fooObj.DeletionTimestamp != nil {
		if err := foocontroller.delete(fooObj); err != nil {
			glog.Errorf("Failed to delete %s: %v", nsFoo, err)
			foocontroller.eventRecorder.Eventf(fooObj, api.EventTypeNormal, "DeleteFailed",
				"Foo delete failed: %v", err)
			foocontroller.deliverFoo(nsFoo, 0, true)
		}
		return
	}

	glog.V(3).Infof("Ensuring delete object from underlying clusters finalizer for foo: %s",
		fooObj.Name)
	// Add the required finalizers before creating a foo in underlying clusters.
	updatedFooObj, err := foocontroller.deletionHelper.EnsureFinalizers(fooObj)
	if err != nil {
		glog.Errorf("Failed to ensure delete object from underlying clusters finalizer in foo %s: %v",
			fooObj.Name, err)
		foocontroller.deliverFoo(nsFoo, 0, false)
		return
	}
	fooObj = updatedFooObj.(*apiv1.Foo)

	glog.V(3).Infof("Syncing foo %s in underlying clusters", fooObj.Name)

	clusters, err := foocontroller.fooFederatedInformer.GetReadyClusters()
	if err != nil {
		glog.Errorf("Failed to get cluster list: %v, retrying shortly", err)
		foocontroller.deliverFoo(nsFoo, foocontroller.clusterAvailableDelay, false)
		return
	}

	operations := make([]util.FederatedOperation, 0)
	for _, cluster := range clusters {
		clusterFooObj, found, err := foocontroller.fooFederatedInformer.GetTargetStore().GetByKey(cluster.Name, key)
		if err != nil {
			glog.Errorf("Failed to get %s from %s: %v, retrying shortly", key, cluster.Name, err)
			foocontroller.deliverFoo(nsFoo, 0, true)
			return
		}

		// Do not modify data.
		desiredFoo := &apiv1.Foo{
			ObjectMeta: util.DeepCopyRelevantObjectMeta(fooObj.ObjectMeta),
			Data:       fooObj.Data,
		}

		if !found {
			foocontroller.eventRecorder.Eventf(fooObj, api.EventTypeNormal, "CreateInCluster",
				"Creating foo in cluster %s", cluster.Name)

			operations = append(operations, util.FederatedOperation{
				Type:        util.OperationTypeAdd,
				Obj:         desiredFoo,
				ClusterName: cluster.Name,
			})
		} else {
			clusterFoo := clusterFooObj.(*apiv1.Foo)

			// Update existing foo, if needed.
			if !util.FooEquivalent(desiredFoo, clusterFoo) {
				foocontroller.eventRecorder.Eventf(fooObj, api.EventTypeNormal, "UpdateInCluster",
					"Updating foo in cluster %s", cluster.Name)
				operations = append(operations, util.FederatedOperation{
					Type:        util.OperationTypeUpdate,
					Obj:         desiredFoo,
					ClusterName: cluster.Name,
				})
			}
		}
	}

	if len(operations) == 0 {
		// Everything is in order
		glog.V(8).Infof("No operations needed for %s", key)
		return
	}
	err = foocontroller.federatedUpdater.UpdateWithOnError(operations, foocontroller.updateTimeout,
		func(op util.FederatedOperation, operror error) {
			foocontroller.eventRecorder.Eventf(fooObj, api.EventTypeNormal, "UpdateInClusterFailed",
				"Foo update in cluster %s failed: %v", op.ClusterName, operror)
		})

	if err != nil {
		glog.Errorf("Failed to execute updates for %s: %v, retrying shortly", key, err)
		foocontroller.deliverFoo(nsFoo, 0, true)
		return
	}
}

// delete deletes the given foo or returns error if the deletion was not complete.
func (foocontroller *FooController) delete(foo *apiv1.Foo) error {
	glog.V(3).Infof("Handling deletion of foo: %v", *foo)
	_, err := foocontroller.deletionHelper.HandleObjectInUnderlyingClusters(foo)
	if err != nil {
		return err
	}

	err = foocontroller.federatedApiClient.Core().Foos(foo.Namespace).Delete(foo.Name, nil)
	if err != nil {
		// Its all good if the error is not found error. That means it is deleted already and we do not have to do anything.
		// This is expected when we are processing an update as a result of foo finalizer deletion.
		// The process that deleted the last finalizer is also going to delete the foo and we do not have to do anything.
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete foo: %v", err)
		}
	}
	return nil
}
