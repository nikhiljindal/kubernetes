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
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	federationapi "k8s.io/kubernetes/federation/apis/federation/v1beta1"
	fakefedclientset "k8s.io/kubernetes/federation/client/clientset_generated/federation_clientset/fake"
	"k8s.io/kubernetes/federation/pkg/federation-controller/util"
	"k8s.io/kubernetes/federation/pkg/federation-controller/util/deletionhelper"
	. "k8s.io/kubernetes/federation/pkg/federation-controller/util/test"
	apiv1 "k8s.io/kubernetes/pkg/api/v1"
	kubeclientset "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	fakekubeclientset "k8s.io/kubernetes/pkg/client/clientset_generated/clientset/fake"

	"github.com/golang/glog"
	"github.com/stretchr/testify/assert"
)

const (
	foos             string = "foos"
	clusters         string = "clusters"
	informerStoreErr string = "foo should have appeared in the informer store"
)

func TestFooController(t *testing.T) {
	cluster1 := NewCluster("cluster1", apiv1.ConditionTrue)
	cluster2 := NewCluster("cluster2", apiv1.ConditionTrue)

	fakeClient := &fakefedclientset.Clientset{}
	RegisterFakeList(clusters, &fakeClient.Fake, &federationapi.ClusterList{Items: []federationapi.Cluster{*cluster1}})
	RegisterFakeList(foos, &fakeClient.Fake, &apiv1.FooList{Items: []apiv1.Foo{}})
	fooWatch := RegisterFakeWatch(foos, &fakeClient.Fake)
	fooUpdateChan := RegisterFakeCopyOnUpdate(foos, &fakeClient.Fake, fooWatch)
	clusterWatch := RegisterFakeWatch(clusters, &fakeClient.Fake)

	cluster1Client := &fakekubeclientset.Clientset{}
	cluster1Watch := RegisterFakeWatch(foos, &cluster1Client.Fake)
	RegisterFakeList(foos, &cluster1Client.Fake, &apiv1.FooList{Items: []apiv1.Foo{}})
	cluster1CreateChan := RegisterFakeCopyOnCreate(foos, &cluster1Client.Fake, cluster1Watch)
	cluster1UpdateChan := RegisterFakeCopyOnUpdate(foos, &cluster1Client.Fake, cluster1Watch)

	cluster2Client := &fakekubeclientset.Clientset{}
	cluster2Watch := RegisterFakeWatch(foos, &cluster2Client.Fake)
	RegisterFakeList(foos, &cluster2Client.Fake, &apiv1.FooList{Items: []apiv1.Foo{}})
	cluster2CreateChan := RegisterFakeCopyOnCreate(foos, &cluster2Client.Fake, cluster2Watch)

	fooController := NewFooController(fakeClient)
	informer := ToFederatedInformerForTestOnly(fooController.fooFederatedInformer)
	informer.SetClientFactory(func(cluster *federationapi.Cluster) (kubeclientset.Interface, error) {
		switch cluster.Name {
		case cluster1.Name:
			return cluster1Client, nil
		case cluster2.Name:
			return cluster2Client, nil
		default:
			return nil, fmt.Errorf("Unknown cluster")
		}
	})

	fooController.clusterAvailableDelay = time.Second
	fooController.fooReviewDelay = 50 * time.Millisecond
	fooController.smallDelay = 20 * time.Millisecond
	fooController.updateTimeout = 5 * time.Second

	stop := make(chan struct{})
	fooController.Run(stop)

	foo1 := &apiv1.Foo{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-foo",
			Namespace: "ns",
			SelfLink:  "/api/v1/namespaces/ns/foos/test-foo",
		},
		Data: map[string]string{
			"A": "ala ma kota",
			"B": "quick brown fox",
		},
	}

	// Test add federated foo.
	fooWatch.Add(foo1)
	// There should be 2 updates to add both the finalizers.
	updatedFoo := GetFooFromChan(fooUpdateChan)
	assert.True(t, fooController.hasFinalizerFunc(updatedFoo, deletionhelper.FinalizerDeleteFromUnderlyingClusters))
	assert.True(t, fooController.hasFinalizerFunc(updatedFoo, metav1.FinalizerOrphanDependents))

	// Verify that the foo is created in underlying cluster1.
	createdFoo := GetFooFromChan(cluster1CreateChan)
	assert.NotNil(t, createdFoo)
	assert.Equal(t, foo1.Namespace, createdFoo.Namespace)
	assert.Equal(t, foo1.Name, createdFoo.Name)
	assert.True(t, util.FooEquivalent(foo1, createdFoo))

	// Wait for the foo to appear in the informer store
	err := WaitForStoreUpdate(
		fooController.fooFederatedInformer.GetTargetStore(),
		cluster1.Name, types.NamespacedName{Namespace: foo1.Namespace, Name: foo1.Name}.String(), wait.ForeverTestTimeout)
	assert.Nil(t, err, informerStoreErr)

	// Test update federated foo.
	foo1.Annotations = map[string]string{
		"A": "B",
	}
	fooWatch.Modify(foo1)
	updatedFoo = GetFooFromChan(cluster1UpdateChan)
	assert.NotNil(t, updatedFoo)
	assert.Equal(t, foo1.Name, updatedFoo.Name)
	assert.Equal(t, foo1.Namespace, updatedFoo.Namespace)
	assert.True(t, util.FooEquivalent(foo1, updatedFoo))

	// Wait for the foo to appear in the informer store
	err = WaitForFooStoreUpdate(
		fooController.fooFederatedInformer.GetTargetStore(),
		cluster1.Name, types.NamespacedName{Namespace: foo1.Namespace, Name: foo1.Name}.String(),
		foo1, wait.ForeverTestTimeout)
	assert.Nil(t, err, informerStoreErr)

	// Test update federated foo.
	foo1.Data = map[string]string{
		"config": "myconfigurationfile",
	}

	fooWatch.Modify(foo1)
	for {
		updatedFoo := GetFooFromChan(cluster1UpdateChan)
		assert.NotNil(t, updatedFoo)
		if updatedFoo == nil {
			break
		}
		assert.Equal(t, foo1.Name, updatedFoo.Name)
		assert.Equal(t, foo1.Namespace, updatedFoo.Namespace)
		if util.FooEquivalent(foo1, updatedFoo) {
			break
		}
	}

	// Test add cluster
	clusterWatch.Add(cluster2)
	createdFoo2 := GetFooFromChan(cluster2CreateChan)
	assert.NotNil(t, createdFoo2)
	assert.Equal(t, foo1.Name, createdFoo2.Name)
	assert.Equal(t, foo1.Namespace, createdFoo2.Namespace)
	assert.True(t, util.FooEquivalent(foo1, createdFoo2))

	close(stop)
}

func GetFooFromChan(c chan runtime.Object) *apiv1.Foo {
	if foo := GetObjectFromChan(c); foo == nil {
		return nil
	} else {
		return foo.(*apiv1.Foo)
	}
}

// Wait till the store is updated with latest foo.
func WaitForFooStoreUpdate(store util.FederatedReadOnlyStore, clusterName, key string, desiredFoo *apiv1.Foo, timeout time.Duration) error {
	retryInterval := 200 * time.Millisecond
	err := wait.PollImmediate(retryInterval, timeout, func() (bool, error) {
		obj, found, err := store.GetByKey(clusterName, key)
		if !found || err != nil {
			glog.Infof("%s is not in the store", key)
			return false, err
		}
		equal := util.FooEquivalent(obj.(*apiv1.Foo), desiredFoo)
		if !equal {
			glog.Infof("wrong content in the store expected:\n%v\nactual:\n%v\n", *desiredFoo, *obj.(*apiv1.Foo))
		}
		return equal, err
	})
	return err
}
