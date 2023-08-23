package networktopology

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"volcano.sh/volcano/pkg/scheduler/api"
	"volcano.sh/volcano/pkg/scheduler/framework"
)

type edge struct {
	Dest   string
	Weight uint
}
type node struct {
	ID      string
	Type    int
	Connect []edge
}

type nodeArray []node

var na nodeArray
var distance [][]int32

func initialDistance() {
	distance = make([][]int32, len(na))
	for row := range distance {
		distance[row] = make([]int32, len(na))
		for col := range distance {
			if row == col {
				distance[row][col] = 0
			} else {
				distance[row][col] = 10000
			}
		}
		for _, col := range na[row].Connect {
			dst, err := getidbyname(col.Dest)
			if err != nil {
				break
			}
			distance[row][dst] = int32(col.Weight)
		}
	}
}

func initYanChengTopo() nodeArray {
	n := nodeArray{}
	n = append(n, node{
		ID:   "Spine",
		Type: 2,
	})
	n = append(n, node{
		ID:   "Leaf0",
		Type: 1,
	})
	n = append(n, node{
		ID:   "Leaf1",
		Type: 1,
	})
	n = append(n, node{
		ID:   "sagegpt-4xa10-node225",
		Type: 0,
	}, node{
		ID:   "node67-4v100",
		Type: 0,
	}, node{
		ID:   "node1",
		Type: 0,
	}, node{
		ID:   "kylin01",
		Type: 0,
	})
	return n
}

func connectNode(n nodeArray) error {
	n[0].Connect = append(n[0].Connect, edge{Dest: "Leaf0", Weight: 15}, edge{Dest: "Leaf1", Weight: 15})
	n[1].Connect = append(n[1].Connect, edge{Dest: "Spine", Weight: 15})
	n[2].Connect = append(n[2].Connect, edge{Dest: "Spine", Weight: 15})
	i := 0
	for i < 4 {
		if i < 2 {
			n[3+i].Connect = append(n[3+i].Connect, edge{Dest: "Leaf0", Weight: 4})
			n[1].Connect = append(n[1].Connect, edge{Dest: n[3+i].ID, Weight: 4})
		} else {
			n[3+i].Connect = append(n[3+i].Connect, edge{Dest: "Leaf1", Weight: 4})
			n[2].Connect = append(n[2].Connect, edge{Dest: n[3+i].ID, Weight: 4})
		}
		i++
	}
	return nil
}

func sort() {
	for mid := range distance {
		for left := range distance {
			for right := range distance {
				if distance[left][mid]+distance[mid][right] < distance[left][right] {
					distance[left][right] = distance[left][mid] + distance[mid][right]
				}
			}
		}
	}
}

func getidbyname(n string) (int, error) {
	for idx, val := range na {
		if strings.Compare(n, val.ID) == 0 {
			return idx, nil
		}
	}
	return 0, errors.New("node not found")
}

func elect(nodes []*api.NodeInfo) string {
	sum := float64(0)
	min := float64(2000000000)
	pick := ""
	for _, val := range nodes {
		sum = 0
		for _, val1 := range nodes {
			sum += val.Idle.ScalarResources["nvidia.com/gpu"] * float64(getDistance(val.Name, val1.Name))
		}
		if min > sum {
			min = sum
			pick = val.Name
		}
	}
	return pick
}

func getDistance(n1 string, n2 string) float64 {
	node1, _ := getidbyname(n1)
	node2, _ := getidbyname(n2)
	return float64(distance[node1][node2])
}

func encode(nr nodeArray) (string, error) {
	bytes, err := json.Marshal(nr)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func importGraph(ssn *framework.Session, cm string) error {
	na = nodeArray{}
	cfg, err := ssn.KubeClient().CoreV1().ConfigMaps("default").Get(context.Background(), cm, v1.GetOptions{})
	if err != nil {
		return err
	}
	err = json.Unmarshal([]byte(cfg.Data["topo.json"]), &na)
	if err != nil {
		return err
	}

	initialDistance()
	sort()
	encodestr, err := json.Marshal(na)
	if err == nil {
		klog.V(3).Infoln(string(encodestr))
	} else {
		klog.Errorln("err=", err.Error())
	}

	return nil
}

func exportGraph(ji *api.JobInfo) (nodeArray, error) {
	tmp := nodeArray{}
	for _, val := range ji.Tasks {
		n := node{
			ID:      val.Name,
			Type:    0,
			Connect: []edge{},
		}
		for _, val1 := range ji.Tasks {
			n.Connect = append(n.Connect, edge{
				Dest:   val1.Name,
				Weight: uint(getDistance(val.NodeName, val1.NodeName)),
			})
		}
		tmp = append(tmp, n)
	}
	return tmp, nil
}
