package controllers

import (
	"github.com/stretchr/testify/mock"
	"io"

	"github.com/intel/power-optimization-library/pkg/power"
)

type ioReader interface {
	io.Reader
}

type nodeMock struct {
	mock.Mock
}

func (m *nodeMock) AvailableCStates() ([]string, error) {
	args := m.Called()
	return args.Get(0).([]string), args.Error(1)
}

func (m *nodeMock) ApplyCStatesToSharedPool(cStates power.CStates) error {
	return m.Called(cStates).Error(0)
}

func (m *nodeMock) ApplyCStateToPool(poolName string, cStates power.CStates) error {
	return m.Called(poolName, cStates).Error(0)
}

func (m *nodeMock) ApplyCStatesToCore(coreID int, cStates power.CStates) error {
	args := m.Called(coreID, cStates)
	return args.Error(0)
}

func (m *nodeMock) IsCStateValid(cStates ...string) bool {
	args := m.Called(cStates)
	return args.Bool(0)
}

func (m *nodeMock) SetNodeName(name string) {
	m.Called(name)
	return
}

func (m *nodeMock) GetName() string {
	args := m.Called()
	return args.String(0)
}

func (m *nodeMock) GetReservedCoreIds() []int {
	args := m.Called()
	return args.Get(0).([]int)
}

func (m *nodeMock) AddProfile(name string, minFreq int, maxFreq int, governor string, epp string) (power.Profile, error) {
	args := m.Called(name, minFreq, maxFreq, governor, epp)
	return args.Get(0).(power.Profile), args.Error(1)
}

func (m *nodeMock) UpdateProfile(name string, minFreq int, maxFreq int, governor string, epp string) error {
	args := m.Called(name, minFreq, maxFreq, governor, epp)
	return args.Error(0)
}

func (m *nodeMock) DeleteProfile(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

func (m *nodeMock) AddCoresToExclusivePool(name string, cores []int) error {
	args := m.Called(name, cores)
	return args.Error(0)
}

func (m *nodeMock) AddExclusivePool(name string, profile power.Profile) (power.Pool, error) {
	args := m.Called(name, profile)
	return args.Get(0).(power.Pool), args.Error(1)
}

func (m *nodeMock) RemoveExclusivePool(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

func (m *nodeMock) AddSharedPool(coreIds []int, profile power.Profile) error {
	args := m.Called(coreIds, profile)
	return args.Error(0)
}

func (m *nodeMock) RemoveCoresFromExclusivePool(poolName string, cores []int) error {
	args := m.Called(poolName, cores)
	return args.Error(0)
}

func (m *nodeMock) RemoveSharedPool() error {
	args := m.Called()
	return args.Error(0)
}

func (m *nodeMock) GetProfile(name string) power.Profile {
	args := m.Called(name)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(power.Profile)
}

func (m *nodeMock) GetExclusivePool(name string) power.Pool {
	args := m.Called(name)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(power.Pool)
}

func (m *nodeMock) GetSharedPool() power.Pool {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(power.Pool)
}

func (m *nodeMock) GetFeaturesInfo() power.FeatureSet {
	return m.Called().Get(0).(power.FeatureSet)
}

type mockProfile struct {
	mock.Mock
}

func (m *mockProfile) GetName() string {
	args := m.Called()
	return args.String(0)
}

func (m *mockProfile) GetEpp() string {
	args := m.Called()
	return args.String(0)
}

func (m *mockProfile) GetMaxFreq() int {
	args := m.Called()
	return args.Int(0)
}

func (m *mockProfile) GetMinFreq() int {
	args := m.Called()
	return args.Int(0)
}

func (m *mockProfile) GetGovernor() string {
	return m.Called().String(0)
}

type mockPool struct {
	power.Pool
	mock.Mock
}

func (m *mockPool) GetName() string {
	args := m.Called()
	return args.String(0)
}

func (m *mockPool) addCore(core power.Core) error {
	args := m.Called(core)
	return args.Error(0)
}

func (m *mockPool) removeCore(core power.Core) error {
	args := m.Called(core)
	return args.Error(0)
}

func (m *mockPool) removeCoreByID(coreID int) (power.Core, error) {
	args := m.Called(coreID)
	return args.Get(0).(power.Core), args.Error(1)
}

func (m *mockPool) SetPowerProfile(profile power.Profile) error {
	args := m.Called(profile)
	return args.Error(0)
}

func (m *mockPool) GetPowerProfile() power.Profile {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(power.Profile)
}

func (m *mockPool) GetCores() []power.Core {
	args := m.Called()
	return args.Get(0).([]power.Core)
}

func (m *mockPool) GetCoreIds() []int {
	args := m.Called()
	return args.Get(0).([]int)
}

func (m *mockPool) SetCStates(states power.CStates) error {
	args := m.Called(states)
	return args.Error(0)
}

func (m *mockPool) getCStates() power.CStates {
	args := m.Called()
	return args.Get(0).(power.CStates)
}
