package ui

import (
	"encoding/json"
	"fmt"
	"io"

	"hanoi-cli/analyzer"
	"hanoi-cli/planner"
	"hanoi-cli/simulator"
)

type analysisJSON struct {
	ImbalanceScore struct {
		Before      float64 `json:"before"`
		After       float64 `json:"after"`
		Improvement float64 `json:"improvement"`
	} `json:"imbalance_score"`
	Hotspots []string       `json:"hotspots,omitempty"`
	Nodes    []nodeJSON     `json:"nodes"`
	Moves    []moveJSON     `json:"moves,omitempty"`
	After    []nodeJSON     `json:"projected_state,omitempty"`
}

type simulationJSON struct {
	FailedNode     string                `json:"failed_node"`
	Feasible       bool                  `json:"feasible"`
	BeforeScore    float64               `json:"imbalance_before"`
	AfterScore     float64               `json:"imbalance_after"`
	Displaced      int                   `json:"displaced_pods"`
	Rescheduled    int                   `json:"rescheduled_pods"`
	Moves          []moveJSON            `json:"rescheduled_moves,omitempty"`
	Unschedulable  []unschedulablePodJSON `json:"unschedulable_pods,omitempty"`
	SurvivingNodes []nodeJSON            `json:"surviving_nodes"`
}

type nodeJSON struct {
	Name       string  `json:"name"`
	CPUPercent float64 `json:"cpu_percent"`
	MemPercent float64 `json:"mem_percent"`
	PodCount   int     `json:"pod_count"`
	Hotspot    bool    `json:"hotspot,omitempty"`
	Cordoned   bool    `json:"cordoned,omitempty"`
}

type moveJSON struct {
	Pod       string `json:"pod"`
	Namespace string `json:"namespace"`
	From      string `json:"from"`
	To        string `json:"to"`
}

type unschedulablePodJSON struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	OwnerKind string `json:"owner_kind"`
	OwnerName string `json:"owner_name"`
}

func RenderAnalysisJSON(w io.Writer, plan *planner.Plan) error {
	out := analysisJSON{
		Hotspots: plan.BeforeAnalysis.Hotspots,
		Nodes:    toNodeJSONSlice(plan.BeforeAnalysis.Nodes),
	}
	out.ImbalanceScore.Before = plan.BeforeScore
	out.ImbalanceScore.After = plan.AfterScore
	out.ImbalanceScore.Improvement = plan.BeforeScore - plan.AfterScore

	for _, m := range plan.Moves {
		out.Moves = append(out.Moves, moveJSON{
			Pod: m.PodName, Namespace: m.PodNamespace,
			From: m.FromNode, To: m.ToNode,
		})
	}
	if len(plan.Moves) > 0 {
		out.After = toNodeJSONSlice(plan.AfterAnalysis.Nodes)
	}

	return writeJSON(w, out)
}

func RenderSimulationJSON(w io.Writer, result *simulator.SimulationResult) error {
	out := simulationJSON{
		FailedNode:     result.FailedNode,
		Feasible:       result.Feasible,
		BeforeScore:    result.BeforeScore,
		AfterScore:     result.AfterScore,
		Displaced:      result.DisplacedPods,
		Rescheduled:    result.RescheduledPods,
		SurvivingNodes: toNodeJSONSlice(result.AfterAnalysis.Nodes),
	}
	for _, m := range result.Moves {
		out.Moves = append(out.Moves, moveJSON{
			Pod: m.PodName, Namespace: m.PodNamespace,
			From: m.FromNode, To: m.ToNode,
		})
	}
	for _, p := range result.Unschedulable {
		out.Unschedulable = append(out.Unschedulable, unschedulablePodJSON{
			Name: p.Name, Namespace: p.Namespace,
			OwnerKind: p.OwnerKind, OwnerName: p.OwnerName,
		})
	}

	return writeJSON(w, out)
}

func toNodeJSONSlice(nodes []analyzer.NodeUtilization) []nodeJSON {
	result := make([]nodeJSON, len(nodes))
	for i, n := range nodes {
		result[i] = nodeJSON{
			Name:       n.Name,
			CPUPercent: n.CPUPercent * 100,
			MemPercent: n.MemPercent * 100,
			PodCount:   n.PodCount,
			Hotspot:    n.IsHotspot,
			Cordoned:   n.Cordoned,
		}
	}
	return result
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}
	return nil
}
