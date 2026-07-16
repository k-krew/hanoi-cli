package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"hanoi-cli/kube"

	"github.com/spf13/cobra"
)

var (
	kubeconfig         string
	kubeContext        string
	namespace          string
	output             string
	explainMove        int
	excludeNamespaces  []string
)

var validOutputFormats = map[string]bool{
	"text":  true,
	"json":  true,
	"short": true,
	"ui":    true,
	"md":    true,
}

var rootCmd = &cobra.Command{
	Use:   "hanoi-cli",
	Short: "Interactive rebalance advisor for Kubernetes",
	Long:  "hanoi-cli analyzes and optimizes pod distribution across Kubernetes cluster nodes, providing non-invasive recommendations for rebalancing workloads.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if !validOutputFormats[output] {
			return fmt.Errorf("invalid output format %q: must be one of text, json, short, ui, md", output)
		}
		if namespace != "" && len(excludeNamespaces) > 0 {
			return fmt.Errorf("--namespace and --exclude-namespace are mutually exclusive")
		}
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", defaultKubeconfig(), "path to kubeconfig file")
	rootCmd.PersistentFlags().StringVar(&kubeContext, "context", envStr("HANOI_CONTEXT", ""), "kubernetes context to use (default: current context from kubeconfig)")
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "", "kubernetes namespace (default: all namespaces)")
	rootCmd.PersistentFlags().StringSliceVarP(&excludeNamespaces, "exclude-namespace", "e", nil, "namespaces to exclude from analysis (comma-separated or repeatable)")
	rootCmd.PersistentFlags().StringVarP(&output, "output", "o", "text", "output format: text, json, short, ui, md")
	rootCmd.PersistentFlags().IntVar(&explainMove, "explain", 0, "explain why move N was chosen (1-based)")
}

// markOutOfScope pins pods that fall outside the user's namespace scope so they
// remain counted in utilization scoring but are never suggested as move candidates.
func markOutOfScope(pods []kube.PodInfo, targetNamespace string, excludedNamespaces []string) []kube.PodInfo {
	if targetNamespace == "" && len(excludedNamespaces) == 0 {
		return pods
	}
	excluded := make(map[string]bool, len(excludedNamespaces))
	for _, ns := range excludedNamespaces {
		excluded[ns] = true
	}
	result := make([]kube.PodInfo, len(pods))
	copy(result, pods)
	for i := range result {
		ns := result[i].Namespace
		outOfScope := (targetNamespace != "" && ns != targetNamespace) || excluded[ns]
		if outOfScope {
			result[i].Pinned = true
		}
	}
	return result
}

func Execute() error {
	return rootCmd.Execute()
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func defaultKubeconfig() string {
	if v := os.Getenv("HANOI_KUBECONFIG"); v != "" {
		return v
	}
	if v := os.Getenv("KUBECONFIG"); v != "" {
		return v
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".kube", "config")
	}
	return ""
}
