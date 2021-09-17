# Intel Power Operator

 
## What is the Power Operator?

In a container orchestration engine such as Kubernetes, the allocation of CPU resources from a pool of platforms is based solely on availability with now consideration of individual capabilities such as Intel Speed Select Technology (SST).

The Intel Kubernetes Power Operator is a software solution designed to expose and utilize Intel specific power management technologies in a Kubernetes Environment.

The SST is a powerful collection of features that offers more granular control over CPU performance and power consumption on a per-core basis. However, as a workload orchestrator, Kubernetes is internationally designed to provide a layer of abstraction between the workload and such hardware capabilities. This presents a challenge to Kubernetes users running performance critical workloads with specific requirements dependent on hardware capabilities.

The Power Operator bridges the gap between the container orchestration layer and hardware features enablement, specifically Intel SST.

### Power Operators main responsibilities:

- Deploying the Power Node Agent DaemonSet, targeting the nodes desired by the user, which contains the AppQoS Agent and Node Agent in a single Pod
- Managing all associated custom resources
- Discovery and advertisement of the Power Profile extended resources.
- Sending HTTO requests to the AppQoS daemons for SST configuration.
    

### Use Cases:

- *High performance workload known at peak times.*
   May want to pre-schedule nodes to move to a performance profile during peak times to minimize spin up.
   At times not during peak, may want to move to a power saving profile.
- *Unpredictable machine use.*
   May use machine learning through monitoring to determine profiles that predict a peak need for a compute, to spin up ahead of time.
- *Power Optimization over Performance.*
   A cloud may be interested in fast response time, but not in maximal response time, so may choose to spin up cores on demand and only those cores used but want to remain in power-saving mode the rest of the time.  

## Initial release functionality of the Power Operator

- **SST-BF - (Speed Select Technology - Base Frequency)** - 

  This feature allows the user to control the base frequency of certain cores.
  The base frequency is a guaranteed level of performance on the CPU (a CPU will never go below its base frequency).
  Priority cores can be set to a higher base frequency than a majority of the other cores on the system to which they can apply their critical workloads for a guaranteed performance.

- **SST-CP - (Speed Select Technology - Core Power)** - 

  
    This feature allows the user to group cores into levels of priority.
    When there is power to spare on the system, it can be distributed among the cores based on their priority level.
    While it is not guaranteed that the extra power will be applied to the highest priority cores, the system will do its best to do so.
    There are four levels of priority available:
1. Performances
2. Balance Performances
3. Balance Power
4. Power

    The Priority level for a core is defined using its EPP (Energy Performance Preference) value, which is one of the options in the Power Profiles. If not all the power is utilized on the CPU, the CPU can put the higher priority cores up to Turbo Frequency (allows the cores to run faster).

- **Frequency Tuning**

    
    Frequency tuning allows the individual cores on the system to be sped up or slowed down by changing their frequency.
    This tuning is done via the CommsPowerManagement python library which is utilized via AppQoS.
    The min and max values for a core are defined in the Power Profile and the tuning is done after the core has been assigned by the Native CPU Manager.
    How exactly the frequency of the cores is changed is by simply writing the new frequency value to the /sys/devices/device/system/cpu/cpuN/cpufreq/scaling_max|min_freq file for the given core.

Note: In the future, we want to move away from the CommsPowerManagement library to create a Go Library that utilizes the Intel Pstate driver.

## Future planned additions to the Power Operator

- **SST-TF - Turbo Frequency**

    This feature allows the user to set different “All-Core Turbo Frequency” values to individual cores based on their priority.
    All-Core Turbo is the Turbo Frequency at which all cores can run on the system at the same time.
    The user can set certain cores to have a higher All-Core Turbo Frequency by lowering this value for other cores or setting them to no value at all.

    This feature is only useful when all cores on the system are being utilized, but the user still wants to be able to configure certain cores to get a higher performance than others.

## Prerequisites
* Node Feature Discovery ([NFD](https://github.com/kubernetes-sigs/node-feature-discovery)) should be deployed in the cluster before running the operator. NFD is used to detect node-level features such as *Intel Speed Select Technology - Base Frequency (SST-BF)*. Once detected, the user can instruct the Operator to deploy the Power Node Agent to Nodes with SST-specific labels, allowing the Power Node Agent to take advantage of such features by configuring cores on the host to optimise performance for containerized workloads.
Note: NFD is recommended, but not essential. Node labels can also be applied manually. See the [NFD repo](https://github.com/kubernetes-sigs/node-feature-discovery#feature-labels) for a full list of features labels.
* A working AppQoS container image from the [AppQoS repo](https://github.com/intel/intel-cmt-cat/appqos).

## Components
### AppQoS
[AppQos](https://github.com/intel/intel-cmt-cat/appqos), an alias for “Application Quality of Service”, is an Intel software suite that provides node level configuration of hardware capabilities such as SST. AppQoS is deployed on the host as a containerized application and exposes a REST interface to the orchestration layer. Using the Intel Comms Power libraries as a back end, AppQoS can perform host-level configurations of SST features based on API calls from the Power Operator.

### Power Node Agent
The Power Node Agent is also a containerized application deployed by the power operator in a DaemonSet. The primary function of the node agent is to communicate with the node's Kubelet PodResources endpoint to discover the exact cores that are allocated per container. The node agent watches for Pods that are created in your cluster and examines them to determine which Power Profile they have requested and then sets off the chain of events that tunes the frequencies of the cores designated to the Pod.

### Config Controller
The Power Operator will wait for the PowerConfig to be created by the user, in which the desired PowerProfiles will be specified. The PowerConfig holds different values: what image is required, what Nodes the user wants to place the node agent on and what PowerProfiles are required. 
* appQoSImage: This is the name/tag given to the AppQoS container image that will be deployed in a DaemonSet by the operator.
* powerNodeSelector: This is a key/value map used for defining a list of node labels that a node must satisfy in order for AppQoS and the Power Node Agent to be deployed.
* powerProfiles: The list of PowerProfiles that the user wants available the nodes.

Once the Config Controller sees that the PowerConfig is created, it reads the values and then deploys the node agent and the AppQoS agent on to each of the nodes that are specified. It then creates the PowerProfiles and extended resources. Extended resources are resources created in the cluster that can requested in the podSpec, the Kubelet can then keep track of these requests, it is important to us as it can specify how many cores on the system can be run at a higher frequency before hitting the heat threshold (kubelet does most of the work here).

### Example
````yaml
apiVersion: "power.intel.com/v1alpha1"
kind: PowerConfig
metadata:
   name: power-config
spec:
   powerImage: "appqos:latest"
   powerNodeSelector:
      feature.node.kubernetes.io/appqos-node: "true"
   powerProfiles:
   - "performance"
````

### Workload Controller 
The Workload Controller is responsible for the actual tuning of the cores. The Workload Controller contacts the AppQoS agent and requests that it creates Pools in AppQoS. The Pools hold the PowerProfile associated with the cores and the cores that need to be configured.
    
The PowerWorkload objects are created automatically by the PowerPod controller. This action is undertaken by the Operator when a pod is created with a container requesting exclusive cores and a PowerProfile.
    
PowerWorkload objects can also be create directly by the user via the PowerWorkload spec. This is only recommended when creating the Shared PowerWorkload for a given Node, as this is the responsibility of the user. If no Shared PowerWorkload is created, the cores that remain in the ‘shared pool’ on the Node will remain at their core frequency values instead of being tuned to lower frequencies. PowerWorkloads are specific to a given node, so one is created for each Node with a Pod requesting a PowerProfile, based on the PowerProfile requested.
    
### Example
````yaml
apiVersion: "power.intel.com/v1alpha1"
kind: PowerWorkload
metadata:
    name: performance-example-node-workload
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
     name: “example”
     cpuIds:
     - 2
     - 3
     - 66
     - 67
   powerProfile: "performance-example-node"
````
This workload assigns the “performance” PowerProfile to cores 2, 3, 66, and 67 on the node “example-node”

The Shared PowerWorkload created by the user is determined by the Workload controller to be the designated Shared PowerWorkload based on the AllCores value in the Workload spec. The reserved CPUs on the Node must also be specified, as these will not be considered for frequency tuning by the controller as they are always being used by Kubernetes’ processes. It is important that the reservedCPUs value directly corresponds to the reservedCPUs value in the user’s Kubelet config to keep them consistent. The user determines the Node for this PowerWorkload using the PowerNodeSelector to match the labels on the Node. The user then specifies the requested PowerProfile to use.

### Example
````yaml
apiVersion: "power.intel.com/v1alpha1"
kind: PowerWorkload
metadata:
    name: shared-example-node-workload
spec:
   name: "shared-example-node-workload"
   allCores: true
   reservedCPUs:
     - 0
     - 1
   powerNodeSelector:
     # Labels other than hostname can be used
     - “Kubernetes.io/hostname”: “example-node”
   powerProfile: "performance-example-node"
````


### Profile Controller
The Profile Controller holds values for specific SST settings which are then applied to cores at host level by the operator as requested. Power Profiles are advertised as extended resources and can be requested via the PodSpec. The Config controller creates the requested high-performance PowerProfiles depending on which are requested in the PowerConfig created by the user.

There are two kinds of PowerProfiles:
	- Base PowerProfiles
	- Extended PowerProfiles

A Base PowerProfile can be one of three values:
	- performance
	- balance-performance
	- balance-power
These correspond to three of the EPP values associated with SST-CP. Base PowerProfiles are used to tell the Profile controller that the specified profile is being requested for the cluster. The Profile controller takes the created Profile and further creates an Extended PowerProfile. An Extended PowerProfile is Node-specific. The reason behind this is that different Nodes in your cluster may have different maximum frequency limitations for example, one Node may have the maximum limitation of 3700GHz, while another may only be able to reach frequency levels of 3200GHz. An Extended PowerProfile queries the Node that it is running on to obtain this maximum limitation and sets the Max and Min values of the profile accordingly. An Extended PowerProfile’s name has the following form: <Base PowerProfile Name>-<Node Name>, for example: “performance-example-node”.

Either the Base PowerProfile or the Extended PowerProfile can be requested in the PodSpec, as the Workload controller can determine the correct PowerProfile to use from the Base PowerProfile.

#### Example
````yaml
apiVersion: "power.intel.com/v1alpha1"
kind: PowerProfile
metadata:
   name: performance-example-node
spec:
   name: "performance-example-node"
   max: 3700
   min: 3300
   epp: "performance"
````

The Shared PowerProfile must be created by the user and does not require a Base PowerProfile. This allows the user to have a Shared PowerProfile per Node in their cluster, giving more room for different configurations. The Power controller determines that a PowerProfile is being designated as ‘Shared’ through the use of the ‘power’ EPP value.
    
#### Example
````yaml
apiVersion: "power.intel.com/v1alpha1"
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
apiVersion: "power.intel.com/v1alpha1"
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
The PowerNode controller is a way to have a view of what is going on in the cluster. 
It details what workloads are being used currently, which profiles are being used, what cores are being used and what containers they have. It also gives insight to the user as to which Shared Pool in the AppQoS agent is being used. The two Shared Pools can be the Default Pool or the Shared Pool. If there is no Shared PowerProfile associated with the Node, then the Default Pool will hold all the cores in the ‘shared pool’, none of which will have their frequencies tuned to a lower value. If a Shared PowerProfile is associated with the Node, the cores in the ‘shared pool’ – excluding cores reserved for Kubernetes processes (reservedCPUs) - will be placed in the Shared Pool in AppQoS and have their cores tuned.

#### Example
````
activeProfiles:
    performance-silpixa00401062: true
  activeWorkloads:
  - cores:
    - 2
    - 3
    - 8
    - 9
    name: performance-silpixa00401062-workload
  nodeName: silpixa00401062
  powerContainers:
  - exclusiveCpus:
    - 2
    - 3
    - 8
    - 9
    id: c392f492e05fc245f77eba8a90bf466f70f19cb48767968f3bf44d7493e18e5b
    name: ubuntu-one-container
    pod: one-container
    powerProfile: performance-silpixa00401062
    workload: performance-silpixa00401062-workload
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
    performance-silpixa00401062: true
  activeWorkloads:
  - cores:
    - 2
    - 3
    - 8
    - 9
    name: performance-silpixa00401062-workload
  nodeName: silpixa00401062
  powerContainers:
  - exclusiveCpus:
    - 2
    - 3
    - 8
    - 9
    id: c392f492e05fc245f77eba8a90bf466f70f19cb48767968f3bf44d7493e18e5b
    name: ubuntu-one-container
    pod: one-container
    powerProfile: performance-silpixa00401062
    workload: performance-silpixa00401062-workload
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



### In the Kubernetes API
- PowerConfig CRD

- PowerWorkload CRD

- PowerProfile CRD

- PowerNode CRD


### AppQoS Agent Pod
There is an AppQoS and a Node Agent on each node in the cluster that you want power optimization to occur. This is necessary because of node specific tuning. The AppQoS agent keeps track of the shared pool. The PowerWorkload creates the pool in AppQoS, which consists of the cores and the desired profile. This is where the call to the CommsPowerManagement library is made.

### Node Agent Pod
The Pod Controller watches for pods. When a pod comes along the Pod Controller checks if the pod is in the guaranteed quality of service class (using exclusive cores), taking a core out of the shared pool (it is the only option in Kubernetes that can do this operation). Then it examines the Pods to determine which PowerProfile has been requested and then creates or updates the appropriate PowerWorkload.

Note: the request and the limits must have a matching number of cores and are also in a container-by-container bases. If two profiles are created together but have different priority levels, the pods will get created but the cores won’t get tuned correctly.

### Pod
The Pod Spec will contain the finer details of the deployment. It will list the number of cores required along with the requested PowerProfile.

NOTE: The number of cores and the number of requested PowerProfiles must match.


## Repository Links
### Power Operator repository
[Power Operator](https://gitlab.devtools.intel.com/OrchSW/CNO/power-operator)
### AppQoS repository
[AppQoS](https://github.com/intel/intel-cmt-cat)

## Installation 
## Step by step build

### AppQoS
- Clone AppQoS  
````
git clone https://github.com/intel/intel-cmt-cat
````

- cd into docker dictory and build the App QoS image
````
cd intel-cmt-cat/appqos/docker

./build_docker.sh
````

### Set up App QoS certificates and App QoS conf file
The Power Node Agent requires certificates to communicate securely with the App QoS Agent. Both agents are run in the same Pod so the IP address required is ‘localhost’. The certificates must be placed into the /etc/certs/public directory as this is the directory that gets mounted into the Pod.
The App QoS Agent also requires a conf file to run, which needs to be placed in teh /etc/certs/public directory as well.
An example of setting up the certificates is as follows:
````
mkdir -p /etc/certs/public
openssl req -nodes -x509 -newkey rsa:4096 -keyout /etc/certs/public/ca.key -out /etc/certs/public/ca.crt -days 365 -subj "/O=AppQoS/OU=root/CN=localhost"
chmod 644 /etc/certs/public/ca.key

openssl req -nodes -newkey rsa:3072 -keyout /etc/certs/public/appqos.key -out /etc/certs/public/appqos.csr -subj "/O=AppQoS/OU=AppQoS Server/CN=localhost"
openssl x509 -req -in /etc/certs/public/appqos.csr -CA /etc/certs/public/ca.crt -CAkey /etc/certs/public/ca.key -CAcreateserial -out /etc/certs/public/appqos.crt
chmod 644 /etc/certs/public/appqos.crt /etc/certs/public/appqos.csr /etc/certs/public/ca.key /etc/certs/public/appqos.key

cp ~/intel-cmt-cat/appqos/appqos.conf /etc/certs/public/
````

### Setting up the Power Operator 
- Clone the Power Operator  
````
git clone https://gitlab.devtools.intel.com/OrchSW/CNO/power-operator
cd power-operator
````

- Set up the necessary Namespace, Service Account, and RBAC rules for the operator:
````
kubectl apply -f config/rbac/namespace.yaml
kubectl apply -f config/rbac/service_account.yaml
kubectl apply -f config/rbac/rbac.yaml
````

- Generate the CRD templates, create the Custom Resource Definitions, and build the Power Operator and Power Node Agent Docker images:
````
make images
````
NOTE: The images will be labelled ‘intel-power-operator:latest’ and ‘intel-power-node-agent:latest’

### Running the Power Operator 
- **Applying the manager**

The manager Deployment in config/manager/manager.yaml contains the following:
````yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: controller-manager
  namespace: default
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
      serviceAccountName: default
      containers:
      - command:
        - /manager
        args:
        - --enable-leader-election
        imagePullPolicy: Never
        image: operator:latest
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
apiVersion: "power.intel.com/v1alpha1"
kind: PowerConfig
metadata:
  name: power-config
spec:
  powerImage: "appqos:latest"
  powerNodeSelector:
    feature.node.kubernetes.io/appqos-node: "true"
  powerProfiles:
  - "performance"
 ````
Apply the Config:
`kubectl apply -f examples/example-powerconfig.yaml`
   
Once deployed the controller-manager pod will see it via the Config controller and create an App QoS instance and a Node Agent instance on nodes specified with the ‘feature.node.kubernetes.io/appqos-node: "true’ label.  

The power-node-agent DaemonSet will be created, managing the Power Node Agent Pods

- **Shared Profile**

The example Shared PowerProfile in examples/example-shared-profile.yaml contains the following PowerProfile spec:
````yaml
apiVersion: "power.intel.com/v1alpha1"
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
apiVersion: "power.intel.com/v1alpha1"
kind: PowerWorkload
metadata:
  # Replace <NODE_NAME> with the Node you intend this PowerWorkload to be associated with
  name: shared-<NODE_NAME>-workload
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
 
Once created the workload controller will see its creation and create the corresponding Pool in the App QoS agent and all of the cores on the system except for the reservedCPUs will be brought down to this lower frequency level.
 
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
    command: ["/bin/sh"]
    args: ["-c", "sleep 15000"]
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

`kubetl apply -f examples/example-pod.yaml`

At this point, if only the ‘performance’ PowerProfile was selected in the PowerConfig, the user’s cluster will contain three PowerProfiles and two PowerWorkloads:

`kubectl get powerprofiles`
````
NAME                          AGE
performance                   59m
performance-<NODE_NAME>       58m
shared-<NODE_NAME>            60m
````

`kubectl get powerworkloads`
````
NAME                                   AGE
performance-<NODE_NAME>-workload       63m
shared-<NODE_NAME>-workload            61m
````

- **Delete Pods**

`kubectl get pods`

`kubectl delete pods <name>`

The powerworkload workload will be deleted.

The AppQoS instance pool for the associated workload will also get deleted.
The cores that were returned from that pool will be re-tuned to the shared frequencies.