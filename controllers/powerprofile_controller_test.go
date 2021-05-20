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
	//"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func createProfileReconcilerObject(powerProfile *powerv1alpha1.PowerProfile) (*PowerProfileReconciler, error) {
	// Register operator types with the runtime scheme.
	s := scheme.Scheme

	// Add route Openshift scheme
	if err := powerv1alpha1.AddToScheme(s); err != nil {
		return nil, err
	}

	// Objects to track in the fake client.
	objs := []runtime.Object{powerProfile}

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
	r := &PowerProfileReconciler{cl, ctrl.Log.WithName("testing"), s, ppC1, State}

	return r, nil

}

// ***********************************************************************************************
// START TESTING //
//************************************************************************************************

func TestPowerProfileState(t *testing.T) {
	tcases := []struct {
		name                 string
		profileName          string
		powerProfile         *powerv1alpha1.PowerProfile
		expectedPowerProfile *powerv1alpha1.PowerProfile
	}{
		{
			name: "test case 1 - profile name",
			powerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared",
					Namespace: "default",
				},
				Spec: powerv1alpha1.PowerProfileSpec{
					Name: "Shared",
				},
			},
			expectedPowerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared",
					Namespace: "default",
				},
				Spec: powerv1alpha1.PowerProfileSpec{
					Name: "Shared",
				},
			},
		},
		{
			name: "test case 2 - profile second name",
			powerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared",
					Namespace: "default",
				},
				Spec: powerv1alpha1.PowerProfileSpec{
					Name: "Shared-workload",
				},
			},
			expectedPowerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared",
					Namespace: "default",
				},
				Spec: powerv1alpha1.PowerProfileSpec{
					Name: "Shared-workload",
				},
			},
		},
	}

	for _, tc := range tcases {
		// Create a ReconcileNode object with the scheme and fake client.
		r, err := createProfileReconcilerObject(tc.powerProfile)
		if err != nil {
			t.Fatalf("error creating ReconcilePowerProfileState object: (%v)", err)
		}

		powerProfileName := tc.powerProfile.GetObjectMeta().GetName()
		powerProfileNamespace := tc.powerProfile.GetObjectMeta().GetNamespace()
		powerNamespacedName := types.NamespacedName{
			Name:      powerProfileName,
			Namespace: powerProfileNamespace,
		}
		powerProfile := &powerv1alpha1.PowerProfile{}
		err = r.Client.Get(context.TODO(), powerNamespacedName, powerProfile)
		if err != nil {
			t.Fatalf("Failed to get power profile!")
		}

		expectedPowerProfileState := tc.expectedPowerProfile.Spec.Name
		actualPowerProfileState := powerProfile.Spec.Name

		if !reflect.DeepEqual(actualPowerProfileState, expectedPowerProfileState) {
			t.Errorf("Failed: %v - Expected %v, got %v", tc.name, expectedPowerProfileState, actualPowerProfileState)
		}
	}
}

///////////////////////////////////////////////////////////////////////
///////  SECOND TEST BLOCK   ////////////////
///////////////////////////////////////////////////////////////////////

// getPodAddress test function
func TestGetPodAddress(t *testing.T) {
	tcases := []struct {
		name            string
		nodeName        string
		nodeList        *corev1.NodeList
		powerProfile    *powerv1alpha1.PowerProfile
		profilePodList  *corev1.PodList
		expectedAddress string
		expectedError   bool
	}{
		{
			name:     "test case 1 - 127.0.0.1 address test",
			nodeName: "Worker",
			powerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared",
					Namespace: "default",
				},
			},
			nodeList: &corev1.NodeList{},
			profilePodList: &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "shared",
							Namespace: "default",
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Ports: []corev1.ContainerPort{
										{
											ContainerPort: 8081,
										},
									},
								},
							},
							NodeName: "worker",
						},
						Status: corev1.PodStatus{
							PodIP: "127.0.0.1",
						},
					},
				},
			},
			expectedAddress: "http://127.0.0.1:8081",
			expectedError:   false,
		},
		{
			name:     "test case 2 - Local host test",
			nodeName: "Worker",
			powerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared",
					Namespace: "default",
				},
			},
			nodeList: &corev1.NodeList{},
			profilePodList: &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "shared",
							Namespace: "default",
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Ports: []corev1.ContainerPort{
										{
											ContainerPort: 5000,
										},
									},
								},
							},
							NodeName: "worker",
						},
						Status: corev1.PodStatus{
							PodIP: "127.0.0.1",
						},
					},
				},
			},
			expectedAddress: "https://localhost:5000",
			expectedError:   false,
		},
		{
			name:     "test case 3 - board address",
			nodeName: "Worker",
			powerProfile: &powerv1alpha1.PowerProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared",
					Namespace: "default",
				},
			},
			nodeList: &corev1.NodeList{},
			profilePodList: &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "shared",
							Namespace: "default",
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Ports: []corev1.ContainerPort{
										{
											ContainerPort: 8081,
										},
									},
								},
							},
							NodeName: "worker",
						},
						Status: corev1.PodStatus{
							PodIP: "127.0.0.1",
						},
					},
				},
			},
			expectedAddress: "http://10.237.213.205:5000",
			expectedError:   false,
		},
	}

	for _, tc := range tcases {
		// Create a ReconcileNode object with the scheme and fake client.
		r, err := createProfileReconcilerObject(tc.powerProfile)
		if err != nil {
			t.Fatalf("error creating ReconcilePowerNodeState object: (%v)", err)
		}
		for _, profilePod := range tc.profilePodList.Items {
			err = r.Client.Create(context.TODO(), &profilePod)
			if err != nil {
				t.Fatalf("Failed to create dummy power pod")
			}
		}
		for _, node := range tc.nodeList.Items {
			err = r.Client.Create(context.TODO(), &node)
			if err != nil {
				t.Fatalf("Failed to create dummy node")
			}
		}

		//powerProfileName := tc.powerProfile.GetObjectMeta().GetName()
		//powerProfileNamespace := tc.powerProfile.GetObjectMeta().GetNamespace()
		/*req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      powerProfileName,
				Namespace: powerProfileNamespace,
			},
		}*/

		errorReturned := false
		address, err := r.getPodAddress(tc.nodeName)
		if err != nil {
			errorReturned = true
		}

		if address != tc.expectedAddress || errorReturned != tc.expectedError {
			t.Errorf("Failed: %v - Expected %v and %v, got %v and %v", tc.name, tc.expectedAddress, tc.expectedError, address, errorReturned)
		}
	}
}
