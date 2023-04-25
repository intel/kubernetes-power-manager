# Kubernetes Power Manager


## What is the Kubernetes Power Manager?

Utilizing a container orchestration engine like Kubernetes, CPU resources are allocated from a pool of platforms
entirely based on availability, without taking into account specific features like Intel Speed Select Technology (SST).

The Kubernetes Power Manager is a Kubernetes Operator that has been developed to provide cluster users with a mechanism
to dynamically request adjustment of worker node power management settings applied to cores allocated to the Pods. The
power management-related settings can be applied to individual cores or to groups of cores, and each
may have different policies applied. It is not required that every core in the system be explicitly managed by this
Kubernetes power manager. When the Power Manager is used to specify core power related policies, it overrides the
default settings

Powerful features from the Intel SST package give users more precise control over CPU performance and power use on a
per-core basis. Yet, Kubernetes is purposefully built to operate as an abstraction layer between the workload and such
hardware capabilities as a workload orchestrator. Users of Kubernetes who are running performance-critical workloads
with particular requirements reliant on hardware capabilities encounter a challenge as a consequence.

The Kubernetes Power Manager bridges the gap between the container orchestration layer and hardware features enablement,
specifically Intel SST.

### Kubernetes Power Manager' main responsibilities:

- The Kubernetes Power Manager consists of two main components - the overarching manager which is deployed anywhere on a
  cluster and the power node agent which is deployed on each node you require power management capabilities.
- The overarching operator is responsible for the configuration and deployment of the power node agent, while the power
  node agent is responsible for the tuning of the cores as requested by the user.

### Use Cases:

- *High performance workload known at peak times.*
  May want to pre-schedule nodes to move to a performance profile during peak times to minimize spin up.
  At times not during peak, may want to move to a power saving profile.
- *Unpredictable machine use.*
  May use machine learning through monitoring to determine profiles that predict a peak need for a compute, to spin up
  ahead of time.
- *Power Optimization over Performance.*
  A cloud may be interested in fast response time, but not in maximal response time, so may choose to spin up cores on
  demand and only those cores used but want to remain in power-saving mode the rest of the time.

## Functionality of the Kubernetes Power Manager

- **SST-BF - (Speed Select Technology - Base Frequency)**

  The base frequency of some cores can be changed by the user using this function. The CPU's performance is ensured at
  the basic frequency (a CPU will never go below its base frequency). Priority cores can apply their crucial workloads
  for a guaranteed performance at a base frequency that is greater than the majority of the other cores on the system.

- **SST-CP - (Speed Select Technology - Core Power)**

  The user can arrange cores according to priority levels using this capability. When the system has extra power, it can
  be distributed among the cores according to their priority level. Although it cannot be guaranteed, the system will
  try to apply the additional power to the cores with the highest priority.
  There are four levels of priority available:
    1. Performance
    2. Balance Performance
    3. Balance Power
    4. Power

  The Priority level for a core is defined using its EPP (Energy Performance Preference) value, which is one of the
  options in the Power Profiles. If not all the power is utilized on the CPU, the CPU can put the higher priority cores
  up to Turbo Frequency (allows the cores to run faster).

- **Frequency Tuning**

  Frequency tuning allows the individual cores on the system to be sped up or slowed down by changing their frequency.
  This tuning is done via the [Intel Power Optimization Library](https://github.com/intel/power-optimization-library).
  The min and max values for a core are defined in the Power Profile and the tuning is done after the core has been
  assigned by the Native CPU Manager.
  How exactly the frequency of the cores is changed is by simply writing the new frequency value to the
  /sys/devices/system/cpu/cpuN/cpufreq/scaling_max|min_freq file for the given core.

- **Time of Day**

  Time of Day is designed to allow the user to select a specific time of day that they can put all their unused CPUs
  into “sleep” state and then reverse the process and select another time to return to an “active” state.

- **P-State**

  Modern Intel CPUs automatically employ the Intel P_State CPU power scaling driver. This driver is integrated rather
  than a module, giving it precedence over other drivers. For Sandy Bridge and newer CPUs, this driver is currently used
  automatically. The BIOS P-State settings might be disregarded by Intel P-State.
  The Intel P-State driver utilizes the "Performance" and "Powersave" governors.
  ***Performance***
  The CPUfreq governor "performance" sets the CPU statically to the highest frequency within the borders of
  scaling_min_freq and scaling_max_freq.
  ***Powersave***
  The CPUfreq governor "powersave" sets the CPU statically to the lowest frequency within the borders of
  scaling_min_freq and scaling_max_freq.

- **Uncore**
  The largest part of modern CPUs is outside the actual cores. On Intel CPUs this is part is called the "Uncore" and has
  last level caches, PCI-Express, memory controller, QPI, power management and other functionalities.
  The previous deployment pattern was that an uncore setting was applied to sets of servers that are allocated as
  capacity for handling a particular type of workload. This is typically a one-time configuration today. The Kubenetes
  Power Manager now makes this dynamic and through a cloud native pattern. The implication is that the cluster-level
  capacity for the workload can then configured dynamically, as well as scaled dynamically. Uncore frequency applies to
  Xeon scalable and D processors could save up to 40% of CPU power or improved performance gains.

## Future planned additions to the Kubernetes Power Manager

- **SST-TF - Turbo Frequency**

  This feature allows the user to set different “All-Core Turbo Frequency” values to individual cores based on their
  priority.
  All-Core Turbo is the Turbo Frequency at which all cores can run on the system at the same time.
  The user can set certain cores to have a higher All-Core Turbo Frequency by lowering this value for other cores or
  setting them to no value at all.

  This feature is only useful when all cores on the system are being utilized, but the user still wants to be able to
  configure certain cores to get a higher performance than others.

## Prerequisites

* Node Feature Discovery ([NFD](https://github.com/kubernetes-sigs/node-feature-discovery)) should be deployed in the
  cluster before running the Kubernetes Power Manager. NFD is used to detect node-level features such as *Intel Speed
  Select Technology - Base Frequency (SST-BF)*. Once detected, the user can instruct the Kubernetes Power Manager to
  deploy the Power Node Agent to Nodes with SST-specific labels, allowing the Power Node Agent to take advantage of such
  features by configuring cores on the host to optimise performance for containerized workloads.
  Note: NFD is recommended, but not essential. Node labels can also be applied manually. See
  the [NFD repo](https://github.com/kubernetes-sigs/node-feature-discovery#feature-labels) for a full list of features
  labels.
* In the kubelet configuration file the cpuManagerPolicy has to set to "static", and the reservedSystemCPUs are set to
  the desired value:

````yaml
apiVersion: kubelet.config.k8s.io/v1beta1
authentication:
  anonymous:
    enabled: false
  webhook:
    cacheTTL: 0s
    enabled: true
  x509:
    clientCAFile: /etc/kubernetes/pki/ca.crt
authorization:
  mode: Webhook
  webhook:
    cacheAuthorizedTTL: 0s
    cacheUnauthorizedTTL: 0s
cgroupDriver: systemd
clusterDNS:
  - 10.96.0.10
clusterDomain: cluster.local
cpuManagerPolicy: "static"
cpuManagerReconcilePeriod: 0s
evictionPressureTransitionPeriod: 0s
fileCheckFrequency: 0s
healthzBindAddress: 127.0.0.1
healthzPort: 10248
httpCheckFrequency: 0s
imageMinimumGCAge: 0s
kind: KubeletConfiguration
logging:
  flushFrequency: 0
  options:
    json:
      infoBufferSize: "0"
  verbosity: 0
memorySwap: { }
nodeStatusReportFrequency: 0s
nodeStatusUpdateFrequency: 0s
reservedSystemCPUs: "0"
resolvConf: /run/systemd/resolve/resolv.conf
rotateCertificates: true
runtimeRequestTimeout: 0s
shutdownGracePeriod: 0s
shutdownGracePeriodCriticalPods: 0s
staticPodPath: /etc/kubernetes/manifests
streamingConnectionIdleTimeout: 0s
syncFrequency: 0s
volumeStatsAggPeriod: 0s
````

## Working environments

The Kubernetes Power Manager has been tested in different environments.  
The below table are results that have been tested and confirmed to function as desired:

|       OS        |           Kernel            |   Container runtime    |  Kubernetes  |
|:---------------:|:---------------------------:|:----------------------:|:------------:|
|    CentOS 7     | 3.10.0-1160.71.1.el7.x86_64 |   Containerd v1.6.6    |   v1.24.3    |
|    CentOS 7     | 3.10.0-1160.71.1.el7.x86_64 |   Containerd v1.6.6    |   v1.24.2    |
|    CentOS 7     | 3.10.0-1160.71.1.el7.x86_64 |   Containerd v1.6.6    |   v1.23.5    |
|    CentOS 7     |  5.4.0-113.el7.elrepo.x86   |   Containerd v1.6.6    |   v1.24.3    |
| Ubuntu 22.04.1  |      5.15.0-43-generic      |   Containerd v1.6.6    |   v1.24.2    |
| Ubuntu 22.04.1  |      5.15.0-43-generic      |   Containerd v1.5.11   |   v1.24.2    |
| Ubuntu 22.04.1  |      5.15.0-43-generic      | Containerd v1.6.6-k3s1 | v1.24.3+k3s1 |
| Ubuntu 22.04.1  |      5.4.0-122-generic      |    Docker 20.10.12     |   v1.23.3    |
| Rocky Linux 8.6 |  4.18.0-372.9.1.el8.x86_64  |   Containerd v1.6.7    |   v1.24.3    |
| Rocky Linux 8.6 |  4.18.0-372.9.1.el8.x86_64  |   Containerd v1.5.10   |   v1.24.3    |
| Rocky Linux 8.6 |  4.18.0-372.9.1.el8.x86_64  |     cri-o://1.24.2     |   v1.24.3    |

Note: this does not include additional environments.

## Components

### Intel Power Optimization Library

[Intel Power Optimization Library](https://github.com/intel/power-optimization-library), takes the desired configuration
for the cores associated with Exclusive Pods and tune them based on the requested Power Profile. The Power Optimization
Library will also facilitate the use of the Intel SST (Speed Select Technology) Suite (SST-BF - Speed Select
Technology-Base Frequency, SST-CP - Speed Select Technology-Core Power, and Frequency Tuning) and C-States
functionality.

### Power Node Agent

The Power Node Agent is also a containerized application deployed by the Kubernetes Power Manager in a DaemonSet. The
primary function of the node agent is to communicate with the node's Kubelet PodResources endpoint to discover the exact
cores that are allocated per container. The node agent watches for Pods that are created in your cluster and examines
them to determine which Power Profile they have requested and then sets off the chain of events that tunes the
frequencies of the cores designated to the Pod.

### Config Controller

The Kubernetes Power Manager will wait for the PowerConfig to be created by the user, in which the desired PowerProfiles
will be specified. The PowerConfig holds different values: what image is required, what Nodes the user wants to place
the node agent on and what PowerProfiles are required.

* powerNodeSelector: This is a key/value map used for defining a list of node labels that a node must satisfy in order
  for the Power Node Agent to be deployed.
* powerProfiles: The list of PowerProfiles that the user wants available on the nodes.

Once the Config Controller sees that the PowerConfig is created, it reads the values and then deploys the node agent on
to each of the Nodes that are specified. It then creates the PowerProfiles and extended resources. Extended resources
are resources created in the cluster that can be requested in the PodSpec. The Kubelet can then keep track of these
requests. It is important to use as it can specify how many cores on the system can be run at a higher frequency before
hitting the heat threshold.

Note: Only one PowerConfig can be present in a cluster. The Config Controller will ignore and delete and subsequent
PowerConfigs created after the first.

### Example

````yaml
apiVersion: "power.intel.com/v1"
kind: PowerConfig
metadata:
  name: power-config
  namespace: intel-power
spec:
  powerNodeSelector:
    feature.node.kubernetes.io/power-node: "true"
  powerProfiles:
    - "performance"
    - "balance-performance"
    - "balance-power"
````

### Workload Controller

The Workload Controller is responsible for the actual tuning of the cores. The Workload Controller uses the Intel Power
Optimization Library and requests that it creates the Pools. The Pools hold the PowerProfile associated with the cores
and the cores that need to be configured.

The PowerWorkload objects are created automatically by the PowerPod controller. This action is undertaken by the
Kubernetes Power Manager when a Pod is created with a container requesting exclusive cores and a PowerProfile.

PowerWorkload objects can also be created directly by the user via the PowerWorkload spec. This is only recommended when
creating the Shared PowerWorkload for a given Node, as this is the responsibility of the user. If no Shared
PowerWorkload is created, the cores that remain in the ‘shared pool’ on the Node will remain at their core frequency
values instead of being tuned to lower frequencies. PowerWorkloads are specific to a given node, so one is created for
each Node with a Pod requesting a PowerProfile, based on the PowerProfile requested.

### Example

````
apiVersion: "power.intel.com/v1"
kind: PowerWorkload
metadata:
    name: performance-example-node-workload
    namespace: intel-power
spec:
   name: "performance-example-node-workload"
   nodeInfo:
     containers:
     - exclusiveCPUs:
       - 2
       - 3
       - 66
       - 67
       id: f1be89f7dda457a7bb8929d4da8d3b3092c9e2a35d91065f1b1c9e71d19bcd4f
       name: example-container
       pod: example-pod
       powerProfile: “performance-example-node”
     name: “example-node”
     cpuIds:
     - 2
     - 3
     - 66
     - 67
   powerProfile: "performance-example-node"
````

This workload assigns the “performance” PowerProfile to cores 2, 3, 66, and 67 on the node “example-node”

The Shared PowerWorkload created by the user is determined by the Workload controller to be the designated Shared
PowerWorkload based on the AllCores value in the Workload spec. The reserved CPUs on the Node must also be specified, as
these will not be considered for frequency tuning by the controller as they are always being used by Kubernetes’
processes. It is important that the reservedCPUs value directly corresponds to the reservedCPUs value in the user’s
Kubelet config to keep them consistent. The user determines the Node for this PowerWorkload using the PowerNodeSelector
to match the labels on the Node. The user then specifies the requested PowerProfile to use.

A shared PowerWorkload must follow the naming convention of beginning with ‘shared-’. Any shared PowerWorkload that does
not begin with ‘shared-’ is rejected and deleted by the PowerWorkload controller. The shared PowerWorkload
powerNodeSelector must also select a unique node, so it is recommended that the ‘kubernetes.io/hostname’ label be used.
A shared PowerProfile can be used for multiple shared PowerWorkloads.

### Example

````yaml
apiVersion: "power.intel.com/v1"
kind: PowerWorkload
metadata:
  name: shared-example-node-workload
  namespace: intel-power
spec:
  name: "shared-example-node-workload"
  allCores: true
  reservedCPUs:
    - 0
    - 1
  powerNodeSelector:
    # Labels other than hostname can be used
    - “kubernetes.io/hostname”: “example-node”
  powerProfile: "shared-example-node"
````

### Profile Controller

The Profile Controller holds values for specific SST settings which are then applied to cores at host level by the
Kubernetes Power Manager as requested. Power Profiles are advertised as extended resources and can be requested via the
PodSpec. The Config controller creates the requested high-performance PowerProfiles depending on which are requested in
the PowerConfig created by the user.

There are two kinds of PowerProfiles:

- Base PowerProfiles
- Extended PowerProfiles

A Base PowerProfile can be one of three values:

- performance
- balance-performance
- balance-power

These correspond to three of the EPP values associated with SST-CP. Base PowerProfiles are used to tell the Profile
controller that the specified profile is being requested for the cluster. The Profile controller takes the created
Profile and further creates an Extended PowerProfile. An Extended PowerProfile is Node-specific. The reason behind this
is that different Nodes in your cluster may have different maximum frequency limitations. For example, one Node may have
the maximum limitation of 3700GHz, while another may only be able to reach frequency levels of 3200GHz. An Extended
PowerProfile queries the Node that it is running on to obtain this maximum limitation and sets the Max and Min values of
the profile accordingly. An Extended PowerProfile’s name has the following form:

BASE_PROFILE_NAME-NODE_NAME - for example: “performance-example-node”.

Either the Base PowerProfile or the Extended PowerProfile can be requested in the PodSpec, as the Workload controller
can determine the correct PowerProfile to use from the Base PowerProfile.

#### Example

````yaml
apiVersion: "power.intel.com/v1"
kind: PowerProfile
metadata:
  name: performance-example-node
spec:
  name: "performance-example-node"
  max: 3700
  min: 3300
  epp: "performance"
````

The Shared PowerProfile must be created by the user and does not require a Base PowerProfile. This allows the user to
have a Shared PowerProfile per Node in their cluster, giving more room for different configurations. The Power
controller determines that a PowerProfile is being designated as ‘Shared’ through the use of the ‘power’ EPP value.

#### Example

````yaml
apiVersion: "power.intel.com/v1"
kind: PowerProfile
metadata:
  name: shared-example-node1
spec:
  name: "shared-example-node1"
  max: 1500
  min: 1000
  epp: "power"
````

````yaml
apiVersion: "power.intel.com/v1"
kind: PowerProfile
metadata:
  name: shared-example-node2
spec:
  name: "shared-example-node2"
  max: 2000
  min: 1500
  epp: "power"
````

### PowerNode Controller

The PowerNode controller provides a window into the cluster's operations.
It exposes the workloads that are now being used, the profiles that are being used, the cores that are being used, and
the containers that those cores are associated to. Moreover, it informs the user of which Shared Pool is in use. The
Default Pool or the Shared Pool can be one of the two shared pools. The Default Pool will hold all the cores in the "
shared pool," none of which will have their frequencies set to a lower value, if there is no Shared PowerProfile
associated with the Node. The cores in the "shared pool"—apart from those reserved for Kubernetes processes (
reservedCPUs)—will be assigned to the Shared Pool and have their cores tuned by the Intel Power Optimization Library if
a Shared PowerProfile is associated with the Node.

#### Example

````
activeProfiles:
    performance-example-node: true
  activeWorkloads:
  - cores:
    - 2
    - 3
    - 8
    - 9
    name: performance-example-node-workload
  nodeName: example-node
  powerContainers:
  - exclusiveCpus:
    - 2
    - 3
    - 8
    - 9
    id: c392f492e05fc245f77eba8a90bf466f70f19cb48767968f3bf44d7493e18e5b
    name: example-container
    pod: example-pod
    powerProfile: performance-example-node
    workload: performance-example-node-workload
  sharedPools:
  - name: Default
    sharedPoolCpuIds:
    - 0
    - 1
    - 4
    - 5
    - 6
    - 7
    - 10
````

````
activeProfiles:
    performance-example-node: true
  activeWorkloads:
  - cores:
    - 2
    - 3
    - 8
    - 9
    name: performance-example-node-workload
  nodeName: example-node
  powerContainers:
  - exclusiveCpus:
    - 2
    - 3
    - 8
    - 9
    id: c392f492e05fc245f77eba8a90bf466f70f19cb48767968f3bf44d7493e18e5b
    name: example-container
    pod: example-pod
    powerProfile: performance-example-node
    workload: performance-example-node-workload
  sharedPools:
  - name: Default
    sharedPoolCpuIds:
    - 0
    - 1
- name: Shared
    sharedPoolCpuIds:
    - 4
    - 5
    - 6
    - 7
    - 10
````

### C-States

To save energy on a system, you can command the CPU to go into a low-power mode. Each CPU has several power modes, which
are collectively called C-States. These work by cutting the clock signal and power from idle CPUs, or CPUs that are not
executing commands.While you save more energy by sending CPUs into deeper C-State modes, it does take more time for the
CPU to fully “wake up” from sleep mode, so there is a trade-off when it comes to deciding the depth of sleep.

#### C-State Implementation in the Power Optimization Library

The driver that is used for C-States is the intel_idle driver. Everything associated with C-States in Linux is stored in
the /sys/devices/system/cpu/cpuN/cpuidle file or the /sys/devices/system/cpu/cpuidle file. To check the driver in use,
the user simply has to check the /sys/devices/system/cpu/cpuidle/current_driver file.

C-States have to be confirmed if they are actually active on the system. If a user requests any C-States, they need to
check on the system if they are activated and if they are not, reject the PowerConfig. The C-States are found in
/sys/devices/system/cpu/cpuN/cpuidle/stateN/.

#### C-State Ranges

````
C0      Operating State
C1      Halt
C1E     Enhanced Halt
C2      Stop Grant   
C2E     Extended Stop Grant
C3      Deep Sleep
C4      Deeper Sleep
C4E/C5  Enhanced Deeper Sleep
C6      Deep Power Down
````

#### Example

````yaml
apiVersion: power.intel.com/v1
kind: CStates
metadata:
  # Replace <NODE_NAME> with the name of the node to configure the C-States on that node
  name: <NODE_NAME>
spec:
  sharedPoolCStates:
    C1: true
  exclusivePoolCStates:
    performance:
      C1: false
  individualCoreCStates:
    "3":
      C1: true
      C6: false
````

### Time Of Day

The TIme Of Day feature allows users to change the configuration of their system at a given time each day. This is done
through the use of a `timeofdaycronjob`
which schedules itself for a specific time each day and gives users the option of tuning cstates, the shared pool
profile as well as the profile used by individual pods.

#### Example

```yaml
apiVersion: power.intel.com/v1
kind: TimeOfDay
metadata:
  name: timeofday-sample
  namespace: intel-power
spec:
  timeZone: "Eire"
  schedule:
    - time: "14:24"
      # this sets the profile for the shared pool
      powerProfile: balance-performance
      # this transitions a pod with the given name from one profile to another
      pods:
        four-pod-v2:
          performance: balance-performance
        four-pod-v4:
          balance-power: performance
      # this field simply takes a cstate spec
      cState:
        sharedPoolCStates:
          C1: false
          C6: true
    - time: "14:26"
      powerProfile: performance
      cState:
        sharedPoolCStates:
          C1: true
          C6: false
    - time: "14:28"
      powerProfile: balance-power
  reservedCPUs: [ 0,1 ]
```

When applying changes to the shared pool, users must specify the CPUs reserved by the system. Additionally the user must
specify a timezone to schedule with.
The configuration for Time Of Day consists of a schedule list. Each item in the list consists of a time and any desired
changes to the system.
The `profile` field specifies the desired profile for the shared pool.
The `pods` field is used to change the profile associated with a specific pod.
It should be noted that when changing the profile of an individual pod, the user must specify its name and current
profile at the time as well as the desired new profile.
Finally the `cState` field accepts the spec values from a CStates configuration and applies them to the system.

### Uncore Frequency

Uncore frequency can be configured on a system-wide, per-package and per-die level. Die config will precede package,
which will in turn precede system-wide configuration.\
Valid max and min uncore frequencies are determined by the hardware

#### Example

````yaml
apiVersion: power.intel.com/v1
kind: Uncore
metadata:
  name: <NODE_NAME>
  namespace: intel-power
spec:
  sysMax: 2300000
  sysMin: 1300000
  dieSelector:
    - package: 0
      die: 0
      min: 1500000
      max: 2400000
````

### intel-pstate CPU Performance Scaling Driver

The intel_pstate is a part of the CPU performance scaling subsystem in the Linux kernel (CPUFreq).

In some situations it is desirable or even necessary to run the program as fast as possible and then there is no reason
to use any P-states different from the highest one (i.e. the highest-performance frequency/voltage configuration
available). In some other cases, however, it may not be necessary to execute instructions so quickly and maintaining the
highest available CPU capacity for a relatively long time without utilizing it entirely may be regarded as wasteful. It
also may not be physically possible to maintain maximum CPU capacity for too long for thermal or power supply capacity
reasons or similar. To cover those cases, there are hardware interfaces allowing CPUs to be switched between different
frequency/voltage configurations or (in the ACPI terminology) to be put into different P-states.

#### P-State Governors

In order to offer dynamic frequency scaling, the cpufreq core must be able to tell these drivers of a "target
frequency". So these specific drivers will be transformed to offer a "->target/target_index/fast_switch()" call instead
of the "->setpolicy()" call. For set_policy drivers, all stays the same, though.

The cpufreq governors decide what frequency within the CPUfreq policy should be used. The P-state driver utilizes the "
powersave" and "performance" governors.

##### Powersave Governor

The CPUfreq governor "powersave" sets the CPU statically to the lowest frequency within the borders of scaling_min_freq
and scaling_max_freq.

##### Performance Governor

The CPUfreq governor "performance" sets the CPU statically to the highest frequency within the borders of
scaling_min_freq and scaling_max_freq.

### In the Kubernetes API

- PowerConfig CRD
- PowerWorkload CRD
- PowerProfile CRD
- PowerNode CRD
- C-State CRD

### Node Agent Pod

The Pod Controller watches for pods. When a pod comes along the Pod Controller checks if the pod is in the guaranteed
quality of service class (using exclusive
cores, [see documentation](https://kubernetes.io/docs/tasks/configure-pod-container/quality-service-pod/), taking a core
out of the shared pool (it is the only option in Kubernetes that can do this operation). Then it examines the Pods to
determine which PowerProfile has been requested and then creates or updates the appropriate PowerWorkload.

Note: the request and the limits must have a matching number of cores and are also in a container-by-container bases.
Currently the Kubernetes Power Manager only supports a single PowerProfile per Pod. If two profiles are requested in
different containers, the pod will get created but the cores will not get tuned.

## Repository Links

[Intel Power Optimization Library](https://github.com/intel/power-optimization-library)

## Installation

## Step by step build

### Setting up the Kubernetes Power Manager

- Clone the Kubernetes Power Manager

````
git clone https://github.com/intel/kubernetes-power-manager
cd kubernetes-power-manager
````

- Set up the necessary Namespace, Service Account, and RBAC rules for the Kubernetes Power Manager:

````
kubectl apply -f config/rbac/namespace.yaml
kubectl apply -f config/rbac/rbac.yaml
````

- Generate the CRD templates, create the Custom Resource Definitions, and install the CRDs:

````
make
````

- Docker Images
  Docker images can either be built locally by using the command:

````
make images
````

or available by pulling from the Intel's public Docker Hub at:

- intel/power-operator:TAG
- intel/power-node-agent:TAG

### Running the Kubernetes Power Manager

- **Applying the manager**

The manager Deployment in config/manager/manager.yaml contains the following:

````yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: controller-manager
  namespace: intel-power
  labels:
    control-plane: controller-manager
spec:
  selector:
    matchLabels:
      control-plane: controller-manager
  replicas: 1
  template:
    metadata:
      labels:
        control-plane: controller-manager
    spec:
      serviceAccountName: intel-power-operator
      containers:
        - command:
            - /manager
          args:
            - --enable-leader-election
          imagePullPolicy: IfNotPresent
          image: power-operator:v2.2.0
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop: [ "ALL" ]
          name: manager
          resources:
            limits:
              cpu: 100m
              memory: 30Mi
            requests:
              cpu: 100m
              memory: 20Mi
          volumeMounts:
            - mountPath: /sys/fs
              name: cgroup
              mountPropagation: HostToContainer
              readOnly: true
      terminationGracePeriodSeconds: 10
      volumes:
        - name: cgroup
          hostPath:
            path: /sys/fs
````

Apply the manager:
`kubectl apply -f config/manager/manager.yaml`

The controller-manager-xxxx-xxxx pod will be created.

- **Power Config**

The example PowerConfig in examples/example-powerconfig.yaml contains the following PowerConfig spec:

````yaml
apiVersion: "power.intel.com/v1"
kind: PowerConfig
metadata:
  name: power-config
spec:
  powerNodeSelector:
    feature.node.kubernetes.io/power-node: "true"
  powerProfiles:
    - "performance"
 ````

Apply the Config:
`kubectl apply -f examples/example-powerconfig.yaml`

Once deployed the controller-manager pod will see it via the Config controller and create a Node Agent instance on nodes
specified with the ‘feature.node.kubernetes.io/power-node: "true"’ label.

The power-node-agent DaemonSet will be created, managing the Power Node Agent Pods. The controller-manager will finally
create the PowerProfiles that were requested on each Node.

- **Shared Profile**

The example Shared PowerProfile in examples/example-shared-profile.yaml contains the following PowerProfile spec:

````yaml
apiVersion: "power.intel.com/v1"
kind: PowerProfile
metadata:
  name: shared
spec:
  name: "shared"
  max: 1500
  min: 1000
  # A shared PowerProfile must have the EPP value of 'power'
  epp: "power"
````

Apply the Profile:
`kubectl apply -f examples/example-shared-profile.yaml`

- **Shared workload**

The example Shared PowerWorkload in examples/example-shared-workload.yaml contains the following PowerWorkload spec:

````yaml
apiVersion: "power.intel.com/v1"
kind: PowerWorkload
metadata:
  # Replace <NODE_NAME> with the Node you intend this PowerWorkload to be associated with
  name: shared-<NODE_NAME>-workload
  namespace: intel-power
spec:
  # Replace <NODE_NAME> with the Node you intend this PowerWorkload to be associated with
  name: "shared-<NODE_NAME>-workload"
  allCores: true
  reservedCPUs:
    # IMPORTANT: The CPUs in reservedCPUs should match the value of the reserved system CPUs in your Kubelet config file
    - 0
    - 1
  powerNodeSelector:
    # The label must be as below, as this workload will be specific to the Node
    kubernetes.io/hostname: <NODE_NAME>
  # Replace this value with the intended shared PowerProfile
  powerProfile: "shared"
````

Replace the necessary values with those that correspond to your cluster and apply the Workload:
`kubectl apply -f examples/example-shared-workload.yaml`

Once created the workload controller will see its creation and create the corresponding Pool in all of the cores on the
system except for the reservedCPUs will be brought down to this lower frequency level.

- **Performance Pod**

The example Pod in examples/example-pod.yaml contains the following PodSpec:

````yaml
apiVersion: v1
kind: Pod
metadata:
  name: example-power-pod
spec:
  containers:
    - name: example-power-container
      image: ubuntu
      command: [ "/bin/sh" ]
      args: [ "-c", "sleep 15000" ]
      resources:
        requests:
          memory: "200Mi"
          cpu: "2"
          # Replace <POWER_PROFILE> with the PowerProfile you wish to request
          # IMPORTANT: The number of requested PowerProfiles must match the number of requested CPUs
          # IMPORTANT: If they do not match, the Pod will be successfully scheduled, but the PowerWorkload for the Pod will not be created
          power.intel.com/<POWER_PROFILE>: "2"
        limits:
          memory: "200Mi"
          cpu: "2"
          # Replace <POWER_PROFILE> with the PowerProfile you wish to request
          # IMPORTANT: The number of requested PowerProfiles must match the number of requested CPUs
          # IMPORTANT: If they do not match, the Pod will be successfully scheduled, but the PowerWorkload for the Pod will not be created
          power.intel.com/<POWER_PROFILE>: "2"
````

Replace the placeholder values with the PowerProfile you require and apply the PodSpec:

`kubectl apply -f examples/example-pod.yaml`

At this point, if only the ‘performance’ PowerProfile was selected in the PowerConfig, the user’s cluster will contain
three PowerProfiles and two PowerWorkloads:

`kubectl get powerprofiles -n intel-power`

````
NAME                          AGE
performance                   59m
performance-<NODE_NAME>       58m
shared-<NODE_NAME>            60m
````

`kubectl get powerworkloads -n intel-power`

````
NAME                                   AGE
performance-<NODE_NAME>-workload       63m
shared-<NODE_NAME>-workload            61m
````

- **Delete Pods**

`kubectl delete pods <name>`

When a Pod that was associated with a PowerWorkload is deleted, the cores associated with that Pod will be removed from
the corresponding PowerWorkload. If that Pod was the last requesting the use of that PowerWorkload, the workload will be
deleted. All cores removed from the PowerWorkload are added back to the Shared PowerWorkload for that Node and returned
to the lower frequencies.

