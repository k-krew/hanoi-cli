package planner

import (
	"fmt"

	"hanoi-cli/analyzer"
	"hanoi-cli/kube"

	corev1 "k8s.io/api/core/v1"
)

type NodeCandidate struct {
	Name                        string
	Eligible                    bool
	Reason                      string
	CPUBefore                   float64
	CPUAfter                    float64
	MemBefore                   float64
	MemAfter                    float64
	ScoreIfChosen               float64
	HasPodAffinity              bool
	PodAffinityOK               bool
	HasAntiAffinity             bool
	AntiAffinityOK              bool
	PreferredAntiAffinityWeight int32
}

type MoveExplanation struct {
	MoveNumber   int
	PodName      string
	PodNamespace string
	PodOwner     string
	PodCPU       int64
	PodMem       int64
	FromNode     string
	ToNode       string
	FromCPU      float64
	FromMem      float64
	IsSimulation bool
	ScoreBefore  float64
	ScoreAfter   float64
	Candidates   []NodeCandidate
}

func ExplainMove(
	nodes []kube.NodeInfo,
	pods []kube.PodInfo,
	moveNum int,
	podName, podNamespace, fromNode, toNode string,
) *MoveExplanation {
	var pod kube.PodInfo
	found := false
	for _, p := range pods {
		if p.Name == podName && p.Namespace == podNamespace {
			pod = p
			found = true
			break
		}
	}
	if !found {
		return nil
	}

	beforeAnalysis := analyzer.Analyze(nodes, pods)
	isSimulation := findNode(nodes, fromNode) == nil
	var fromCPU, fromMem float64
	if !isSimulation {
		for _, n := range beforeAnalysis.Nodes {
			if n.Name == fromNode {
				fromCPU = n.CPUPercent
				fromMem = n.MemPercent
				break
			}
		}
	}

	movedPods := make([]kube.PodInfo, len(pods))
	copy(movedPods, pods)
	for i := range movedPods {
		if movedPods[i].Name == podName && movedPods[i].Namespace == podNamespace && movedPods[i].NodeName == fromNode {
			movedPods[i].NodeName = toNode
			break
		}
	}
	afterAnalysis := analyzer.Analyze(nodes, movedPods)

	candidates := evaluateCandidates(nodes, pods, pod, fromNode, beforeAnalysis)

	ownerStr := ""
	if pod.OwnerKind != "" {
		ownerStr = pod.OwnerKind + "/" + pod.OwnerName
	}

	return &MoveExplanation{
		MoveNumber:   moveNum,
		PodName:      podName,
		PodNamespace: podNamespace,
		PodOwner:     ownerStr,
		PodCPU:       pod.Requests.CPU.MilliValue(),
		PodMem:       pod.Requests.Memory.Value(),
		FromNode:     fromNode,
		ToNode:       toNode,
		FromCPU:      fromCPU,
		FromMem:      fromMem,
		IsSimulation: isSimulation,
		ScoreBefore:  beforeAnalysis.ImbalanceScore,
		ScoreAfter:   afterAnalysis.ImbalanceScore,
		Candidates:   candidates,
	}
}

func evaluateCandidates(
	nodes []kube.NodeInfo,
	pods []kube.PodInfo,
	pod kube.PodInfo,
	fromNode string,
	before *analyzer.ClusterAnalysis,
) []NodeCandidate {
	snap := analyzer.BuildSnapshot(nodes, pods)
	cpuM := pod.Requests.CPU.MilliValue()
	memB := pod.Requests.Memory.Value()
	var candidates []NodeCandidate

	for _, node := range nodes {
		if node.Name == fromNode {
			continue
		}

		var cpuBefore, memBefore float64
		for _, n := range before.Nodes {
			if n.Name == node.Name {
				cpuBefore = n.CPUPercent
				memBefore = n.MemPercent
				break
			}
		}

		reason := rejectReason(pod, node, pods, nodes)
		if reason != "" {
			candidates = append(candidates, NodeCandidate{
				Name:     node.Name,
				Eligible: false,
				Reason:   reason,
			})
			continue
		}

		snap.MovePod(fromNode, node.Name, cpuM, memB)
		trialScore := snap.Score()

		allocCPU := snap.AllocCPU[node.Name]
		allocMem := snap.AllocMem[node.Name]
		var cpuAfter, memAfter float64
		if allocCPU > 0 {
			cpuAfter = float64(snap.UsedCPU[node.Name]) / float64(allocCPU)
		}
		if allocMem > 0 {
			memAfter = float64(snap.UsedMem[node.Name]) / float64(allocMem)
		}

		snap.RevertPod(fromNode, node.Name, cpuM, memB)

		hasPodAff := pod.Affinity != nil && pod.Affinity.PodAffinity != nil &&
			len(pod.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution) > 0
		hasAntiAff := pod.Affinity != nil && pod.Affinity.PodAntiAffinity != nil &&
			len(pod.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution) > 0

		candidates = append(candidates, NodeCandidate{
			Name:                        node.Name,
			Eligible:                    true,
			CPUBefore:                   cpuBefore,
			CPUAfter:                    cpuAfter,
			MemBefore:                   memBefore,
			MemAfter:                    memAfter,
			ScoreIfChosen:               trialScore,
			HasPodAffinity:              hasPodAff,
			PodAffinityOK:               !hasPodAff || matchesPodAffinity(pod, node, pods, nodes),
			HasAntiAffinity:             hasAntiAff,
			AntiAffinityOK:              !hasAntiAff || matchesPodAntiAffinity(pod, node, pods, nodes),
			PreferredAntiAffinityWeight: PreferredAntiAffinityWeight(pod, node, pods, nodes),
		})
	}

	return candidates
}

func rejectReason(pod kube.PodInfo, node kube.NodeInfo, allPods []kube.PodInfo, allNodes []kube.NodeInfo) string {
	if node.Unschedulable {
		return "node is cordoned (unschedulable)"
	}
	for k, v := range pod.NodeSelector {
		if node.Labels[k] != v {
			return fmt.Sprintf("nodeSelector %s=%s not matched", k, v)
		}
	}
	if pod.Affinity != nil && pod.Affinity.NodeAffinity != nil {
		req := pod.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
		if req != nil {
			matched := false
			for _, term := range req.NodeSelectorTerms {
				if matchesSelectorTerm(term, node.Labels) {
					matched = true
					break
				}
			}
			if !matched {
				return "node affinity rules not satisfied"
			}
		}
	}
	for _, taint := range node.Taints {
		if taint.Effect == corev1.TaintEffectNoSchedule || taint.Effect == corev1.TaintEffectNoExecute {
			if !isTaintTolerated(taint, pod.Tolerations) {
				return fmt.Sprintf("taint %s=%s:%s not tolerated", taint.Key, taint.Value, taint.Effect)
			}
		}
	}
	if !matchesPodAffinity(pod, node, allPods, allNodes) {
		return "required pod affinity not satisfied"
	}
	if !matchesPodAntiAffinity(pod, node, allPods, allNodes) {
		return "pod anti-affinity conflict"
	}
	return ""
}
