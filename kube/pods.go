package kube

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PodInfo struct {
	Name         string
	Namespace    string
	NodeName     string
	OwnerKind    string
	OwnerName    string
	Labels       map[string]string
	NodeSelector map[string]string
	Requests     Resources
	Limits       Resources
	Tolerations  []corev1.Toleration
	Affinity     *corev1.Affinity
}

func (c *Client) GetPods(ctx context.Context, namespace string) ([]PodInfo, error) {
	podList, err := c.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: "status.phase=Running",
	})
	if err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}

	pods := make([]PodInfo, 0, len(podList.Items))
	for _, p := range podList.Items {
		pods = append(pods, podInfoFrom(p))
	}
	return pods, nil
}

func podInfoFrom(p corev1.Pod) PodInfo {
	var ownerKind, ownerName string
	if len(p.OwnerReferences) > 0 {
		ownerKind = p.OwnerReferences[0].Kind
		ownerName = p.OwnerReferences[0].Name
	}

	requests, limits := aggregateContainerResources(p.Spec.Containers, p.Spec.InitContainers)

	return PodInfo{
		Name:         p.Name,
		Namespace:    p.Namespace,
		NodeName:     p.Spec.NodeName,
		OwnerKind:    ownerKind,
		OwnerName:    ownerName,
		Labels:       p.Labels,
		NodeSelector: p.Spec.NodeSelector,
		Requests:     requests,
		Limits:       limits,
		Tolerations:  p.Spec.Tolerations,
		Affinity:     p.Spec.Affinity,
	}
}

func aggregateContainerResources(containers, initContainers []corev1.Container) (Resources, Resources) {
	requests := Resources{
		CPU:    resource.Quantity{},
		Memory: resource.Quantity{},
	}
	limits := Resources{
		CPU:    resource.Quantity{},
		Memory: resource.Quantity{},
	}

	for _, c := range containers {
		if cpu, ok := c.Resources.Requests[corev1.ResourceCPU]; ok {
			requests.CPU.Add(cpu)
		}
		if mem, ok := c.Resources.Requests[corev1.ResourceMemory]; ok {
			requests.Memory.Add(mem)
		}
		if cpu, ok := c.Resources.Limits[corev1.ResourceCPU]; ok {
			limits.CPU.Add(cpu)
		}
		if mem, ok := c.Resources.Limits[corev1.ResourceMemory]; ok {
			limits.Memory.Add(mem)
		}
	}

	for _, ic := range initContainers {
		if cpu, ok := ic.Resources.Requests[corev1.ResourceCPU]; ok {
			if cpu.Cmp(requests.CPU) > 0 {
				requests.CPU = cpu.DeepCopy()
			}
		}
		if mem, ok := ic.Resources.Requests[corev1.ResourceMemory]; ok {
			if mem.Cmp(requests.Memory) > 0 {
				requests.Memory = mem.DeepCopy()
			}
		}
		if cpu, ok := ic.Resources.Limits[corev1.ResourceCPU]; ok {
			if cpu.Cmp(limits.CPU) > 0 {
				limits.CPU = cpu.DeepCopy()
			}
		}
		if mem, ok := ic.Resources.Limits[corev1.ResourceMemory]; ok {
			if mem.Cmp(limits.Memory) > 0 {
				limits.Memory = mem.DeepCopy()
			}
		}
	}

	return requests, limits
}
