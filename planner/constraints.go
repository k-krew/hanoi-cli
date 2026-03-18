package planner

import (
	"hanoi-cli/kube"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func IsMovable(pod kube.PodInfo) bool {
	return pod.OwnerKind != "DaemonSet"
}

func CanScheduleOn(pod kube.PodInfo, node kube.NodeInfo, allPods []kube.PodInfo, allNodes []kube.NodeInfo) bool {
	if node.Unschedulable {
		return false
	}
	if !matchesNodeSelector(pod, node) {
		return false
	}
	if !toleratesTaints(pod, node) {
		return false
	}
	if !matchesNodeAffinity(pod, node) {
		return false
	}
	if !matchesPodAffinity(pod, node, allPods, allNodes) {
		return false
	}
	if !matchesPodAntiAffinity(pod, node, allPods, allNodes) {
		return false
	}
	return true
}

func matchesNodeSelector(pod kube.PodInfo, node kube.NodeInfo) bool {
	for k, v := range pod.NodeSelector {
		if node.Labels[k] != v {
			return false
		}
	}
	return true
}

func matchesNodeAffinity(pod kube.PodInfo, node kube.NodeInfo) bool {
	if pod.Affinity == nil || pod.Affinity.NodeAffinity == nil {
		return true
	}
	req := pod.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
	if req == nil {
		return true
	}
	for _, term := range req.NodeSelectorTerms {
		if matchesSelectorTerm(term, node.Labels) {
			return true
		}
	}
	return false
}

func matchesSelectorTerm(term corev1.NodeSelectorTerm, nodeLabels map[string]string) bool {
	for _, expr := range term.MatchExpressions {
		if !matchesExpression(expr, nodeLabels) {
			return false
		}
	}
	return true
}

func matchesExpression(expr corev1.NodeSelectorRequirement, nodeLabels map[string]string) bool {
	val, exists := nodeLabels[expr.Key]
	switch expr.Operator {
	case corev1.NodeSelectorOpIn:
		if !exists {
			return false
		}
		for _, v := range expr.Values {
			if v == val {
				return true
			}
		}
		return false
	case corev1.NodeSelectorOpNotIn:
		if !exists {
			return true
		}
		for _, v := range expr.Values {
			if v == val {
				return false
			}
		}
		return true
	case corev1.NodeSelectorOpExists:
		return exists
	case corev1.NodeSelectorOpDoesNotExist:
		return !exists
	default:
		return false
	}
}

func toleratesTaints(pod kube.PodInfo, node kube.NodeInfo) bool {
	for _, taint := range node.Taints {
		if taint.Effect == corev1.TaintEffectNoSchedule || taint.Effect == corev1.TaintEffectNoExecute {
			if !isTaintTolerated(taint, pod.Tolerations) {
				return false
			}
		}
	}
	return true
}

func isTaintTolerated(taint corev1.Taint, tolerations []corev1.Toleration) bool {
	for _, tol := range tolerations {
		if tol.Operator == corev1.TolerationOpExists && tol.Key == "" {
			return true
		}
		if tol.Key == taint.Key {
			if tol.Operator == corev1.TolerationOpExists {
				if tol.Effect == "" || tol.Effect == taint.Effect {
					return true
				}
			}
			if tol.Operator == "" || tol.Operator == corev1.TolerationOpEqual {
				if tol.Value == taint.Value && (tol.Effect == "" || tol.Effect == taint.Effect) {
					return true
				}
			}
		}
	}
	return false
}

func matchesPodAffinity(pod kube.PodInfo, node kube.NodeInfo, allPods []kube.PodInfo, allNodes []kube.NodeInfo) bool {
	if pod.Affinity == nil || pod.Affinity.PodAffinity == nil {
		return true
	}
	for _, term := range pod.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution {
		if !affinityTermSatisfied(term, node, allPods, allNodes) {
			return false
		}
	}
	return true
}

func matchesPodAntiAffinity(pod kube.PodInfo, node kube.NodeInfo, allPods []kube.PodInfo, allNodes []kube.NodeInfo) bool {
	if pod.Affinity == nil || pod.Affinity.PodAntiAffinity == nil {
		return true
	}
	for _, term := range pod.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution {
		if antiAffinityTermViolated(term, node, allPods, allNodes) {
			return false
		}
	}

	for _, existing := range allPods {
		if existing.NodeName == "" {
			continue
		}
		if existing.Affinity == nil || existing.Affinity.PodAntiAffinity == nil {
			continue
		}
		for _, term := range existing.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution {
			if sameTopologyDomain(node, existingNodeInfo(existing.NodeName, allNodes), term.TopologyKey) &&
				podMatchesSelector(pod, term.LabelSelector, term.Namespaces, existing.Namespace) {
				return false
			}
		}
	}
	return true
}

func affinityTermSatisfied(term corev1.PodAffinityTerm, targetNode kube.NodeInfo, allPods []kube.PodInfo, allNodes []kube.NodeInfo) bool {
	for _, existing := range allPods {
		if existing.NodeName == "" {
			continue
		}
		existNode := existingNodeInfo(existing.NodeName, allNodes)
		if existNode == nil {
			continue
		}
		if !sameTopologyDomain(targetNode, existNode, term.TopologyKey) {
			continue
		}
		if podMatchesSelector(existing, term.LabelSelector, term.Namespaces, existing.Namespace) {
			return true
		}
	}
	return false
}

func antiAffinityTermViolated(term corev1.PodAffinityTerm, targetNode kube.NodeInfo, allPods []kube.PodInfo, allNodes []kube.NodeInfo) bool {
	for _, existing := range allPods {
		if existing.NodeName == "" {
			continue
		}
		existNode := existingNodeInfo(existing.NodeName, allNodes)
		if existNode == nil {
			continue
		}
		if !sameTopologyDomain(targetNode, existNode, term.TopologyKey) {
			continue
		}
		if podMatchesSelector(existing, term.LabelSelector, term.Namespaces, existing.Namespace) {
			return true
		}
	}
	return false
}

func sameTopologyDomain(a kube.NodeInfo, b *kube.NodeInfo, topologyKey string) bool {
	if b == nil || topologyKey == "" {
		return false
	}
	aVal, aOk := a.Labels[topologyKey]
	bVal, bOk := b.Labels[topologyKey]
	return aOk && bOk && aVal == bVal
}

func existingNodeInfo(nodeName string, allNodes []kube.NodeInfo) *kube.NodeInfo {
	for i := range allNodes {
		if allNodes[i].Name == nodeName {
			return &allNodes[i]
		}
	}
	return nil
}

func podMatchesSelector(pod kube.PodInfo, selector *metav1.LabelSelector, namespaces []string, selectorNamespace string) bool {
	if len(namespaces) > 0 {
		found := false
		for _, ns := range namespaces {
			if ns == pod.Namespace {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	} else if selectorNamespace != "" && pod.Namespace != selectorNamespace {
		return false
	}

	if selector == nil {
		return true
	}
	sel, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return false
	}
	return sel.Matches(labels.Set(pod.Labels))
}

func PreferredAntiAffinityWeight(pod kube.PodInfo, node kube.NodeInfo, allPods []kube.PodInfo, allNodes []kube.NodeInfo) int32 {
	if pod.Affinity == nil || pod.Affinity.PodAntiAffinity == nil {
		return 0
	}
	var totalWeight int32
	for _, pref := range pod.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
		if preferredAntiAffinityViolated(pref.PodAffinityTerm, node, allPods, allNodes) {
			totalWeight += pref.Weight
		}
	}
	return totalWeight
}

func preferredAntiAffinityViolated(term corev1.PodAffinityTerm, targetNode kube.NodeInfo, allPods []kube.PodInfo, allNodes []kube.NodeInfo) bool {
	for _, existing := range allPods {
		if existing.NodeName == "" {
			continue
		}
		existNode := existingNodeInfo(existing.NodeName, allNodes)
		if existNode == nil {
			continue
		}
		if !sameTopologyDomain(targetNode, existNode, term.TopologyKey) {
			continue
		}
		if podMatchesSelector(existing, term.LabelSelector, term.Namespaces, existing.Namespace) {
			return true
		}
	}
	return false
}
