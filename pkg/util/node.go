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

package util

import (
	"encoding/json"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	apiext "github.com/koordinator-sh/koordinator/apis/extension"
	"github.com/koordinator-sh/koordinator/pkg/util/cpuset"
)

// GenerateNodeKey returns a generated key with given meta
func GenerateNodeKey(node *metav1.ObjectMeta) string {
	return fmt.Sprintf("%v/%v", node.GetNamespace(), node.GetName())
}

// GetNodeAddress get node specified type address.
func GetNodeAddress(node *corev1.Node, addrType corev1.NodeAddressType) (string, error) {
	for _, address := range node.Status.Addresses {
		if address.Type == addrType {
			return address.Address, nil
		}
	}
	return "", fmt.Errorf("no address matched types %v", addrType)
}

// IsNodeAddressTypeSupported determine whether addrType is a supported type.
func IsNodeAddressTypeSupported(addrType corev1.NodeAddressType) bool {
	if addrType == corev1.NodeHostName ||
		addrType == corev1.NodeExternalIP ||
		addrType == corev1.NodeExternalDNS ||
		addrType == corev1.NodeInternalIP ||
		addrType == corev1.NodeInternalDNS {
		return true
	}
	return false
}

// GetCPUsReservedByNodeAnno gets reserved cpus by annotation of node.
func GetCPUsReservedByNodeAnno(anno map[string]string) (corev1.ResourceList, error) {
	rl := make(corev1.ResourceList)

	if reserved, ok := anno[apiext.ReservedByNode]; ok {
		cpuReservedByNode, qos := GetCPUsReservedByNode(reserved)
		if qos == apiext.QoSLSE {
			rl[corev1.ResourceCPU] = cpuReservedByNode
		}
	}

	return rl, nil
}

func GetCPUsReservedByNode(reserved string) (resource.Quantity, apiext.QoSClass) {
	cpuReservedByNode := *resource.NewMilliQuantity(0, resource.DecimalSI)
	reservedObj := apiext.KoordReserved{}
	qos := apiext.QoSNone

	if err := json.Unmarshal([]byte(reserved), &reservedObj); err != nil {
		klog.Errorf("Failed to unmarshal cpus reserved by node annotation. err=%v.\n", err)
		return cpuReservedByNode, qos
	}

	if reservedcpu, ok := reservedObj.ReservedResources[corev1.ResourceCPU]; ok {
		cpuReservedByNode = reservedcpu
	}
	if reservedObj.ReservedCPUs != "" {
		if cpus, err := cpuset.Parse(reservedObj.ReservedCPUs); err == nil {
			cpuReservedByNode = resource.MustParse(strconv.Itoa(cpus.Size()))
		}
	}

	return cpuReservedByNode, reservedObj.QOSEffected
}

// RemoveNodeReservedCPUs filter out cpus that reserved by annotation of node.
func RemoveNodeReservedCPUs(cpuSharePools []apiext.CPUSharedPool, cpusReservedByNodeAnno string) []apiext.CPUSharedPool {
	if cpusReservedByNodeAnno == "" {
		return cpuSharePools
	}

	newCPUSharePools := make([]apiext.CPUSharedPool, len(cpuSharePools))
	for idx, val := range cpuSharePools {
		newCPUSharePools[idx] = val
	}

	reservedCPUs, err := cpuset.Parse(cpusReservedByNodeAnno)
	if err != nil {
		return newCPUSharePools
	}

	for idx, pool := range cpuSharePools {
		originCPUs, err := cpuset.Parse(pool.CPUSet)
		if err != nil {
			return newCPUSharePools
		}

		newCPUSharePools[idx].CPUSet = originCPUs.Difference(reservedCPUs).String()
	}

	return newCPUSharePools
}
