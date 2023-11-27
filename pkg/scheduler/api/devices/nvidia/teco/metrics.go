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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// auto-registry collectors in default registry

const (
	// VolcanoNamespace - namespace in prometheus used by volcano
	VolcanoNamespace = "volcano"

	// OnSessionOpen label
	OnSessionOpen = "OnSessionOpen"

	// OnSessionClose label
	OnSessionClose = "OnSessionClose"
)

var (
	TecoDevicesSharedNumber = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: VolcanoNamespace,
			Name:      "teco_device_shared_number",
			Help:      "The number of teco tasks sharing this card",
		},
		[]string{"devID"},
	)
	TecoDevicesCoreMask = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: VolcanoNamespace,
			Name:      "teco_device_allocated_cores_masks",
			Help:      "The mask of compute cores allocated in this card",
		},
		[]string{"devID"},
	)
	TecoDevicesTotalCoreMask = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: VolcanoNamespace,
			Name:      "teco_device_total_cores_mask",
			Help:      "The number of total cores in this card",
		},
		[]string{"devID"},
	)
)

func (gs *TecoDevices) GetStatus() string {
	for _, val := range gs.Device {
		TecoDevicesSharedNumber.WithLabelValues(val.UUID).Set(float64(val.UsedNum))
		TecoDevicesCoreMask.WithLabelValues(val.UUID).Set(float64(val.CoreMask))
		TecoDevicesTotalCoreMask.WithLabelValues(val.UUID).Set(float64(val.CoreUnMasked))
	}
	return ""
}
