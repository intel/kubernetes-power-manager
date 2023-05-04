package controllers

import (
	"github.com/intel/power-optimization-library/pkg/power"
	"github.com/stretchr/testify/mock"
)

type hostMock struct {
	mock.Mock
}

func (m *hostMock) Topology() power.Topology {
	return m.Called().Get(0).(power.Topology)
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

func (m *hostMock) GetAllCpus() *power.CpuList {
	ret := m.Called().Get(0)
	if ret == nil {
		return nil
	} else {
		return ret.(*power.CpuList)
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

func (m *poolMock) Cpus() *power.CpuList {
	args := m.Called().Get(0)
	if args == nil {
		return nil
	}
	return args.(*power.CpuList)
}

func (m *poolMock) SetCpus(cores power.CpuList) error {
	return m.Called(cores).Error(0)
}

func (m *poolMock) SetCpuIDs(cpuIDs []uint) error {
	return m.Called(cpuIDs).Error(0)
}

func (m *poolMock) Remove() error {
	return m.Called().Error(0)
}

func (m *poolMock) MoveCpuIDs(coreIDs []uint) error {
	return m.Called(coreIDs).Error(0)
}

func (m *poolMock) MoveCpus(cores power.CpuList) error {
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
	power.Cpu
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

type mockCpuTopology struct {
	mock.Mock
	power.Topology
}

func (m *mockCpuTopology) getID() uint {
	return m.Called().Get(0).(uint)
}

func (m *mockCpuTopology) SetUncore(uncore power.Uncore) error {
	return m.Called(uncore).Error(0)
}

func (m *mockCpuTopology) applyUncore() error {
	return m.Called().Error(0)
}

func (m *mockCpuTopology) getEffectiveUncore() power.Uncore {
	ret := m.Called()
	if ret.Get(0) != nil {
		return ret.Get(0).(power.Uncore)
	}
	return nil
}

func (m *mockCpuTopology) addCpu(u uint) (power.Cpu, error) {
	ret := m.Called(u)

	var r0 power.Cpu
	var r1 error

	if ret.Get(0) != nil {
		r0 = ret.Get(0).(power.Cpu)
	}
	r1 = ret.Error(1)

	return r0, r1
}

func (m *mockCpuTopology) CPUs() *power.CpuList {
	ret := m.Called()

	var r0 *power.CpuList
	if ret.Get(0) != nil {
		r0 = ret.Get(0).(*power.CpuList)
	}

	return r0
}

func (m *mockCpuTopology) Packages() *[]power.Package {
	ret := m.Called()

	var r0 *[]power.Package
	if ret.Get(0) != nil {
		r0 = ret.Get(0).(*[]power.Package)

	}
	return r0
}

func (m *mockCpuTopology) Package(id uint) power.Package {
	ret := m.Called(id)

	var r0 power.Package
	if ret.Get(0) != nil {
		r0 = ret.Get(0).(power.Package)
	}

	return r0
}

type mockCpuPackage struct {
	mock.Mock
	power.Package
}
type mockPackageList struct {
	mock.Mock
}

func (m *mockCpuPackage) MakeList() []power.Package {
	return []power.Package{m}
}
func (m *mockCpuPackage) getID() uint {
	return m.Called().Get(0).(uint)
}

func (m *mockCpuPackage) SetUncore(uncore power.Uncore) error {
	return m.Called(uncore).Error(0)
}

func (m *mockCpuPackage) applyUncore() error {
	return m.Called().Error(0)
}

func (m *mockCpuPackage) getEffectiveUncore() power.Uncore {
	ret := m.Called()
	if ret.Get(0) != nil {
		return ret.Get(0).(power.Uncore)
	}
	return nil
}

func (m *mockCpuPackage) addCpu(u uint) (power.Cpu, error) {
	ret := m.Called(u)

	var r0 power.Cpu
	var r1 error

	if ret.Get(0) != nil {
		r0 = ret.Get(0).(power.Cpu)
	}
	r1 = ret.Error(1)

	return r0, r1
}

func (m *mockCpuPackage) CPUs() *power.CpuList {
	ret := m.Called()

	var r0 *power.CpuList
	if ret.Get(0) != nil {
		r0 = ret.Get(0).(*power.CpuList)
	}

	return r0
}

func (m *mockCpuPackage) Dies() *[]power.Die {
	ret := m.Called()

	var r0 *[]power.Die
	if ret.Get(0) != nil {
		r0 = ret.Get(0).(*[]power.Die)

	}
	return r0
}

func (m *mockCpuPackage) Die(id uint) power.Die {
	ret := m.Called(id)

	var r0 power.Die
	if ret.Get(0) != nil {
		r0 = ret.Get(0).(power.Die)
	}

	return r0
}

type mockCpuDie struct {
	mock.Mock
	power.Die
}

func (m *mockCpuDie) MakeList() []power.Die {
	return []power.Die{m}
}

func (m *mockCpuDie) getID() uint {
	return m.Called().Get(0).(uint)
}

func (m *mockCpuDie) SetUncore(uncore power.Uncore) error {
	return m.Called(uncore).Error(0)
}

func (m *mockCpuDie) applyUncore() error {
	return m.Called().Error(0)
}

func (m *mockCpuDie) getEffectiveUncore() power.Uncore {
	ret := m.Called()
	if ret.Get(0) != nil {
		return ret.Get(0).(power.Uncore)
	}
	return nil
}

func (m *mockCpuDie) addCpu(u uint) (power.Cpu, error) {
	ret := m.Called(u)

	var r0 power.Cpu
	var r1 error

	if ret.Get(0) != nil {
		r0 = ret.Get(0).(power.Cpu)
	}
	r1 = ret.Error(1)

	return r0, r1
}

func (m *mockCpuDie) CPUs() *power.CpuList {
	ret := m.Called()

	var r0 *power.CpuList
	if ret.Get(0) != nil {
		r0 = ret.Get(0).(*power.CpuList)
	}

	return r0
}

func (m *mockCpuDie) Cores() *[]power.Core {
	ret := m.Called()

	var r0 *[]power.Core
	if ret.Get(0) != nil {
		r0 = ret.Get(0).(*[]power.Core)

	}
	return r0
}

func (m *mockCpuDie) Core(id uint) power.Core {
	ret := m.Called(id)

	var r0 power.Core
	if ret.Get(0) != nil {
		r0 = ret.Get(0).(power.Core)
	}

	return r0
}
