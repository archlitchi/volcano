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
	"context"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
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

var topoWeight int
var mountTopo bool
var initialized bool

// New return priority plugin
func New(arguments framework.Arguments) framework.Plugin {
	topoWeight = 200
	mountTopo = false
	initialized = false
	return &priorityPlugin{pluginArguments: arguments, nodeleader: make(map[string]string)}
}

func (pp *priorityPlugin) Name() string {
	return PluginName
}

func (pp *priorityPlugin) OnSessionOpen(ssn *framework.Session) {
	pp.pluginArguments.GetInt(&topoWeight, "weight")
	pp.pluginArguments.GetBool(&mountTopo, "mountTopo")
	batchFunc := func(t *api.TaskInfo, nodes []*api.NodeInfo) (map[string]float64, error) {
		if !initialized {
			err := importGraph(ssn, "nettopo")
			if err == nil {
				initialized = true
			}
		}
		res := make(map[string]float64)
		node, ok := pp.nodeleader[string(t.Job)]
		if !ok {
			node = elect(nodes)
			pp.nodeleader[string(t.Job)] = node
			klog.V(3).Infoln("Leader picked", node)
		}
		for _, val := range nodes {
			res[val.Name] = 10000 - float64(topoWeight)*getDistance(node, val.Name)
		}
		klog.V(3).Infoln("BatchNodeOrder res=", res, "leader=", node, "weight=", topoWeight)
		return res, nil
	}
	ssn.AddBatchNodeOrderFn(pp.Name(), batchFunc)

	jobReadyFunc := func(obj interface{}) bool {
		ji := obj.(*api.JobInfo)
		klog.V(3).Infoln("JobInfo=", ji.Name, ji.PodGroup.ObjectMeta.Name, ji.UID)
		for _, val := range ji.Tasks {
			klog.V(3).Infoln("node ", val.Name, ":", val.NodeName)
			if len(val.NodeName) == 0 {
				return false
			}
		}
		nodegraph, _ := exportGraph(ji)
		cmname := string(ji.Name)[0:len(ji.Name)-37] + "-cm"
		klog.V(3).Infoln(nodegraph, "configMapName=", cmname)
		cm, err := ssn.KubeClient().CoreV1().ConfigMaps("default").Get(context.Background(), cmname, v1.GetOptions{})
		if err != nil {
			klog.Errorln("get cm failed", err.Error())
		}
		_, ok := cm.Data["nettopo.json"]
		if !ok {
			cm.Data = make(map[string]string)
			cm.Data["nettopo.json"], _ = encode(nodegraph)
			ssn.KubeClient().CoreV1().ConfigMaps("default").Update(context.Background(), cm, v1.UpdateOptions{})
		}
		return true
	}
	if mountTopo {
		ssn.AddJobReadyFn(pp.Name(), jobReadyFunc)
	}
}

func (pp *priorityPlugin) OnSessionClose(ssn *framework.Session) {}
