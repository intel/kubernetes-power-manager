# Intel Power Operator

Table of Contents
=================

   * [Intel Power Operator](#intel-power-operator)
      * [Kubernetes Operator for Dynamic Configuration of Intel Speed Select Technologies (SST).](#kubernetes-operator-for-dynamic-configuration-of-intel-speed-select-technologies-sst)
      * [What is the Power Operator?](#What-is-the-Power-Operator?)
      * [Power Operators main responsibilities](#Power-Operators-main-responsibilities)
      * [Use Cases](#Use-Cases)
      * [Prerequisites](#prerequisites)
      * [Components](#components)
         * [AppQoS](#appqos)
         * [Node Agent](#node-agent)
         * [Power Operator](#power-operator)
    

 
## What is the Power Operator?

In a container orchestration engine such as Kubernetes, the allocation of CPU resources from a pool of platforms is based solely on availability with now consideration of individual capabilities such as Intel Speed Select Technology(SST).

The Intel Kubernetes Power Operator is a software solution designed to expose and utilize Intel specific power management technologies in a Kubernetes Environment.

The SST is a powerful collection of features that offers more granular control over CPU performance and power consumption on a per-core basis. However, as a workload orchestrator, Kubernetes is internationally designed to provide a layer of abstraction between the workload and such hardware capabilities. This presents a challenge to Kubernetes users running performance critical workloads with specific requirements dependent on hardware capabilities.

The Power Operator bridges the gap between the container orchestration layer and hardware features enablement, specifically Intel SST.


### Power Operators main responsibilities:

- Deploying the AppQoS and Node Agent DaemonSets
- Managing all associated custom resources
- Discovery and advertisement of the Power Profile extended resources.
- Sending HTTO requests to the AppQoS daemons for SST configuration.
    

### Use Cases:

- *High performance workload known at peak times.*
   May want to pre-schedule nodes to move to a performance profile during peak times to minimize spin up.
  At times not during peak, may want to move to a power saving profile.
- *Unpredictable machine use.*
   May use machine learning through monitoring to determine profiles that predict a peak need for a compute, to spin up ahead of  time.
- *Power Optimization over Performance.*
   A cloud may be interested in fast response time, but not in maximal response time, so may choose to spin up cores on demand and only those cores used, but want to remain in power-saving mode the rest of the time.  


## Initial release functionality of the Power Operator

- **SST-BF - (Speed Select Technology - Base Frequency)** - 


	This feature allows the user to control the base frequency of certain cores.
	The base frequency is a guaranteed level of performance on the CPU (a CPU will never go below its base frequency).
	They can be set to a higher than a majority (priority CPUs) of the other cores on the system to which they can apply their critical workloads for a guaranteed performance.

- **SST-CP - (Speed Select Technology - Core Power)** - 

	
    This feature allows the user to group cores into levels of priority.
    When there is power to spare on the system, it can be distributed among the cores based on their priority level.
    There are four levels of priority available:
1. Performances
2. Balance Performances
3. Balance Power
4. Power

    The Priority level for a core is defined using its EPP (Energy Performance Preference) value, which is one of the options in the Power Profiles. If not all the power is utilized on the CPU, the CPU can put the higher priority cores up to Turbo Frequency (allows the CPUs to run faster).


- **Frequency Tuning**

    
    Frequency tuning allows the individual cores on the system to be sped up or slowed down by changing their frequency.
    This tuning is done via the CommsPowerManagement python library which is utilized via AppQoS.
    The min and max values for a core are defined in the Power Profile and the tuning is done after the core has been assigned by the Native CPU Manager.
    How exactly the frequency of the cores is changed is by simply writing the new frequency value to the /sys/devices/device/system/cpu/cpuN/cpufreq/scaling_max|min_freq file for the given core.

Note: In the future, we want to move away from the CommsPowerManagement library to create a Go Library that utilizes the Intel Pstate drive. The Pstate driver checks for overheating on the system.

## Future planned additions to the Power Operator

- **SST-TF - Turbo Frequency**

    This feature allows the user to set different “All-Core Turbo Frequency” (max freq all cores can be running at) values to individual cores based on their priority.
	All-Core Turbo is the elevated frequency at which all cores can run on the system at the same time.
    The user can set certain cores to have a higher turbo frequency by lowering the value of other cores or setting them to no value at all.

    This feature is only useful when all cores on the system are being utilized, but the user still wants to be able to configure certain cores to get a higher performance than others.


## Prerequisites
* Node Feature Discovery ([NFD](https://github.com/kubernetes-sigs/node-feature-discovery)) should be deployed in the cluster before running the operator. Once NFD has applied labels to nodes with capabilities. NFD is used to detect node-level features such as *Intel Speed Select Technology - Base Frequency (SST-BF)*. Once detected, the operator can take advantage of such features by configuring CPUs on the host to optimise performance for containerized workloads.
Note: NFD is recommended, but not essential. Node labels can also be applied manually. See the [NFD repo](https://github.com/kubernetes-sigs/node-feature-discovery#feature-labels) for a full list of features labels.
* A working AppQoS container image from the [AppQoS repo](https://github.com/intel/intel-cmt-cat/appqos).

## Components
### AppQoS
[AppQos](https://github.com/intel/intel-cmt-cat/appqos), an alias for “Application Quality of Service”, is an Intel software suite that provides node level configuration of hardware capabilities such as SST. AppQoS is deployed on the host as a containerized application and exposes a REST interface to the orchestration layer. Using the Intel Comms Power libraries as a back end, AppQoS can perform host-level configurations of SST features based on API calls from the Power Operator.

### Node Agent
The node agent is also a containerized application deployed by the power operator in a DaemonSet. The primary function of the node agent is to communicate with the node's Kubelet PodResources endpoint to discover the exact CPUs that are allocated per container. The node agent watches for Pods that are created in your cluster and examines them to determine which Power Profile they have requested and then sets off the chain of events that tunes the frequencies of the cores designated to the Pod.

### Config Controller
The Power Operator will wait for the Config  to be created by the user, in which the desired PowerProfiles will be specified.  The powerConfig holds different values: what image is required, what nodes the user wants to place the node agent on and what profile that is required. 
* appQoSImage: This is the name/tag given to the AppQoS container image that will be deployed in a DaemonSet by the operator.
* powerNodeSelector: This is a key/value map used for defining a list of node labels that a node must satisfy in order for AppQoS and the operator's node agent to be deployed.
* powerProfiles: The list of PowerProfiles that the user ants available the nodes.

Once the Config Controller sees that the powerConfig is created, it reads the values and then deploys the node agent and the AppQoS agent on to each of the nodes that are specified. It then creates the profiles and extended resources. Extended resources are resources created in the cluster that can request in the podSpec, the Kubelet can then keep track of these requests, it is important to us as it can specify how many CPUs on the system can be run at a higher frequency before hitting the heat threshold (kubelet does most of the work here).

### Example
````yaml
apiVersion: "power.intel.com/v1alpha1"
kind: PowerConfig
metadata:
   namee: power-config
spec:
   powerImage: "appqos:latest"
   powerNodeSelector:
      feature.node.kubernetes.io/appqos-node: "true"
   powerProfiles:
   - "performance"
````

### Workload Controller 
The Workload Controller is responsible for the actual tuning of the CPUs. The Workload Controller contacts the AppQoS agent and requests that it creates powerProfiles in AppQoS. The powerProfiles hold the value of the maximum and minimum frequencies, the CPUs that need to be configured and then carries out the changes.
		
The PowerWorkload objects can be created automatically via the pod spec.  This action is undertaken by the Operator when a pod is crated with a container requesting exclusive CPUs and a Power Profile.
		
PowerWorkload objects can also be create directly by the user via the PowerWorkload spec.  This is only recommended for configuring the CPU Manager upon request via pod spec.
		
### Example
````yaml
apiVersion: "power.intel.com/v1alpha1"
kind: PowerWorkload
metadata:
    name: performance-workload
spec:
   node:
   - name: "ice-lake"
     cpuIds:
     - 5
     - 6
   powerProfile: "performance"
````
This workload assigns the performance Power Profile to CPUs 5 and 6 on the node "ice-lake"		


### Profile Controller
The Profile Controller holds values for specific SST settings which are then applied to CPUs at host level by the operator as requested. Power Profiles are advertised as extended resources and can be requested via the pod spec.
Keep track of the requested profiles. The profile controller makes sure that all the requirement and specs are all correct. There is only one workload per profile.
		
#### Example
````yaml
apiVersion: "power.intel.com/v1alpha1"
kind: PowerProfile
metadata:
   name: shared
spec:
   name: "Shared"
   max: 1500
   min: 1000
   epp: "power"
````


### PowerNode Controller
The PowerNode controller is a way to have a view of what is going on in the cluster. 
It will detail what workloads are being used currently, which profiles are being used, what CPUs are being used and what containers they have (it links the containers to the workload)



### In the Kubernetes API
The PowerWorkload custom resource is the object used to define the list(s) of CPUs configured with a particular power profile. A power workload can span multiple nodes.

- PowerConfig CRD

- PowerWorkload CRD

- PowerProfile CRD

- PowerNode CRD



### AppQoS Agent Pod
There is an AppQoS and a Node Agent on each node in the cluster that you want to be running the cluster on. This is necessary because of node specific tuning that goes on. The AppQoS agent keeps track of the shared pool. Workload to pool mapping, the pool will hold the cores that are being tuned by a certain powerProfile. This is where the call to the CommsPowerManagement library is made. Creates the pool in AppQoS. The pool consists of the CPUs and the desired profile.


### Node Agent Pod
The Pod Controller watches for pods. When a pod comes along the Pod Controller checks if the pod is a guaranteed quality of service class (using exclusive cores), taking a core out of the shared pool (it is the only option in Kubernetes that can do this operation). Then it examines the Pods to determine which Power Profile has been requested and then  sets off the chain of events that tunes the frequencies of the cores designated to the Pod.

Note: the request and the limits must have a matching number of CPUs and are also in a container-by-container bases. If two profiles are created together but have different priority levels, the pods will get created but the CPUs won’t get tuned correctly.


### Pod
The Pod Spec will contain the finer details of the deployment. It will list the number of CPUs required, also define the requests and the limits.


## Workflow
### Execution Diagram


### Power Opperator Workflow
1. The Config Controller sees the user-created PowerConfig CRD and determines the desired nodes for the Power Operator.
2. The ConfigController updates the node status for each of these nodes in the Kubernetes API.
3. The ConfigController publishes the PodSpec to the Kubernetes API for the AppQoS Agent and the Power Operator Node Agent.
4. The Kubelet creates the Pods for both agents on the desired nodes.
5. The ConfigController creates the PowerProfile CRDs that were requested in the Power Config.
6. The Profile Controller sees the created PowerProfile CRDs and creates the corresponding Power Profile Objects in each AppQoS Agent instance.
7. The user creates a “Shared” Profile along with a “Shared Workload” for the nodes in the cluster. The Workload Controller Recognises this workload as shared and tunes the cores in the shared pool of each appropriate node.
8. The user deploys a Pod requesting a Power Profile.
9. The PodController sees this this Pod and determines the Power Profile it want to use, then creates or updates the corresponding PowerWorkload CRD in the Kubernetes API.
10. The Workload Controller sees the created or updated PowerWorkload CRD and creates or updates the corresponding Pool object in the AppQoS instance on the appropriate node.
11. The AppQoS Agent then tunes the frequency of the required cores on its node.
 





## Repository Links
### Power Operator repository
[Power Operator](https://gitlab.devtools.intel.com/OrchSW/CNO/power-operator)
### AppQoS repository
[AppQoS](https://github.com/intel/intel-cmt-cat)


## Dependencies and Installation 
### Dependencies

`yum -y install yum-utils`

`yum install python3`

`yum install python-pip`

`pip install -U pip`

`pip install -U virtualenv`

 
**Install JQ**
 
 `yum install epel-release -y`
 
 `yum update -y`
 
 `yum install jq -y`
 


## Step by step build

### AppQoS
- Clone AppQoS https://github.com/intel/intel-cmt-cat
- cd into docker dictory
- Run `./build_docker.sh`

### Setting up the Power Operator 
- Clone Power Operator https://gitlab.devtools.intel.com/OrchSW/CNO/power-operator
- cd into config/rbac/
- Apply the following:

 `kubectl apply -f leader_election_role_binding.yaml`

 `kubectl apply -f leader_election_role.yaml`

 `kubectl apply -f role_binding.yaml`

 `kubectl apply -f role.yaml`

- `make generate`
This will generate the CRDs templates.
- `make manifests`
This creates the Custom Resource Definitons.  The CRs are created int the config.crd/bases/*
- `make install`
This configures the powerconfigs, powernodes, powerpods, powerprofiles and powerworkload.
- `make docker-build`
This will build all the binaries needed.  This will also build the docker image.
- `docker images`
The output should include operator and intel-power-node-agent






