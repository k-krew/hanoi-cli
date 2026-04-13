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
		return "CORDONED, HOTSPOT"
	case n.Cordoned:
		return "CORDONED"
	case n.IsHotspot:
		return "HOTSPOT"
	default:
		return "OK"
	}
}

func RenderAnalysisMarkdown(w io.Writer, plan *planner.Plan) {
	_, _ = fmt.Fprintf(w, "### Hanoi-CLI Cluster Analysis\n")
	_, _ = fmt.Fprintf(w, "**Imbalance Score:** %.1f%% -> %.1f%%", plan.BeforeScore, plan.AfterScore)
	if improvement := plan.BeforeScore - plan.AfterScore; improvement > 0 {
		_, _ = fmt.Fprintf(w, " (Improvement: **%.1f%%**)", improvement)
	}
	_, _ = fmt.Fprintln(w, "")
	_, _ = fmt.Fprintln(w)

	if len(plan.BeforeAnalysis.Hotspots) > 0 {
		_, _ = fmt.Fprintf(w, "**Hotspots:** %d\n", len(plan.BeforeAnalysis.Hotspots))
		for _, h := range plan.BeforeAnalysis.Hotspots {
			_, _ = fmt.Fprintf(w, "  - %s\n", h)
		}
		_, _ = fmt.Fprintln(w)
	}

	_, _ = fmt.Fprintln(w, "#### Nodes State")
	_, _ = fmt.Fprintln(w, "| Node | CPU | Memory | Pods | Status |")
	_, _ = fmt.Fprintln(w, "|------|-----|--------|------|--------|")

	for _, n := range plan.BeforeAnalysis.Nodes {
		_, _ = fmt.Fprintf(w, "| %s | %.1f%% | %.1f%% | %d | %s |\n",
			n.Name, n.CPUPercent*100, n.MemPercent*100, n.PodCount, nodeStatusMD(n))
	}
	_, _ = fmt.Fprintln(w)

	if len(plan.Moves) > 0 {
		_, _ = fmt.Fprintf(w, "#### Suggested Moves (%d)\n", len(plan.Moves))
		for i, m := range plan.Moves {
			_, _ = fmt.Fprintf(w, "%d. `%s/%s`: `%s` -> `%s`\n", i+1, m.PodNamespace, m.PodName, m.FromNode, m.ToNode)
		}
		_, _ = fmt.Fprintln(w)

		_, _ = fmt.Fprintln(w, "#### Projected State")
		_, _ = fmt.Fprintln(w, "| Node | CPU | Memory | Pods | Status |")
		_, _ = fmt.Fprintln(w, "|------|-----|--------|------|--------|")
		for _, n := range plan.AfterAnalysis.Nodes {
			_, _ = fmt.Fprintf(w, "| %s | %.1f%% | %.1f%% | %d | %s |\n",
				n.Name, n.CPUPercent*100, n.MemPercent*100, n.PodCount, nodeStatusMD(n))
		}
	} else {
		_, _ = fmt.Fprintln(w, "#### No beneficial moves found")
		_, _ = fmt.Fprintln(w, "Cluster is well-balanced.")
	}
}

func RenderSimulationMarkdown(w io.Writer, result *simulator.SimulationResult) {
	_, _ = fmt.Fprintf(w, "### Hanoi-CLI Node Failure Simulation\n")
	_, _ = fmt.Fprintf(w, "**Node:** %s\n", result.FailedNode)
	_, _ = fmt.Fprintf(w, "**Displaced pods:** %d\n", result.DisplacedPods)
	_, _ = fmt.Fprintf(w, "**Rescheduled:** %d\n", result.RescheduledPods)
	_, _ = fmt.Fprintf(w, "**Unschedulable:** %d\n", len(result.Unschedulable))

	if result.Feasible {
		_, _ = fmt.Fprintln(w, "**Recovery:** FEASIBLE")
	} else {
		_, _ = fmt.Fprintln(w, "**Recovery:** NOT FEASIBLE")
	}
	_, _ = fmt.Fprintln(w)

	_, _ = fmt.Fprintf(w, "**Imbalance Score:** %.1f%% -> %.1f%%\n", result.BeforeScore, result.AfterScore)
	_, _ = fmt.Fprintln(w)

	if len(result.Moves) > 0 {
		_, _ = fmt.Fprintf(w, "#### Rescheduled Pods (%d)\n", len(result.Moves))
		for i, m := range result.Moves {
			_, _ = fmt.Fprintf(w, "%d. `%s/%s`: `%s` -> `%s`\n", i+1, m.PodNamespace, m.PodName, m.FromNode, m.ToNode)
		}
		_, _ = fmt.Fprintln(w)
	}

	if len(result.Unschedulable) > 0 {
		_, _ = fmt.Fprintln(w, "#### Unschedulable Pods")
		for _, p := range result.Unschedulable {
			_, _ = fmt.Fprintf(w, "- `%s/%s` (owner: %s/%s)\n", p.Namespace, p.Name, p.OwnerKind, p.OwnerName)
		}
		_, _ = fmt.Fprintln(w)
	}

	_, _ = fmt.Fprintln(w, "#### Surviving Nodes")
	_, _ = fmt.Fprintln(w, "| Node | CPU | Memory | Pods | Status |")
	_, _ = fmt.Fprintln(w, "|------|-----|--------|------|--------|")

	for _, n := range result.AfterAnalysis.Nodes {
		_, _ = fmt.Fprintf(w, "| %s | %.1f%% | %.1f%% | %d | %s |\n",
			n.Name, n.CPUPercent*100, n.MemPercent*100, n.PodCount, nodeStatusMD(n))
	}
}
