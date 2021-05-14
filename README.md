# Intel Power Operator

----------
Kubernetes Operator for Dynamic Configuration of Intel Speed Select Technologies (SST).
----------

Table of Contents
=================

   * [Intel Power Operator](#intel-power-operator)
      * [Kubernetes Operator for Dynamic Configuration of Intel Speed Select Technologies (SST).](#kubernetes-operator-for-dynamic-configuration-of-intel-speed-select-technologies-sst)
      * [Prerequisites](#prerequisites)
      * [Components](#components)
         * [AppQoS](#appqos)
         * [Node Agent](#node-agent)
         * [Power Operator](#power-operator)
      * [Custom Resource Definitions (CRDs)](#custom-resource-definitions-crds)
         * [PowerConfig](#powerconfig)
            * [Example](#example)
         * [PowerProfile](#powerprofile)
            * [Examples](#examples)
               * [High Performance (to be requested via pod spec)](#high-performance-to-be-requested-via-pod-spec)
               * [Standard Performance (to be applied to CPU Manager's <em>Shared Pool</em>)](#standard-performance-to-be-applied-to-cpu-managers-shared-pool)
         * [PowerWorkload](#powerworkload)
            * [PowerWorkload Created Automatically by Operator](#powerworkload-created-automatically-by-operator)
               * [Example - Specific Node and CPU IDs](#example---specific-node-and-cpu-ids)
            * [PowerWorkload Created Directly by User](#powerworkload-created-directly-by-user)
               * [Example - Shared Pool (Minus Reserved CPUs) Using PowerNodeSelector](#example---shared-pool-minus-reserved-cpus-using-powernodeselector)
               * [Create PowerWorkload](#create-powerworkload)
               * [List PowerWorkloads](#list-powerworkloads)
               * [Display a particular PowerWorkload:](#display-a-particular-powerworkload)
               * [Delete PowerWorkload](#delete-powerworkload)
         * [PowerNode](#powernode)
               * [List all PowerNodes on the cluster](#list-all-powernodes-on-the-cluster)
               * [Display a particular PowerNode such as the example above](#display-a-particular-powernode-such-as-the-example-above)
      * [Extended Resources](#extended-resources)
      * [Recommended Approach for Use With the <a href="https://kubernetes.io/docs/tasks/administer-cluster/cpu-management-policies/" rel="nofollow">CPU Manager</a>](#recommended-approach-for-use-with-the-cpu-manager)
         * [Example](#example-1)
            * [Node Setup](#node-setup)
            * [Create PowerProfiles](#create-powerprofiles)
               * [High Performance (to be requested via pod spec)](#high-performance-to-be-requested-via-pod-spec-1)
               * [Standard Performance (to be applied to CPU Manager's <em>Shared Pool</em>)](#standard-performance-to-be-applied-to-cpu-managers-shared-pool-1)
            * [Create PowerWorkload for Shared Pool](#create-powerworkload-for-shared-pool)
            * [Create Pod](#create-pod)
            * [Result](#result)
               * [Shared Pool](#shared-pool)
               * [Exclusively Allocated CPUs](#exclusively-allocated-cpus)

## Prerequisites
* Node Feature Discovery ([NFD](https://github.com/kubernetes-sigs/node-feature-discovery)) should be deployed in the cluster before running the operator. Once NFD has applied labels to nodes with capabilities. NFD is used to detect node-level features such as *Intel Speed Select Technology - Base Frequency (SST-BF)*. Once detected, the operator can take advantage of such features by configuring CPUs on the host to optimise performance for containerized workloads.
Note: NFD is recommended, but not essential. Node labels can also be applied manually. See the [NFD repo](https://github.com/kubernetes-sigs/node-feature-discovery#feature-labels) for a full list of features labels.
* A working AppQoS container image from the [AppQoS repo](https://github.com/intel/intel-cmt-cat/appqos).

## Components
### AppQoS
[AppQos](https://github.com/intel/intel-cmt-cat/appqos) is containerized application deployed by the power operator in a DaemonSet. AppQos is responsible for node level configuration of SST capabilities, as delegated to by the power operator via the AppQoS HTTP REST interface.

### Node Agent
The node agent is also a containerized application deployed by the power operator in a DaemonSet. The primary function of the node agent is to communicate with the node's Kubelet PodResources endpoint to discover the exact CPUs that are allocated per container.
The node agent watches for Pods that are created in your cluster and examines them to determine which Power Profile they have requested and then sets off the chain of events that tunes the frequencies of the cores designated to the Pod.

### Power Operator
The power operator is the main component of this project and is responsible for: 
* Deploying the AppQoS and Node Agent DaemonSets.
* Managing all associated custom resources.
* Discovery and advertisement of Power Profile extended resources.
* Sending HTTP requests to the AppQoS daemons for SST configuration.

## Custom Resource Definitions (CRDs)
### PowerConfig
The PowerConfig custom resource is the operator's main configuration object.
The PowerConfig spec consists of:
-   `appQoSImage`: This is the name/tag given to the AppQoS container image that will be deployed in a DaemonSet by the operator.
-   `powerNodeSelector`: This is a key/value map used for defining a list of node labels that a node must satisfy in order for AppQoS and the operator's node agent to be deployed.
-   `powerProfiles`: The list of PowerProfiles that the user ants available  the nodes.

The PowerConfig status represents the nodes which match the `powerNodeSelector` and, as such, have AppQoS and the power operator node agent deployed.

The Operator will wait for a PowerConfig to be created by the user, in which the desired PowerProfiles will be specified. From this, the Operator will deploy the AppQoS and node agent DaemonSets and communicate with the AppQoS Pod to create the desired PowerProfiles.

#### Example
````yaml
apiVersion: intel.com/v1alpha1
kind: PowerConfig
metadata:
    name: powerconfig
spec:
    appQoSImage: "appqos:latest"
    powerNodeSelector:
      "feature.node.kubernetes.io/cpu-sst_bf.enabled": "true"
      "feature.node.kubernetes.io/cpu-sst_cp.enabled": "true"
    powerProfiles: 
    - “performance” 
    - “power”
````
**Note:** Only one PowerConfig object is necessary per cluster. This is enforced by virtue of the default naming convention `"powerconfig"`.

### PowerProfile
The PowerProfile custom resource holds values for specific SST settings which are then applied to CPUs at host level by the operator as requested. Power Profiles are advertised as extended resources and can be requested via the pod spec.

#### Examples

##### High Performance (to be requested via pod spec)
````yaml
apiVersion: intel.com/v1alpha1
kind: PowerProfile
metadata:
    name: performance
spec:
  maxFrequency: 2700
  minFrequency: 2700
  epp: "performace"  
````

##### Standard Performance (to be applied to CPU Manager's *Shared Pool*)
````yaml
apiVersion: intel.com/v1alpha1
kind: PowerProfile
metadata:
    name: balance-performance
spec:
  maxFrequency: 2100
  minFrequency: 2100
  epp: "balance-performance"  
````

### PowerWorkload
The PowerWorkload custom resource is the object used to define the list(s) of CPUs configured with a particular power profile. A power workload can span multiple nodes.

#### PowerWorkload Created Automatically by Operator 
PowerWorkload objects are create **automatically** via the pod spec. This action is undertaken by the operator when a pod is created with a container(s) requesting exclusive CPU(s) *and* a power profile extended resource.

##### Example - Specific Node and CPU IDs
````yaml
apiVersion: intel.com/v1alpha1
kind: PowerWorkload
metadata:
    name: performance-workload
spec:
    nodes:
    - name: "worker-node-1"
      cpuIds: [2, 3, 22, 23]
    powerProfile: "performance"  
````
This workload assigns the `performance` power profile to CPUs 2, 3, 22, and 23 on node "worker-node-1".

This example shows a power workload that might be created automatically by the operator based on CPUs allocated to a container by the CPU Manager upon request via pod spec.

#### PowerWorkload Created Directly by User
PowerWorkload objects can also be created **directly** by the user via the PowerWorkload spec. This is only recommended for configuring the CPU Manager's shared pool.

It is not recommended to directly configure power workloads with specific nodes and CPU IDs. Instead, directly configuring power workloads should be done by utilising the `powerNodeSelector`, `sharedPool` and/or `reservedCPUs` options shown in the following example:

##### Example - Shared Pool (Minus Reserved CPUs) Using PowerNodeSelector
````yaml
apiVersion: intel.com/v1alpha1
kind: PowerWorkload
metadata:
    name: shared-workload
spec:
    sharedPool: true
    powerNodeSelector: 
      "feature.node.kubernetes.io/cpu-sst_bf.enabled": "true"
      "feature.node.kubernetes.io/cpu-sst_cp.enabled": "true"
    reservedCPUs: [0, 1, 20, 21]
    powerProfile: "shared"  
````
This workload assigns the `shared` power profile to all CPUs in the CPU Manager's shared pool (minus those specified in the `reservedCPUs` field) on nodes that match all the `powerNodeSelector` labels.

The `reservedCPUs` option is used to represent the list of CPUs which have been [reserved by the Kubelet](https://kubernetes.io/docs/tasks/administer-cluster/reserve-compute-resources/#explicitly-reserved-cpu-list). 
This option enables the user to only configure CPUs that are exclusively allocatable (i.e. shared pool - reserved CPUs) to containers with a given profile. This allows the reserved CPUs to continue with default settings, exempt from any power profile. 

**Note**: If `powerNodeSelector` is specified and a `nodes` list is also specified, `powerNodeSelector` will take precedence and the specified `nodes` list will be redundant.


##### Create PowerWorkload

`kubectl create -f <path>`

##### List PowerWorkloads
`kubectl get powerworkloads`

##### Display a particular PowerWorkload:
`kubectl describe powerworkload power-workload-performance`

````yaml
Name:         powerworkload-hp
Namespace:    default
API Version:  intel.com/v1alpha1
Kind:         PowerWorkload
Spec:
  Nodes:
    Name: worker-node-1
    CpuIds:
      2
      3
      22
      23
  PowerProfile: performance
Status:
  Nodes:
    Name: worker-node-1:
    CpuIds:
      2
      3
      22
      23
    PoolId: 3 
    PoolName: performance
    PowerProfile: performance
    Response: Success: 200
````
This displays the PowerWorkload object including the spec as defined above and the status of the workload. Here, the status shows that this workload was configured successfully on node "worker-node-1".

`kubectl describe powerworkload power-workload-performance`

````yaml
Name:         powerworkload-sp
Namespace:    default
API Version:  intel.com/v1alpha1
Kind:         PowerWorkload
Spec:
  sharedPool: true
  powerNodeSelector: 
    "feature.node.kubernetes.io/cpu-sst_bf.enabled": "true"
    "feature.node.kubernetes.io/cpu-sst_cp.enabled": "true"
  reservedCPUs:
    0
    1
    20
    21 	
  powerProfile: "shared"  
Status:
  Nodes:
  - Name: worker-node-1:
    CpuIds:
      4-19	 
      24-39 
    PoolId: 7
    PoolName: shared
    PowerProfile: shared
    Response: Success: 200
  - Name: worker-node-2:
    CpuIds:
      2-19	 
      22-39 
    PoolId: 4
    PoolName: shared
    PowerProfile: shared
    Response: Success: 200

````
This displays the PowerWorkload object including the spec as defined above and the status of the workload. Here, the status shows that this workload was configured successfully on nodes "worker-node-1" and "worker-node-2". Note the status of "worker-node-1" `CpuIds` does not include the CPUs allocated to the `power-workload-hp` (2, 3, 22, 23) as they are no longer part of the shared pool.



##### Delete PowerWorkload
When the user deletes a PowerWorkload object, a delete request is sent to the AppQoS API on every AppQoS instance on which that PowerWorkload is configured.

`kubectl delete powerworkload powerworkload-sp`

### PowerNode
The PowerNode custom resource is created for each node in the cluster which matches the `powerNodeSelector` labels in the `PowerConfig` object. The purpose of this object is to allow the user to view all running workloads on a particular node at any given time.

Each PowerNode object will be named according to its corresponding node (ie `power-node-<node-name>`).

##### List all PowerNodes on the cluster
`kubectl get powernodes`

##### Display a particular PowerNode such as the example above
`kubectl describe powernode power-node-worker-node-1`

````yaml
Name:         power-node-worker-node-1
Namespace:    default
API Version:  intel.com/v1alpha1
Kind:         PowerNode
Spec:
  nodeName:      worker-node-1
Status:
  powerNodeCPUState:
    guaranteedPods:
      - name: worker-node-pod
        node: worker-node-1
        uid: 54f92b0d-ec99-4cf5-8a5e-00cd3a5ee63f
        containers:
          - name: container1
            id: fhsaldkjfh18431nfqwe14441293
            powerProfile: performance
            exclusiveCpus:
              [2, 3, 66, 67]
    workloads:
      - name: performance-workload
        powerProfile: performance
        cores:
          [2, 3, 66, 67]
        containers:
          - name: container1
            pod: worker-node-pod
            id: fhsaldkjfh18431nfqwe14441293
        
````
This example displays the PowerNode for worker-node-1.

## Extended Resources
Power profiles are advertised as extended resources on all nodes that match the `PowerConfig.PowerNodeSelector` labels.

For `v0.1`, the power operator will advertise a single `high-performance` profile on all applicable nodes. This will be represented as extended resource: `sst.intel.com/high-performance`.

The number of `sst.intel.com/high-performance` resources advertised will be equal to 40% of `cpu` resources on the node. This is to ensure that a node is never overprescribed with high frequency workloads.

For example, an SST capable node with a capacity of 72 `cpu` resources will advertise 28 `sst.intel.com/high-performance` resources (72 x 40% = 28):

````yaml
Capacity:
  cpu:                            72
  sst.intel.com/high-performance: 28
Allocatable:
  cpu:                            70
  sst.intel.com/high-performance: 28
````
This is an example of a node's resources, requestable via the pod spec's container resource requests.

It is essential that the user requests `cpu` and `sst.intel.com/high-performance` resources in equal amounts. This is covered in more detail in the example below.

## Recommended Approach for Use With the [CPU Manager](https://kubernetes.io/docs/tasks/administer-cluster/cpu-management-policies/)

It is recommended that nodes which are to be governed by the power operator (i.e. nodes that match the `PowerConfig`'s `powerNodeSelector` labels) are [designated](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/#example-use-cases) exclusively to containers that require CPUs with optimised SST settings.

Once these nodes are labelled and tainted as such, the operator can work harmoniously with the node's CPU Manager to *(a)* configure the shared pool with a power saving profile and *(b)* configure exclusively allocated CPUs with performance enhancing profiles as requested.

The following is an example of how this can be achieved. 

**Important**: Please read all necessary documentation linked in the example before attempting this approach or similar.

### Example
#### Node Setup
The following steps should be carried out on nodes that already possess Intel SST capabilities and are labelled as such by NFD.

* Set the Kubelet flag `reserved-cpus` with a list of specific [CPUs to be reserved for system and Kubernetes daemons](https://kubernetes.io/docs/tasks/administer-cluster/reserve-compute-resources/#explicitly-reserved-cpu-list):
  
  `--reserved-cpus=0-1,20-21`
  
  Reserved CPUs 0-1,20-21 are no longer **exclusively** allocatable by the CPU Manager. Simply put, a container requesting exclusive CPUs cannot be allocated CPUs 0-1,20-21.
  
  CPUs 0-1,20-21 do, however, remain in the CPU Manager's shared pool.
  
  **Note:** The list of `reservedCPUs` specified here is an example only. Which CPUs are to be reserved is at the user's discretion. 
  
* Apply a label to this node representing that this node is to be a designated "power node" (eg `intel.power.node=true`). Please read K8s documentation on [node labelling](https://kubernetes.io/docs/tasks/configure-pod-container/assign-pods-nodes/).
    
  `kubectl label node <node-name> intel.power.node=true`
    
* Apply a taint to this node in order to only allow pods that tolerate the taint to be scheduled to this node. Please read K8s documentation on [node tainting](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/).
    
  `kubectl taint nodes <node-name> intel.power.node=true:NoSchedule`
    
* Repeat these steps for all nodes that are to be designated, ensuring the same `reserved-cpus` list is used for all nodes.

#### Create PowerProfiles

##### High Performance (to be requested via pod spec)
````yaml
apiVersion: intel.com/v1alpha1
kind: PowerProfile
metadata:
    name: performance
spec:
  maxFrequency: 2700
  minFrequency: 2700
  epp: "performance"  
````

##### Standard Performance (to be applied to CPU Manager's *Shared Pool*)
````yaml
apiVersion: intel.com/v1alpha1
kind: PowerProfile
metadata:
    name: shared
spec:
  maxFrequency: 2100
  minFrequency: 2100
  epp: "power"  
````

#### Create PowerWorkload for Shared Pool

````yaml
apiVersion: intel.com/v1alpha1
kind: PowerWorkload
metadata:
    name: shared-workload
spec:
  sharedPool: true
  reservedCPUs: [0, 1, 20, 21"]
  powerNodeSelector: 
    intel.power.node: "true"
  powerProfile: shared
````

This workload is to be configured for all *Allocatable* CPUs on the desired node(s). This is done by setting `sharedPool` to true and specifying `reservedCPUs` with the **same CPU list that has been reserved by the Kubelet** in the earlier step. 

This workload will be configured on all nodes that have been labelled `intel.power.node=true` in the earlier step. This is achieved through the PowerWorkload spec field `powerNodeSelector`.

**Note:** The list of `reservedCPUs` is only taken into consideration when `sharedPool` is set to true. 
#### Create Pod

````yaml
 apiVersion corev1
kind Pod
metadata:
  name: hp-critical
spec:
  nodeSelector:
    intel.power.node: "true"
  tolerations:
  - key: "intel.power.node"
    operator: Equal
    value: "true"
    effect: NoSchedule
  containers:
  - name: container1
    resources:
      requests:
        memory: "64Mi"
        cpu: 3
        sst.intel.com/performance: 3
      limits:
        memory: "64Mi"
        cpu: 3
        sst.intel.com/performance: 3 
````
This pod is provided with a `nodeSelector` for the designated node label, and a `taintToleration` for the designated node taint. 

This is a guaranteed pod requesting exclusive CPUs.

Also, this pod is requesting extended resource `sst.intel.com/high_performance`. This ensures that the operator will allocate this power profile to the container's exclusively allocated CPUs.

**Note**: The number of `sst.intel.com/high_performance` resources requested **must** equal the number of `cpu` resources requested.

Please read K8s documentation on [assigning pods to nodes](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#:~:text=Run%20kubectl%20get%20nodes%20to,the%20node%20you've%20chosen).

#### Result

##### Shared Pool
* The `reservedCPUs` on all `intel.power.node` nodes continue to run with default configuration and are not impacted by any power profile.
* The allocatable CPUs (shared pool - reserved CPUs) on all `intel.power.node` nodes are configured with the `standard-performace` power profile.

##### Exclusively Allocated CPUs
* The `hp-critical` pod will be scheduled to a designated `intel.power.node` node.
* The `hp-critical` pod's container will be allocated 3 exclusive CPUs by the CPU Manager.
* The operator will configure these 3 CPUs with the settings of the `high-performance` power profile.

