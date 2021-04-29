/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/* TODO:
- reconcile function


*/
package controllers

import (
	"context"
	"reflect"
	"testing"

	powerv1alpha1 "gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/api/v1alpha1"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/appqos"
	//"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/newstate"
	"gitlab.devtools.intel.com/OrchSW/CNO/power-operator.git/pkg/state"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func createWorkloadReconcilerObject(powerWorkload *powerv1alpha1.PowerWorkload) (*PowerWorkloadReconciler, error) {
	// Register operator types with the runtime scheme.
	s := scheme.Scheme

	// Add route Openshift scheme
	if err := powerv1alpha1.AddToScheme(s); err != nil {
		return nil, err
	}

	// Objects to track in the fake client.
	objs := []runtime.Object{powerWorkload}

	// Create a fake client to mock API calls.
	cl := fake.NewFakeClient(objs...)

	// Create a fake profile client
	ppC1 := appqos.NewDefaultAppQoSClient()

	// state
	State := &state.PowerNodeData{
		PowerNodeList: []string{},
	}
	/*
		State := &newstate.PowerNodeData{
			PowerNodeList: []string{},
			//ProfileAssociation: map[string][]string{},
		}
	*/

	// Create a ReconcileNode object with the scheme and fake client.
	r := &PowerWorkloadReconciler{cl, ctrl.Log.WithName("testing"), s, ppC1, State}

	return r, nil

}

// ***********************************************************************************************
// START TESTING //
//************************************************************************************************

func TestPowerWorkload(t *testing.T) {
	tcases := []struct {
		name                  string
		profileName           string
		powerWorkload         *powerv1alpha1.PowerWorkload
		expectedPowerWorkload *powerv1alpha1.PowerWorkload
	}{
		{
			name: "test case 1 - profile name",
			powerWorkload: &powerv1alpha1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared",
					Namespace: "default",
				},
				Spec: powerv1alpha1.PowerWorkloadSpec{
					PowerProfile: "performance",
					AllCores:     true,
					ReservedCPUs: []int{0, 1},
					//Nodes: []powerv1alpha1.NodeInfo{
					//	Name: "worker",
					//	CpuIds: []int{0,1},
					//},
				},
			},
			expectedPowerWorkload: &powerv1alpha1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared",
					Namespace: "default",
				},
				Spec: powerv1alpha1.PowerWorkloadSpec{
					ReservedCPUs: []int{0, 1},
				},
			},
		},
	}

	for _, tc := range tcases {
		// Create a ReconcileNode object with the scheme and fake client.
		r, err := createWorkloadReconcilerObject(tc.powerWorkload)
		if err != nil {
			t.Fatalf("error creating createWorkloadReconcilerObject object: (%v)", err)
		}

		powerWorkloadName := tc.powerWorkload.GetObjectMeta().GetName()
		powerWorkloadNamespace := tc.powerWorkload.GetObjectMeta().GetNamespace()
		powerNamespacedName := types.NamespacedName{
			Name:      powerWorkloadName,
			Namespace: powerWorkloadNamespace,
		}
		powerWorkload := &powerv1alpha1.PowerWorkload{}
		err = r.Client.Get(context.TODO(), powerNamespacedName, powerWorkload)
		if err != nil {
			t.Fatalf("Failed to get power Workload!")
		}

		expectedPowerWorkloadState := tc.expectedPowerWorkload.Spec.Nodes
		actualPowerWorkloadState := powerWorkload.Spec.Nodes

		if !reflect.DeepEqual(actualPowerWorkloadState, expectedPowerWorkloadState) {
			t.Errorf("Failed: %v - Expected %v, got %v", tc.name, expectedPowerWorkloadState, actualPowerWorkloadState)
		}
	}
}

///////////////////////////////////////////////////////////////////////
///////  SECOND TEST BLOCK   ////////////////
///////////////////////////////////////////////////////////////////////

func TestFindObseleteWorkloads(t *testing.T) {
	tcases := []struct {
		name          string
		State         []string
		request       reconcile.Request
		powerWorkload *powerv1alpha1.PowerWorkload
		powerPods     *corev1.PodList
		//getWorkloadsResponse      map[string]([]rmdtypes.RDTWorkLoad)
		expectedObseleteWorkloads map[string]string
		expectedErr               bool
	}{
		{
			name:  "test case 1 - 1 obselete workload only",
			State: []string{"example-node.com"},
			request: reconcile.Request{
				types.NamespacedName{
					Namespace: "default",
					Name:      "shared",
				},
			},
			powerWorkload: &powerv1alpha1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "Shared",
					Namespace: "default",
				},
				Spec: powerv1alpha1.PowerWorkloadSpec{
					PowerProfile: "shared",
					AllCores:     true,
					ReservedCPUs: []int{0, 1},
				},
			},
			powerPods: &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "power-example-node.com",
							Namespace: "default",
							//Labels:    map[string]string{"name": "rmd-pod"},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Ports: []corev1.ContainerPort{
										{
											ContainerPort: 8080,
										},
									},
								},
							},
							NodeName: "example-node.com",
						},
						Status: corev1.PodStatus{
							PodIPs: []corev1.PodIP{
								{
									IP: "127.0.0.1",
								},
							},
						},
					},
				},
			},
			expectedObseleteWorkloads: map[string]string{
				"http://127.0.0.1:8080": "1",
			},
			expectedErr: false,
		},
	}

	for _, tc := range tcases {
		r, err := createWorkloadReconcilerObject(tc.powerWorkload)
		if err != nil {
			t.Fatalf("error creating ReconcileRmdWorkload object: (%v)", err)
		}

		//ts := make([]*httptest.Server, 0)
		//for i := range tc.powerPods.Items {
		//get address (i.e. IP and port number)
		//podIP := tc.powerPods.Items[i].Status.PodIPs[0].IP
		//containerPort := tc.powerPods.Items[i].Spec.Containers[0].Ports[0].ContainerPort
		//address := fmt.Sprintf("%s:%v", podIP, containerPort)

		// Create listeners to manage http GET requests
		//server, err := createListeners(address, tc.getWorkloadsResponse)
		//if err != nil {
		//	t.Fatalf("error creating listeners: (%v)", err)
		//}
		//ts = append(ts, server)
		//}

		for i := range tc.powerPods.Items {
			err = r.Client.Create(context.TODO(), &tc.powerPods.Items[i])
			if err != nil {
				t.Fatalf("Failed to create dummy rmd pod")
			}
		}

		returnedErr := false
		r.State.PowerNodeList = tc.State
		nodes := []string{"a", "b"}
		obseleteWorkloads, err := r.findObseleteWorkloads(tc.request, nodes)
		if err != nil {
			returnedErr = true
		}
		if !reflect.DeepEqual(tc.expectedObseleteWorkloads, obseleteWorkloads) {
			t.Errorf("%v failed: Expected:  %v, Got:  %v\n", tc.name, tc.expectedObseleteWorkloads, obseleteWorkloads)
		}
		if tc.expectedErr != returnedErr {
			t.Errorf("%v failed: Expected error: %v, Error gotten: %v\n", tc.name, tc.expectedErr, returnedErr)
		} /*
			//for i := range tc.rmdPods.Items {
			//Close the listeners
			//ts[i].Close()
			//}*/
	}
}
