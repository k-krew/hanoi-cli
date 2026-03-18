package ui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"hanoi-cli/analyzer"
	"hanoi-cli/planner"
	"hanoi-cli/simulator"

	"golang.org/x/term"
)

const defaultBarWidth = 30

func termBarWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w < 60 {
		return defaultBarWidth
	}
	bw := (w - 60) / 2
	if bw < 10 {
		return 10
	}
	if bw > 60 {
		return 60
	}
	return bw
}

const (
	colorReset    = "\033[0m"
	colorRed      = "\033[31m"
	colorDarkGrey = "\033[90m"
)

func RenderAnalysis(w io.Writer, plan *planner.Plan) {
	fmt.Fprintln(w, "")
	renderScore(w, plan.BeforeScore, plan.AfterScore)
	fmt.Fprintln(w, "")

	if len(plan.BeforeAnalysis.Hotspots) > 0 {
		fmt.Fprintf(w, "  Hotspots: %s\n\n", strings.Join(plan.BeforeAnalysis.Hotspots, ", "))
	}

	renderNodeUtilization(w, "Current State", plan.BeforeAnalysis.Nodes)

	if len(plan.Moves) > 0 {
		fmt.Fprintln(w, "")
		renderMoves(w, plan.Moves)
		fmt.Fprintln(w, "")
		renderNodeUtilization(w, "After Optimization", plan.AfterAnalysis.Nodes)
	} else {
		fmt.Fprintln(w, "\n  No beneficial moves found. Cluster is well-balanced.")
	}
	fmt.Fprintln(w, "")
}

func RenderSimulation(w io.Writer, result *simulator.SimulationResult) {
	fmt.Fprintln(w, "")
	fmt.Fprintf(w, "  Simulating failure of node: %s\n", result.FailedNode)
	fmt.Fprintf(w, "  Displaced pods:    %d\n", result.DisplacedPods)
	fmt.Fprintf(w, "  Rescheduled:       %d\n", result.RescheduledPods)
	fmt.Fprintf(w, "  Unschedulable:     %d\n", len(result.Unschedulable))

	if result.Feasible {
		fmt.Fprintln(w, "  Recovery:          FEASIBLE")
	} else {
		fmt.Fprintln(w, "  Recovery:          NOT FEASIBLE")
	}
	fmt.Fprintln(w, "")

	renderScore(w, result.BeforeScore, result.AfterScore)
	fmt.Fprintln(w, "")

	renderNodeUtilization(w, "Before Failure", result.BeforeAnalysis.Nodes)

	if len(result.Moves) > 0 {
		fmt.Fprintln(w, "")
		fmt.Fprintf(w, "  Rescheduled Pods (%d):\n", len(result.Moves))
		for i, m := range result.Moves {
			fmt.Fprintf(w, "    %d. %s/%s: %s -> %s\n", i+1, m.PodNamespace, m.PodName, m.FromNode, m.ToNode)
		}
	}

	fmt.Fprintln(w, "")
	renderNodeUtilization(w, "After Failure", result.AfterAnalysis.Nodes)

	if len(result.Unschedulable) > 0 {
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "  Unschedulable Pods:")
		for _, p := range result.Unschedulable {
			fmt.Fprintf(w, "    - %s/%s (owner: %s/%s)\n", p.Namespace, p.Name, p.OwnerKind, p.OwnerName)
		}
	}
	fmt.Fprintln(w, "")
}

func renderScore(w io.Writer, before, after float64) {
	fmt.Fprintln(w, "  Cluster Imbalance Score:")
	fmt.Fprintf(w, "    Before: %.1f%%\n", before)
	fmt.Fprintf(w, "    After:  %.1f%%\n", after)
	improvement := before - after
	if improvement > 0 {
		fmt.Fprintf(w, "    Improvement: %.1f%%\n", improvement)
	} else if improvement < 0 {
		fmt.Fprintf(w, "    Degradation: %.1f%%\n", -improvement)
	}
}

func renderNodeUtilization(w io.Writer, title string, nodes []analyzer.NodeUtilization) {
	fmt.Fprintf(w, "  %s:\n", title)

	bw := termBarWidth()
	maxNameLen := 0
	for _, n := range nodes {
		if len(n.Name) > maxNameLen {
			maxNameLen = len(n.Name)
		}
	}

	for _, n := range nodes {
		marker := " "
		colorStart := ""
		colorEnd := ""
		if n.Cordoned {
			marker = "C"
			colorStart = colorDarkGrey
			colorEnd = colorReset
		} else if n.IsHotspot {
			marker = "!"
			colorStart = colorRed
			colorEnd = colorReset
		}
		cpuBar := progressBar(n.CPUPercent, bw)
		memBar := progressBar(n.MemPercent, bw)
		fmt.Fprintf(w, "%s  %s %-*s  CPU %s %5.1f%%  MEM %s %5.1f%%  pods: %d%s\n",
			colorStart, marker, maxNameLen, n.Name, cpuBar, n.CPUPercent*100, memBar, n.MemPercent*100, n.PodCount, colorEnd)
	}
}

func renderMoves(w io.Writer, moves []planner.Move) {
	fmt.Fprintf(w, "  Suggested Moves (%d):\n", len(moves))
	for i, m := range moves {
		fmt.Fprintf(w, "    %d. %s/%s: %s -> %s\n", i+1, m.PodNamespace, m.PodName, m.FromNode, m.ToNode)
	}
}

func progressBar(ratio float64, width int) string {
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	filled := int(ratio * float64(width))
	empty := width - filled
	return "[" + strings.Repeat("#", filled) + strings.Repeat(".", empty) + "]"
}
