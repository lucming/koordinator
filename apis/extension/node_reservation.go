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

package extension

import (
	"encoding/json"
	"math"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

const (
	AnnotationNodeReservation = NodeDomainPrefix + "/reservation"
)

// KoordReserved resource reserved by node.annotation,
// If node.annotation declares the resources to be reserved, like this:
//  annotations:
//    node.koordinator.sh/node-reservation: >-
//	    {"specificCPUs":"0-5"}

//   In the filter phase it needs to satisfy: node.alloc - node.req - reserved(6c) > pod.req
//   if qos==LSE/LSR: the cores 0-5 are not used in the reserve phase
type NodeReservation struct {
	// resources need to be reserved. like, {"cpu":"1C", "memory":"2Gi"}
	Resources corev1.ResourceList `json:"resources,omitempty"`
	// specific cpus need to be reserved, such as 1-6, or 2,4,6,8
	SpecificCPUs string `json:"specificCPUs,omitempty"`
}

func GetReservedCPUs(anno map[string]string) (string, int) {
	specificCPUsReservedStr := ""
	numReservedCPUs := 0

	val, ok := anno[AnnotationNodeReservation]
	if !ok || val == "" {
		return specificCPUsReservedStr, numReservedCPUs
	}

	reserved := NodeReservation{}
	if err := json.Unmarshal([]byte(val), &reserved); err != nil {
		klog.Errorf("failed to unmarshal reserved resources from node.annotation in nodenumaresource scheduler plugin.err:%v", err)
	}

	CPUsQuantityReserved, ok := reserved.Resources[corev1.ResourceCPU]
	if ok && !CPUsQuantityReserved.IsZero() {
		reservedCPUsFloat := float64(CPUsQuantityReserved.MilliValue()) / 1000
		numReservedCPUs = int(math.Ceil(reservedCPUsFloat))
	}

	if reserved.SpecificCPUs != "" {
		numReservedCPUs = 0
	}
	specificCPUsReservedStr = reserved.SpecificCPUs

	return specificCPUsReservedStr, numReservedCPUs
}

func GetCPUsReservedByTopoAnno(anno map[string]string) string {
	reserved := ""
	nodeReserved, ok := anno[AnnotationNodeReservation]
	if !ok {
		return reserved
	}

	reservedobj := NodeReservation{}
	if err := json.Unmarshal([]byte(nodeReserved), &reservedobj); err != nil {
		klog.Errorf("Failed to unmarshal reserved resources from node.err:%v.", err)
		return reserved
	}
	reserved=reservedobj.SpecificCPUs

	return reserved
}
