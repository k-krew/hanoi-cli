package simulator

import (
	"testing"

	"hanoi-cli/kube"

	corev1 "k8s.io/api/core/v1"
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

func makePod(name, node, ownerKind string, cpuMillis, memBytes int64) kube.PodInfo {
	return kube.PodInfo{
		Name:      name,
		Namespace: "default",
		NodeName:  node,
		OwnerKind: ownerKind,
		OwnerName: "test",
		Requests: kube.Resources{
			CPU:    *resource.NewMilliQuantity(cpuMillis, resource.DecimalSI),
			Memory: *resource.NewQuantity(memBytes, resource.BinarySI),
		},
	}
}

func TestSimulateNodeFailure_AllRescheduled(t *testing.T) {
	nodes := []kube.NodeInfo{
		makeNode("node-1", 4000, 8000000000),
		makeNode("node-2", 4000, 8000000000),
		makeNode("node-3", 4000, 8000000000),
	}
	pods := []kube.PodInfo{
		makePod("pod-a", "node-1", "Deployment", 1000, 1000000000),
		makePod("pod-b", "node-1", "Deployment", 1000, 1000000000),
		makePod("pod-c", "node-2", "Deployment", 500, 500000000),
		makePod("pod-d", "node-3", "Deployment", 500, 500000000),
	}

	result := SimulateNodeFailure(nodes, pods, "node-1")

	if !result.Feasible {
		t.Error("expected recovery to be feasible")
	}
	if result.DisplacedPods != 2 {
		t.Errorf("expected 2 displaced pods, got %d", result.DisplacedPods)
	}
	if result.RescheduledPods != 2 {
		t.Errorf("expected 2 rescheduled pods, got %d", result.RescheduledPods)
	}
	if len(result.Unschedulable) != 0 {
		t.Errorf("expected 0 unschedulable pods, got %d", len(result.Unschedulable))
	}
	if result.FailedNode != "node-1" {
		t.Errorf("expected failed node node-1, got %s", result.FailedNode)
	}
	if len(result.Moves) != 2 {
		t.Fatalf("expected 2 moves, got %d", len(result.Moves))
	}
	for _, m := range result.Moves {
		if m.FromNode != "node-1" {
			t.Errorf("expected move from node-1, got from %s", m.FromNode)
		}
		if m.ToNode != "node-2" && m.ToNode != "node-3" {
			t.Errorf("expected move to node-2 or node-3, got %s", m.ToNode)
		}
	}
}

func TestSimulateNodeFailure_SomeUnschedulable(t *testing.T) {
	nodes := []kube.NodeInfo{
		makeNode("node-1", 4000, 8000000000),
		makeNode("node-2", 1000, 2000000000),
	}
	pods := []kube.PodInfo{
		makePod("pod-a", "node-1", "Deployment", 500, 500000000),
		makePod("pod-b", "node-1", "Deployment", 500, 500000000),
		makePod("pod-c", "node-1", "Deployment", 500, 500000000),
	}

	result := SimulateNodeFailure(nodes, pods, "node-1")

	if result.Feasible {
		t.Error("expected recovery to be infeasible (not enough capacity)")
	}
	if len(result.Unschedulable) == 0 {
		t.Error("expected some unschedulable pods")
	}
	if result.DisplacedPods != 3 {
		t.Errorf("expected 3 displaced pods, got %d", result.DisplacedPods)
	}
}

func TestSimulateNodeFailure_DaemonSetSkipped(t *testing.T) {
	nodes := []kube.NodeInfo{
		makeNode("node-1", 4000, 8000000000),
		makeNode("node-2", 4000, 8000000000),
	}
	pods := []kube.PodInfo{
		makePod("ds-pod", "node-1", "DaemonSet", 1000, 1000000000),
		makePod("dep-pod", "node-1", "Deployment", 1000, 1000000000),
	}

	result := SimulateNodeFailure(nodes, pods, "node-1")

	if result.DisplacedPods != 2 {
		t.Errorf("expected 2 displaced pods, got %d", result.DisplacedPods)
	}
	if result.RescheduledPods != 1 {
		t.Errorf("expected 1 rescheduled (DaemonSet skipped), got %d", result.RescheduledPods)
	}
}

func TestSimulateNodeFailure_TaintedTarget(t *testing.T) {
	nodes := []kube.NodeInfo{
		makeNode("node-1", 4000, 8000000000),
		{
			Name:          "node-2",
			Unschedulable: false,
			Taints: []corev1.Taint{
				{Key: "dedicated", Value: "gpu", Effect: corev1.TaintEffectNoSchedule},
			},
			Allocatable: kube.Resources{
				CPU:    *resource.NewMilliQuantity(4000, resource.DecimalSI),
				Memory: *resource.NewQuantity(8000000000, resource.BinarySI),
			},
		},
	}
	pods := []kube.PodInfo{
		makePod("pod-a", "node-1", "Deployment", 500, 500000000),
	}

	result := SimulateNodeFailure(nodes, pods, "node-1")

	if result.Feasible {
		t.Error("should not be feasible: target is tainted and pod has no toleration")
	}
	if len(result.Unschedulable) != 1 {
		t.Errorf("expected 1 unschedulable pod, got %d", len(result.Unschedulable))
	}
}

func TestSimulateNodeFailure_NonexistentNode(t *testing.T) {
	nodes := []kube.NodeInfo{
		makeNode("node-1", 4000, 8000000000),
	}
	pods := []kube.PodInfo{
		makePod("pod-a", "node-1", "Deployment", 500, 500000000),
	}

	result := SimulateNodeFailure(nodes, pods, "nonexistent")

	if result.DisplacedPods != 0 {
		t.Errorf("expected 0 displaced pods for nonexistent node, got %d", result.DisplacedPods)
	}
	if !result.Feasible {
		t.Error("simulating nonexistent node should be trivially feasible")
	}
}
