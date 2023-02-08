/*
Copyright 2022 The Koordinator Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package nodereservation

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	resschedplug "k8s.io/kubernetes/pkg/scheduler/framework/plugins/noderesources"

	apiext "github.com/koordinator-sh/koordinator/apis/extension"
	"github.com/koordinator-sh/koordinator/pkg/util"
)

const (
	Name              = "NodeReservation"
	preFilterStateKey = "PreFilter" + Name
)

var (
	_ framework.FilterPlugin    = &Plugin{}
	_ framework.PreFilterPlugin = &Plugin{}
	//_ framework.ScorePlugin = &Plugin{}
)

type Plugin struct {
}

// preFilterState computed at PreFilter and used at Filter.
type preFilterState struct {
	framework.Resource
}

// Clone the prefilter state.
func (s *preFilterState) Clone() framework.StateData {
	return s
}

func New(args runtime.Object, handle framework.Handle) (framework.Plugin, error) {
	return &Plugin{}, nil
}

func (p *Plugin) Name() string {
	return Name
}

// PreFilter invoked at the prefilter extension point.
func (f *Plugin) PreFilter(ctx context.Context, cycleState *framework.CycleState, pod *corev1.Pod) *framework.Status {
	cycleState.Write(preFilterStateKey, computePodResourceRequest(pod))
	return nil
}

// PreFilterExtensions returns prefilter extensions, pod add and remove.
func (f *Plugin) PreFilterExtensions() framework.PreFilterExtensions {
	return nil
}

// computePodResourceRequest returns a framework.Resource that covers the largest
// width in each resource dimension. Because init-containers run sequentially, we collect
// the max in each dimension iteratively. In contrast, we sum the resource vectors for
// regular containers since they run simultaneously.
//
// Example:
//
// Pod:
//   InitContainers
//     IC1:
//       CPU: 2
//       Memory: 1G
//     IC2:
//       CPU: 2
//       Memory: 3G
//   Containers
//     C1:
//       CPU: 2
//       Memory: 1G
//     C2:
//       CPU: 1
//       Memory: 1G
//
// Result: CPU: 3, Memory: 3G
func computePodResourceRequest(pod *corev1.Pod) *preFilterState {
	result := &preFilterState{}
	for _, container := range pod.Spec.Containers {
		result.Add(container.Resources.Requests)
	}

	// take max_resource(sum_pod, any_init_container)
	for _, container := range pod.Spec.InitContainers {
		result.SetMaxResource(container.Resources.Requests)
	}

	// If Overhead is being utilized, add to the total requests for the pod
	if pod.Spec.Overhead != nil {
		result.Add(pod.Spec.Overhead)
	}

	return result
}

func getPreFilterState(cycleState *framework.CycleState) (*preFilterState, error) {
	c, err := cycleState.Read(preFilterStateKey)
	if err != nil {
		// preFilterState doesn't exist, likely PreFilter wasn't invoked.
		return nil, fmt.Errorf("error reading %q from cycleState: %w", preFilterStateKey, err)
	}

	s, ok := c.(*preFilterState)
	if !ok {
		return nil, fmt.Errorf("%+v  convert to NodeResourcesFit.preFilterState error", c)
	}
	return s, nil
}

func (f *Plugin) Filter(ctx context.Context, cycleState *framework.CycleState, pod *corev1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {
	podQos := apiext.GetPodQoSClass(pod)
	cpuReservedByNode := *resource.NewMilliQuantity(0, resource.DecimalSI)
	node := nodeInfo.Node()
	if node != nil {
		if reserved, ok := node.Annotations[apiext.ReservedByNode]; ok {
			cpus, qos := util.GetCPUsReservedByNode(reserved)
			if qos == podQos {
				cpuReservedByNode = cpus
			}
		}
	}

	s, err := getPreFilterState(cycleState)
	if err != nil {
		return framework.AsStatus(err)
	}

	insufficientResources := fitsRequest(s, nodeInfo, cpuReservedByNode)

	if len(insufficientResources) != 0 {
		// We will keep all failure reasons.
		failureReasons := make([]string, 0, len(insufficientResources))
		for _, r := range insufficientResources {
			failureReasons = append(failureReasons, r.Reason)
		}
		return framework.NewStatus(framework.Unschedulable, failureReasons...)
	}

	return nil
}

type InsufficientResource struct {
	resschedplug.InsufficientResource
	reservedByNode int64
}

func fitsRequest(podRequest *preFilterState, nodeInfo *framework.NodeInfo, cpuReservedByNode resource.Quantity) []InsufficientResource {
	insufficientResources := make([]InsufficientResource, 0, 2)
	if podRequest.MilliCPU == 0 && podRequest.Memory == 0 {
		return insufficientResources
	}

	if podRequest.MilliCPU > (nodeInfo.Allocatable.MilliCPU - nodeInfo.Requested.MilliCPU - cpuReservedByNode.MilliValue()) {
		insufficientResources = append(insufficientResources, InsufficientResource{
			InsufficientResource: resschedplug.InsufficientResource{
				ResourceName: corev1.ResourceCPU,
				Reason:       "Insufficient cpu",
				Requested:    podRequest.MilliCPU,
				Used:         nodeInfo.Requested.MilliCPU,
				Capacity:     nodeInfo.Allocatable.MilliCPU,
			},
			reservedByNode: cpuReservedByNode.MilliValue(),
		})
	}
	if podRequest.Memory > (nodeInfo.Allocatable.Memory - nodeInfo.Requested.Memory) {
		insufficientResources = append(insufficientResources, InsufficientResource{
			InsufficientResource: resschedplug.InsufficientResource{
				ResourceName: corev1.ResourceMemory,
				Reason:       "Insufficient memory",
				Requested:    podRequest.Memory,
				Used:         nodeInfo.Requested.Memory,
				Capacity:     nodeInfo.Allocatable.Memory,
			},
		})
	}

	return insufficientResources
}
