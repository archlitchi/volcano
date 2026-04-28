/*
Copyright 2026 The Volcano Authors.

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

package vgpu

import (
	"strings"
	"testing"
)

// TestBinpackPolicySelectsBusiestGPU verifies that with policy=binpack, a new
// pod is placed on the GPU that already has the most memory in use, rather
// than the legacy first-fit-by-descending-index behavior.
func TestBinpackPolicySelectsBusiestGPU(t *testing.T) {
	VGPUEnable = true
	defer func() { VGPUEnable = false }()

	// 2 GPUs, 16GB each, max 4 pods sharing per GPU.
	gs := makeGPUDevices("node-1", 2, 16384, 4)
	// Pre-seed device 0 as the busier one. Device 1 is idle.
	gs.Device[0].UsedMem = 4096
	gs.Device[0].UsedNum = 1
	busyUUID := gs.Device[0].UUID
	idleUUID := gs.Device[1].UUID

	pod := makeVGPUPod("worker-0", "default", "aaa", 4096, false, "")
	fit, devs, _, err := checkNodeGPUSharingPredicateAndScore(pod, gs, true, binpackPolicy)
	if err != nil || !fit {
		t.Fatalf("pod should fit: fit=%v err=%v", fit, err)
	}
	got := getDeviceUUID(devs)
	if !strings.Contains(got, busyUUID) {
		t.Errorf("binpack: expected pod on busier device %s, got %s (idle=%s)", busyUUID, got, idleUUID)
	}
}

// TestSpreadPolicySelectsIdleGPU verifies that with policy=spread, a new pod
// is placed on an idle GPU rather than co-locating with the existing pod.
func TestSpreadPolicySelectsIdleGPU(t *testing.T) {
	VGPUEnable = true
	defer func() { VGPUEnable = false }()

	gs := makeGPUDevices("node-1", 2, 16384, 4)
	gs.Device[0].UsedMem = 4096
	gs.Device[0].UsedNum = 1
	busyUUID := gs.Device[0].UUID
	idleUUID := gs.Device[1].UUID

	pod := makeVGPUPod("worker-0", "default", "aaa", 4096, false, "")
	fit, devs, _, err := checkNodeGPUSharingPredicateAndScore(pod, gs, true, spreadPolicy)
	if err != nil || !fit {
		t.Fatalf("pod should fit: fit=%v err=%v", fit, err)
	}
	got := getDeviceUUID(devs)
	if !strings.Contains(got, idleUUID) {
		t.Errorf("spread: expected pod on idle device %s, got %s (busy=%s)", idleUUID, got, busyUUID)
	}
}

// TestUnsetPolicyPreservesLegacyOrder verifies that when no schedulePolicy is
// configured, device selection still walks indices in descending order (the
// pre-existing behavior), so users who have not opted in see no change.
func TestUnsetPolicyPreservesLegacyOrder(t *testing.T) {
	VGPUEnable = true
	defer func() { VGPUEnable = false }()

	// 3 idle GPUs.
	gs := makeGPUDevices("node-1", 3, 16384, 4)
	highestIdxUUID := gs.Device[2].UUID

	pod := makeVGPUPod("worker-0", "default", "aaa", 4096, false, "")
	fit, devs, _, err := checkNodeGPUSharingPredicateAndScore(pod, gs, true, "")
	if err != nil || !fit {
		t.Fatalf("pod should fit: fit=%v err=%v", fit, err)
	}
	got := getDeviceUUID(devs)
	if !strings.Contains(got, highestIdxUUID) {
		t.Errorf("unset policy: expected highest-index device %s (legacy order), got %s", highestIdxUUID, got)
	}
}

// TestGPUScoreSpreadRewardsIdleGPU verifies the node-level scoring agrees with
// the per-device pick: under spread, an idle GPU (UsedNum == 0) must score
// higher than a partially-used GPU. The previous implementation rewarded
// UsedNum == 1, producing a split-brain where node scoring picked a node with
// shared GPUs while the per-device pick on that node would prefer an idle GPU.
func TestGPUScoreSpreadRewardsIdleGPU(t *testing.T) {
	idle := &GPUDevice{UsedNum: 0, UsedMem: 0, Memory: 16384}
	shared := &GPUDevice{UsedNum: 1, UsedMem: 4096, Memory: 16384}
	full := &GPUDevice{UsedNum: 4, UsedMem: 16384, Memory: 16384}

	if GPUScore(spreadPolicy, idle) <= GPUScore(spreadPolicy, shared) {
		t.Errorf("spread: idle GPU must score higher than shared GPU, got idle=%f shared=%f",
			GPUScore(spreadPolicy, idle), GPUScore(spreadPolicy, shared))
	}
	if GPUScore(spreadPolicy, idle) <= GPUScore(spreadPolicy, full) {
		t.Errorf("spread: idle GPU must score higher than full GPU, got idle=%f full=%f",
			GPUScore(spreadPolicy, idle), GPUScore(spreadPolicy, full))
	}
}

// TestBinpackStacksSequentialPods reproduces the user's bug: with policy=binpack,
// three pods scheduled in sequence onto a node with three idle GPUs should all
// land on the same GPU (filling it before opening a new one), not spread across
// three separate cards as the pre-fix first-fit loop did.
func TestBinpackStacksSequentialPods(t *testing.T) {
	VGPUEnable = true
	defer func() { VGPUEnable = false }()

	// 3 idle GPUs, 16GB each, max 4 pods sharing per GPU.
	gs := makeGPUDevices("node-1", 3, 16384, 4)

	uuids := make([]string, 0, 3)
	for i, name := range []string{"a", "b", "c"} {
		pod := makeVGPUPod("worker-"+name, "default", name, 4096, false, "")
		fit, devs, _, err := checkNodeGPUSharingPredicateAndScore(pod, gs, false, binpackPolicy)
		if err != nil || !fit {
			t.Fatalf("pod %d should fit: fit=%v err=%v", i, fit, err)
		}
		uuids = append(uuids, getDeviceUUID(devs))
	}

	if uuids[0] != uuids[1] || uuids[1] != uuids[2] {
		t.Errorf("binpack should stack 3 sequential 4GB pods on the same GPU (16GB / 4GB = 4 slots), got: %v", uuids)
	}
}
