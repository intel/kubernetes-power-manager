package workloadstate

import (
//"k8s.io/apimachinery/pkg/api/errors"
)

type WorkloadState struct {
	CPUs []string
	Max  int
	Min  int
}

type Workloads struct {
	// Maps the name of the PowerWorkload as a string to the WorkloadStates it affects
	Workloads map[string]WorkloadState
}

func NewWorkloads() (*Workloads, error) {
	w := &Workloads{}
	workloads := make(map[string]WorkloadState, 0)
	w.Workloads = workloads

	return w, nil
}

func (w *Workloads) RemoveCPUFromState(workloadName string) WorkloadState {
	// Removes the CPU from the WorkloadState and returns the WorkloadState object
	workloadState := w.Workloads[workloadName]
	delete(w.Workloads, workloadName)

	return workloadState
}

func (w *Workloads) UpdateWorkloadState(workloadName string, cpusEffected []string, maxCPUFreq int, minCPUFreq int) {
	// Maybe put some error handling here
	ws := &WorkloadState{}
	ws.CPUs = cpusEffected
	ws.Max = maxCPUFreq
	ws.Min = minCPUFreq
	w.Workloads[workloadName] = *ws
}
