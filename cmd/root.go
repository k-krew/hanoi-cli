package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	kubeconfig  string
	kubeContext string
	namespace   string
	output      string
	explainMove int
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
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", defaultKubeconfig(), "path to kubeconfig file")
	rootCmd.PersistentFlags().StringVar(&kubeContext, "context", envStr("HANOI_CONTEXT", ""), "kubernetes context to use (default: current context from kubeconfig)")
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "", "kubernetes namespace (default: all namespaces)")
	rootCmd.PersistentFlags().StringVarP(&output, "output", "o", "text", "output format: text, json, short, ui, md")
	rootCmd.PersistentFlags().IntVar(&explainMove, "explain", 0, "explain why move N was chosen (1-based)")
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
