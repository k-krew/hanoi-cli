package simulator

import (
	"hanoi-cli/analyzer"
	"hanoi-cli/kube"
	"hanoi-cli/planner"
	"math"
)

type UnschedulablePod struct {
	Name      string
	Namespace string
	OwnerKind string
	OwnerName string
}

type RescheduleMove struct {
	PodName      string
	PodNamespace string
	FromNode     string
	ToNode       string
}

type SimulationResult struct {
	FailedNode      string
	Feasible        bool
	BeforeAnalysis  *analyzer.ClusterAnalysis
	AfterAnalysis   *analyzer.ClusterAnalysis
	BeforeScore     float64
	AfterScore      float64
	DisplacedPods   int
	RescheduledPods int
	Moves           []RescheduleMove
	Unschedulable   []UnschedulablePod
}

func SimulateNodeFailure(nodes []kube.NodeInfo, pods []kube.PodInfo, targetNode string) *SimulationResult {
	before := analyzer.Analyze(nodes, pods)

	survivingNodes := excludeNode(nodes, targetNode)
	displacedPods := podsOnNode(pods, targetNode)
	remainingPods := podsNotOnNode(pods, targetNode)

	snap := analyzer.BuildSnapshot(survivingNodes, remainingPods)
	var moves []RescheduleMove
	var unschedulable []UnschedulablePod

	for _, pod := range displacedPods {
		if !planner.IsMovable(pod) {
			continue
		}
		dest := findBestTarget(pod, survivingNodes, remainingPods, snap)
		if dest != "" {
			moves = append(moves, RescheduleMove{
				PodName:      pod.Name,
				PodNamespace: pod.Namespace,
				FromNode:     targetNode,
				ToNode:       dest,
			})
			snap.UsedCPU[dest] += pod.Requests.CPU.MilliValue()
			snap.UsedMem[dest] += pod.Requests.Memory.Value()
			pod.NodeName = dest
			remainingPods = append(remainingPods, pod)
		} else {
			unschedulable = append(unschedulable, UnschedulablePod{
				Name:      pod.Name,
				Namespace: pod.Namespace,
				OwnerKind: pod.OwnerKind,
				OwnerName: pod.OwnerName,
			})
		}
	}

	after := analyzer.Analyze(survivingNodes, remainingPods)

	return &SimulationResult{
		FailedNode:      targetNode,
		Feasible:        len(unschedulable) == 0,
		BeforeAnalysis:  before,
		AfterAnalysis:   after,
		BeforeScore:     before.ImbalanceScore,
		AfterScore:      after.ImbalanceScore,
		DisplacedPods:   len(displacedPods),
		RescheduledPods: len(moves),
		Moves:           moves,
		Unschedulable:   unschedulable,
	}
}

func findBestTarget(pod kube.PodInfo, nodes []kube.NodeInfo, currentPods []kube.PodInfo, snap *analyzer.NodeResourceSnapshot) string {
	bestNode := ""
	bestScore := math.MaxFloat64
	cpuM := pod.Requests.CPU.MilliValue()
	memB := pod.Requests.Memory.Value()

	for _, n := range nodes {
		if !planner.CanScheduleOn(pod, n, currentPods, nodes) {
			continue
		}

		allocCPU := n.Allocatable.CPU.MilliValue()
		allocMem := n.Allocatable.Memory.Value()
		if allocCPU == 0 || allocMem == 0 {
			continue
		}

		cpuAfter := float64(snap.UsedCPU[n.Name]+cpuM) / float64(allocCPU)
		memAfter := float64(snap.UsedMem[n.Name]+memB) / float64(allocMem)
		if cpuAfter > 1.0 || memAfter > 1.0 {
			continue
		}

		snap.UsedCPU[n.Name] += cpuM
		snap.UsedMem[n.Name] += memB
		score := snap.Score()
		snap.UsedCPU[n.Name] -= cpuM
		snap.UsedMem[n.Name] -= memB

		if score < bestScore {
			bestScore = score
			bestNode = n.Name
		}
	}

	return bestNode
}

func excludeNode(nodes []kube.NodeInfo, name string) []kube.NodeInfo {
	result := make([]kube.NodeInfo, 0, len(nodes)-1)
	for _, n := range nodes {
		if n.Name != name {
			result = append(result, n)
		}
	}
	return result
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

func podsNotOnNode(pods []kube.PodInfo, nodeName string) []kube.PodInfo {
	var result []kube.PodInfo
	for _, p := range pods {
		if p.NodeName != nodeName {
			result = append(result, p)
		}
	}
	return result
}
