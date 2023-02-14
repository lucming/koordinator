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
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/scheduler/framework"

	apiext "github.com/koordinator-sh/koordinator/apis/extension"
)

func makeResources(milliCPU, memory int64) corev1.NodeResources {
	return corev1.NodeResources{
		Capacity: corev1.ResourceList{
			corev1.ResourceCPU:    *resource.NewMilliQuantity(milliCPU, resource.DecimalSI),
			corev1.ResourceMemory: *resource.NewQuantity(memory, resource.BinarySI),
		},
	}
}

func makeAllocatableResources(milliCPU, memory int64) corev1.ResourceList {
	return corev1.ResourceList{
		corev1.ResourceCPU:    *resource.NewMilliQuantity(milliCPU, resource.DecimalSI),
		corev1.ResourceMemory: *resource.NewQuantity(memory, resource.BinarySI),
	}
}

func newResourcePod(usage ...framework.Resource) *corev1.Pod {
	var containers []corev1.Container
	for _, req := range usage {
		rl := corev1.ResourceList{
			corev1.ResourceCPU:    *resource.NewMilliQuantity(req.MilliCPU, resource.DecimalSI),
			corev1.ResourceMemory: *resource.NewQuantity(req.Memory, resource.BinarySI),
		}
		containers = append(containers, corev1.Container{
			Resources: corev1.ResourceRequirements{Requests: rl},
		})
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				apiext.LabelPodQoS: "LSE",
			},
		},
		Spec: corev1.PodSpec{
			Containers: containers,
		},
	}
}

func newResourceInitPod(pod *corev1.Pod, usage ...framework.Resource) *corev1.Pod {
	pod.Spec.InitContainers = newResourcePod(usage...).Spec.Containers
	return pod
}

func getErrReason(rn corev1.ResourceName) string {
	return fmt.Sprintf("Insufficient %v", rn)
}

func TestPlugin_Filter_Only_Specific_CPU_Quantity(t *testing.T) {
	type args struct {
		pod      *corev1.Pod
		nodeInfo *framework.NodeInfo
	}
	tests := []struct {
		name string
		args args
		want *framework.Status
	}{
		// TODO: Add test cases.
		{
			name: "no resources requested always fits",
			args: args{
				pod: &corev1.Pod{},
				nodeInfo: framework.NewNodeInfo(
					newResourcePod(framework.Resource{MilliCPU: 10, Memory: 20})),
			},
			want: nil,
		},
		{
			name: "too many resources fails",
			args: args{
				pod: newResourcePod(framework.Resource{MilliCPU: 1, Memory: 1}),
				nodeInfo: framework.NewNodeInfo(
					newResourcePod(framework.Resource{MilliCPU: 10, Memory: 20})),
			},
			want: framework.NewStatus(framework.Unschedulable, getErrReason(corev1.ResourceCPU), getErrReason(corev1.ResourceMemory)),
		},
		{
			name: "too many resources fails due to init container cpu",
			args: args{
				pod: newResourceInitPod(newResourcePod(framework.Resource{MilliCPU: 1, Memory: 1}), framework.Resource{MilliCPU: 3, Memory: 1}),
				nodeInfo: framework.NewNodeInfo(
					newResourcePod(framework.Resource{MilliCPU: 8, Memory: 19})),
			},
			want: framework.NewStatus(framework.Unschedulable, getErrReason(corev1.ResourceCPU)),
		},
		{
			name: "too many resources fails due to init container cpu",
			args: args{
				pod: newResourceInitPod(newResourcePod(framework.Resource{MilliCPU: 1, Memory: 1}), framework.Resource{MilliCPU: 3, Memory: 1}),
				nodeInfo: framework.NewNodeInfo(
					newResourcePod(framework.Resource{MilliCPU: 8, Memory: 19})),
			},
			want: framework.NewStatus(framework.Unschedulable, getErrReason(corev1.ResourceCPU)),
		},
		{
			name: "too many resources fails due to highest init container cpu",
			args: args{
				pod: newResourceInitPod(newResourcePod(framework.Resource{MilliCPU: 1, Memory: 1}), framework.Resource{MilliCPU: 3, Memory: 1}, framework.Resource{MilliCPU: 2, Memory: 1}),
				nodeInfo: framework.NewNodeInfo(
					newResourcePod(framework.Resource{MilliCPU: 8, Memory: 19})),
			},
			want: framework.NewStatus(framework.Unschedulable, getErrReason(corev1.ResourceCPU)),
		},
		{
			name: "too many resources fails due to init container memory",
			args: args{
				pod: newResourceInitPod(newResourcePod(framework.Resource{MilliCPU: 1, Memory: 1}), framework.Resource{MilliCPU: 1, Memory: 3}),
				nodeInfo: framework.NewNodeInfo(
					newResourcePod(framework.Resource{MilliCPU: 8, Memory: 19})),
			},
			want: framework.NewStatus(framework.Unschedulable, getErrReason(corev1.ResourceMemory)),
		},
		{
			name: "too many resources fails due to highest init container memory",
			args: args{
				pod: newResourceInitPod(newResourcePod(framework.Resource{MilliCPU: 1, Memory: 1}), framework.Resource{MilliCPU: 1, Memory: 3}, framework.Resource{MilliCPU: 1, Memory: 2}),
				nodeInfo: framework.NewNodeInfo(
					newResourcePod(framework.Resource{MilliCPU: 8, Memory: 19})),
			},
			want: framework.NewStatus(framework.Unschedulable, getErrReason(corev1.ResourceMemory)),
		},
		{
			name: "init container fits because it's the max, not sum, of containers and init containers",
			args: args{
				pod: newResourceInitPod(newResourcePod(framework.Resource{MilliCPU: 1, Memory: 1}), framework.Resource{MilliCPU: 1, Memory: 1}),
				nodeInfo: framework.NewNodeInfo(
					newResourcePod(framework.Resource{MilliCPU: 8, Memory: 19})),
			},
		},
		{
			name: "multiple init containers fit because it's the max, not sum, of containers and init containers",
			args: args{
				pod: newResourceInitPod(newResourcePod(framework.Resource{MilliCPU: 1, Memory: 1}), framework.Resource{MilliCPU: 1, Memory: 1}, framework.Resource{MilliCPU: 1, Memory: 1}),
				nodeInfo: framework.NewNodeInfo(
					newResourcePod(framework.Resource{MilliCPU: 8, Memory: 19})),
			},
		},
		{
			name: "both resources fit",
			args: args{
				pod: newResourcePod(framework.Resource{MilliCPU: 1, Memory: 1}),
				nodeInfo: framework.NewNodeInfo(
					newResourcePod(framework.Resource{MilliCPU: 5, Memory: 5})),
			},
		},
		{
			name: "one resource memory fits",
			args: args{
				pod: newResourcePod(framework.Resource{MilliCPU: 2, Memory: 1}),
				nodeInfo: framework.NewNodeInfo(
					newResourcePod(framework.Resource{MilliCPU: 9, Memory: 5})),
			},
			want: framework.NewStatus(framework.Unschedulable, getErrReason(corev1.ResourceCPU)),
		},
		{
			name: "one resource cpu fits",
			args: args{
				pod: newResourcePod(framework.Resource{MilliCPU: 1, Memory: 2}),
				nodeInfo: framework.NewNodeInfo(
					newResourcePod(framework.Resource{MilliCPU: 5, Memory: 19})),
			},
			want: framework.NewStatus(framework.Unschedulable, getErrReason(corev1.ResourceMemory)),
		},
		{
			name: "equal edge case",
			args: args{
				pod: newResourcePod(framework.Resource{MilliCPU: 5, Memory: 1}),
				nodeInfo: framework.NewNodeInfo(
					newResourcePod(framework.Resource{MilliCPU: 4, Memory: 19})),
			},
		},
		{
			name: "equal edge case for init container",
			args: args{
				pod: newResourceInitPod(newResourcePod(framework.Resource{MilliCPU: 4, Memory: 1}), framework.Resource{MilliCPU: 5, Memory: 1}),
				nodeInfo: framework.NewNodeInfo(
					newResourcePod(framework.Resource{MilliCPU: 4, Memory: 19})),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reserved := apiext.KoordReserved{
				ReservedResources: corev1.ResourceList{
					corev1.ResourceCPU: *resource.NewMilliQuantity(1, resource.DecimalSI),
				},
				QOSEffected: []apiext.QoSClass{apiext.QoSLSE},
			}
			reservedStr, err := json.Marshal(reserved)
			if err != nil {
				t.Errorf("failed to marshal reserved obj,err=%v.", err)
			}

			node := corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						apiext.ReservedByNode: string(reservedStr),
					},
				},
				Status: corev1.NodeStatus{Capacity: makeResources(10, 20).Capacity,
					Allocatable: makeAllocatableResources(10, 20)},
			}
			tt.args.nodeInfo.SetNode(&node)
			f := &Plugin{}
			cycleState := framework.NewCycleState()

			preFilterStatus := f.PreFilter(context.Background(), cycleState, tt.args.pod)
			if !preFilterStatus.IsSuccess() {
				t.Errorf("prefilter failed with status: %v", preFilterStatus)
			}

			if got := f.Filter(context.Background(), cycleState, tt.args.pod, tt.args.nodeInfo); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Filter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlugin_Filter_Only_Specific_CPUs(t *testing.T) {
	type args struct {
		pod      *corev1.Pod
		nodeInfo *framework.NodeInfo
	}
	tests := []struct {
		name string
		args args
		want *framework.Status
	}{
		// TODO: Add test cases.
		{
			name: "no resources requested always fits",
			args: args{
				pod: &corev1.Pod{},
				nodeInfo: framework.NewNodeInfo(
					newResourcePod(framework.Resource{MilliCPU: 10, Memory: 20})),
			},
			want: nil,
		},
		{
			name: "too many resources fails",
			args: args{
				pod: newResourcePod(framework.Resource{MilliCPU: 10000, Memory: 1}),
				nodeInfo: framework.NewNodeInfo(
					newResourcePod(framework.Resource{MilliCPU: 10000, Memory: 20})),
			},
			want: framework.NewStatus(framework.Unschedulable, getErrReason(corev1.ResourceCPU), getErrReason(corev1.ResourceMemory)),
		},
		{
			name: "too many resources fails due to init container cpu",
			args: args{
				pod: newResourceInitPod(newResourcePod(framework.Resource{MilliCPU: 1000, Memory: 1}), framework.Resource{MilliCPU: 3000, Memory: 1}),
				nodeInfo: framework.NewNodeInfo(
					newResourcePod(framework.Resource{MilliCPU: 8000, Memory: 19})),
			},
			want: framework.NewStatus(framework.Unschedulable, getErrReason(corev1.ResourceCPU)),
		},
		{
			name: "too many resources fails due to init container cpu",
			args: args{
				pod: newResourceInitPod(newResourcePod(framework.Resource{MilliCPU: 1000, Memory: 1}), framework.Resource{MilliCPU: 3000, Memory: 1}),
				nodeInfo: framework.NewNodeInfo(
					newResourcePod(framework.Resource{MilliCPU: 8000, Memory: 19})),
			},
			want: framework.NewStatus(framework.Unschedulable, getErrReason(corev1.ResourceCPU)),
		},
		{
			name: "too many resources fails due to highest init container cpu",
			args: args{
				pod: newResourceInitPod(newResourcePod(framework.Resource{MilliCPU: 1000, Memory: 1}), framework.Resource{MilliCPU: 3000, Memory: 1}, framework.Resource{MilliCPU: 2, Memory: 1}),
				nodeInfo: framework.NewNodeInfo(
					newResourcePod(framework.Resource{MilliCPU: 8000, Memory: 19})),
			},
			want: framework.NewStatus(framework.Unschedulable, getErrReason(corev1.ResourceCPU)),
		},
		{
			name: "too many resources fails due to init container memory",
			args: args{
				pod: newResourceInitPod(newResourcePod(framework.Resource{MilliCPU: 1, Memory: 1}), framework.Resource{MilliCPU: 1, Memory: 3}),
				nodeInfo: framework.NewNodeInfo(
					newResourcePod(framework.Resource{MilliCPU: 8, Memory: 19})),
			},
			want: framework.NewStatus(framework.Unschedulable, getErrReason(corev1.ResourceMemory)),
		},
		{
			name: "too many resources fails due to highest init container memory",
			args: args{
				pod: newResourceInitPod(newResourcePod(framework.Resource{MilliCPU: 1, Memory: 1}), framework.Resource{MilliCPU: 1, Memory: 3}, framework.Resource{MilliCPU: 1, Memory: 2}),
				nodeInfo: framework.NewNodeInfo(
					newResourcePod(framework.Resource{MilliCPU: 8, Memory: 19})),
			},
			want: framework.NewStatus(framework.Unschedulable, getErrReason(corev1.ResourceMemory)),
		},
		{
			name: "init container fits because it's the max, not sum, of containers and init containers",
			args: args{
				pod: newResourceInitPod(newResourcePod(framework.Resource{MilliCPU: 1, Memory: 1}), framework.Resource{MilliCPU: 1, Memory: 1}),
				nodeInfo: framework.NewNodeInfo(
					newResourcePod(framework.Resource{MilliCPU: 8, Memory: 19})),
			},
		},
		{
			name: "multiple init containers fit because it's the max, not sum, of containers and init containers",
			args: args{
				pod: newResourceInitPod(newResourcePod(framework.Resource{MilliCPU: 1, Memory: 1}), framework.Resource{MilliCPU: 1, Memory: 1}, framework.Resource{MilliCPU: 1, Memory: 1}),
				nodeInfo: framework.NewNodeInfo(
					newResourcePod(framework.Resource{MilliCPU: 8, Memory: 19})),
			},
		},
		{
			name: "both resources fit",
			args: args{
				pod: newResourcePod(framework.Resource{MilliCPU: 1, Memory: 1}),
				nodeInfo: framework.NewNodeInfo(
					newResourcePod(framework.Resource{MilliCPU: 5, Memory: 5})),
			},
		},
		{
			name: "one resource memory fits",
			args: args{
				pod: newResourcePod(framework.Resource{MilliCPU: 2000, Memory: 1}),
				nodeInfo: framework.NewNodeInfo(
					newResourcePod(framework.Resource{MilliCPU: 9000, Memory: 5})),
			},
			want: framework.NewStatus(framework.Unschedulable, getErrReason(corev1.ResourceCPU)),
		},
		{
			name: "one resource cpu fits",
			args: args{
				pod: newResourcePod(framework.Resource{MilliCPU: 1, Memory: 2}),
				nodeInfo: framework.NewNodeInfo(
					newResourcePod(framework.Resource{MilliCPU: 5, Memory: 19})),
			},
			want: framework.NewStatus(framework.Unschedulable, getErrReason(corev1.ResourceMemory)),
		},
		{
			name: "equal edge case",
			args: args{
				pod: newResourcePod(framework.Resource{MilliCPU: 5, Memory: 1}),
				nodeInfo: framework.NewNodeInfo(
					newResourcePod(framework.Resource{MilliCPU: 4, Memory: 19})),
			},
		},
		{
			name: "equal edge case for init container",
			args: args{
				pod: newResourceInitPod(newResourcePod(framework.Resource{MilliCPU: 4, Memory: 1}), framework.Resource{MilliCPU: 5, Memory: 1}),
				nodeInfo: framework.NewNodeInfo(
					newResourcePod(framework.Resource{MilliCPU: 4, Memory: 19})),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reserved := apiext.KoordReserved{
				ReservedResources: corev1.ResourceList{
					corev1.ResourceCPU: *resource.NewMilliQuantity(1, resource.DecimalSI),
				},
				QOSEffected: []apiext.QoSClass{apiext.QoSLSE},
			}
			reservedStr, err := json.Marshal(reserved)
			if err != nil {
				t.Errorf("failed to marshal reserved obj,err=%v.", err)
			}

			node := corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						apiext.ReservedByNode: string(reservedStr),
					},
				},
				Status: corev1.NodeStatus{Capacity: makeResources(10000, 20).Capacity,
					Allocatable: makeAllocatableResources(10000, 20)},
			}
			tt.args.nodeInfo.SetNode(&node)
			f := &Plugin{}
			cycleState := framework.NewCycleState()

			preFilterStatus := f.PreFilter(context.Background(), cycleState, tt.args.pod)
			if !preFilterStatus.IsSuccess() {
				t.Errorf("prefilter failed with status: %v", preFilterStatus)
			}

			if got := f.Filter(context.Background(), cycleState, tt.args.pod, tt.args.nodeInfo); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Filter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPreFilterDisabled(t *testing.T) {
	pod := &corev1.Pod{}
	nodeInfo := framework.NewNodeInfo()
	node := corev1.Node{}
	nodeInfo.SetNode(&node)
	p := Plugin{}
	cycleState := framework.NewCycleState()
	gotStatus := p.Filter(context.Background(), cycleState, pod, nodeInfo)
	wantStatus := framework.AsStatus(fmt.Errorf(`error reading "PreFilterNodeReservation" from cycleState: %w`, framework.ErrNotFound))
	if !reflect.DeepEqual(gotStatus, wantStatus) {
		t.Errorf("status does not match: %v, want: %v", gotStatus, wantStatus)
	}
}
