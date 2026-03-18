package analyzer

import (
	"math"
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

func makePod(name, node string, cpuMillis, memBytes int64) kube.PodInfo {
	return kube.PodInfo{
		Name:      name,
		Namespace: "default",
		NodeName:  node,
		OwnerKind: "Deployment",
		OwnerName: "test",
		Requests: kube.Resources{
			CPU:    *resource.NewMilliQuantity(cpuMillis, resource.DecimalSI),
			Memory: *resource.NewQuantity(memBytes, resource.BinarySI),
		},
	}
}

func TestAnalyze_BalancedCluster(t *testing.T) {
	nodes := []kube.NodeInfo{
		makeNode("node-1", 4000, 8000000000),
		makeNode("node-2", 4000, 8000000000),
	}
	pods := []kube.PodInfo{
		makePod("pod-a", "node-1", 2000, 4000000000),
		makePod("pod-b", "node-2", 2000, 4000000000),
	}

	result := Analyze(nodes, pods)

	if len(result.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(result.Nodes))
	}

	for _, n := range result.Nodes {
		if math.Abs(n.CPUPercent-0.5) > 0.001 {
			t.Errorf("node %s: expected CPU 50%%, got %.1f%%", n.Name, n.CPUPercent*100)
		}
		if math.Abs(n.MemPercent-0.5) > 0.001 {
			t.Errorf("node %s: expected MEM 50%%, got %.1f%%", n.Name, n.MemPercent*100)
		}
	}

	if result.CPUStdDev != 0 {
		t.Errorf("expected CPU stddev 0, got %f", result.CPUStdDev)
	}
	if result.ImbalanceScore != 0 {
		t.Errorf("expected imbalance score 0, got %f", result.ImbalanceScore)
	}
	if len(result.Hotspots) != 0 {
		t.Errorf("expected no hotspots, got %v", result.Hotspots)
	}
}

func TestAnalyze_ImbalancedCluster(t *testing.T) {
	nodes := []kube.NodeInfo{
		makeNode("node-1", 4000, 8000000000),
		makeNode("node-2", 4000, 8000000000),
	}
	pods := []kube.PodInfo{
		makePod("pod-a", "node-1", 3600, 7200000000),
		makePod("pod-b", "node-2", 400, 800000000),
	}

	result := Analyze(nodes, pods)

	if result.ImbalanceScore <= 0 {
		t.Error("expected positive imbalance score for imbalanced cluster")
	}
	if result.CPUStdDev <= 0 {
		t.Error("expected positive CPU stddev")
	}
}

func TestAnalyze_HotspotDetection(t *testing.T) {
	nodes := []kube.NodeInfo{
		makeNode("node-1", 4000, 8000000000),
		makeNode("node-2", 4000, 8000000000),
	}
	pods := []kube.PodInfo{
		makePod("pod-a", "node-1", 3400, 1000000000),
		makePod("pod-b", "node-2", 1000, 1000000000),
	}

	result := Analyze(nodes, pods)

	if len(result.Hotspots) != 1 {
		t.Fatalf("expected 1 hotspot, got %d", len(result.Hotspots))
	}
	if result.Hotspots[0] != "node-1" {
		t.Errorf("expected node-1 as hotspot, got %s", result.Hotspots[0])
	}
}

func TestAnalyze_EmptyCluster(t *testing.T) {
	nodes := []kube.NodeInfo{
		makeNode("node-1", 4000, 8000000000),
	}
	var pods []kube.PodInfo

	result := Analyze(nodes, pods)

	if result.Nodes[0].CPUPercent != 0 {
		t.Errorf("expected 0%% CPU on empty node, got %.1f%%", result.Nodes[0].CPUPercent*100)
	}
	if result.Nodes[0].PodCount != 0 {
		t.Errorf("expected 0 pods, got %d", result.Nodes[0].PodCount)
	}
}

func TestAnalyze_PodCount(t *testing.T) {
	nodes := []kube.NodeInfo{
		makeNode("node-1", 4000, 8000000000),
	}
	pods := []kube.PodInfo{
		makePod("pod-a", "node-1", 500, 1000000000),
		makePod("pod-b", "node-1", 500, 1000000000),
		makePod("pod-c", "node-1", 500, 1000000000),
	}

	result := Analyze(nodes, pods)

	if result.Nodes[0].PodCount != 3 {
		t.Errorf("expected 3 pods, got %d", result.Nodes[0].PodCount)
	}
}

func TestMean(t *testing.T) {
	tests := []struct {
		values   []float64
		expected float64
	}{
		{nil, 0},
		{[]float64{}, 0},
		{[]float64{1.0}, 1.0},
		{[]float64{1.0, 3.0}, 2.0},
		{[]float64{0.2, 0.4, 0.6}, 0.4},
	}
	for _, tt := range tests {
		got := mean(tt.values)
		if math.Abs(got-tt.expected) > 0.0001 {
			t.Errorf("mean(%v) = %f, want %f", tt.values, got, tt.expected)
		}
	}
}

func TestStddev(t *testing.T) {
	vals := []float64{0.2, 0.4, 0.6}
	avg := mean(vals)
	sd := stddev(vals, avg)
	if math.Abs(sd-0.163299) > 0.001 {
		t.Errorf("expected stddev ~0.163, got %f", sd)
	}
}

func TestImbalanceScore_Bounds(t *testing.T) {
	if s := imbalanceScore(0, 0); s != 0 {
		t.Errorf("expected 0 for perfectly balanced, got %f", s)
	}
	if s := imbalanceScore(0.5, 0.5); s != 100 {
		t.Errorf("expected 100 for max imbalance, got %f", s)
	}
	if s := imbalanceScore(1.0, 1.0); s != 100 {
		t.Errorf("expected capped at 100, got %f", s)
	}
}
