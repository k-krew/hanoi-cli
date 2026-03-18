package planner

import (
	"testing"

	"hanoi-cli/kube"

	"k8s.io/apimachinery/pkg/api/resource"
)

func makeNode(name string, cpuMillis, memBytes int64) kube.NodeInfo {
	return kube.NodeInfo{
		Name: name,
		Allocatable: kube.Resources{
			CPU:    *resource.NewMilliQuantity(cpuMillis, resource.DecimalSI),
			Memory: *resource.NewQuantity(memBytes, resource.BinarySI),
		},
	}
}

func makePod(name, namespace, node, ownerKind string, cpuMillis, memBytes int64) kube.PodInfo {
	return kube.PodInfo{
		Name:      name,
		Namespace: namespace,
		NodeName:  node,
		OwnerKind: ownerKind,
		OwnerName: "test",
		Requests: kube.Resources{
			CPU:    *resource.NewMilliQuantity(cpuMillis, resource.DecimalSI),
			Memory: *resource.NewQuantity(memBytes, resource.BinarySI),
		},
	}
}

func TestGeneratePlan_BalancedCluster(t *testing.T) {
	nodes := []kube.NodeInfo{
		makeNode("node-1", 4000, 8000000000),
		makeNode("node-2", 4000, 8000000000),
	}
	pods := []kube.PodInfo{
		makePod("pod-a", "default", "node-1", "Deployment", 2000, 4000000000),
		makePod("pod-b", "default", "node-2", "Deployment", 2000, 4000000000),
	}

	plan := GeneratePlan(nodes, pods, "cpu", 0)

	if len(plan.Moves) != 0 {
		t.Errorf("expected no moves for balanced cluster, got %d", len(plan.Moves))
	}
	if plan.BeforeScore != plan.AfterScore {
		t.Errorf("scores should be equal when no moves: before=%f after=%f", plan.BeforeScore, plan.AfterScore)
	}
}

func TestGeneratePlan_ImbalancedCluster(t *testing.T) {
	nodes := []kube.NodeInfo{
		makeNode("node-1", 4000, 8000000000),
		makeNode("node-2", 4000, 8000000000),
	}
	pods := []kube.PodInfo{
		makePod("pod-a", "default", "node-1", "Deployment", 1000, 2000000000),
		makePod("pod-b", "default", "node-1", "Deployment", 1000, 2000000000),
		makePod("pod-c", "default", "node-1", "Deployment", 1000, 2000000000),
	}

	plan := GeneratePlan(nodes, pods, "cpu", 0)

	if len(plan.Moves) == 0 {
		t.Error("expected at least one move for imbalanced cluster")
	}
	if plan.AfterScore >= plan.BeforeScore {
		t.Errorf("after score (%f) should be less than before score (%f)", plan.AfterScore, plan.BeforeScore)
	}
	for _, m := range plan.Moves {
		if m.FromNode != "node-1" {
			t.Errorf("expected moves from node-1, got from %s", m.FromNode)
		}
		if m.ToNode != "node-2" {
			t.Errorf("expected moves to node-2, got to %s", m.ToNode)
		}
	}
}

func TestGeneratePlan_MaxMoves(t *testing.T) {
	nodes := []kube.NodeInfo{
		makeNode("node-1", 4000, 8000000000),
		makeNode("node-2", 4000, 8000000000),
	}
	pods := []kube.PodInfo{
		makePod("pod-a", "default", "node-1", "Deployment", 1000, 1000000000),
		makePod("pod-b", "default", "node-1", "Deployment", 1000, 1000000000),
		makePod("pod-c", "default", "node-1", "Deployment", 1000, 1000000000),
		makePod("pod-d", "default", "node-1", "Deployment", 500, 500000000),
	}

	plan := GeneratePlan(nodes, pods, "cpu", 1)

	if len(plan.Moves) > 1 {
		t.Errorf("expected at most 1 move, got %d", len(plan.Moves))
	}
}

func TestGeneratePlan_SkipsDaemonSets(t *testing.T) {
	nodes := []kube.NodeInfo{
		makeNode("node-1", 4000, 8000000000),
		makeNode("node-2", 4000, 8000000000),
	}
	pods := []kube.PodInfo{
		makePod("ds-pod", "default", "node-1", "DaemonSet", 3000, 6000000000),
	}

	plan := GeneratePlan(nodes, pods, "cpu", 0)

	if len(plan.Moves) != 0 {
		t.Errorf("expected no moves when only DaemonSet pods exist, got %d", len(plan.Moves))
	}
}

func TestApplyMove(t *testing.T) {
	pods := []kube.PodInfo{
		makePod("pod-a", "default", "node-1", "Deployment", 1000, 1000000000),
		makePod("pod-b", "default", "node-2", "Deployment", 1000, 1000000000),
	}

	move := Move{PodName: "pod-a", PodNamespace: "default", FromNode: "node-1", ToNode: "node-2"}
	result := applyMove(pods, move)

	if result[0].NodeName != "node-2" {
		t.Errorf("expected pod-a on node-2, got %s", result[0].NodeName)
	}
	if pods[0].NodeName != "node-1" {
		t.Error("original pods should not be mutated")
	}
}

func TestClonePods(t *testing.T) {
	pods := []kube.PodInfo{
		makePod("pod-a", "default", "node-1", "Deployment", 1000, 1000000000),
	}
	cloned := clonePods(pods)
	cloned[0].NodeName = "node-2"

	if pods[0].NodeName != "node-1" {
		t.Error("clone should not affect original")
	}
}
