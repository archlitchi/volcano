package networktopology

import (
	"encoding/json"
	"fmt"
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

func init() {
	n := initYanChengTopo()
	connectNode(n)
	encodestr, err := json.Marshal(n)
	fmt.Println("Nodes=")
	if err == nil {
		fmt.Println(string(encodestr))
	} else {
		fmt.Println("err=", err.Error())
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

func elect() string {
	return ""
}

func distance(n1 string, n2 string) float64 {
	return 0
}
