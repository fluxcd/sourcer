/*
Copyright 2020 The Flux CD contributors.

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
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	sourcev1 "github.com/fluxcd/source-controller/api/v1alpha1"
)

type SourceChangePredicate struct {
	predicate.Funcs
}

// Update implements the default UpdateEvent filter for validating
// source changes.
func (SourceChangePredicate) Update(e event.UpdateEvent) bool {
	if e.MetaOld == nil || e.MetaNew == nil {
		// ignore objects without metadata
		return false
	}
	if e.MetaNew.GetGeneration() != e.MetaOld.GetGeneration() {
		// reconcile on spec changes
		return true
	}

	// handle force sync
	if val, ok := e.MetaNew.GetAnnotations()[sourcev1.SyncAtAnnotation]; ok {
		if valOld, okOld := e.MetaOld.GetAnnotations()[sourcev1.SyncAtAnnotation]; okOld {
			if val != valOld {
				return true
			}
		} else {
			return true
		}
	}

	return false
}

type GarbageCollectPredicate struct {
	predicate.Funcs
	Scheme  *runtime.Scheme
	Log     logr.Logger
	Storage *Storage
}

// Delete removes all artifacts from storage that belong to the
// referenced object.
func (gc GarbageCollectPredicate) Delete(e event.DeleteEvent) bool {
	gvk, err := apiutil.GVKForObject(e.Object, gc.Scheme)
	if err != nil {
		gc.Log.Error(err, "unable to get GroupVersionKind for deleted object")
		return false
	}
	// delete artifacts
	artifact := gc.Storage.ArtifactFor(gvk.Kind, e.Meta, "*", "")
	if err := gc.Storage.RemoveAll(artifact); err != nil {
		gc.Log.Error(err, "unable to delete artifacts",
			gvk.Kind, fmt.Sprintf("%s/%s", e.Meta.GetNamespace(), e.Meta.GetName()))
	} else {
		gc.Log.Info(gvk.Kind+" artifacts deleted",
			gvk.Kind, fmt.Sprintf("%s/%s", e.Meta.GetNamespace(), e.Meta.GetName()))
	}
	return true
}
