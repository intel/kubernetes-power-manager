package controllers

import (
	// "errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"context"

	"github.com/go-logr/logr"
	"github.com/intel/power-optimization-library/pkg/power"
	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

type hostMock struct {
	mock.Mock
	power.Host
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

func (m *hostMock) GetFreqRanges() power.CoreTypeList {
	return m.Called().Get(0).(power.CoreTypeList)
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

type profMock struct {
	mock.Mock
	power.Profile
}

func (m *profMock) Name() string {
	return m.Called().String(0)
}

func (m *profMock) Epp() string {
	return m.Called().String(0)
}

func (m *profMock) MaxFreq() uint {
	return uint(m.Called().Int(0))
}

func (m *profMock) MinFreq() uint {
	return uint(m.Called().Int(0))
}

func (m *profMock) Governor() string {
	return m.Called().String(0)
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

type frequencySetMock struct {
	mock.Mock
	power.CpuFrequencySet
}

func (m *frequencySetMock) GetMax() uint {
	return m.Called().Get(0).(uint)
}

func (m *frequencySetMock) GetMin() uint {
	return m.Called().Get(0).(uint)
}

func setupDummyFiles(cores int, packages int, diesPerPackage int, cpufiles map[string]string) (power.Host, func(), error) {
	//variables for various files
	path := "testing/cpus"
	pStatesDrvFile := "cpufreq/scaling_driver"

	cpuMaxFreqFile := "cpufreq/cpuinfo_max_freq"
	cpuMinFreqFile := "cpufreq/cpuinfo_min_freq"
	scalingMaxFile := "cpufreq/scaling_max_freq"
	scalingMinFile := "cpufreq/scaling_min_freq"
	scalingGovFile := "cpufreq/scaling_governor"
	availGovFile := "cpufreq/scaling_available_governors"
	eppFile := "cpufreq/energy_performance_preference"
	cpuTopologyDir := "topology/"
	packageIdFile := cpuTopologyDir + "physical_package_id"
	dieIdFile := cpuTopologyDir + "die_id"
	coreIdFile := cpuTopologyDir + "core_id"
	uncoreDir := path + "/intel_uncore_frequency/"
	uncoreInitMaxFreqFile := "initial_max_freq_khz"
	uncoreInitMinFreqFile := "initial_min_freq_khz"
	uncoreMaxFreqFile := "max_freq_khz"
	uncoreMinFreqFile := "min_freq_khz"
	cstates := []string{"C0", "C1", "C1E", "C2", "C3", "6"}
	// if we're setting uncore we need to spoof the module being loaded
	_, ok := cpufiles["uncore_max"]
	if ok {
		os.Mkdir("testing", os.ModePerm)
		os.WriteFile("testing/proc.modules", []byte("intel_uncore_frequency"+"\n"), 0644)
		os.MkdirAll(filepath.Join(uncoreDir, "package_00_die_00"), os.ModePerm)
	}
	die := 0
	pkg := 0
	strPkg := "00"
	strDie := "00"
	pkgDir := "package_00_die_00/"
	increment := diesPerPackage * packages
	coresPerDie := cores / increment
	for i := 0; i < cores; i++ {
		cpuName := "cpu" + fmt.Sprint(i)
		cpudir := filepath.Join(path, cpuName)
		os.MkdirAll(filepath.Join(cpudir, "cpufreq"), os.ModePerm)
		os.MkdirAll(filepath.Join(cpudir, "topology"), os.ModePerm)
		//used to divide cores between packages and dies
		if i%coresPerDie == 0 && i != 0 && packages != 0 {
			if die == diesPerPackage-1 && pkg != (packages-1) {
				die = 0
				pkg++
			} else if die != (diesPerPackage - 1) {
				die++
			}

			if pkg > 10 {
				strPkg = fmt.Sprint(pkg)
			} else {
				strPkg = "0" + fmt.Sprint(pkg)
			}
			if die > 10 {
				strDie = fmt.Sprint(die)
			} else {
				strDie = "0" + fmt.Sprint(die)
			}
			pkgDir = "package_" + strPkg + "_die_" + strDie + "/"
			os.MkdirAll(filepath.Join(uncoreDir, pkgDir), os.ModePerm)
		}
		if packages != 0 {
			os.WriteFile(filepath.Join(cpudir, packageIdFile), []byte(fmt.Sprint(pkg)+"\n"), 0664)
			os.WriteFile(filepath.Join(cpudir, dieIdFile), []byte(fmt.Sprint(die)+"\n"), 0664)
			os.WriteFile(filepath.Join(cpudir, coreIdFile), []byte(fmt.Sprint(i)+"\n"), 0664)
		}
		for prop, value := range cpufiles {
			switch prop {
			case "driver":
				os.WriteFile(filepath.Join(cpudir, pStatesDrvFile), []byte(value+"\n"), 0664)
			case "max":
				os.WriteFile(filepath.Join(cpudir, scalingMaxFile), []byte(value+"\n"), 0644)
				os.WriteFile(filepath.Join(cpudir, cpuMaxFreqFile), []byte(value+"\n"), 0644)
			case "min":
				os.WriteFile(filepath.Join(cpudir, scalingMinFile), []byte(value+"\n"), 0644)
				os.WriteFile(filepath.Join(cpudir, cpuMinFreqFile), []byte(value+"\n"), 0644)
			case "epp":
				os.WriteFile(filepath.Join(cpudir, eppFile), []byte(value+"\n"), 0644)
			case "governor":
				os.WriteFile(filepath.Join(cpudir, scalingGovFile), []byte(value+"\n"), 0644)
			case "available_governors":
				os.WriteFile(filepath.Join(cpudir, availGovFile), []byte(value+"\n"), 0644)
			case "uncore_max":
				os.WriteFile(filepath.Join(uncoreDir, pkgDir, uncoreInitMaxFreqFile), []byte(value+"\n"), 0644)
				os.WriteFile(filepath.Join(uncoreDir, pkgDir, uncoreMaxFreqFile), []byte(value+"\n"), 0644)
			case "uncore_min":
				os.WriteFile(filepath.Join(uncoreDir, pkgDir, uncoreInitMinFreqFile), []byte(value+"\n"), 0644)
				os.WriteFile(filepath.Join(uncoreDir, pkgDir, uncoreMinFreqFile), []byte(value+"\n"), 0644)
			case "cstates":
				for i, state := range cstates {
					statedir := "cpuidle/state" + fmt.Sprint(i)
					os.MkdirAll(filepath.Join(cpudir, statedir), os.ModePerm)
					os.MkdirAll(filepath.Join(path, "cpuidle"), os.ModePerm)
					os.WriteFile(filepath.Join(path, "cpuidle", "current_driver"), []byte(value+"\n"), 0644)
					os.WriteFile(filepath.Join(cpudir, statedir, "name"), []byte(state+"\n"), 0644)
					os.WriteFile(filepath.Join(cpudir, statedir, "disable"), []byte("0\n"), 0644)
				}
			}

		}
	}
	host, err := power.CreateInstanceWithConf("test-node", power.LibConfig{CpuPath: "testing/cpus", ModulePath: "testing/proc.modules", Cores: uint(cores)})
	return host, func() {
		os.RemoveAll(strings.Split(path, "/")[0])
	}, err
}

// default dummy file system to be used in standard tests
func fullDummySystem() (power.Host, func(), error) {
	return setupDummyFiles(86, 1, 2, map[string]string{
		"driver": "intel_pstate", "max": "3700000", "min": "1000000",
		"epp": "performance", "governor": "performance",
		"available_governors": "powersave performance",
		"uncore_max":          "2400000", "uncore_min": "1200000",
		"cstates": "intel_idle"})
}

// mock required for testing setupwithmanager
type clientMock struct {
	mock.Mock
	client.Client
}

// mock required for testing client errs
type errClient struct {
	client.Client
	mock.Mock
}

func (e *errClient) Get(ctx context.Context, NamespacedName types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
	if len(opts) != 0 {
		return e.Called(ctx, NamespacedName, obj, opts).Error(0)
	}
	return e.Called(ctx, NamespacedName, obj).Error(0)
}
func (e *errClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if len(opts) != 0 {
		return e.Called(ctx, list, opts).Error(0)

	}
	return e.Called(ctx, list).Error(0)
}

func (e *errClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if len(opts) != 0 {
		return e.Called(ctx, obj, opts).Error(0)
	}
	return e.Called(ctx, obj).Error(0)
}

func (e *errClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if len(opts) != 0 {
		return e.Called(ctx, obj, opts).Error(0)
	}
	return e.Called(ctx, obj).Error(0)
}

func (e *errClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if len(opts) != 0 {
		return e.Called(ctx, obj, opts).Error(0)
	}
	return e.Called(ctx, obj).Error(0)
}

func (e *errClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	if len(opts) != 0 {
		return e.Called(ctx, obj, opts).Error(0)
	}
	return e.Called(ctx, obj).Error(0)
}

func (e *errClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if len(opts) != 0 {
		return e.Called(ctx, obj, patch, opts).Error(0)
	}
	return e.Called(ctx, obj, patch).Error(0)
}

func (e *errClient) Status() client.SubResourceWriter {
	return e.Called().Get(0).(client.SubResourceWriter)
}

type mockResourceWriter struct {
	mock.Mock
	client.SubResourceWriter
}

func (m *mockResourceWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	if len(opts) != 0 {
		return m.Called(ctx, obj, opts).Error(0)
	}
	return m.Called(ctx, obj).Error(0)
}

type mgrMock struct {
	mock.Mock
	manager.Manager
}

func (m *mgrMock) Add(r manager.Runnable) error {
	return m.Called(r).Error(0)
}

func (m *mgrMock) Elected() <-chan struct{} {
	return m.Called().Get(0).(<-chan struct{})
}

func (m *mgrMock) AddMetricsExtraHandler(path string, handler http.Handler) error {
	return m.Called(path, handler).Get(0).(error)
}

func (m *mgrMock) AddHealthzCheck(name string, check healthz.Checker) error {
	return m.Called(name, check).Get(0).(error)
}

func (m *mgrMock) AddReadyzCheck(name string, check healthz.Checker) error {
	return m.Called(name, check).Get(0).(error)
}

func (m *mgrMock) Start(ctx context.Context) error {
	return m.Called(ctx).Get(0).(error)
}

func (m *mgrMock) GetWebhookServer() webhook.Server {
	return m.Called().Get(0).(webhook.Server)
}

func (m *mgrMock) GetLogger() logr.Logger {
	return m.Called().Get(0).(logr.Logger)

}

func (m *mgrMock) GetControllerOptions() config.Controller {
	return m.Called().Get(0).(config.Controller)
}

func (m *mgrMock) GetScheme() *runtime.Scheme {
	return m.Called().Get(0).(*runtime.Scheme)
}
func (m *mgrMock) SetFields(i interface{}) error {
	return m.Called(i).Error(0)
}

func (m *mgrMock) GetCache() cache.Cache {
	return m.Called().Get(0).(cache.Cache)
}

type cacheMk struct {
	cache.Cache
	mock.Mock
}
