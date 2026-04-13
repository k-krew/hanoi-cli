package ui

import (
	"fmt"
	"io"

	"hanoi-cli/analyzer"
	"hanoi-cli/planner"
	"hanoi-cli/simulator"
)

func nodeTags(n analyzer.NodeUtilization) string {
	var tags string
	if n.Cordoned {
		tags += " [CORDONED]"
	}
	if n.IsHotspot {
		tags += " [HOTSPOT]"
	}
	return tags
}

func RenderAnalysisText(w io.Writer, plan *planner.Plan) {
	_, _ = fmt.Fprintf(w, "Cluster Imbalance Score: %.1f%% -> %.1f%%\n", plan.BeforeScore, plan.AfterScore)
	if improvement := plan.BeforeScore - plan.AfterScore; improvement > 0 {
		_, _ = fmt.Fprintf(w, "Improvement: %.1f%%\n", improvement)
	}
	_, _ = fmt.Fprintln(w)

	if len(plan.BeforeAnalysis.Hotspots) > 0 {
		_, _ = fmt.Fprintf(w, "Hotspots: %d\n", len(plan.BeforeAnalysis.Hotspots))
		for _, h := range plan.BeforeAnalysis.Hotspots {
			_, _ = fmt.Fprintf(w, "  - %s\n", h)
		}
		_, _ = fmt.Fprintln(w)
	}

	_, _ = fmt.Fprintln(w, "Nodes:")
	for _, n := range plan.BeforeAnalysis.Nodes {
		_, _ = fmt.Fprintf(w, "  %-20s CPU: %5.1f%%  MEM: %5.1f%%  pods: %d%s\n",
			n.Name, n.CPUPercent*100, n.MemPercent*100, n.PodCount, nodeTags(n))
	}

	if len(plan.Moves) > 0 {
		_, _ = fmt.Fprintf(w, "\nSuggested Moves: %d\n", len(plan.Moves))
		for i, m := range plan.Moves {
			_, _ = fmt.Fprintf(w, "  %d. %s/%s: %s -> %s\n", i+1, m.PodNamespace, m.PodName, m.FromNode, m.ToNode)
		}
		_, _ = fmt.Fprintln(w, "\nProjected State:")
		for _, n := range plan.AfterAnalysis.Nodes {
			_, _ = fmt.Fprintf(w, "  %-20s CPU: %5.1f%%  MEM: %5.1f%%  pods: %d%s\n",
				n.Name, n.CPUPercent*100, n.MemPercent*100, n.PodCount, nodeTags(n))
		}
	} else {
		_, _ = fmt.Fprintln(w, "\nNo beneficial moves found. Cluster is well-balanced.")
	}
}

func RenderSimulationText(w io.Writer, result *simulator.SimulationResult) {
	_, _ = fmt.Fprintf(w, "Simulating failure of node: %s\n", result.FailedNode)
	_, _ = fmt.Fprintf(w, "Displaced pods:    %d\n", result.DisplacedPods)
	_, _ = fmt.Fprintf(w, "Rescheduled:       %d\n", result.RescheduledPods)
	_, _ = fmt.Fprintf(w, "Unschedulable:     %d\n", len(result.Unschedulable))

	if result.Feasible {
		_, _ = fmt.Fprintln(w, "Recovery:          FEASIBLE")
	} else {
		_, _ = fmt.Fprintln(w, "Recovery:          NOT FEASIBLE")
	}

	_, _ = fmt.Fprintf(w, "\nImbalance Score: %.1f%% -> %.1f%%\n", result.BeforeScore, result.AfterScore)

	if len(result.Moves) > 0 {
		_, _ = fmt.Fprintf(w, "\nRescheduled Pods (%d):\n", len(result.Moves))
		for i, m := range result.Moves {
			_, _ = fmt.Fprintf(w, "  %d. %s/%s: %s -> %s\n", i+1, m.PodNamespace, m.PodName, m.FromNode, m.ToNode)
		}
	}

	if len(result.Unschedulable) > 0 {
		_, _ = fmt.Fprintln(w, "\nUnschedulable Pods:")
		for _, p := range result.Unschedulable {
			_, _ = fmt.Fprintf(w, "  - %s/%s (owner: %s/%s)\n", p.Namespace, p.Name, p.OwnerKind, p.OwnerName)
		}
	}

	_, _ = fmt.Fprintln(w, "\nSurviving Nodes:")
	for _, n := range result.AfterAnalysis.Nodes {
		_, _ = fmt.Fprintf(w, "  %-20s CPU: %5.1f%%  MEM: %5.1f%%  pods: %d%s\n",
			n.Name, n.CPUPercent*100, n.MemPercent*100, n.PodCount, nodeTags(n))
	}
}

func RenderAnalysisShort(w io.Writer, plan *planner.Plan) {
	_, _ = fmt.Fprintf(w, "Score: %.1f%% -> %.1f%%", plan.BeforeScore, plan.AfterScore)
	if improvement := plan.BeforeScore - plan.AfterScore; improvement > 0 {
		_, _ = fmt.Fprintf(w, " (improvement: %.1f%%)", improvement)
	}
	_, _ = fmt.Fprintln(w)

	if len(plan.Moves) > 0 {
		_, _ = fmt.Fprintf(w, "\nSuggested Moves (%d):\n", len(plan.Moves))
		for i, m := range plan.Moves {
			_, _ = fmt.Fprintf(w, "  %d. %s/%s: %s -> %s\n", i+1, m.PodNamespace, m.PodName, m.FromNode, m.ToNode)
		}
	} else {
		_, _ = fmt.Fprintln(w, "No moves needed.")
	}
}

func RenderSimulationShort(w io.Writer, result *simulator.SimulationResult) {
	_, _ = fmt.Fprintf(w, "Node: %s | Score: %.1f%% -> %.1f%%", result.FailedNode, result.BeforeScore, result.AfterScore)
	if result.Feasible {
		_, _ = fmt.Fprintln(w, " | FEASIBLE")
	} else {
		_, _ = fmt.Fprintln(w, " | NOT FEASIBLE")
	}

	if len(result.Moves) > 0 {
		_, _ = fmt.Fprintf(w, "\nRescheduled (%d):\n", len(result.Moves))
		for i, m := range result.Moves {
			_, _ = fmt.Fprintf(w, "  %d. %s/%s: %s -> %s\n", i+1, m.PodNamespace, m.PodName, m.FromNode, m.ToNode)
		}
	}

	if len(result.Unschedulable) > 0 {
		_, _ = fmt.Fprintf(w, "\nUnschedulable (%d):\n", len(result.Unschedulable))
		for _, p := range result.Unschedulable {
			_, _ = fmt.Fprintf(w, "  - %s/%s\n", p.Namespace, p.Name)
		}
	}
}
