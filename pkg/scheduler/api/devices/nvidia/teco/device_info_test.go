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
	"testing"
)

func TestGetGPUMemoryOfPod(t *testing.T) {
	testCases := []struct {
		device TecoDevice
		want   uint
	}{
		{
			device: TecoDevice{
				UsedNum:      1,
				CoreMask:     3,
				CoreUnMasked: 7,
			},
			want: 1,
		},
		{
			device: TecoDevice{
				UsedNum:      2,
				CoreMask:     5,
				CoreUnMasked: 15,
			},
			want: 1,
		},
		{
			device: TecoDevice{
				UsedNum:      0,
				CoreMask:     0,
				CoreUnMasked: 7,
			},
			want: 1,
		},
		{
			device: TecoDevice{
				UsedNum:      0,
				CoreMask:     0,
				CoreUnMasked: 15,
			},
			want: 1,
		},
	}

	t.Run("", func(t *testing.T) {
		got, err := tryAddNumToMask(&testCases[0].device, 1)
		if err != nil || got != 7 {
			t.Errorf("unexpected result, got: %v %s", got, err)
		}
		got, err = tryAddNumToMask(&testCases[1].device, 2)
		if err != nil || got != 15 {
			t.Errorf("unexpected result, got: %v %s", got, err)
		}
		got, err = tryAddNumToMask(&testCases[1].device, 1)
		if err != nil || got != 7 {
			t.Errorf("unexpected result, got: %v %s", got, err)
		}
		got, err = tryAddNumToMask(&testCases[1].device, 3)
		if err == nil {
			t.Errorf("unexpected result, got: %v %s", got, err)
		}
		got, err = tryAddNumToMask(&testCases[2].device, 3)
		if err != nil || got != 7 {
			t.Errorf("unexpected result, got: %v %s", got, err)
		}
		got, err = tryAddNumToMask(&testCases[3].device, 4)
		if err != nil || got != 15 {
			t.Errorf("unexpected result, got: %v %s", got, err)
		}
	})
}
