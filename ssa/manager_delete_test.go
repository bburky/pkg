/*
Copyright 2021 Stefan Prodan
Copyright 2021 The Flux authors

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

package ssa

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestDelete(t *testing.T) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	id := generateName("delete")
	objects, err := readManifest("testdata/test1.yaml", id)
	if err != nil {
		t.Fatal(err)
	}

	manager.SetOwnerLabels(objects, "app1", "default")

	opts := DefaultDeleteOptions()
	opts.Inclusions = manager.GetOwnerLabels("app1", "default")

	_, configMap := getFirstObject(objects, "ConfigMap", id)
	_, role := getFirstObject(objects, "ClusterRole", id)

	if _, err = manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions()); err != nil {
		t.Fatal(err)
	}

	t.Run("deletes objects in order", func(t *testing.T) {
		changeSet, err := manager.DeleteAll(ctx, objects, opts)
		if err != nil {
			t.Fatal(err)
		}

		// expected deleted order
		var expected []string
		for _, object := range objects {
			expected = append(expected, FmtUnstructured(object))
		}

		// verify the change set contains only created actions
		var output []string
		for _, entry := range changeSet.Entries {
			if diff := cmp.Diff(entry.Action, DeletedAction); diff != "" {
				t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
			}
			output = append(output, entry.Subject)
		}

		// verify the change set contains all objects in the right order
		if diff := cmp.Diff(expected, output); diff != "" {
			t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
		}

		configMapClone := configMap.DeepCopy()
		err = manager.client.Get(ctx, client.ObjectKeyFromObject(configMapClone), configMapClone)
		if !apierrors.IsNotFound(err) {
			t.Error(err)
		}

		roleClone := role.DeepCopy()
		err = manager.client.Get(ctx, client.ObjectKeyFromObject(roleClone), roleClone)
		if !apierrors.IsNotFound(err) {
			t.Error(err)
		}
	})

	t.Run("waits for objects termination", func(t *testing.T) {
		_, err := manager.DeleteAll(ctx, objects, opts)
		if err != nil {
			t.Error(err)
		}

		if err := manager.WaitForTermination(objects, WaitOptions{time.Second, 5 * time.Second}); err != nil {
			// workaround for https://github.com/kubernetes-sigs/controller-runtime/issues/880
			if !strings.Contains(err.Error(), "Namespace/") {
				t.Error(err)
			}
		}
	})
}

func TestDelete_Exclusions(t *testing.T) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	id := generateName("ignore")
	objects, err := readManifest("testdata/test1.yaml", id)
	if err != nil {
		t.Fatal(err)
	}

	_, configMap := getFirstObject(objects, "ConfigMap", id)

	t.Run("creates objects", func(t *testing.T) {
		// create objects
		_, err := manager.ApplyAllStaged(ctx, objects, DefaultApplyOptions())
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("skips delete", func(t *testing.T) {
		// mutate in-cluster object
		configMapClone := configMap.DeepCopy()
		err = manager.client.Get(ctx, client.ObjectKeyFromObject(configMapClone), configMapClone)
		if err != nil {
			t.Fatal(err)
		}

		meta := map[string]string{
			"fluxcd.io/ignore": "true",
		}
		configMapClone.SetAnnotations(meta)

		if err := manager.client.Update(ctx, configMapClone); err != nil {
			t.Fatal(err)
		}

		opts := DefaultDeleteOptions()
		opts.Exclusions = meta
		changeSet, err := manager.DeleteAll(ctx, objects, opts)
		if err != nil {
			t.Fatal(err)
		}

		for _, entry := range changeSet.Entries {
			if entry.Action != SkippedAction && entry.Subject == FmtUnstructured(configMap) {
				t.Errorf("Expected %s, got %s", SkippedAction, entry.Action)
			}
		}

		if err := manager.client.Get(ctx, client.ObjectKeyFromObject(configMapClone), configMapClone); err != nil {
			t.Error(err)
		}
	})
}
