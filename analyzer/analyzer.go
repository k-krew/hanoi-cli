package analyzer

import (
	"math"

	"hanoi-cli/kube"
)

const hotspotThreshold = 0.80

type NodeUtilization struct {
	Name            string
	CPURequestsM    int64
	CPUAllocatableM int64
	CPUPercent      float64
	MemRequestsB    int64
	MemAllocatableB int64
	MemPercent      float64
	IsHotspot       bool
	Cordoned        bool
	PodCount        int
}

type ClusterAnalysis struct {
	Nodes          []NodeUtilization
	CPUMean        float64
	MemMean        float64
	CPUStdDev      float64
	MemStdDev      float64
	ImbalanceScore float64
	Hotspots       []string
}

func Analyze(nodes []kube.NodeInfo, pods []kube.PodInfo) *ClusterAnalysis {
	podsByNode := groupPodsByNode(pods)
	nodeUtils := computeUtilizations(nodes, podsByNode)

	cpuValues := make([]float64, len(nodeUtils))
	memValues := make([]float64, len(nodeUtils))
	for i, nu := range nodeUtils {
		cpuValues[i] = nu.CPUPercent
		memValues[i] = nu.MemPercent
	}

	cpuMean := mean(cpuValues)
	memMean := mean(memValues)
	cpuStd := stddev(cpuValues, cpuMean)
	memStd := stddev(memValues, memMean)

	var hotspots []string
	for i := range nodeUtils {
		if nodeUtils[i].CPUPercent >= hotspotThreshold || nodeUtils[i].MemPercent >= hotspotThreshold {
			nodeUtils[i].IsHotspot = true
			hotspots = append(hotspots, nodeUtils[i].Name)
		}
	}

	score := imbalanceScore(cpuStd, memStd)

	return &ClusterAnalysis{
		Nodes:          nodeUtils,
		CPUMean:        cpuMean,
		MemMean:        memMean,
		CPUStdDev:      cpuStd,
		MemStdDev:      memStd,
		ImbalanceScore: score,
		Hotspots:       hotspots,
	}
}

func groupPodsByNode(pods []kube.PodInfo) map[string][]kube.PodInfo {
	m := make(map[string][]kube.PodInfo)
	for _, p := range pods {
		if p.NodeName != "" {
			m[p.NodeName] = append(m[p.NodeName], p)
		}
	}
	return m
}

func computeUtilizations(nodes []kube.NodeInfo, podsByNode map[string][]kube.PodInfo) []NodeUtilization {
	result := make([]NodeUtilization, 0, len(nodes))
	for _, n := range nodes {
		allocCPU := n.Allocatable.CPU.MilliValue()
		allocMem := n.Allocatable.Memory.Value()

		var reqCPU, reqMem int64
		nodePods := podsByNode[n.Name]
		for _, p := range nodePods {
			reqCPU += p.Requests.CPU.MilliValue()
			reqMem += p.Requests.Memory.Value()
		}

		cpuPct := safeDivide(float64(reqCPU), float64(allocCPU))
		memPct := safeDivide(float64(reqMem), float64(allocMem))

		result = append(result, NodeUtilization{
			Name:            n.Name,
			CPURequestsM:    reqCPU,
			CPUAllocatableM: allocCPU,
			CPUPercent:      cpuPct,
			MemRequestsB:    reqMem,
			MemAllocatableB: allocMem,
			MemPercent:      memPct,
			Cordoned:        n.Unschedulable,
			PodCount:        len(nodePods),
		})
	}
	return result
}

func safeDivide(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func stddev(values []float64, avg float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sumSq float64
	for _, v := range values {
		d := v - avg
		sumSq += d * d
	}
	return math.Sqrt(sumSq / float64(len(values)))
}

func imbalanceScore(cpuStd, memStd float64) float64 {
	combined := (cpuStd + memStd) / 2.0
	maxStdDev := 0.5
	score := (combined / maxStdDev) * 100.0
	return math.Min(score, 100.0)
}

type NodeResourceSnapshot struct {
	Nodes    []string
	AllocCPU map[string]int64
	AllocMem map[string]int64
	UsedCPU  map[string]int64
	UsedMem  map[string]int64
}

func BuildSnapshot(nodes []kube.NodeInfo, pods []kube.PodInfo) *NodeResourceSnapshot {
	s := &NodeResourceSnapshot{
		Nodes:    make([]string, len(nodes)),
		AllocCPU: make(map[string]int64, len(nodes)),
		AllocMem: make(map[string]int64, len(nodes)),
		UsedCPU:  make(map[string]int64, len(nodes)),
		UsedMem:  make(map[string]int64, len(nodes)),
	}
	for i, n := range nodes {
		s.Nodes[i] = n.Name
		s.AllocCPU[n.Name] = n.Allocatable.CPU.MilliValue()
		s.AllocMem[n.Name] = n.Allocatable.Memory.Value()
	}
	for _, p := range pods {
		if p.NodeName != "" {
			s.UsedCPU[p.NodeName] += p.Requests.CPU.MilliValue()
			s.UsedMem[p.NodeName] += p.Requests.Memory.Value()
		}
	}
	return s
}

func (s *NodeResourceSnapshot) Score() float64 {
	n := len(s.Nodes)
	if n == 0 {
		return 0
	}
	cpuVals := make([]float64, n)
	memVals := make([]float64, n)
	for i, name := range s.Nodes {
		cpuVals[i] = safeDivide(float64(s.UsedCPU[name]), float64(s.AllocCPU[name]))
		memVals[i] = safeDivide(float64(s.UsedMem[name]), float64(s.AllocMem[name]))
	}
	cpuMean := mean(cpuVals)
	memMean := mean(memVals)
	return imbalanceScore(stddev(cpuVals, cpuMean), stddev(memVals, memMean))
}

func (s *NodeResourceSnapshot) MovePod(from, to string, cpuMilli, memBytes int64) {
	s.UsedCPU[from] -= cpuMilli
	s.UsedMem[from] -= memBytes
	s.UsedCPU[to] += cpuMilli
	s.UsedMem[to] += memBytes
}

func (s *NodeResourceSnapshot) RevertPod(from, to string, cpuMilli, memBytes int64) {
	s.UsedCPU[from] += cpuMilli
	s.UsedMem[from] += memBytes
	s.UsedCPU[to] -= cpuMilli
	s.UsedMem[to] -= memBytes
}
