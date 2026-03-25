package cmd

import (
	"context"
	"fmt"
	"os"

	"hanoi-cli/kube"
	"hanoi-cli/planner"
	"hanoi-cli/simulator"
	"hanoi-cli/ui"

	"github.com/spf13/cobra"
)

var simulateCmd = &cobra.Command{
	Use:   "simulate <node-name>",
	Short: "Simulate node failure and assess recovery feasibility",
	Args:  cobra.ExactArgs(1),
	RunE:  runSimulate,
}

func init() {
	rootCmd.AddCommand(simulateCmd)
}

func runSimulate(cmd *cobra.Command, args []string) error {
	nodeName := args[0]
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

	result := simulator.SimulateNodeFailure(nodes, pods, nodeName)

	switch output {
	case "json":
		if err := ui.RenderSimulationJSON(os.Stdout, result); err != nil {
			return err
		}
	case "short":
		ui.RenderSimulationShort(os.Stdout, result)
	case "ui":
		ui.RenderSimulation(os.Stdout, result)
	case "md":
		ui.RenderSimulationMarkdown(os.Stdout, result)
	default:
		ui.RenderSimulationText(os.Stdout, result)
	}

	if explainMove > 0 {
		if explainMove > len(result.Moves) {
			return fmt.Errorf("--explain %d: only %d rescheduled moves available", explainMove, len(result.Moves))
		}
		m := result.Moves[explainMove-1]
		survivingNodes := make([]kube.NodeInfo, 0, len(nodes)-1)
		for _, n := range nodes {
			if n.Name != nodeName {
				survivingNodes = append(survivingNodes, n)
			}
		}

		adjustedPods := make([]kube.PodInfo, 0, len(pods))
		for _, p := range pods {
			if p.NodeName != nodeName {
				adjustedPods = append(adjustedPods, p)
			}
		}
		for i := 0; i < explainMove-1; i++ {
			prior := result.Moves[i]
			for _, p := range pods {
				if p.Name == prior.PodName && p.Namespace == prior.PodNamespace && p.NodeName == nodeName {
					p.NodeName = prior.ToNode
					adjustedPods = append(adjustedPods, p)
					break
				}
			}
		}
		targetPod := m.PodName
		targetNs := m.PodNamespace
		for _, p := range pods {
			if p.Name == targetPod && p.Namespace == targetNs && p.NodeName == nodeName {
				adjustedPods = append(adjustedPods, p)
				break
			}
		}

		exp := planner.ExplainMove(survivingNodes, adjustedPods, explainMove, m.PodName, m.PodNamespace, m.FromNode, m.ToNode)
		if exp != nil {
			ui.RenderExplanation(os.Stdout, exp, output)
		}
	}
	return nil
}
