# Deploy Intel Kubernetes Power Manager on OCP cluster

## Technical Requirements and Dependencies

The Intel Kubernetes Power Manager on OCP has the following requirements:

- OpenShift 4.13
- Node Feature Discovery Operator with basic NFD CR applied
- If building from source, an external Docker registry and cluster configured to access it is also needed.


##### ***[NOTICE]Intel Kubernetes PowerManager (KPM) and RedHat Node Tuning Operator (NTO)***

When both are deployed on an OCP cluster, undefined behaviors will occur.  We recommend turning off NTO's tuning feature and using KPM.

If you wish to use NTO with KPM in order to get the added functionality of uncore, then you can use KPM for uncore only and use NTO for NTO-supported features.  For simplicity, it is better to choose KPM if you need the added capabilities of KPM, rather than mixing the capabilities of the two, at this time.

## Deploying the Operator

### From Red Hat Certified-Operators catalog

The Intel Kubernetes Power Manager can be deployed by installing directly from the Red Hat Certified-Operators catalog via the the OpenShift Container Platform web console or the `Subscription` resource kind.

To start the install process, please follow the steps below:

1. Create a namespace for the operator:

```shell
# oc create ns intel-power
```

2. Create the following `OperatorGroup` `yaml` file:

```yaml
---
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: intel-kubernetes-power-manager
  namespace: intel-power
spec:
  targetNamespaces:
    - intel-power
```

3. Create the `OperatorGroup`

```shell
# oc apply -f <filename>
```

4. Install the operator from the Certified-Operators catalog:

4.1 Using `OpenShift Container Platform web console`:

1. In the OpenShift Container Platform web console, click Operators → OperatorHub.
2. Select Intel Kubernetes Power Manager from the list of available Operators, and then click Install.
3. On the Install Operator page, under a specific namespace on the cluster, select intel-kubernetes-power-manager.
4. Click Install.

4.2 By creating a `Subscription` `yaml` file:

```yaml
---
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: intel-power-subscription
  namespace: intel-power
spec:
  channel: alpha
  name: intel-kubernetes-power-manager
  source: certified-operators
  sourceNamespace: openshift-marketplace
```

Create the `Subscription`

```shell
# oc apply -f <filename>
```

5. Check that the operator is deployed:

```oc get pods -n intel-power
NAME                                 READY   STATUS    RESTARTS   AGE
controller-manager-6999c5c49-nxnmt   1/1     Running   0          9m34s
```

