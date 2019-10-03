/*
Copyright 2019 The Kubernetes Authors.

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

package v1alpha2

import (
	"testing"

	"github.com/onsi/gomega"
	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestSpecIsValid(t *testing.T) {
	cases := []struct {
		Spec          BareMetalMachineSpec
		ErrorExpected bool
		Name          string
	}{
		{
			Spec:          BareMetalMachineSpec{},
			ErrorExpected: true,
			Name:          "empty spec",
		},
		{
			Spec: BareMetalMachineSpec{
				Image: Image{
					URL:      "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2",
					Checksum: "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2.md5sum",
				},
				UserData: &corev1.SecretReference{
					Name: "worker-user-data",
				},
			},
			ErrorExpected: false,
			Name:          "Valid spec without UserData.Namespace",
		},
		{
			Spec: BareMetalMachineSpec{
				Image: Image{
					URL:      "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2",
					Checksum: "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2.md5sum",
				},
				UserData: &corev1.SecretReference{
					Name:      "worker-user-data",
					Namespace: "otherns",
				},
			},
			ErrorExpected: false,
			Name:          "Valid spec with UserData.Namespace",
		},
		{
			Spec: BareMetalMachineSpec{
				Image: Image{
					Checksum: "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2.md5sum",
				},
				UserData: &corev1.SecretReference{
					Name: "worker-user-data",
				},
			},
			ErrorExpected: true,
			Name:          "missing Image.URL",
		},
		{
			Spec: BareMetalMachineSpec{
				Image: Image{
					URL: "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2",
				},
				UserData: &corev1.SecretReference{
					Name: "worker-user-data",
				},
			},
			ErrorExpected: true,
			Name:          "missing Image.Checksum",
		},
		{
			Spec: BareMetalMachineSpec{
				Image: Image{
					URL:      "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2",
					Checksum: "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2.md5sum",
				},
			},
			ErrorExpected: false,
			Name:          "missing optional UserData",
		},
		{
			Spec: BareMetalMachineSpec{
				Image: Image{
					URL:      "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2",
					Checksum: "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2.md5sum",
				},
				UserData: &corev1.SecretReference{
					Namespace: "otherns",
				},
			},
			ErrorExpected: false,
			Name:          "missing optional UserData.Name",
		},
		{
			Spec: BareMetalMachineSpec{
				Image: Image{
					URL:      "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2",
					Checksum: "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2.md5sum",
				},
				HostSelector: HostSelector{},
			},
			ErrorExpected: false,
			Name:          "Empty HostSelector provided",
		},
		{
			Spec: BareMetalMachineSpec{
				Image: Image{
					URL:      "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2",
					Checksum: "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2.md5sum",
				},
				HostSelector: HostSelector{
					MatchLabels: map[string]string{"key": "value"},
				},
			},
			ErrorExpected: false,
			Name:          "HostSelector Single MatchLabel provided",
		},
		{
			Spec: BareMetalMachineSpec{
				Image: Image{
					URL:      "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2",
					Checksum: "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2.md5sum",
				},
				HostSelector: HostSelector{
					MatchLabels: map[string]string{"key": "value", "key2": "value2"},
				},
			},
			ErrorExpected: false,
			Name:          "HostSelector Multiple MatchLabels provided",
		},
	}

	for _, tc := range cases {
		err := tc.Spec.IsValid()
		if tc.ErrorExpected && err == nil {
			t.Errorf("Did not get error from case \"%v\"", tc.Name)
		}
		if !tc.ErrorExpected && err != nil {
			t.Errorf("Got unexpected error from case \"%v\": %v", tc.Name, err)
		}
	}
}

func TestStorageBareMetalMachineSpec(t *testing.T) {
	key := types.NamespacedName{
		Name:      "foo",
		Namespace: "default",
	}
	created := &BareMetalMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "default",
		},
		Spec: BareMetalMachineSpec{
			UserData: &corev1.SecretReference{
				Name: "foo",
			},
		},
	}
	g := gomega.NewGomegaWithT(t)

	// Test Create
	fetched := &BareMetalMachine{}
	g.Expect(c.Create(context.TODO(), created)).NotTo(gomega.HaveOccurred())

	g.Expect(c.Get(context.TODO(), key, fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(fetched).To(gomega.Equal(created))

	// Test Updating the Labels
	updated := fetched.DeepCopy()
	g.Expect(c.Update(context.TODO(), updated)).NotTo(gomega.HaveOccurred())

	g.Expect(c.Get(context.TODO(), key, fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(fetched).To(gomega.Equal(updated))

	// Test Delete
	g.Expect(c.Delete(context.TODO(), fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(c.Get(context.TODO(), key, fetched)).To(gomega.HaveOccurred())
}
