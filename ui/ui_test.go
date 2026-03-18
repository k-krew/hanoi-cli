package ui

import (
	"bytes"
	"strings"
	"testing"

	"hanoi-cli/analyzer"
	"hanoi-cli/planner"
	"hanoi-cli/simulator"
)

func TestProgressBar(t *testing.T) {
	tests := []struct {
		ratio    float64
		width    int
		expected string
	}{
		{0.0, 10, "[..........]"},
		{1.0, 10, "[##########]"},
		{0.5, 10, "[#####.....]"},
		{-0.1, 10, "[..........]"},
		{1.5, 10, "[##########]"},
		{0.3, 20, "[######..............]"},
	}
	for _, tt := range tests {
		got := progressBar(tt.ratio, tt.width)
		if got != tt.expected {
			t.Errorf("progressBar(%f, %d) = %q, want %q", tt.ratio, tt.width, got, tt.expected)
		}
	}
}

func TestRenderAnalysis_NoMoves(t *testing.T) {
	plan := &planner.Plan{
		BeforeScore: 5.0,
		AfterScore:  5.0,
		BeforeAnalysis: &analyzer.ClusterAnalysis{
			Nodes: []analyzer.NodeUtilization{
				{Name: "node-1", CPUPercent: 0.5, MemPercent: 0.5, PodCount: 3},
			},
		},
		AfterAnalysis: &analyzer.ClusterAnalysis{
			Nodes: []analyzer.NodeUtilization{
				{Name: "node-1", CPUPercent: 0.5, MemPercent: 0.5, PodCount: 3},
			},
		},
	}

	var buf bytes.Buffer
	RenderAnalysis(&buf, plan)
	output := buf.String()

	if !strings.Contains(output, "No beneficial moves") {
		t.Error("expected 'No beneficial moves' message")
	}
	if !strings.Contains(output, "Before: 5.0%") {
		t.Error("expected before score in output")
	}
}

func TestRenderAnalysis_WithMoves(t *testing.T) {
	plan := &planner.Plan{
		BeforeScore: 40.0,
		AfterScore:  10.0,
		Moves: []planner.Move{
			{PodName: "pod-a", PodNamespace: "default", FromNode: "node-1", ToNode: "node-2"},
		},
		BeforeAnalysis: &analyzer.ClusterAnalysis{
			Nodes: []analyzer.NodeUtilization{
				{Name: "node-1", CPUPercent: 0.8, MemPercent: 0.6, PodCount: 5, IsHotspot: true},
				{Name: "node-2", CPUPercent: 0.2, MemPercent: 0.2, PodCount: 1},
			},
			Hotspots: []string{"node-1"},
		},
		AfterAnalysis: &analyzer.ClusterAnalysis{
			Nodes: []analyzer.NodeUtilization{
				{Name: "node-1", CPUPercent: 0.5, MemPercent: 0.4, PodCount: 4},
				{Name: "node-2", CPUPercent: 0.5, MemPercent: 0.4, PodCount: 2},
			},
		},
	}

	var buf bytes.Buffer
	RenderAnalysis(&buf, plan)
	output := buf.String()

	if !strings.Contains(output, "Suggested Moves (1)") {
		t.Error("expected moves section")
	}
	if !strings.Contains(output, "pod-a") {
		t.Error("expected pod name in moves")
	}
	if !strings.Contains(output, "Improvement: 30.0%") {
		t.Error("expected improvement in output")
	}
	if !strings.Contains(output, "Hotspots: node-1") {
		t.Error("expected hotspot listed")
	}
	if !strings.Contains(output, "After Optimization") {
		t.Error("expected after optimization section")
	}
}

func TestRenderSimulation(t *testing.T) {
	result := &simulator.SimulationResult{
		FailedNode:      "node-1",
		Feasible:        false,
		BeforeScore:     20.0,
		AfterScore:      45.0,
		DisplacedPods:   3,
		RescheduledPods: 2,
		Moves: []simulator.RescheduleMove{
			{PodName: "pod-a", PodNamespace: "default", FromNode: "node-1", ToNode: "node-2"},
			{PodName: "pod-b", PodNamespace: "default", FromNode: "node-1", ToNode: "node-2"},
		},
		Unschedulable: []simulator.UnschedulablePod{
			{Name: "pod-c", Namespace: "default", OwnerKind: "Deployment", OwnerName: "web"},
		},
		BeforeAnalysis: &analyzer.ClusterAnalysis{
			Nodes: []analyzer.NodeUtilization{
				{Name: "node-1", CPUPercent: 0.5, MemPercent: 0.5, PodCount: 3},
				{Name: "node-2", CPUPercent: 0.3, MemPercent: 0.3, PodCount: 2},
			},
		},
		AfterAnalysis: &analyzer.ClusterAnalysis{
			Nodes: []analyzer.NodeUtilization{
				{Name: "node-2", CPUPercent: 0.8, MemPercent: 0.7, PodCount: 4},
			},
		},
	}

	var buf bytes.Buffer
	RenderSimulation(&buf, result)
	output := buf.String()

	if !strings.Contains(output, "NOT FEASIBLE") {
		t.Error("expected NOT FEASIBLE in output")
	}
	if !strings.Contains(output, "Displaced pods:    3") {
		t.Error("expected displaced pods count")
	}
	if !strings.Contains(output, "Unschedulable Pods:") {
		t.Error("expected unschedulable pods section")
	}
	if !strings.Contains(output, "pod-c") {
		t.Error("expected unschedulable pod name")
	}
	if !strings.Contains(output, "Degradation: 25.0%") {
		t.Error("expected degradation shown")
	}
	if !strings.Contains(output, "Rescheduled Pods (2)") {
		t.Error("expected rescheduled pods section")
	}
	if !strings.Contains(output, "pod-a") {
		t.Error("expected rescheduled pod name in output")
	}
}
