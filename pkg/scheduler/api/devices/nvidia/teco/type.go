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

var TecoTopoEnable bool
var NodeLockEnable bool

const (
	TecoInUse                        = "teco.com/use-gputype"
	TecoNoUse                        = "teco.com/nouse-gputype"
	AssignedTimeAnnotations          = "volcano.sh/teco-assigned-time"
	AssignedIDsAnnotations           = "volcano.sh/teco-ids-new"
	AssignedIDsToAllocateAnnotations = "volcano.sh/teco-to-allocate"
	AssignedNodeAnnotations          = "volcano.sh/vgpu-node"
	BindTimeAnnotations              = "volcano.sh/bind-time"
	DeviceBindPhase                  = "volcano.sh/bind-phase"

	NvidiaGPUDevice = "Teco"

	// DeviceName used to indicate this device
	DeviceName = "tecocore"

	// VolcanoVGPUNumber virtual GPU card number
	TecoCoreNumber = "teco.com/cores"
)

type ContainerDeviceRequest struct {
	Coresreq int32
}

type ContainerDevice struct {
	UUID string
	//Type string
	CoreMask int32
}

type ContainerDevices []ContainerDevice
