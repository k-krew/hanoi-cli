package ui

import (
	"fmt"
	"io"

	"hanoi-cli/analyzer"
	"hanoi-cli/planner"
	"hanoi-cli/simulator"
)

func nodeStatusMD(n analyzer.NodeUtilization) string {
	switch {
	case n.Cordoned && n.IsHotspot:
		return "cordoned, hotspot"
	case n.Cordoned:
		return "cordoned"
	case n.IsHotspot:
		return "hotspot"
	default:
		return ""
	}
}

func RenderAnalysisMarkdown(w io.Writer, plan *planner.Plan) {
	fmt.Fprintf(w, "### Hanoi-CLI Cluster Analysis\n")
	fmt.Fprintf(w, "**Imbalance Score:** %.1f%% -> %.1f%%", plan.BeforeScore, plan.AfterScore)
	if improvement := plan.BeforeScore - plan.AfterScore; improvement > 0 {
		fmt.Fprintf(w, " (Improvement: **%.1f%%**)", improvement)
	}
	fmt.Fprintln(w, "")
	fmt.Fprintln(w)

	if len(plan.BeforeAnalysis.Hotspots) > 0 {
		fmt.Fprintf(w, "**Hotspots:** %d\n", len(plan.BeforeAnalysis.Hotspots))
		for _, h := range plan.BeforeAnalysis.Hotspots {
			fmt.Fprintf(w, "  - %s\n", h)
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "#### Nodes State")
	fmt.Fprintln(w, "| Node | CPU | Memory | Pods | Status |")
	fmt.Fprintln(w, "|------|-----|--------|------|--------|")

	for _, n := range plan.BeforeAnalysis.Nodes {
		fmt.Fprintf(w, "| %s | %.1f%% | %.1f%% | %d | %s |\n",
			n.Name, n.CPUPercent*100, n.MemPercent*100, n.PodCount, nodeStatusMD(n))
	}
	fmt.Fprintln(w)

	if len(plan.Moves) > 0 {
		fmt.Fprintf(w, "#### Suggested Moves (%d)\n", len(plan.Moves))
		for i, m := range plan.Moves {
			fmt.Fprintf(w, "%d. `%s/%s`: `%s` -> `%s`\n", i+1, m.PodNamespace, m.PodName, m.FromNode, m.ToNode)
		}
	} else {
		fmt.Fprintln(w, "#### No beneficial moves found")
		fmt.Fprintln(w, "Cluster is well-balanced.")
	}
}

func RenderSimulationMarkdown(w io.Writer, result *simulator.SimulationResult) {
	fmt.Fprintf(w, "### Hanoi-CLI Node Failure Simulation\n")
	fmt.Fprintf(w, "**Node:** %s\n", result.FailedNode)
	fmt.Fprintf(w, "**Displaced pods:** %d\n", result.DisplacedPods)
	fmt.Fprintf(w, "**Rescheduled:** %d\n", result.RescheduledPods)
	fmt.Fprintf(w, "**Unschedulable:** %d\n", len(result.Unschedulable))

	if result.Feasible {
		fmt.Fprintln(w, "**Recovery:** FEASIBLE")
	} else {
		fmt.Fprintln(w, "**Recovery:** NOT FEASIBLE")
	}
	fmt.Fprintln(w)

	fmt.Fprintf(w, "**Imbalance Score:** %.1f%% -> %.1f%%\n", result.BeforeScore, result.AfterScore)
	fmt.Fprintln(w)

	if len(result.Moves) > 0 {
		fmt.Fprintf(w, "#### Rescheduled Pods (%d)\n", len(result.Moves))
		for i, m := range result.Moves {
			fmt.Fprintf(w, "%d. `%s/%s`: `%s` -> `%s`\n", i+1, m.PodNamespace, m.PodName, m.FromNode, m.ToNode)
		}
		fmt.Fprintln(w)
	}

	if len(result.Unschedulable) > 0 {
		fmt.Fprintln(w, "#### Unschedulable Pods")
		for _, p := range result.Unschedulable {
			fmt.Fprintf(w, "- `%s/%s` (owner: %s/%s)\n", p.Namespace, p.Name, p.OwnerKind, p.OwnerName)
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "#### Surviving Nodes")
	fmt.Fprintln(w, "| Node | CPU | Memory | Pods | Status |")
	fmt.Fprintln(w, "|------|-----|--------|------|--------|")

	for _, n := range result.AfterAnalysis.Nodes {
		fmt.Fprintf(w, "| %s | %.1f%% | %.1f%% | %d | %s |\n",
			n.Name, n.CPUPercent*100, n.MemPercent*100, n.PodCount, nodeStatusMD(n))
	}
}