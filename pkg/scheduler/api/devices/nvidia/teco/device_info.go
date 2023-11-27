/*
Copyright 2023 The Volcano Authors.

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

package teco

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"volcano.sh/volcano/pkg/scheduler/api/devices"
	"volcano.sh/volcano/pkg/scheduler/plugins/util/nodelock"
)

// GPUDevice include gpu id, memory and the pods that are sharing it.
type TecoDevice struct {
	// GPU ID
	ID int
	// GPU Unique ID
	UUID string
	// The pods that are sharing this GPU
	PodMap map[string]*v1.Pod
	// memory per card
	CoreMask uint
	// 0xf for 4-core and 0x7 for 3-core
	CoreUnMasked uint
	// number of allocated
	UsedNum uint
}

type TecoDevices struct {
	Name string

	Device map[int]*TecoDevice
}

// NewGPUDevice creates a device
func NewTecoDevice(id int, core uint, nodename string) *TecoDevice {
	return &TecoDevice{
		ID:           id,
		UUID:         fmt.Sprintf("Teco-card-%s-%d", nodename, id),
		PodMap:       map[string]*v1.Pod{},
		CoreMask:     0,
		CoreUnMasked: core,
		UsedNum:      0,
	}
}

func NewTecoDevices(name string, node *v1.Node) *TecoDevices {
	if node == nil {
		return nil
	}
	totcores, converted := node.Status.Capacity.Name("teco.com/cores", resource.DecimalSI).AsInt64()
	if !converted {
		return nil
	}
	totcards, converted := node.Status.Capacity.Name("teco.com/card", resource.DecimalSI).AsInt64()
	if !converted || totcards == 0 {
		return nil
	}
	nodedevices := &TecoDevices{
		Name:   name,
		Device: make(map[int]*TecoDevice),
	}
	umask := uint(0)
	if totcores == 3*totcards {
		umask = 7
	} else if totcards*4 == totcores {
		umask = 15
	} else {
		return nil
	}
	i := 0
	for i < int(totcards) {
		nodedevices.Device[i] = NewTecoDevice(i, umask, node.Name)
		i++
	}
	for _, val := range nodedevices.Device {
		klog.V(3).Infoln("Teco Device registered name=", nodedevices.Name, "val=", *val)
	}
	return nodedevices
}

func (gs *TecoDevices) GetIgnoredDevices() []string {
	return []string{}
}

// AddResource adds the pod to GPU pool if it is assigned
func (gs *TecoDevices) AddResource(pod *v1.Pod) {
	ids, ok := pod.Annotations[AssignedIDsAnnotations]
	if !ok {
		return
	}
	podDev := decodePodDevices(ids)
	for _, val := range podDev {
		for _, deviceused := range val {
			if gs == nil {
				break
			}
			for index, gsdevice := range gs.Device {
				if strings.Compare(gsdevice.UUID, deviceused.UUID) == 0 {
					gs.Device[index].CoreMask += uint(deviceused.CoreMask)
					gs.Device[index].UsedNum++
				}
			}
		}
	}
	gs.GetStatus()
}

// SubResource frees the gpu hold by the pod
func (gs *TecoDevices) SubResource(pod *v1.Pod) {
	ids, ok := pod.Annotations[AssignedIDsAnnotations]
	if !ok {
		return
	}
	podDev := decodePodDevices(ids)
	for _, val := range podDev {
		for _, deviceused := range val {
			if gs == nil {
				break
			}
			for index, gsdevice := range gs.Device {
				if strings.Compare(gsdevice.UUID, deviceused.UUID) == 0 {
					gs.Device[index].UsedNum--
					gs.Device[index].CoreMask -= uint(deviceused.CoreMask)
				}
			}
		}
	}
	gs.GetStatus()
}

func (gs *TecoDevices) HasDeviceRequest(pod *v1.Pod) bool {
	if TecoTopoEnable && checkTecoResourcesInPod(pod) {
		return true
	}
	return false
}

func (gs *TecoDevices) Release(kubeClient kubernetes.Interface, pod *v1.Pod) error {
	// Nothing needs to be done here
	return nil
}

func (gs *TecoDevices) FilterNode(pod *v1.Pod) (int, string, error) {
	if TecoTopoEnable {
		klog.V(5).Infoln("Teco DeviceSharing starts filtering pods", pod.Name)
		fit, _, err := checkNodeTecoSharingPredicate(pod, gs, true)
		if err != nil || !fit {
			klog.Errorln("deviceSharing err=", err.Error())
			return devices.Unschedulable, fmt.Sprintf("TecoDeviceSharing %s", err.Error()), err
		}
		klog.V(5).Infoln("Teco DeviceSharing successfully filters pods")
	}
	return devices.Success, "", nil
}

func (gs *TecoDevices) Allocate(kubeClient kubernetes.Interface, pod *v1.Pod) error {
	if TecoTopoEnable {
		klog.V(5).Infoln("Teco DeviceSharing:Into AllocateToPod", pod.Name)
		fit, device, err := checkNodeTecoSharingPredicate(pod, gs, false)
		if err != nil || !fit {
			klog.Errorln("DeviceSharing err=", err.Error())
			return err
		}
		if NodeLockEnable {
			nodelock.UseClient(kubeClient)
			err = nodelock.LockNode(gs.Name, DeviceName)
			if err != nil {
				return errors.Errorf("node %s locked for lockname gpushare %s", gs.Name, err.Error())
			}
		}

		annotations := make(map[string]string)
		annotations[AssignedNodeAnnotations] = gs.Name
		annotations[AssignedTimeAnnotations] = strconv.FormatInt(time.Now().Unix(), 10)
		annotations[AssignedIDsAnnotations] = encodePodDevices(device)
		annotations[AssignedIDsToAllocateAnnotations] = annotations[AssignedIDsAnnotations]
		annotations[DeviceBindPhase] = "allocating"
		annotations[BindTimeAnnotations] = strconv.FormatInt(time.Now().Unix(), 10)
		err = patchPodAnnotations(pod, annotations)
		if err != nil {
			return err
		}
		gs.GetStatus()
		klog.V(5).Infoln("Teco DeviceSharing:Allocate Success")
	}
	return nil
}
