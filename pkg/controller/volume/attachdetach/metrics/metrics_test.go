/*
Copyright 2018 The Kubernetes Authors.

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

package metrics

import (
	"testing"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/kubernetes/pkg/controller"
	volumetesting "k8s.io/kubernetes/pkg/volume/testing"
)

func TestMetricCollection(t *testing.T) {
	fakeVolumePluginMgr, _ := volumetesting.GetTestVolumePluginMgr(t)
	fakeClient := &fake.Clientset{}

	fakeInformerFactory := informers.NewSharedInformerFactory(fakeClient, controller.NoResyncPeriodFunc())
	fakePodInformer := fakeInformerFactory.Core().V1().Pods()
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "metric-test-pod",
			UID:       "metric-test-pod-uid",
			Namespace: "metric-test",
		},
		Spec: v1.PodSpec{
			NodeName: "metric-test-host",
			Volumes: []v1.Volume{
				{
					Name: "metric-test-volume-name",
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
							ClaimName: "metric-test-pvc",
						},
					},
				},
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodPhase("Running"),
		},
	}

	fakePodInformer.Informer().GetStore().Add(pod)
	pvcInformer := fakeInformerFactory.Core().V1().PersistentVolumeClaims()
	pvInformer := fakeInformerFactory.Core().V1().PersistentVolumes()

	pvc := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "metric-test-pvc",
			Namespace: "metric-test",
			UID:       "metric-test-pvc-1",
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadOnlyMany, v1.ReadWriteOnce},
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): resource.MustParse("2G"),
				},
			},
			VolumeName: "test-metric-pv-1",
		},
		Status: v1.PersistentVolumeClaimStatus{
			Phase: v1.ClaimBound,
		},
	}
	pv := &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			UID:  "test-metric-pv-1",
			Name: "test-metric-pv-1",
		},
		Spec: v1.PersistentVolumeSpec{
			Capacity: v1.ResourceList{
				v1.ResourceName(v1.ResourceStorage): resource.MustParse("5G"),
			},
			PersistentVolumeSource: v1.PersistentVolumeSource{
				GCEPersistentDisk: &v1.GCEPersistentDiskVolumeSource{},
			},
			AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce, v1.ReadOnlyMany},
			// this one we're pretending is already bound
			ClaimRef: &v1.ObjectReference{UID: "metric-test-pvc-1", Namespace: "metric-test"},
		},
	}
	pvcInformer.Informer().GetStore().Add(pvc)
	pvInformer.Informer().GetStore().Add(pv)
	pvcLister := pvcInformer.Lister()
	pvLister := pvInformer.Lister()

	metricCollector := &volumeInUseCollector{
		pvcLister:       pvcLister,
		podLister:       fakePodInformer.Lister(),
		pvLister:        pvLister,
		volumePluginMgr: fakeVolumePluginMgr,
	}

	nodeUseMap := metricCollector.getVolumeInUseCount()
	if len(nodeUseMap) < 1 {
		t.Errorf("Expected one volume in use got %d", len(nodeUseMap))
	}
	testNodeMetric := nodeUseMap["metric-test-host"]
	pluginUseCount, ok := testNodeMetric["fake-plugin"]
	if !ok {
		t.Errorf("Expected fake plugin pvc got nothing")
	}

	if pluginUseCount < 1 {
		t.Errorf("Expected at least in-use volume metric got %d", pluginUseCount)
	}

}
