package configstate

import (
	//"k8s.io/apimachinery/pkg/api/errors"
)

type ConfigState struct {
	CPUs []string
	Max int
	Min int
}

type Configs struct {
	// Maps the name of the Config as a string to the CPUStates it affects
	Configs map[string]ConfigState
}

func NewConfigs() (*Configs, error) {
	c := &Configs{}
	configs := make(map[string]ConfigState, 0)
	c.Configs = configs

	return c, nil
}

func (c *Configs) RemoveCPUFromState(configName string) ConfigState {
	// Removes the CPU from the ConfigState and returns the ConfigState object
	configState := c.Configs[configName]
	delete(c.Configs, configName)

	return configState
}

func (c *Configs) UpdateConfigState(configName string, configState ConfigState) {
	// Maybe put some error handling here
	c.Configs[configName] = configState
}
