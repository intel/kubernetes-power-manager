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
	"fmt"
	"testing"

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeClient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func Test_writeStatusErrors(t *testing.T) {
	var object powerv1.PowerCRWithStatusErrors
	var errorList error
	var ctx = context.Background()
	clientMockObj := mock.Mock{}
	clientFuncs := interceptor.Funcs{
		SubResourceUpdate: func(ctx context.Context, client client.Client, subResourceName string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
			return clientMockObj.MethodCalled("SubResourceUpdate", ctx, client, subResourceName, obj, opts).Error(0)
		},
	}
	clientStatusWriter := fakeClient.NewClientBuilder().WithInterceptorFuncs(clientFuncs).Build().Status()

	object = &powerv1.Uncore{}
	assert.Nil(t, writeUpdatedStatusErrsIfRequired(ctx, nil, object, nil), "invalid object should return nil without doing anything")

	deletionTimestamp := v1.Now()
	object = &powerv1.CStates{
		ObjectMeta: v1.ObjectMeta{
			DeletionTimestamp: &deletionTimestamp,
		},
	}
	assert.Nil(t, writeUpdatedStatusErrsIfRequired(ctx, nil, object, nil), "object marked for deletion should return nil without doing anything")

	object = &powerv1.PowerProfile{
		ObjectMeta: v1.ObjectMeta{
			UID: "not empty",
		},
		Status: powerv1.PowerProfileStatus{
			StatusErrors: powerv1.StatusErrors{
				Errors: []string{"err1"},
			},
		},
	}
	errorList = fmt.Errorf("err1")
	assert.Nil(t, writeUpdatedStatusErrsIfRequired(ctx, nil, object, errorList), "no updates should be needed")

	object = &powerv1.PowerWorkload{
		ObjectMeta: v1.ObjectMeta{
			UID: "not empty",
		},
	}
	clientMockObj.On("SubResourceUpdate", ctx, mock.Anything, "status", &powerv1.PowerWorkload{
		ObjectMeta: v1.ObjectMeta{
			UID: "not empty",
		},
		Status: powerv1.PowerWorkloadStatus{
			StatusErrors: powerv1.StatusErrors{
				Errors: []string{"err1"},
			},
		},
	}, mock.Anything).Return(nil)
	errorList = fmt.Errorf("err1")
	assert.Nil(t, writeUpdatedStatusErrsIfRequired(ctx, clientStatusWriter, object, errorList), "API should get updated with object with errors")

	object = &powerv1.TimeOfDay{
		ObjectMeta: v1.ObjectMeta{
			UID: "not empty",
		},
	}

	clientMockObj = mock.Mock{}
	updateErr := fmt.Errorf("update error")
	clientMockObj.On("SubResourceUpdate", ctx, mock.Anything, "status", mock.Anything, mock.Anything).Return(updateErr)
	assert.ErrorIs(t, writeUpdatedStatusErrsIfRequired(ctx, clientStatusWriter, object, fmt.Errorf("err2")), updateErr, "error updating APi should return that error")

}
