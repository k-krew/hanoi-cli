package planner

import (
	"testing"

	"hanoi-cli/kube"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func simpleNode(name string, labels map[string]string, taints []corev1.Taint, unschedulable bool) kube.NodeInfo {
	return kube.NodeInfo{
		Name:          name,
		Labels:        labels,
		Taints:        taints,
		Unschedulable: unschedulable,
		Allocatable: kube.Resources{
			CPU:    *resource.NewMilliQuantity(4000, resource.DecimalSI),
			Memory: *resource.NewQuantity(8000000000, resource.BinarySI),
		},
	}
}

func simplePod(name, ownerKind string) kube.PodInfo {
	return kube.PodInfo{
		Name:      name,
		Namespace: "default",
		OwnerKind: ownerKind,
		OwnerName: "test",
	}
}

func TestIsMovable_DaemonSet(t *testing.T) {
	ds := simplePod("ds-pod", "DaemonSet")
	if IsMovable(ds) {
		t.Error("DaemonSet pod should not be movable")
	}
}

func TestIsMovable_Deployment(t *testing.T) {
	dep := simplePod("dep-pod", "Deployment")
	if !IsMovable(dep) {
		t.Error("Deployment pod should be movable")
	}
}

func TestIsMovable_StatefulSet(t *testing.T) {
	ss := simplePod("ss-pod", "StatefulSet")
	if !IsMovable(ss) {
		t.Error("StatefulSet pod should be movable")
	}
}

func TestCanScheduleOn_Unschedulable(t *testing.T) {
	pod := simplePod("pod-a", "Deployment")
	node := simpleNode("node-1", nil, nil, true)
	if CanScheduleOn(pod, node, nil, nil) {
		t.Error("should not schedule on unschedulable node")
	}
}

func TestCanScheduleOn_NoConstraints(t *testing.T) {
	pod := simplePod("pod-a", "Deployment")
	node := simpleNode("node-1", map[string]string{"zone": "us-east-1a"}, nil, false)
	if !CanScheduleOn(pod, node, nil, nil) {
		t.Error("pod with no constraints should schedule on any schedulable node")
	}
}

func TestCanScheduleOn_TaintNoToleration(t *testing.T) {
	pod := simplePod("pod-a", "Deployment")
	node := simpleNode("node-1", nil, []corev1.Taint{
		{Key: "dedicated", Value: "gpu", Effect: corev1.TaintEffectNoSchedule},
	}, false)
	if CanScheduleOn(pod, node, nil, nil) {
		t.Error("pod without toleration should not schedule on tainted node")
	}
}

func TestCanScheduleOn_TaintWithToleration(t *testing.T) {
	pod := simplePod("pod-a", "Deployment")
	pod.Tolerations = []corev1.Toleration{
		{Key: "dedicated", Operator: corev1.TolerationOpEqual, Value: "gpu", Effect: corev1.TaintEffectNoSchedule},
	}
	node := simpleNode("node-1", nil, []corev1.Taint{
		{Key: "dedicated", Value: "gpu", Effect: corev1.TaintEffectNoSchedule},
	}, false)
	if !CanScheduleOn(pod, node, nil, nil) {
		t.Error("pod with matching toleration should schedule on tainted node")
	}
}

func TestCanScheduleOn_TolerationOpExists(t *testing.T) {
	pod := simplePod("pod-a", "Deployment")
	pod.Tolerations = []corev1.Toleration{
		{Key: "dedicated", Operator: corev1.TolerationOpExists},
	}
	node := simpleNode("node-1", nil, []corev1.Taint{
		{Key: "dedicated", Value: "anything", Effect: corev1.TaintEffectNoSchedule},
	}, false)
	if !CanScheduleOn(pod, node, nil, nil) {
		t.Error("Exists toleration should match any value for the key")
	}
}

func TestCanScheduleOn_WildcardToleration(t *testing.T) {
	pod := simplePod("pod-a", "Deployment")
	pod.Tolerations = []corev1.Toleration{
		{Operator: corev1.TolerationOpExists},
	}
	node := simpleNode("node-1", nil, []corev1.Taint{
		{Key: "any-key", Value: "any-val", Effect: corev1.TaintEffectNoSchedule},
	}, false)
	if !CanScheduleOn(pod, node, nil, nil) {
		t.Error("wildcard toleration should match all taints")
	}
}

func TestCanScheduleOn_NodeAffinityMatch(t *testing.T) {
	pod := simplePod("pod-a", "Deployment")
	pod.Affinity = &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{Key: "zone", Operator: corev1.NodeSelectorOpIn, Values: []string{"us-east-1a", "us-east-1b"}},
						},
					},
				},
			},
		},
	}
	node := simpleNode("node-1", map[string]string{"zone": "us-east-1a"}, nil, false)
	if !CanScheduleOn(pod, node, nil, nil) {
		t.Error("pod should schedule on node matching node affinity In expression")
	}
}

func TestCanScheduleOn_NodeAffinityNoMatch(t *testing.T) {
	pod := simplePod("pod-a", "Deployment")
	pod.Affinity = &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{Key: "zone", Operator: corev1.NodeSelectorOpIn, Values: []string{"us-west-2a"}},
						},
					},
				},
			},
		},
	}
	node := simpleNode("node-1", map[string]string{"zone": "us-east-1a"}, nil, false)
	if CanScheduleOn(pod, node, nil, nil) {
		t.Error("pod should not schedule on node not matching node affinity")
	}
}

func TestCanScheduleOn_NodeAffinityNotIn(t *testing.T) {
	pod := simplePod("pod-a", "Deployment")
	pod.Affinity = &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{Key: "zone", Operator: corev1.NodeSelectorOpNotIn, Values: []string{"us-west-2a"}},
						},
					},
				},
			},
		},
	}
	node := simpleNode("node-1", map[string]string{"zone": "us-east-1a"}, nil, false)
	if !CanScheduleOn(pod, node, nil, nil) {
		t.Error("pod should schedule on node not in excluded values")
	}
}

func TestCanScheduleOn_NodeAffinityExists(t *testing.T) {
	pod := simplePod("pod-a", "Deployment")
	pod.Affinity = &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{Key: "gpu", Operator: corev1.NodeSelectorOpExists},
						},
					},
				},
			},
		},
	}

	withLabel := simpleNode("node-1", map[string]string{"gpu": "true"}, nil, false)
	if !CanScheduleOn(pod, withLabel, nil, nil) {
		t.Error("Exists should match when label is present")
	}

	withoutLabel := simpleNode("node-2", map[string]string{"zone": "us-east"}, nil, false)
	if CanScheduleOn(pod, withoutLabel, nil, nil) {
		t.Error("Exists should not match when label is absent")
	}
}

func TestCanScheduleOn_NodeAffinityDoesNotExist(t *testing.T) {
	pod := simplePod("pod-a", "Deployment")
	pod.Affinity = &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{Key: "gpu", Operator: corev1.NodeSelectorOpDoesNotExist},
						},
					},
				},
			},
		},
	}

	withLabel := simpleNode("node-1", map[string]string{"gpu": "true"}, nil, false)
	if CanScheduleOn(pod, withLabel, nil, nil) {
		t.Error("DoesNotExist should not match when label is present")
	}

	withoutLabel := simpleNode("node-2", map[string]string{"zone": "us-east"}, nil, false)
	if !CanScheduleOn(pod, withoutLabel, nil, nil) {
		t.Error("DoesNotExist should match when label is absent")
	}
}

func TestCanScheduleOn_NodeSelector(t *testing.T) {
	pod := simplePod("pod-a", "Deployment")
	pod.NodeSelector = map[string]string{"disktype": "ssd"}

	match := simpleNode("node-1", map[string]string{"disktype": "ssd"}, nil, false)
	if !CanScheduleOn(pod, match, nil, nil) {
		t.Error("pod should schedule on node matching nodeSelector")
	}

	noMatch := simpleNode("node-2", map[string]string{"disktype": "hdd"}, nil, false)
	if CanScheduleOn(pod, noMatch, nil, nil) {
		t.Error("pod should not schedule on node not matching nodeSelector")
	}
}

func TestCanScheduleOn_PodAntiAffinity(t *testing.T) {
	nodes := []kube.NodeInfo{
		simpleNode("node-1", map[string]string{"kubernetes.io/hostname": "node-1"}, nil, false),
		simpleNode("node-2", map[string]string{"kubernetes.io/hostname": "node-2"}, nil, false),
	}
	existing := kube.PodInfo{
		Name: "web-1", Namespace: "default", NodeName: "node-1",
		Labels: map[string]string{"app": "web"},
	}
	pod := simplePod("web-2", "Deployment")
	pod.Labels = map[string]string{"app": "web"}
	pod.Affinity = &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "web"},
					},
					TopologyKey: "kubernetes.io/hostname",
				},
			},
		},
	}
	allPods := []kube.PodInfo{existing}

	if CanScheduleOn(pod, nodes[0], allPods, nodes) {
		t.Error("pod anti-affinity should prevent scheduling on node-1")
	}
	if !CanScheduleOn(pod, nodes[1], allPods, nodes) {
		t.Error("pod anti-affinity should allow scheduling on node-2")
	}
}

func TestCanScheduleOn_PodAffinity(t *testing.T) {
	nodes := []kube.NodeInfo{
		simpleNode("node-1", map[string]string{"kubernetes.io/hostname": "node-1"}, nil, false),
		simpleNode("node-2", map[string]string{"kubernetes.io/hostname": "node-2"}, nil, false),
	}
	existing := kube.PodInfo{
		Name: "cache-1", Namespace: "default", NodeName: "node-1",
		Labels: map[string]string{"app": "cache"},
	}
	pod := simplePod("app-1", "Deployment")
	pod.Affinity = &corev1.Affinity{
		PodAffinity: &corev1.PodAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "cache"},
					},
					TopologyKey: "kubernetes.io/hostname",
				},
			},
		},
	}
	allPods := []kube.PodInfo{existing}

	if !CanScheduleOn(pod, nodes[0], allPods, nodes) {
		t.Error("pod affinity should allow scheduling on node-1 (cache pod present)")
	}
	if CanScheduleOn(pod, nodes[1], allPods, nodes) {
		t.Error("pod affinity should prevent scheduling on node-2 (no cache pod)")
	}
}
