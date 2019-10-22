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

package baremetal

import (
	"reflect"
	"testing"
)

func TestMergeList(t *testing.T) {

	for _, tc := range []struct {
		Scenario string
		List1    []interface{}
		List2    []interface{}
		Output   []interface{}
	}{
		{
			Scenario: "Merge two lists",
			List1:    []interface{}{"abc", "def"},
			List2:    []interface{}{"ghi", "jkl"},
			Output:   []interface{}{"abc", "def", "ghi", "jkl"},
		},
		{
			Scenario: "Merge two lists, second empty",
			List1:    []interface{}{"abc", "def"},
			List2:    []interface{}{},
			Output:   []interface{}{"abc", "def"},
		},
		{
			Scenario: "Merge two lists, first empty",
			List1:    []interface{}{},
			List2:    []interface{}{"ghi", "jkl"},
			Output:   []interface{}{"ghi", "jkl"},
		},
		{
			Scenario: "Merge two lists, both empty",
			List1:    []interface{}{},
			List2:    []interface{}{},
			Output:   []interface{}{},
		},
	} {
		t.Run(tc.Scenario, func(t *testing.T) {
			testList, err := mergeList(tc.List1, tc.List2)
			if err != nil {
				t.Error("Unexpected error")
			}
			if !reflect.DeepEqual(tc.Output, testList) {
				t.Errorf("Output different from expected list : \nExpected: %#v\nGot:      %#v",
					tc.Output, testList)
			}
		})
	}
}

func TestMergeMap(t *testing.T) {

	for _, tc := range []struct {
		Scenario    string
		Map1        map[interface{}]interface{}
		Map2        map[interface{}]interface{}
		Output      map[interface{}]interface{}
		ExpectError bool
	}{
		{
			Scenario:    "Merge two maps",
			Map1:        map[interface{}]interface{}{"abc": "abc", "def": "def"},
			Map2:        map[interface{}]interface{}{"bcd": "bcd", "efg": "efg"},
			Output:      map[interface{}]interface{}{"abc": "abc", "def": "def", "bcd": "bcd", "efg": "efg"},
			ExpectError: false,
		},
		{
			Scenario:    "Merge two maps, first empty",
			Map1:        map[interface{}]interface{}{},
			Map2:        map[interface{}]interface{}{"bcd": "bcd", "efg": "efg"},
			Output:      map[interface{}]interface{}{"bcd": "bcd", "efg": "efg"},
			ExpectError: false,
		},
		{
			Scenario:    "Merge two maps, second empty",
			Map1:        map[interface{}]interface{}{"abc": "abc", "def": "def"},
			Map2:        map[interface{}]interface{}{},
			Output:      map[interface{}]interface{}{"abc": "abc", "def": "def"},
			ExpectError: false,
		},
		{
			Scenario:    "Merge two maps, conflict",
			Map1:        map[interface{}]interface{}{"abc": "abc", "def": "def"},
			Map2:        map[interface{}]interface{}{"abc": "bcd", "efg": "efg"},
			Output:      map[interface{}]interface{}{},
			ExpectError: true,
		},
	} {
		t.Run(tc.Scenario, func(t *testing.T) {
			testMap, err := mergeMap(tc.Map1, tc.Map2)
			if err != nil {
				if !tc.ExpectError {
					t.Error("Unexpected error")
				}
				return
			} else {
				if tc.ExpectError {
					t.Error("Expected error")
					return
				}
			}
			if !reflect.DeepEqual(tc.Output, testMap) {
				t.Errorf("Output different from expected map : \nExpected: %#v\nGot:      %#v",
					tc.Output, testMap)
			}
		})
	}
}

func TestMergeCloudInitMaps(t *testing.T) {

	for _, tc := range []struct {
		Scenario    string
		Map1        map[string]interface{}
		Map2        map[string]interface{}
		Output      map[string]interface{}
		ExpectError bool
	}{
		{
			Scenario: "Merge two cloud init maps",
			Map1: map[string]interface{}{
				"abc": "abc",
				"def": []interface{}{"def"},
				"ghi": map[interface{}]interface{}{
					"jkl": "jkl",
				},
			},
			Map2: map[string]interface{}{
				"bcd": "bcd",
				"def": []interface{}{"efg"},
				"ghi": map[interface{}]interface{}{
					"klm": "klm",
				},
			},
			Output: map[string]interface{}{
				"abc": "abc",
				"def": []interface{}{"def", "efg"},
				"bcd": "bcd",
				"ghi": map[interface{}]interface{}{
					"jkl": "jkl",
					"klm": "klm",
				},
			},
			ExpectError: false,
		},
		{
			Scenario: "Merge two cloud init maps, first empty",
			Map1:     map[string]interface{}{},
			Map2: map[string]interface{}{
				"bcd": "bcd",
				"def": []interface{}{"efg"},
				"ghi": map[interface{}]interface{}{
					"klm": "klm",
				},
			},
			Output: map[string]interface{}{
				"def": []interface{}{"efg"},
				"bcd": "bcd",
				"ghi": map[interface{}]interface{}{
					"klm": "klm",
				},
			},
			ExpectError: false,
		},
		{
			Scenario: "Merge two cloud init maps, second empty",
			Map1: map[string]interface{}{
				"abc": "abc",
				"def": []interface{}{"def"},
				"ghi": map[interface{}]interface{}{
					"jkl": "jkl",
				},
			},
			Map2: map[string]interface{}{},
			Output: map[string]interface{}{
				"abc": "abc",
				"def": []interface{}{"def"},
				"ghi": map[interface{}]interface{}{
					"jkl": "jkl",
				},
			},
			ExpectError: false,
		},
		{
			Scenario:    "Merge two cloud init maps, conflict",
			Map1:        map[string]interface{}{"abc": "abc"},
			Map2:        map[string]interface{}{"abc": "bcd"},
			Output:      map[string]interface{}{},
			ExpectError: true,
		},
		{
			Scenario: "Merge two cloud init maps, type mismatch",
			Map1: map[string]interface{}{
				"def": []interface{}{"def"},
			},
			Map2: map[string]interface{}{
				"def": map[interface{}]interface{}{
					"klm": "klm",
				},
			},
			Output:      map[string]interface{}{},
			ExpectError: true,
		},
		{
			Scenario: "Merge two cloud init maps, inner conflict",
			Map1: map[string]interface{}{
				"ghi": map[interface{}]interface{}{
					"jkl": "jkl",
				},
			},
			Map2: map[string]interface{}{
				"ghi": map[interface{}]interface{}{
					"jkl": "klm",
				},
			},
			Output:      map[string]interface{}{},
			ExpectError: true,
		},
	} {
		t.Run(tc.Scenario, func(t *testing.T) {
			testMap, err := mergeCloudInitMaps(tc.Map1, tc.Map2)
			if err != nil {
				if !tc.ExpectError {
					t.Error("Unexpected error")
				}
				return
			} else {
				if tc.ExpectError {
					t.Error("Expected error")
					return
				}
			}
			if !reflect.DeepEqual(tc.Output, testMap) {
				t.Errorf("Output different from expected map : \nExpected: %#v\nGot:      %#v",
					tc.Output, testMap)
			}
		})
	}
}

func TestUnmarshall(t *testing.T) {

	for _, tc := range []struct {
		Scenario    string
		Input       []byte
		Output      map[string]interface{}
		ExpectError bool
	}{
		{
			Scenario: "unmarshalling",
			Input: []byte(`#cloud-config
abc: abc
bcd:
- bcd
- cde
cde:
  def: def
  efg: efg
`),
			Output: map[string]interface{}{
				"abc": interface{}("abc"),
				"bcd": interface{}(
					[]interface{}{
						interface{}("bcd"),
						interface{}("cde"),
					},
				),
				"cde": interface{}(map[interface{}]interface{}{
					interface{}("def"): interface{}("def"),
					interface{}("efg"): interface{}("efg"),
				}),
			},
			ExpectError: false,
		},
		{
			Scenario: "unmarshalling",
			Input: []byte(`#cloud-config
abc: abc
bcd:
- bcd
- cde
cde:
def: def
  efg: efg
`),
			Output:      map[string]interface{}{},
			ExpectError: true,
		},
	} {
		t.Run(tc.Scenario, func(t *testing.T) {
			testOutput, err := unmarshal(tc.Input)
			if err != nil {
				if !tc.ExpectError {
					t.Error("Unexpected error")
				}
				return
			} else {
				if tc.ExpectError {
					t.Error("Expected error")
					return
				}
			}
			if !reflect.DeepEqual(tc.Output, testOutput) {
				t.Errorf("Output different from expected map : \nExpected: %#v\nGot:      %#v",
					tc.Output, testOutput)
			}
		})
	}
}
