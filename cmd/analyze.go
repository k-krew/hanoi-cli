package cmd

import (
	"context"
	"fmt"
	"os"

	"hanoi-cli/kube"
	"hanoi-cli/planner"
	"hanoi-cli/ui"

	"github.com/spf13/cobra"
)

var (
	resource string
	maxMoves int
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze cluster resource allocation and detect imbalance",
	RunE:  runAnalyze,
}

func init() {
	analyzeCmd.Flags().StringVar(&resource, "resource", "cpu", "resource type to analyze: cpu, memory")
	analyzeCmd.Flags().IntVar(&maxMoves, "max-moves", 0, "maximum number of pod moves to suggest (0 = unlimited)")
	rootCmd.AddCommand(analyzeCmd)
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	client, err := kube.NewClient(kubeconfig, kubeContext)
	if err != nil {
		return fmt.Errorf("connecting to cluster: %w", err)
	}

	nodes, err := client.GetNodes(ctx)
	if err != nil {
		return fmt.Errorf("fetching nodes: %w", err)
	}

	pods, err := client.GetPods(ctx, namespace)
	if err != nil {
		return fmt.Errorf("fetching pods: %w", err)
	}

	plan := planner.GeneratePlan(nodes, pods, resource, maxMoves)

	switch output {
	case "json":
		if err := ui.RenderAnalysisJSON(os.Stdout, plan); err != nil {
			return err
		}
	case "short":
		ui.RenderAnalysisShort(os.Stdout, plan)
	case "ui":
		ui.RenderAnalysis(os.Stdout, plan)
	default:
		ui.RenderAnalysisText(os.Stdout, plan)
	}

	if explainMove > 0 {
		if explainMove > len(plan.Moves) {
			return fmt.Errorf("--explain %d: only %d moves available", explainMove, len(plan.Moves))
		}
		m := plan.Moves[explainMove-1]
		exp := planner.ExplainMove(nodes, pods, explainMove, m.PodName, m.PodNamespace, m.FromNode, m.ToNode)
		if exp != nil {
			ui.RenderExplanation(os.Stdout, exp, output)
		}
	}
	return nil
}
