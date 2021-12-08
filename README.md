# Intel Power Manager for Kubernetes Software

 
## What is the Power Manager for Kubernetes?

In a container orchestration engine such as Kubernetes, the allocation of CPU resources from a pool of platforms is based solely on availability with no consideration of individual capabilities such as Intel Speed Select Technology (SST).

The Intel Power Manager for Kubernetes is a Kubernetes Operator designed to expose and utilize Intel specific power management technologies in a Kubernetes Environment.

The SST is a powerful collection of features that offers more granular control over CPU performance and power consumption on a per-core basis. However, as a workload orchestrator, Kubernetes is intentionally designed to provide a layer of abstraction between the workload and such hardware capabilities. This presents a challenge to Kubernetes users running performance critical workloads with specific requirements dependent on hardware capabilities.

The Power Manager for Kubernetes bridges the gap between the container orchestration layer and hardware features enablement, specifically Intel SST.

### Power Manager for Kubernetes' main responsibilities:

- Deploying the Power Node Agent DaemonSet, targeting the nodes desired by the user, which contains the App QoS Agent and Node Agent in a single Pod
- Managing all associated custom resources
- Discovery and advertisement of the Power Profile extended resources.
- Sending HTTO requests to the App QoS daemons for SST configuration.
    

### Use Cases:

- *High performance workload known at peak times.*
   May want to pre-schedule nodes to move to a performance profile during peak times to minimize spin up.
   At times not during peak, may want to move to a power saving profile.
- *Unpredictable machine use.*
   May use machine learning through monitoring to determine profiles that predict a peak need for a compute, to spin up ahead of time.
- *Power Optimization over Performance.*
   A cloud may be interested in fast response time, but not in maximal response time, so may choose to spin up cores on demand and only those cores   used but want to remain in power-saving mode the rest of the time.  

## Initial release functionality of the Power Manager for Kubernetes

- **SST-BF - (Speed Select Technology - Base Frequency)**

  This feature allows the user to control the base frequency of certain cores.
  The base frequency is a guaranteed level of performance on the CPU (a CPU will never go below its base frequency).
  Priority cores can be set to a higher base frequency than a majority of the other cores on the system to which they can apply their critical workloads for a guaranteed performance.

- **SST-CP - (Speed Select Technology - Core Power)**

  
    This feature allows the user to group cores into levels of priority.
    When there is power to spare on the system, it can be distributed among the cores based on their priority level.
    While it is not guaranteed that the extra power will be applied to the highest priority cores, the system will do its best to do so.
    There are four levels of priority available:
    1. Performance
    2. Balance Performance
    3. Balance Power
    4. Power

    The Priority level for a core is defined using its EPP (Energy Performance Preference) value, which is one of the options in the Power Profiles. If not all the power is utilized on the CPU, the CPU can put the higher priority cores up to Turbo Frequency (allows the cores to run faster).

- **Frequency Tuning**

    
    Frequency tuning allows the individual cores on the system to be sped up or slowed down by changing their frequency.
    This tuning is done via the CommsPowerManagement python library which is utilized via App QoS.
    The min and max values for a core are defined in the Power Profile and the tuning is done after the core has been assigned by the Native CPU Manager.
    How exactly the frequency of the cores is changed is by simply writing the new frequency value to the /sys/devices/system/cpu/cpuN/cpufreq/scaling_max|min_freq file for the given core.

Note: In the future, we want to move away from the CommsPowerManagement library to create a Go Library that utilizes the Intel Pstate driver.

## Future planned additions to the Power Manager for Kubernetes

- **SST-TF - Turbo Frequency**

    This feature allows the user to set different “All-Core Turbo Frequency” values to individual cores based on their priority.
    All-Core Turbo is the Turbo Frequency at which all cores can run on the system at the same time.
    The user can set certain cores to have a higher All-Core Turbo Frequency by lowering this value for other cores or setting them to no value at all.

    This feature is only useful when all cores on the system are being utilized, but the user still wants to be able to configure certain cores to get a higher performance than others.

## Prerequisites
* Node Feature Discovery ([NFD](https://github.com/kubernetes-sigs/node-feature-discovery)) should be deployed in the cluster before running the operator. NFD is used to detect node-level features such as *Intel Speed Select Technology - Base Frequency (SST-BF)*. Once detected, the user can instruct the operator to deploy the Power Node Agent to Nodes with SST-specific labels, allowing the Power Node Agent to take advantage of such features by configuring cores on the host to optimise performance for containerized workloads.
Note: NFD is recommended, but not essential. Node labels can also be applied manually. See the [NFD repo](https://github.com/kubernetes-sigs/node-feature-discovery#feature-labels) for a full list of features labels.
* A working App QoS container image from the [App QoS repo](https://github.com/intel/intel-cmt-cat/appqos).

## Components
### App QoS
[App Qos](https://github.com/intel/intel-cmt-cat/appqos), an alias for “Application Quality of Service”, is an Intel software suite that provides node level configuration of hardware capabilities such as SST. App QoS is deployed on the host as a containerized application and exposes a REST interface to the orchestration layer. Using the Intel Comms Power libraries as a back end, App QoS can perform host-level configurations of SST features based on API calls from the Power Manager for Kubernetes.

### Power Node Agent
The Power Node Agent is also a containerized application deployed by the operator in a DaemonSet. The primary function of the node agent is to communicate with the node's Kubelet PodResources endpoint to discover the exact cores that are allocated per container. The node agent watches for Pods that are created in your cluster and examines them to determine which Power Profile they have requested and then sets off the chain of events that tunes the frequencies of the cores designated to the Pod.

### Power Config
The operator will wait for the PowerConfig to be created by the user, in which the desired PowerProfiles will be specified. The PowerConfig holds different values:
* appQoSImage: This is the name/tag given to the App QoS container image that will be deployed in a DaemonSet by the operator.
* powerNodeSelector: This is a key/value map used for defining a list of node labels that a node must satisfy in order for App QoS and the Power Node Agent to be deployed.
* powerProfiles: The list of PowerProfiles that the user wants available on the nodes.

Once the Power Config Controller sees that the PowerConfig is created, it reads the values and then deploys the Power Node Agent and the App QoS Agent on to each of the Nodes that are specified. It then creates the PowerProfiles and Extended Resources. Extended Resources are resources created in the cluster that can be requested in the PodSpec. The Kubelet can then keep track of these requests. It is important to use as it can specify how many cores on the system can be run at a higher frequency before hitting the heat threshold.

Note: Only one PowerConfig can be present in a cluster. The Power Config Controller will ignore and delete any subsequent PowerConfigs created after the first.
An example PowerConfig can be found in the examples folder, [here](https://github.com/intel/kubernetes-power-manager/blob/master/examples/example-powerconfig.yaml).


### Power Workload
The PowerWorkload is responsible for the actual tuning of the cores. The Power Workload Controller contacts the App QoS Agent and requests that it creates Pools in App QoS. The Pools hold the App QoS Power Profile (separate to the Power Manager's PowerProfile) associated with the cores and the actual cores themselves that need to be configured.
    
The PowerWorkload objects are created automatically by the Pod Controller. This action is undertaken by the Power Manager when a Pod is created with a container requesting exclusive cores and a PowerProfile.
    
PowerWorkload objects can also be created directly by the user via the PowerWorkload spec. This is only recommended when creating the Shared PowerWorkload for a given Node, as this is the responsibility of the user. If no Shared PowerWorkload is created, the cores that remain in the ‘Shared Pool’ on the Node will remain at their core frequency values instead of being tuned to lower frequencies. PowerWorkloads are specific to a given node, so one is created for each Node with a Pod requesting a PowerProfile, based on the PowerProfile requested. For more information on Shared PowerWorkloads and the Shared Pool, see the [Shared PowerWorkloads](#shared-powerworkloads) section.

A PowerWorkload associated with a PowerProfile will have the following values:
- Its Node Info: This holds all the necessary information about the PowerWorkload, such as the Containers using this PowerWorkload, the Pods using this PowerWorkload, and the cores that have been tuned by this PowerWorkload
- The PowerProfile associated with this PowerWorkload


### Power Profile
The Power Profile Controller holds values for specific SST settings which are then applied to cores at host level by the Power Manager as requested. Power Profiles are advertised as extended resources and can be requested via the PodSpec. The Power Config Controller creates the requested high-performance PowerProfiles depending on which are requested in the PowerConfig created by the user.

There are two kinds of PowerProfiles:

- Base PowerProfiles
- Extended PowerProfiles

A Base PowerProfile can be one of three values:
- performance
- balance-performance
- balance-power

These correspond to three of the EPP values associated with SST-CP. Base PowerProfiles are used to tell the Power Profile Controller that the specified profile is being requested for the cluster. The Power Profile Controller takes the created Profile and further creates an Extended PowerProfile. An Extended PowerProfile is Node-specific. The reason behind this is that different Nodes in your cluster may have different maximum frequency limitations. For example, one Node may have the maximum limitation of 3700GHz, while another may only be able to reach frequency levels of 3200GHz. An Extended PowerProfile queries the Node that it is running on to obtain this maximum limitation and sets the Max and Min values of the profile accordingly. An Extended PowerProfile’s name has the following form:

BASE_PROFILE_NAME-NODE_NAME - for example: “performance-example-node”.

Either the Base PowerProfile or the Extended PowerProfile can be requested in the PodSpec, as the Power Workload Controller can determine the correct PowerProfile to use from the Base PowerProfile.

A PowerProfile has the following values when created:
- Name: The name used to identify and specify the PowerProfile, used in the PodSpec
- Max: The maximum frequency at which a core can run
- Min: The minimum frequency at which a core can run
- Epp: The EPP value for the core, specifying its priority in terms of the distribution of additional power on the Node
- 

The Shared PowerProfile must be created by the user and does not require a Base PowerProfile. This allows the user to have a Shared PowerProfile per Node in their cluster, giving more room for different configurations. The Power Profile Controller determines that a PowerProfile is being designated as ‘Shared’ through the use of the ‘power’ EPP value. See the [Shared PowerWorkloads](#shared-powerworkloads) section for more information on Shared PowerProfile functionality.


### Power Node
The Power Node Controller is a way to have a view of what is going on in the cluster. 
It details what workloads are being used currently, which profiles are being used, what cores are being used and what containers they are associated with. It also gives insight to the user as to which Shared Pool in the App QoS agent is being used. The two Shared Pools can be the Default Pool or the Shared Pool. If there is no Shared PowerProfile associated with the Node, then the Default Pool will hold all the cores in the ‘shared pool’, none of which will have their frequencies tuned to a lower value. If a Shared PowerProfile is associated with the Node, the cores in the ‘shared pool’ – excluding cores reserved for Kubernetes processes (reservedCPUs) - will be placed in the Shared Pool in App QoS and have their cores tuned.

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

### Shared PowerWorkloads
In a Kubernetes cluster while using the Static CPU Manager Policy, a growing and shrinking 'Shared Pool' is maintained to keep track of cores on the Node that are used exclusively for certain Pods and cores that are available for use by all other Pods. Cores that are available to all non-exclusive Pods are considered to be in this 'Shared Pool'. The purpose of the Kubernetes Power Manager is to take the cores in this pool and set their frequencies to a lower threshold to lower the power output of that Node. This functionality will only happen when the user creates a Shared PowerWorkload, which is a special type of PowerWorkload. Without a Shared PowerWorkload the cores in this Shared Pool will not have their frequencies changed. It is the responsibility of the user to create a Shared PowerWorkload for each Node in their cluster. To create a Shared PowerWorkload, specific flags need to be set in the PowerWorkload's spec:
- allCores must be set to True
- reservedCPUs must be set to a list of the cores that are reserved for Kubernetes system processes, which will be the same as the reservedCPUs flag in the Kubelet config file

A shared PowerWorkload must follow the naming convention of beginning with ‘shared-’. Any shared PowerWorkload that does not begin with ‘shared-’ is rejected and deleted by the Power Workload Controller. The shared PowerWorkload powerNodeSelector must also select a unique node, so it is recommended that the ‘kubernetes.io/hostname’ label be used. A shared PowerProfile can be used for multiple shared PowerWorkloads.

After a Shared PowerWorkload is created for a Node, the cores in its Shared Pool - excluding those specified in the reservedCPUs flag - will have their frequencies set to that of the Shared PowerProfile requested. A single Shared PowerProfile can be used for multiple Shared PowerWorkloads, but a single Shared PowerWorkload cannot be used for multiple Nodes.

A Shared PowerProfile is created by the user and is specified by setting the epp option to "power". This signals to the Power Profile Controller that the created PowerProfile is to be used as a Shared one and that no subsequent Extended PowerProfiles need to be created.
An example of a Shared PowerProfile can be found [here](https://github.com/intel/kubernetes-power-manager/blob/master/examples/example-shared-profile.yaml).
An example of a Shared PowerWorkload can be found [here](https://github.com/intel/kubernetes-power-manager/blob/master/examples/example-shared-workload.yaml).

The App QoS Agent can store up to two Shared Pools at a time, with a minimum of one. Note that these are App QoS pools and are separate to the 'Shared Pool' in the Kubernetes cluster mentioned above. Upon startup, App QoS takes all of the cores on the Node it has been placed and places them in a Pool it maintains called the Default Pool. If no Shared PowerWorkload is present on that given Node, cores are taken out of and returned to this Default Pool when exclusive Pods are created. When a Shared PowerWorkload is created, all cores except for those specified in the reservedCPUs option are removed from the Default Pool and placed in a newly created App QoS Pool called the Shared Pool. Upon creation of this Shared Pool in App QoS, these cores have their frequencies tuned. The Kubernetes Power Manager will always remove cores from the Shared Pool in App QoS if it is available, only going to the Default Pool when it is absent.


### In the Kubernetes API
- PowerConfig CRD

- PowerWorkload CRD

- PowerProfile CRD

- PowerNode CRD


### App QoS Agent Pod
There is an App QoS and a Node Agent on each node in the cluster that you want power optimization to occur. This is necessary because of node specific tuning. The App QoS agent keeps track of pools. The PowerWorkload creates a pool in App QoS, which consists of the cores and the desired profile. This is where the call to the CommsPowerManagement library is made.

The App QoS agent will automatically create a Pool labeled "Default" which will contain all cores on the Node. The cores in this pool will not have their frequencies changed. When a Shared PowerWorkload is created, a new pool in the App QoS agent is created labeled "Shared", and removes all cores from the "Default" pool, excluding those in the reservedCPUs list, and reduces their frequencies to the desired state. When a non-shared PowerWorkload is created, the cores are removed from the "Shared" pool if it exists, or the "Default" pool if it does not.

### Node Agent Pod
The Pod Controller watches for pods. When a pod comes along the Pod Controller checks if the pod is in the guaranteed quality of service class (using exclusive cores, [see documentation](https://kubernetes.io/docs/tasks/configure-pod-container/quality-service-pod/), taking a core out of the shared pool (it is the only option in Kubernetes that can do this operation). Then it examines the Pods to determine which PowerProfile has been requested and then creates or updates the appropriate PowerWorkload.

Note: the request and the limits must have a matching number of cores and are also in a container-by-container bases. Currently the Power Manager for Kubernetes only supports a single PowerProfile per Pod. If two profiles are requested in different containers, the pod will get created but the cores will not get tuned.

## Repository Links
### App QoS repository
[App QoS](https://github.com/intel/intel-cmt-cat)
[CommsPowerManagement](https://github.com/intel/CommsPowerManagement)

## Installation 
NOTE: For App QoS to work as intended, your Node must use a CPU that is capable of SST-CP configuration and it must be configured. The SST-CP enablement on a Node allows the App QoS agent to use power profiles. For more information on SST and SST-CP, see these documents:
- https://lore.kernel.org/lkml/20191119232428.62932-1-srinivas.pandruvada@linux.intel.com/
- https://networkbuilders.intel.com/solutionslibrary/intel-speed-select-technology-core-power-intel-sst-cp-overview-technology-guide

## Step by step build

### App QoS
- Clone App QoS  
````
git clone https://github.com/intel/intel-cmt-cat
````

- cd into the docker directory and build the App QoS image
````
cd intel-cmt-cat/appqos/docker

./build_docker.sh
````

### Set up App QoS certificates and App QoS conf file
The Power Node Agent requires certificates to communicate securely with the App QoS Agent. Both agents are run in the same Pod so the IP address required is ‘localhost’. The certificates must be placed into the /etc/certs/public directory as this is the directory that gets mounted into the Pod.
The App QoS Agent also requires a conf file to run, which needs to be placed in the /etc/certs/public directory as well.
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

The following configuration is recommended for the appqos.conf file, which will need to replace the original conf file. More information can be found in the repository https://github.com/intel/intel-cmt-cat/tree/master/appqos
````
{
  "apps": [], 
  "sstbf": {
    "configured": false
  }, 
  "pools": [], 
  "power_profiles_expert_mode": true
}
````

### Setting up the Power Manager for Kubernetes 
- Clone the Power Manager for Kubernetes  
````
git clone https://github.com/intel/kubernetes-power-manager
cd kubernetes-power-manager
````

- Set up the necessary Namespace, Service Account, and RBAC rules for the operator:
````
kubectl apply -f config/rbac/namespace.yaml
kubectl apply -f config/rbac/service_account.yaml
kubectl apply -f config/rbac/rbac.yaml
````

- Generate the CRD templates, create the Custom Resource Definitions, and build the operator and Power Node Agent Docker images:
````
make images
````
NOTE: The images will be labelled ‘intel-power-operator:latest’ and ‘intel-power-node-agent:latest’

### Running the Power Manager for Kubernetes 
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
   
Once deployed the controller-manager pod will see it via the Power Config Controller and create an App QoS instance and a Node Agent instance on nodes specified with the ‘feature.node.kubernetes.io/appqos-node: "true"’ label.

The power-node-agent DaemonSet will be created, managing the Power Node Agent Pods. The controller-manager will finally create the PowerProfiles that were requested on each Node.

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
 
Once created the Power Workload Controller will see its creation and create the corresponding Pool in the App QoS agent and all of the cores on the system except for the reservedCPUs will be brought down to this lower frequency level.
 
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

`kubectl delete pods <name>`

When a Pod that was associated with a PowerWorkload is deleted, the cores associated with that Pod will be removed from the corresponding PowerWorkload. If that Pod was the last requesting the use of that PowerWorkload, the workload will be deleted. All cores removed from the PowerWorkload are added back to the Shared PowerWorkload for that Node and retuned to the lower frequencies.

### DISCLAIMER:
The App QoS Agent Pod requires elevated privileges to run, and the Container is run with Root privileges.
