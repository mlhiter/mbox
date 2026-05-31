package domain

import (
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
)

type SandboxResourceRequestAccumulator struct {
	count   int
	cpu     resourceQuantityAccumulator
	memory  resourceQuantityAccumulator
	storage resourceQuantityAccumulator
}

func NewSandboxResourceRequestAccumulator() *SandboxResourceRequestAccumulator {
	return &SandboxResourceRequestAccumulator{
		cpu:     resourceQuantityAccumulator{format: formatResourceQuantity},
		memory:  resourceQuantityAccumulator{format: formatResourceQuantity},
		storage: resourceQuantityAccumulator{format: formatResourceQuantity},
	}
}

func (acc *SandboxResourceRequestAccumulator) Add(cpuRequest string, memoryRequest string, storageRequest string) {
	acc.count++
	acc.cpu.add(cpuRequest)
	acc.memory.add(memoryRequest)
	acc.storage.add(storageRequest)
}

func (acc *SandboxResourceRequestAccumulator) Usage() SandboxResourceRequestUsage {
	return SandboxResourceRequestUsage{
		Count:   acc.count,
		CPU:     acc.cpu.usage(),
		Memory:  acc.memory.usage(),
		Storage: acc.storage.usage(),
	}
}

type resourceQuantityAccumulator struct {
	total    resource.Quantity
	declared int
	missing  int
	invalid  int
	format   func(resource.Quantity) string
}

func (acc *resourceQuantityAccumulator) add(value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		acc.missing++
		return
	}
	quantity, err := resource.ParseQuantity(value)
	if err != nil {
		acc.invalid++
		return
	}
	acc.total.Add(quantity)
	acc.declared++
}

func (acc resourceQuantityAccumulator) usage() ResourceQuantityUsage {
	usage := ResourceQuantityUsage{
		Declared: acc.declared,
		Missing:  acc.missing,
		Invalid:  acc.invalid,
	}
	if acc.declared > 0 {
		usage.Total = acc.format(acc.total)
	}
	return usage
}

func formatResourceQuantity(quantity resource.Quantity) string {
	return quantity.String()
}
