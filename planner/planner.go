package planner

import (
	"sort"

	"hanoi-cli/analyzer"
	"hanoi-cli/kube"
)

type Move struct {
	PodName      string
	PodNamespace string
	FromNode     string
	ToNode       string
}

type Plan struct {
	Moves          []Move
	BeforeScore    float64
	AfterScore     float64
	BeforeAnalysis *analyzer.ClusterAnalysis
	AfterAnalysis  *analyzer.ClusterAnalysis
}

func GeneratePlan(nodes []kube.NodeInfo, pods []kube.PodInfo, resourceType string, maxMoves int) *Plan {
	before := analyzer.Analyze(nodes, pods)

	workingPods := clonePods(pods)
	var moves []Move

	limit := len(workingPods)
	if maxMoves > 0 {
		limit = maxMoves
	}

	for i := 0; i < limit; i++ {
		move, improved := findBestMove(nodes, workingPods, resourceType)
		if !improved {
			break
		}
		moves = append(moves, move)
		workingPods = applyMove(workingPods, move)
	}

	after := analyzer.Analyze(nodes, workingPods)

	return &Plan{
		Moves:          moves,
		BeforeScore:    before.ImbalanceScore,
		AfterScore:     after.ImbalanceScore,
		BeforeAnalysis: before,
		AfterAnalysis:  after,
	}
}

func findBestMove(nodes []kube.NodeInfo, pods []kube.PodInfo, resourceType string) (Move, bool) {
	current := analyzer.Analyze(nodes, pods)
	snap := analyzer.BuildSnapshot(nodes, pods)
	bestScore := current.ImbalanceScore
	var bestMove Move
	found := false

	sortedNodes := rankNodes(current.Nodes, resourceType)
	if len(sortedNodes) < 2 {
		return bestMove, false
	}

	for srcIdx := len(sortedNodes) - 1; srcIdx >= len(sortedNodes)/2; srcIdx-- {
		srcNode := sortedNodes[srcIdx].Name
		srcPods := podsOnNode(pods, srcNode)

		for dstIdx := 0; dstIdx < len(sortedNodes)/2+1; dstIdx++ {
			if dstIdx == srcIdx {
				continue
			}
			dstNodeName := sortedNodes[dstIdx].Name
			dstNode := findNode(nodes, dstNodeName)
			if dstNode == nil {
				continue
			}

			for _, pod := range srcPods {
				if !IsMovable(pod) {
					continue
				}
				if !CanScheduleOn(pod, *dstNode, pods, nodes) {
					continue
				}

				cpuM := pod.Requests.CPU.MilliValue()
				memB := pod.Requests.Memory.Value()
				snap.MovePod(srcNode, dstNodeName, cpuM, memB)
				trialScore := snap.Score()
				snap.RevertPod(srcNode, dstNodeName, cpuM, memB)

				if trialScore < bestScore {
					bestScore = trialScore
					bestMove = Move{
						PodName:      pod.Name,
						PodNamespace: pod.Namespace,
						FromNode:     srcNode,
						ToNode:       dstNodeName,
					}
					found = true
				}
			}
		}
	}

	return bestMove, found
}

func rankNodes(nodeUtils []analyzer.NodeUtilization, resourceType string) []analyzer.NodeUtilization {
	ranked := make([]analyzer.NodeUtilization, len(nodeUtils))
	copy(ranked, nodeUtils)
	sort.Slice(ranked, func(i, j int) bool {
		if resourceType == "memory" {
			return ranked[i].MemPercent < ranked[j].MemPercent
		}
		return ranked[i].CPUPercent < ranked[j].CPUPercent
	})
	return ranked
}

func podsOnNode(pods []kube.PodInfo, nodeName string) []kube.PodInfo {
	var result []kube.PodInfo
	for _, p := range pods {
		if p.NodeName == nodeName {
			result = append(result, p)
		}
	}
	return result
}

func findNode(nodes []kube.NodeInfo, name string) *kube.NodeInfo {
	for i := range nodes {
		if nodes[i].Name == name {
			return &nodes[i]
		}
	}
	return nil
}

func clonePods(pods []kube.PodInfo) []kube.PodInfo {
	result := make([]kube.PodInfo, len(pods))
	copy(result, pods)
	return result
}

func applyMove(pods []kube.PodInfo, move Move) []kube.PodInfo {
	result := clonePods(pods)
	for i := range result {
		if result[i].Name == move.PodName && result[i].Namespace == move.PodNamespace && result[i].NodeName == move.FromNode {
			result[i].NodeName = move.ToNode
			break
		}
	}
	return result
}
