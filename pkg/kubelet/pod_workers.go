/*
Copyright 2014 Google Inc. All rights reserved.

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

package kubelet

import (
	"sync"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/record"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet/dockertools"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/types"
	"github.com/golang/glog"
)

type syncPodFunType func(*api.BoundPod, dockertools.DockerContainers) error

// TODO(wojtek-t) Add unit tests for this type.
type podWorkers struct {
	// Protects podUpdates field.
	podLock sync.Mutex

	// Tracks all running per-pod goroutines - per-pod goroutine will be
	// processing updates received through its corresponding channel.
	podUpdates map[types.UID]chan api.BoundPod
	// DockerCache is used for listing running containers.
	dockerCache dockertools.DockerCache

	// This function is run to sync the desired stated of pod.
	// NOTE: This function has to be thread-safe - it can be called for
	// different pods at the same time.
	syncPodFun syncPodFunType
}

func newPodWorkers(dockerCache dockertools.DockerCache, syncPodFun syncPodFunType) *podWorkers {
	return &podWorkers{
		podUpdates:  map[types.UID]chan api.BoundPod{},
		dockerCache: dockerCache,
		syncPodFun:  syncPodFun,
	}
}

func (p *podWorkers) managePodLoop(podUpdates <-chan api.BoundPod) {
	for newPod := range podUpdates {
		// Since we use docker cache, getting current state shouldn't cause
		// performance overhead on Docker. Moreover, as long as we run syncPod
		// no matter if it changes anything, having an old version of "containers"
		// can cause starting eunended containers.
		containers, err := p.dockerCache.RunningContainers()
		if err != nil {
			glog.Errorf("Error listing containers while syncing pod: %v", err)
			continue
		}
		err = p.syncPodFun(&newPod, containers)
		if err != nil {
			glog.Errorf("Error syncing pod %s, skipping: %v", newPod.UID, err)
			record.Eventf(&newPod, "failedSync", "Error syncing pod, skipping: %v", err)
			continue
		}
	}
}

func (p *podWorkers) UpdatePod(pod api.BoundPod) {
	uid := pod.UID
	var podUpdates chan api.BoundPod
	var exists bool

	p.podLock.Lock()
	defer p.podLock.Unlock()
	if podUpdates, exists = p.podUpdates[uid]; !exists {
		// TODO(wojtek-t): Adjust the size of the buffer in this channel
		podUpdates = make(chan api.BoundPod, 5)
		p.podUpdates[uid] = podUpdates
		go p.managePodLoop(podUpdates)
	}
	podUpdates <- pod
}

func (p *podWorkers) ForgetNonExistingPodWorkers(desiredPods map[types.UID]empty) {
	p.podLock.Lock()
	defer p.podLock.Unlock()
	for key, channel := range p.podUpdates {
		if _, exists := desiredPods[key]; !exists {
			close(channel)
			delete(p.podUpdates, key)
		}
	}
}
