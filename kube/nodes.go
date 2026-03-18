package kube

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NodeInfo struct {
	Name          string
	Labels        map[string]string
	Taints        []corev1.Taint
	Unschedulable bool
	Capacity      Resources
	Allocatable   Resources
}

type Resources struct {
	CPU    resource.Quantity
	Memory resource.Quantity
}

func (c *Client) GetNodes(ctx context.Context) ([]NodeInfo, error) {
	nodeList, err := c.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing nodes: %w", err)
	}

	nodes := make([]NodeInfo, 0, len(nodeList.Items))
	for _, n := range nodeList.Items {
		nodes = append(nodes, nodeInfoFrom(n))
	}
	return nodes, nil
}

func nodeInfoFrom(n corev1.Node) NodeInfo {
	return NodeInfo{
		Name:          n.Name,
		Labels:        n.Labels,
		Taints:        n.Spec.Taints,
		Unschedulable: n.Spec.Unschedulable,
		Capacity: Resources{
			CPU:    safeQuantity(n.Status.Capacity, corev1.ResourceCPU),
			Memory: safeQuantity(n.Status.Capacity, corev1.ResourceMemory),
		},
		Allocatable: Resources{
			CPU:    safeQuantity(n.Status.Allocatable, corev1.ResourceCPU),
			Memory: safeQuantity(n.Status.Allocatable, corev1.ResourceMemory),
		},
	}
}

func safeQuantity(rl corev1.ResourceList, name corev1.ResourceName) resource.Quantity {
	if rl == nil {
		return resource.Quantity{}
	}
	if q, ok := rl[name]; ok {
		return q
	}
	return resource.Quantity{}
}
