/*
Copyright 2016 The Kubernetes Authors.

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

package drain

import (
	"fmt"
	"testing"
	"time"

	//appsv1beta1 "k8s.io/api/apps/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	policyv1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	. "github.com/gardener/autoscaler/cluster-autoscaler/utils/test"
	"k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
	"k8s.io/kubernetes/pkg/api/testapi"
)

func TestDrain(t *testing.T) {
	replicas := int32(5)

	rc := apiv1.ReplicationController{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rc",
			Namespace: "default",
			SelfLink:  testapi.Default.SelfLink("replicationcontrollers", "rc"),
		},
		Spec: apiv1.ReplicationControllerSpec{
			Replicas: &replicas,
		},
	}

	rcPod := &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "bar",
			Namespace:       "default",
			OwnerReferences: GenerateOwnerReferences(rc.Name, "ReplicationController", "extensions/v1beta1", ""),
		},
		Spec: apiv1.PodSpec{
			NodeName: "node",
		},
	}

	kubeSystemRc := apiv1.ReplicationController{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rc",
			Namespace: "kube-system",
			SelfLink:  testapi.Default.SelfLink("replicationcontrollers", "rc"),
		},
		Spec: apiv1.ReplicationControllerSpec{
			Replicas: &replicas,
		},
	}

	kubeSystemRcPod := &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "bar",
			Namespace:       "kube-system",
			OwnerReferences: GenerateOwnerReferences(kubeSystemRc.Name, "ReplicationController", "extensions/v1beta1", ""),
			Labels: map[string]string{
				"k8s-app": "bar",
			},
		},
		Spec: apiv1.PodSpec{
			NodeName: "node",
		},
	}

	ds := extensions.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ds",
			Namespace: "default",
			SelfLink:  "/apiv1s/extensions/v1beta1/namespaces/default/daemonsets/ds",
		},
	}

	dsPod := &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "bar",
			Namespace:       "default",
			OwnerReferences: GenerateOwnerReferences(ds.Name, "DaemonSet", "extensions/v1beta1", ""),
		},
		Spec: apiv1.PodSpec{
			NodeName: "node",
		},
	}

	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "job",
			Namespace: "default",
			SelfLink:  "/apiv1s/extensions/v1beta1/namespaces/default/jobs/job",
		},
	}

	jobPod := &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "bar",
			Namespace:       "default",
			OwnerReferences: GenerateOwnerReferences(job.Name, "Job", "extensions/v1beta1", ""),
		},
	}

	/*	Disable stateful set test for a moment due to fake client problems with handling v1beta1 SS

		statefulset := appsv1beta1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ss",
				Namespace: "default",
				SelfLink:  "/apiv1s/extensions/v1beta1/namespaces/default/statefulsets/ss",
			},
		}

		ssPod := &apiv1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "bar",
				Namespace:   "default",
				Annotations: map[string]string{apiv1.CreatedByAnnotation: RefJSON(&statefulset)},
			},
		}
	*/
	rs := extensions.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rs",
			Namespace: "default",
			SelfLink:  testapi.Default.SelfLink("replicasets", "rs"),
		},
		Spec: extensions.ReplicaSetSpec{
			Replicas: &replicas,
		},
	}

	rsPod := &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "bar",
			Namespace:       "default",
			OwnerReferences: GenerateOwnerReferences(rs.Name, "ReplicaSet", "extensions/v1beta1", ""),
		},
		Spec: apiv1.PodSpec{
			NodeName: "node",
		},
	}

	rsPodDeleted := &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "bar",
			Namespace:         "default",
			OwnerReferences:   GenerateOwnerReferences(rs.Name, "ReplicaSet", "extensions/v1beta1", ""),
			DeletionTimestamp: &metav1.Time{Time: time.Now().Add(-time.Hour)},
		},
		Spec: apiv1.PodSpec{
			NodeName: "node",
		},
	}

	nakedPod := &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bar",
			Namespace: "default",
		},
		Spec: apiv1.PodSpec{
			NodeName: "node",
		},
	}

	emptydirPod := &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bar",
			Namespace: "default",
		},
		Spec: apiv1.PodSpec{
			NodeName: "node",
			Volumes: []apiv1.Volume{
				{
					Name:         "scratch",
					VolumeSource: apiv1.VolumeSource{EmptyDir: &apiv1.EmptyDirVolumeSource{Medium: ""}},
				},
			},
		},
	}

	terminalPod := &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bar",
			Namespace: "default",
		},
		Spec: apiv1.PodSpec{
			NodeName:      "node",
			RestartPolicy: apiv1.RestartPolicyOnFailure,
		},
		Status: apiv1.PodStatus{
			Phase: apiv1.PodSucceeded,
		},
	}

	failedPod := &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bar",
			Namespace: "default",
		},
		Spec: apiv1.PodSpec{
			NodeName:      "node",
			RestartPolicy: apiv1.RestartPolicyNever,
		},
		Status: apiv1.PodStatus{
			Phase: apiv1.PodFailed,
		},
	}

	evictedPod := &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bar",
			Namespace: "default",
		},
		Spec: apiv1.PodSpec{
			NodeName:      "node",
			RestartPolicy: apiv1.RestartPolicyAlways,
		},
		Status: apiv1.PodStatus{
			Phase: apiv1.PodFailed,
		},
	}

	safePod := &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bar",
			Namespace: "default",
			Annotations: map[string]string{
				PodSafeToEvictKey: "true",
			},
		},
		Spec: apiv1.PodSpec{
			NodeName: "node",
		},
	}

	unsafeRcPod := &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "bar",
			Namespace:       "default",
			OwnerReferences: GenerateOwnerReferences(rc.Name, "ReplicationController", "extensions/v1beta1", ""),
			Annotations: map[string]string{
				PodSafeToEvictKey: "false",
			},
		},
		Spec: apiv1.PodSpec{
			NodeName: "node",
		},
	}

	unsafeJobPod := &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "bar",
			Namespace:       "default",
			OwnerReferences: GenerateOwnerReferences(job.Name, "Job", "extensions/v1beta1", ""),
			Annotations: map[string]string{
				PodSafeToEvictKey: "false",
			},
		},
	}

	kubeSystemSafePod := &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bar",
			Namespace: "kube-system",
			Annotations: map[string]string{
				PodSafeToEvictKey: "true",
			},
		},
		Spec: apiv1.PodSpec{
			NodeName: "node",
		},
	}

	emptydirSafePod := &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bar",
			Namespace: "default",
			Annotations: map[string]string{
				PodSafeToEvictKey: "true",
			},
		},
		Spec: apiv1.PodSpec{
			NodeName: "node",
			Volumes: []apiv1.Volume{
				{
					Name:         "scratch",
					VolumeSource: apiv1.VolumeSource{EmptyDir: &apiv1.EmptyDirVolumeSource{Medium: ""}},
				},
			},
		},
	}

	emptyPDB := &policyv1.PodDisruptionBudget{}

	kubeSystemPDB := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "kube-system",
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"k8s-app": "bar",
				},
			},
		},
	}

	kubeSystemFakePDB := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "kube-system",
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"k8s-app": "foo",
				},
			},
		},
	}

	defaultNamespacePDB := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"k8s-app": "PDB-managed pod",
				},
			},
		},
	}

	tests := []struct {
		description string
		pods        []*apiv1.Pod
		pdbs        []*policyv1.PodDisruptionBudget
		rcs         []apiv1.ReplicationController
		replicaSets []extensions.ReplicaSet
		expectFatal bool
		expectPods  []*apiv1.Pod
	}{
		{
			description: "RC-managed pod",
			pods:        []*apiv1.Pod{rcPod},
			pdbs:        []*policyv1.PodDisruptionBudget{},
			rcs:         []apiv1.ReplicationController{rc},
			expectFatal: false,
			expectPods:  []*apiv1.Pod{rcPod},
		},
		{
			description: "DS-managed pod",
			pods:        []*apiv1.Pod{dsPod},
			pdbs:        []*policyv1.PodDisruptionBudget{},
			expectFatal: false,
			expectPods:  []*apiv1.Pod{},
		},
		{
			description: "Job-managed pod",
			pods:        []*apiv1.Pod{jobPod},
			pdbs:        []*policyv1.PodDisruptionBudget{},
			rcs:         []apiv1.ReplicationController{rc},
			expectFatal: false,
			expectPods:  []*apiv1.Pod{jobPod},
		},
		/*  Disable SS tests for a moment
		{
			description: "SS-managed pod",
			pods:        []*apiv1.Pod{ssPod},
			pdbs:        []*policyv1.PodDisruptionBudget{},
			rcs:         []apiv1.ReplicationController{rc},
			expectFatal: false,
			expectPods:  []*apiv1.Pod{ssPod},
		},
		*/
		{
			description: "RS-managed pod",
			pods:        []*apiv1.Pod{rsPod},
			pdbs:        []*policyv1.PodDisruptionBudget{},
			replicaSets: []extensions.ReplicaSet{rs},
			expectFatal: false,
			expectPods:  []*apiv1.Pod{rsPod},
		},
		{
			description: "RS-managed pod that is being deleted",
			pods:        []*apiv1.Pod{rsPodDeleted},
			pdbs:        []*policyv1.PodDisruptionBudget{},
			replicaSets: []extensions.ReplicaSet{rs},
			expectFatal: false,
			expectPods:  []*apiv1.Pod{},
		},
		{
			description: "naked pod",
			pods:        []*apiv1.Pod{nakedPod},
			pdbs:        []*policyv1.PodDisruptionBudget{},
			expectFatal: true,
			expectPods:  []*apiv1.Pod{},
		},
		{
			description: "pod with EmptyDir",
			pods:        []*apiv1.Pod{emptydirPod},
			pdbs:        []*policyv1.PodDisruptionBudget{},
			expectFatal: true,
			expectPods:  []*apiv1.Pod{},
		},
		{
			description: "failed pod",
			pods:        []*apiv1.Pod{failedPod},
			pdbs:        []*policyv1.PodDisruptionBudget{},
			expectFatal: false,
			expectPods:  []*apiv1.Pod{failedPod},
		},
		{
			description: "evicted pod",
			pods:        []*apiv1.Pod{evictedPod},
			pdbs:        []*policyv1.PodDisruptionBudget{},
			expectFatal: false,
			expectPods:  []*apiv1.Pod{evictedPod},
		},
		{
			description: "pod in terminal state",
			pods:        []*apiv1.Pod{terminalPod},
			pdbs:        []*policyv1.PodDisruptionBudget{},
			expectFatal: false,
			expectPods:  []*apiv1.Pod{terminalPod},
		},
		{
			description: "pod with PodSafeToEvict annotation",
			pods:        []*apiv1.Pod{safePod},
			pdbs:        []*policyv1.PodDisruptionBudget{},
			expectFatal: false,
			expectPods:  []*apiv1.Pod{safePod},
		},
		{
			description: "kube-system pod with PodSafeToEvict annotation",
			pods:        []*apiv1.Pod{kubeSystemSafePod},
			pdbs:        []*policyv1.PodDisruptionBudget{},
			expectFatal: false,
			expectPods:  []*apiv1.Pod{kubeSystemSafePod},
		},
		{
			description: "pod with EmptyDir and PodSafeToEvict annotation",
			pods:        []*apiv1.Pod{emptydirSafePod},
			pdbs:        []*policyv1.PodDisruptionBudget{},
			expectFatal: false,
			expectPods:  []*apiv1.Pod{emptydirSafePod},
		},
		{
			description: "RC-managed pod with PodSafeToEvict=false annotation",
			pods:        []*apiv1.Pod{unsafeRcPod},
			rcs:         []apiv1.ReplicationController{rc},
			pdbs:        []*policyv1.PodDisruptionBudget{},
			expectFatal: true,
			expectPods:  []*apiv1.Pod{},
		},
		{
			description: "Job-managed pod with PodSafeToEvict=false annotation",
			pods:        []*apiv1.Pod{unsafeJobPod},
			pdbs:        []*policyv1.PodDisruptionBudget{},
			rcs:         []apiv1.ReplicationController{rc},
			expectFatal: true,
			expectPods:  []*apiv1.Pod{},
		},
		{
			description: "empty PDB with RC-managed pod",
			pods:        []*apiv1.Pod{rcPod},
			pdbs:        []*policyv1.PodDisruptionBudget{emptyPDB},
			rcs:         []apiv1.ReplicationController{rc},
			expectFatal: false,
			expectPods:  []*apiv1.Pod{rcPod},
		},
		{
			description: "kube-system PDB with matching kube-system pod",
			pods:        []*apiv1.Pod{kubeSystemRcPod},
			pdbs:        []*policyv1.PodDisruptionBudget{kubeSystemPDB},
			rcs:         []apiv1.ReplicationController{rc},
			expectFatal: false,
			expectPods:  []*apiv1.Pod{kubeSystemRcPod},
		},
		{
			description: "kube-system PDB with non-matching kube-system pod",
			pods:        []*apiv1.Pod{kubeSystemRcPod},
			pdbs:        []*policyv1.PodDisruptionBudget{kubeSystemFakePDB},
			rcs:         []apiv1.ReplicationController{rc},
			expectFatal: true,
			expectPods:  []*apiv1.Pod{},
		},
		{
			description: "kube-system PDB with default namespace pod",
			pods:        []*apiv1.Pod{rcPod},
			pdbs:        []*policyv1.PodDisruptionBudget{kubeSystemPDB},
			rcs:         []apiv1.ReplicationController{rc},
			expectFatal: false,
			expectPods:  []*apiv1.Pod{rcPod},
		},
		{
			description: "default namespace PDB with matching labels kube-system pod",
			pods:        []*apiv1.Pod{kubeSystemRcPod},
			pdbs:        []*policyv1.PodDisruptionBudget{defaultNamespacePDB},
			rcs:         []apiv1.ReplicationController{rc},
			expectFatal: true,
			expectPods:  []*apiv1.Pod{},
		},
	}

	for _, test := range tests {
		fakeClient := &fake.Clientset{}
		register := func(resource string, obj runtime.Object, meta metav1.ObjectMeta) {
			fakeClient.Fake.AddReactor("get", resource, func(action core.Action) (bool, runtime.Object, error) {
				getAction := action.(core.GetAction)
				if getAction.GetName() == meta.GetName() && getAction.GetNamespace() == meta.GetNamespace() {
					return true, obj, nil
				}
				return false, nil, fmt.Errorf("Not found")
			})
		}
		if len(test.rcs) > 0 {
			register("replicationcontrollers", &test.rcs[0], test.rcs[0].ObjectMeta)
		}
		register("daemonsets", &ds, ds.ObjectMeta)
		register("jobs", &job, job.ObjectMeta)
		// register("statefulsets", &statefulset, statefulset.ObjectMeta)

		if len(test.replicaSets) > 0 {
			register("replicasets", &test.replicaSets[0], test.replicaSets[0].ObjectMeta)
		}
		pods, err := GetPodsForDeletionOnNodeDrain(test.pods, test.pdbs,
			false, true, true, true, fakeClient, 0, time.Now())

		if test.expectFatal {
			if err == nil {
				t.Fatalf("%s: unexpected non-error", test.description)
			}
		}

		if !test.expectFatal {
			if err != nil {
				t.Fatalf("%s: error occurred: %v", test.description, err)
			}
		}

		if len(pods) != len(test.expectPods) {
			t.Fatalf("Wrong pod list content: %v", test.description)
		}
	}
}
