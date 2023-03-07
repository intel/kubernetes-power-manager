package controllers

import (
	"github.com/intel/power-optimization-library/pkg/power"
	"github.com/stretchr/testify/mock"
)

type hostMock struct {
	mock.Mock
}

func (m *hostMock) ValidateCStates(states power.CStates) error {
	return m.Called(states).Error(0)
}

func (m *hostMock) AvailableCStates() []string {
	return m.Called().Get(0).([]string)
}

func (m *hostMock) GetAllExclusivePools() *power.PoolList {
	return m.Called().Get(0).(*power.PoolList)
}

func (m *hostMock) SetName(name string) {
	m.Called(name)
}

func (m *hostMock) GetName() string {
	return m.Called().String(0)
}

func (m *hostMock) GetFeaturesInfo() power.FeatureSet {
	ret := m.Called().Get(0)
	if ret == nil {
		return nil
	} else {
		return ret.(power.FeatureSet)
	}
}

func (m *hostMock) GetReservedPool() power.Pool {
	ret := m.Called().Get(0)
	if ret == nil {
		return nil
	} else {
		return ret.(power.Pool)
	}
}

func (m *hostMock) GetSharedPool() power.Pool {
	ret := m.Called().Get(0)
	if ret == nil {
		return nil
	} else {
		return ret.(power.Pool)
	}
}
func (m *hostMock) AddExclusivePool(poolName string) (power.Pool, error) {
	args := m.Called(poolName)
	retPool := args.Get(0)
	if retPool == nil {
		return nil, args.Error(1)
	} else {
		return retPool.(power.Pool), args.Error(1)
	}
}

func (m *hostMock) GetExclusivePool(poolName string) power.Pool {
	ret := m.Called(poolName).Get(0)
	if ret == nil {
		return nil
	} else {
		return ret.(power.Pool)
	}
}

func (m *hostMock) GetAllCores() *power.CoreList {
	ret := m.Called().Get(0)
	if ret == nil {
		return nil
	} else {
		return ret.(*power.CoreList)
	}
}

type poolMock struct {
	mock.Mock
	power.Pool
}

func (m *poolMock) SetCStates(states power.CStates) error {
	return m.Called(states).Error(0)
}

func (m *poolMock) Clear() error {
	return m.Called().Error(0)
}

func (m *poolMock) Name() string {
	return m.Called().String(0)
}

func (m *poolMock) Cores() *power.CoreList {
	args := m.Called().Get(0)
	if args == nil {
		return nil
	}
	return args.(*power.CoreList)
}

func (m *poolMock) SetCores(cores power.CoreList) error {
	return m.Called(cores).Error(0)
}

func (m *poolMock) SetCoreIDs(coreIDs []uint) error {
	return m.Called(coreIDs).Error(0)
}

func (m *poolMock) Remove() error {
	return m.Called().Error(0)
}

func (m *poolMock) MoveCoresIDs(coreIDs []uint) error {
	return m.Called(coreIDs).Error(0)
}

func (m *poolMock) MoveCores(cores power.CoreList) error {
	return m.Called(cores).Error(0)
}

func (m *poolMock) SetPowerProfile(profile power.Profile) error {
	args := m.Called(profile)
	return args.Error(0)
}

func (m *poolMock) GetPowerProfile() power.Profile {
	args := m.Called().Get(0)
	if args == nil {
		return nil
	}
	return args.(power.Profile)
}

type coreMock struct {
	mock.Mock
	power.Core
}

func (m *coreMock) SetCStates(cStates power.CStates) error {
	return m.Called(cStates).Error(0)
}

func (m *coreMock) GetID() uint {
	args := m.Called()
	return args.Get(0).(uint)
}
func (m *coreMock) SetPool(pool power.Pool) error {
	return m.Called(pool).Error(0)
}
