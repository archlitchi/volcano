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

package networktopology

import (
	"volcano.sh/volcano/pkg/scheduler/api"
	"volcano.sh/volcano/pkg/scheduler/framework"
)

// PluginName indicates name of volcano scheduler plugin.
const PluginName = "networktopology"

type priorityPlugin struct {
	// Arguments given for the plugin
	pluginArguments framework.Arguments
	nodeleader      map[string]string
}

// New return priority plugin
func New(arguments framework.Arguments) framework.Plugin {
	return &priorityPlugin{pluginArguments: arguments, nodeleader: make(map[string]string)}
}

func (pp *priorityPlugin) Name() string {
	return PluginName
}

func (pp *priorityPlugin) OnSessionOpen(ssn *framework.Session) {

	batchFunc := func(t *api.TaskInfo, nodes []*api.NodeInfo) (map[string]float64, error) {
		res := make(map[string]float64)
		node, ok := pp.nodeleader[string(t.Job)]
		if !ok {
			pp.nodeleader[string(t.Job)] = "node67-4v100"
		}
		for _, val := range nodes {
			res[val.Name] = 100 * distance(node, val.Name)
		}
		return res, nil
	}
	ssn.AddBatchNodeOrderFn(pp.Name(), batchFunc)
}

func (pp *priorityPlugin) OnSessionClose(ssn *framework.Session) {}
