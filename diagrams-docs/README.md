# Power Manger Architectural and Sequence Diagrams

An up-to-date architectural and sequence diagram will be available here.  
With each update or release an updated diagram will be available with the version number attached.

## Install PlnatUML

- install plantuml.jar: http://sourceforge.net/projects/plantuml/files/plantuml.jar/download
- To run PlantUML you need:
    - Java: https://www.java.com/en/download/
    - Graphviz (optional): https://plantuml.com/graphviz-dot

- Example
    - Create file: sequenceDiagram.txt
    - example code:
      @startuml
      Alice -> Bob: test
      @enduml
    - Run: java -jar plantuml.jar sequenceDiagram.txt
    - Output file sequenceDiagram.png will be created with the diagram

## Creating Power Manager Diagrams

- Running pm-sequenc.puml in this directory:
    - java -jar plantuml.jar pm-sequence.puml
    - output file is pm-sequence.png
- Running the Power Manager overall architecture diagram
    - java -jar plantuml.jar power-arch.puml
    - output file is power-arch.png
- Running the Power Manager components diagram
    - java -jar plantuml.jar power-com.puml
    - output file is power-com.png

## Summary of elements in the K8s Cluster

## Control Plane ~ Master Node

The elements of the control plane detect and react to cluster events as well as make broad choices about the cluster.
Kubernetes is dependent on a number of control plane-based administrative services.
These services control things like task scheduling, cluster state persistence, and communication between cluster
components.

### API Server (kube-apiserver)

	• API server exposes the Kubernetes API.
	• Entry point for REST/kubectl — It is the front end for the Kubernetes control plane.
	• It tracks the state of all cluster components and managing the interaction between them.
	• It is designed to scale horizontally.
	• It consumes YAML/JSON manifest files.
	• It validates and processes the requests made via API.

### etcd (key-value store)

	• It is a consistent, distributed, and highly-available key value store.
	• It is stateful, persistent storage that stores all of Kubernetes cluster data (cluster state and config).
	• It is the source of truth for the cluster.
	• It can be part of the control plane, or, it can be configured externally.

### Scheduler (kube-scheduler)

	• It schedules pods to worker nodes.
	• It watches api-server for newly created Pods with no assigned node, and selects a healthy node for them to run on.
	• If there are no suitable nodes, the pods are put in a pending state until such a healthy node appears.
	• It watches API Server for new work tasks.

#### Factors taken into account for scheduling decisions include:

	• Individual and collective resource requirements.
	• Hardware/software/policy constraints.
	• Affinity and anti-affinity specifications.
	• Data locality.
	• Inter-workload interference.
	• Deadlines and taints.

### Controller Manager (kube-controller-manager)

	• It watches the desired state of the objects it manages and watches their current state through the API server.
	• It takes corrective steps to make sure that the current state is the same as the desired state.
	• It is controller of controllers. It runs controller processes. Logically, each controller is a separate process, but to reduce complexity, they are all compiled into a single binary and run in a single process.

## Worker Node

Node components run on every node, maintaining running pods and providing the Kubernetes runtime environment.

### kubelet

	• It is an agent that runs on each node in the cluster.
	• It acts as a conduit between the API server and the node.
	• It makes sure that containers are running in a Pod and they are healthy.
	• It instantiates and executes Pods.
	• It watches API Server for work tasks.
	• It gets instructions from master and reports back to Masters.

### kube-proxy

	• It is networking component that plays vital role in networking.
	• It manages IP translation and routing.
	• It is a network proxy that runs on each node in cluster.
	• It maintains network rules on nodes. These network rules allow network communication to Pods from inside or outside of cluster.
	• It ensure each Pod gets unique IP address.
	• It makes possible that all containers in a pod share a single IP.
	• It facilitating Kubernetes networking services and load-balancing across all pods in a service.
	• It deals with individual host sub-netting and ensure that the services are available to external parties.

### Container runtime

	• The container runtime is the software that is responsible for running containers (in Pods).
	• To run the containers, each worker node has a container runtime engine.
	• It pulls images from a container image registry and starts and stops containers.
