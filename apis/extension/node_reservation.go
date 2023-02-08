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

import corev1 "k8s.io/api/core/v1"

const (
	ReservedByNode = NodeDomainPrefix + "/node-reservation"
)

// KoordReserved resource reserved by node.annotation,
// If node.annotation declares the resources to be reserved, like this:
//  annotations:
//    node.koordinator.sh/node-reservation: >-
//	    {"reservedCPUs":"0-5","qosEffected":["LSE"]}

// if pod.qos==LSE
//   In the filter phase it needs to satisfy: node.alloc - node.req - reserved(6c) > pod.req
//   the cores 0-5 are not used in the reserve phase.
// if pod.qos==LSR
//   In the filter phase it needs to satisfy: node.alloc - node.req > pod.req
//   the cores 0-5 are not used in the reserve phase.
// if pod.qos!=LSE && pod.qos!=LSR
//	 In the filter phase it needs to satisfy: node.alloc - node.req > pod.req
type KoordReserved struct {
	// resources need to be reserved. like, {"cpu":"1C", "memory":"2Gi"}
	ReservedResources corev1.ResourceList `json:"reservedResources,omitempty"`
	// specific cpus need to be reserved, such as 1-6, or 2,4,6,8
	ReservedCPUs string `json:"reservedCPUs,omitempty"`

	// Which qos need to consider these reserved resources
	QOSEffected QoSClass `json:"qosEffected,omitempty"`
}
