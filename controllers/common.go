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

	"github.com/go-logr/logr"
	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	"github.com/intel/kubernetes-power-manager/pkg/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// write errors to the status filed, pass nil to clear errors, will only do update resource is valid and not being deleted
// if object already has the correct errors it will not be updated in the API
func writeUpdatedStatusErrsIfRequired(ctx context.Context, statusWriter client.SubResourceWriter, object powerv1.PowerCRWithStatusErrors, objectErrors error) error {
	var err error
	// if invalid or marked for deletion don't do anything
	if object.GetUID() == "" || object.GetDeletionTimestamp() != nil {
		return err
	}
	errList := util.UnpackErrsToStrings(objectErrors)
	// no updates are needed
	if reflect.DeepEqual(*errList, *object.GetStatusErrors()) {
		return err
	}
	object.SetStatusErrors(errList)
	err = statusWriter.Update(ctx, object)
	if err != nil {
		logr.FromContextOrDiscard(ctx).Error(err, "failed to write status update")
	}
	return err
}
