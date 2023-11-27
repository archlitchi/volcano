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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

var kubeClient kubernetes.Interface

func init() {
	var err error
	kubeClient, err = NewClient()
	if err != nil {
		klog.Errorf("init kubeclient in 4pdvgpu failed: %s", err.Error())
	} else {
		klog.V(3).Infoln("init kubeclient success")
	}
}

// NewClient connects to an API server
func NewClient() (kubernetes.Interface, error) {
	kubeConfig := os.Getenv("KUBECONFIG")
	if kubeConfig == "" {
		kubeConfig = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}
	config, err := rest.InClusterConfig()
	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", kubeConfig)
		if err != nil {
			return nil, err
		}
	}
	client, err := kubernetes.NewForConfig(config)
	kubeClient = client
	return client, err
}

func encodeContainerDevices(cd []ContainerDevice) string {
	tmp := ""
	for _, val := range cd {
		tmp += val.UUID + "," + strconv.Itoa(int(val.CoreMask)) + ":"
	}
	klog.V(4).Infoln("Encoded container Devices=", tmp)
	return tmp
	//return strings.Join(cd, ",")
}

func encodePodDevices(pd []ContainerDevices) string {
	var ss []string
	for _, cd := range pd {
		ss = append(ss, encodeContainerDevices(cd))
	}
	return strings.Join(ss, ";")
}

func decodeContainerDevices(str string) ContainerDevices {
	if len(str) == 0 {
		return ContainerDevices{}
	}
	cd := strings.Split(str, ":")
	contdev := ContainerDevices{}
	tmpdev := ContainerDevice{}
	//fmt.Println("before container device", str)
	if len(str) == 0 {
		return contdev
	}
	for _, val := range cd {
		if strings.Contains(val, ",") {
			//fmt.Println("cd is ", val)
			tmpstr := strings.Split(val, ",")
			tmpdev.UUID = tmpstr[0]
			coremask, _ := strconv.ParseInt(tmpstr[1], 10, 32)
			tmpdev.CoreMask = int32(coremask)
			contdev = append(contdev, tmpdev)
		}
	}
	//fmt.Println("Decoded container device", contdev)
	return contdev
}

func decodePodDevices(str string) []ContainerDevices {
	if len(str) == 0 {
		return []ContainerDevices{}
	}
	var pd []ContainerDevices
	for _, s := range strings.Split(str, ";") {
		cd := decodeContainerDevices(s)
		pd = append(pd, cd)
	}
	return pd
}

func checkTecoResourcesInPod(pod *v1.Pod) bool {
	for _, container := range pod.Spec.Containers {
		_, ok := container.Resources.Limits[TecoCoreNumber]
		if ok {
			return true
		}
	}
	return false
}

func resourcereqs(pod *v1.Pod) ([]ContainerDeviceRequest, error) {
	resourceName := v1.ResourceName(TecoCoreNumber)
	counts := []ContainerDeviceRequest{}
	//Count Nvidia GPU
	for i := 0; i < len(pod.Spec.Containers); i++ {
		v, ok := pod.Spec.Containers[i].Resources.Limits[resourceName]
		if ok {
			n, _ := v.AsInt64()
			if n > 4 && n%4 != 0 {
				if n > 3 && n%3 != 0 {
					return counts, errors.New("teco core resource not legal")
				}
			}
			counts = append(counts, ContainerDeviceRequest{
				Coresreq: int32(n),
			})
		}
	}
	klog.V(3).Infoln("counts=", counts)
	return counts, nil
}

func checkType(d TecoDevice, n ContainerDeviceRequest) bool {
	if d.CoreUnMasked == 7 {
		if n.Coresreq < 3 || n.Coresreq%3 == 0 {
			return true
		}
		return false
	}
	if d.CoreUnMasked == 15 {
		if n.Coresreq < 4 || n.Coresreq%4 == 0 {
			return true
		}
		return false
	}
	return false
}

func getTecoDeviceSnapShot(snap *TecoDevices) *TecoDevices {
	ret := TecoDevices{
		Name:   snap.Name,
		Device: make(map[int]*TecoDevice),
	}
	for index, val := range snap.Device {
		ret.Device[index] = &TecoDevice{
			ID:           val.ID,
			UUID:         val.UUID,
			PodMap:       val.PodMap,
			CoreMask:     val.CoreMask,
			CoreUnMasked: val.CoreUnMasked,
			UsedNum:      val.UsedNum,
		}
	}
	return &ret
}

func getCoreNumFromMask(mask int) int {
	tmp := mask
	count := 0
	for tmp > 0 {
		count += tmp % 2
		tmp = tmp / 2
	}
	return count
}

func tryAddNumToMask(d *TecoDevice, count int) (int, error) {
	remains := getCoreNumFromMask(int(d.CoreUnMasked - d.CoreMask))
	if remains < count {
		return 0, errors.New("this card not fit")
	}
	needs := count
	pool := d.CoreUnMasked - d.CoreMask
	newmask := 0
	pos := 1
	for needs > 0 && pool > 0 {
		if pool%2 == 1 {
			needs--
			newmask += pos
		}
		pool /= 2
		pos *= 2
	}
	return int(d.CoreMask) + newmask, nil
}

// checkNodeTecoSharingPredicate checks if a pod with gpu requirement can be scheduled on a node.
func checkNodeTecoSharingPredicate(pod *v1.Pod, gssnap *TecoDevices, replicate bool) (bool, []ContainerDevices, error) {
	// no gpu sharing request
	if !checkTecoResourcesInPod(pod) {
		return true, []ContainerDevices{}, nil
	}
	ctrReq, err := resourcereqs(pod)
	if len(ctrReq) == 0 || err != nil {
		return true, []ContainerDevices{}, err
	}
	var gs *TecoDevices
	if replicate {
		gs = getTecoDeviceSnapShot(gssnap)
	} else {
		gs = gssnap
	}
	ctrdevs := []ContainerDevices{}
	for _, val := range ctrReq {
		devs := []ContainerDevice{}
		devcore := int(0)
		allocatemask := int(0)
		if len(gs.Device) == 0 || gs.Device[0] == nil {
			return false, []ContainerDevices{}, fmt.Errorf("no enough gpu cards on node %s", gs.Name)
		}
		if !checkType(*gs.Device[0], val) {
			klog.Errorln("failed checktype")
			continue
		}
		exclusive := false
		if gs.Device[0].CoreUnMasked == 7 {
			devcore = 3
			allocatemask = 7
			if val.Coresreq > 3 {
				exclusive = true
			}
		} else {
			devcore = 4
			allocatemask = 15
			if val.Coresreq > 4 {
				exclusive = true
			}
		}
		if int(val.Coresreq) > len(gs.Device)*devcore {
			return false, []ContainerDevices{}, fmt.Errorf("no enough gpu cards on node %s", gs.Name)
		}
		klog.V(3).Infoln("Allocating device for container request", val)

		for i := len(gs.Device) - 1; i >= 0; i-- {
			klog.V(3).Info("Scoring pod ", val.Coresreq, " deviceid:", i, " device:", gs.Device[i].ID)
			klog.V(3).Infoln("gs", i, "=", gs.Device[i].CoreMask, ":", gs.Device[i].UsedNum)
			if exclusive {
				if gs.Device[i].CoreMask != 0 {
					continue
				}
			} else {
				allocatemask, err = tryAddNumToMask(gs.Device[i], int(val.Coresreq))
				if err != nil {
					continue
				}
			}
			//total += gs.Devices[i].Count
			//free += node.Devices[i].Count - node.Devices[i].Used
			if val.Coresreq > 0 {
				klog.V(3).Infoln("device", gs.Device[i].ID, "fitted, exclusive=", exclusive)
				if exclusive {
					val.Coresreq -= int32(devcore)
				} else {
					val.Coresreq = 0
				}
				gs.Device[i].UsedNum++
				devs = append(devs, ContainerDevice{
					UUID:     gs.Device[i].UUID,
					CoreMask: int32(allocatemask) - int32(gs.Device[i].CoreMask),
				})
				gs.Device[i].CoreMask = uint(allocatemask)
			}
			if val.Coresreq == 0 {
				break
			}
		}
		if val.Coresreq > 0 {
			return false, []ContainerDevices{}, fmt.Errorf("not enough gpu fitted on this node")
		}
		ctrdevs = append(ctrdevs, devs)
	}
	return true, ctrdevs, nil
}

func patchPodAnnotations(pod *v1.Pod, annotations map[string]string) error {
	type patchMetadata struct {
		Annotations map[string]string `json:"annotations,omitempty"`
	}
	type patchPod struct {
		Metadata patchMetadata `json:"metadata"`
		//Spec     patchSpec     `json:"spec,omitempty"`
	}

	p := patchPod{}
	p.Metadata.Annotations = annotations

	bytes, err := json.Marshal(p)
	if err != nil {
		return err
	}
	_, err = kubeClient.CoreV1().Pods(pod.Namespace).
		Patch(context.Background(), pod.Name, k8stypes.StrategicMergePatchType, bytes, metav1.PatchOptions{})
	if err != nil {
		klog.Errorf("patch pod %v failed, %v", pod.Name, err)
	}

	return err
}
