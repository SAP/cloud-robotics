// Copyright 2021 The Cloud Robotics Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package chartassignment

import (
	"testing"

	apps "github.com/SAP/cloud-robotics/src/go/pkg/apis/apps/v1alpha1"
	"github.com/SAP/cloud-robotics/src/go/pkg/kubetest"
	"github.com/golang/mock/gomock"
	"k8s.io/client-go/tools/record"
	"k8s.io/helm/pkg/chartutil"
)

const (
	ChartName = "testchart"
)

func verifyValues(t *testing.T, have string, wantValues chartutil.Values) {
	if want, err := wantValues.YAML(); err != nil {
		t.Fatal(err)
	} else if want != have {
		t.Fatalf("config values do not match: want\n%s\n\ngot\n%s\n", want, have)
	}
}

func Test_loadChart_mergesValues(t *testing.T) {
	var as apps.ChartAssignment
	unmarshalYAML(t, &as, `
metadata:
  name: test-assignment-1
  namespace: default
spec:
  chart:
    values:
      bar1: 4
      bar2:
        baz2: test
	`)
	as.Spec.Chart.Inline = kubetest.BuildInlineChart(t, ChartName /*template=*/, "", `
foo1:
  baz1: "hello"
bar1: 3`)
	wantValues := chartutil.Values{
		"bar1": 4,
		"bar2": chartutil.Values{"baz2": "test"},
		"foo1": chartutil.Values{"baz1": "hello"},
	}

	_, vals, err := loadChart(&as.Spec.Chart)
	if err != nil {
		t.Fatal(err)
	}
	verifyValues(t, vals, wantValues)
}

func Test_loadChartWithoutTemplates_returnsZeroManifests(t *testing.T) {
	var as apps.ChartAssignment
	unmarshalYAML(t, &as, `
metadata:
  name: test-assignment-1
  namespace: default
spec:
  chart:
    values:
	`)
	as.Spec.Chart.Inline = kubetest.BuildInlineChart(t, ChartName /*template=*/, "", `foo: 1`)
	resources, _, err := loadAndExpandChart(&as)
	if err != nil {
		t.Fatal(err)
	}
	if len(resources) > 0 {
		t.Errorf("Expected no resources, got %d", len(resources))
	}

}

func Test_updateSynk_callsApply(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var as apps.ChartAssignment
	unmarshalYAML(t, &as, `
metadata:
  name: test-assignment-1
  namespace: default
spec:
  chart:
    values:
	`)
	as.Spec.Chart.Inline = kubetest.BuildInlineChart(t, ChartName /*template=*/, "", `foo: 1`)

	mockSynk := NewMockInterface(ctrl)
	r := &release{
		synk:     mockSynk,
		recorder: &record.FakeRecorder{},
	}

	rs := &apps.ResourceSet{}
	mockSynk.EXPECT().Apply(gomock.Any(), "default.test-assignment-1", gomock.Any(), gomock.Any()).Return(rs, nil).Times(1)

	// First apply, the chart should be installed.
	r.update(&as)
}

func Test_deleteSynk_callsDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var as apps.ChartAssignment
	unmarshalYAML(t, &as, `
metadata:
  name: test-assignment-1
  namespace: default
spec:
  chart:
    values:
	`)
	as.Spec.Chart.Inline = kubetest.BuildInlineChart(t, ChartName /*template=*/, "", `foo: 1`)

	mockSynk := NewMockInterface(ctrl)
	r := &release{
		synk:     mockSynk,
		recorder: &record.FakeRecorder{},
	}

	mockSynk.EXPECT().Delete(gomock.Any(), "default.test-assignment-1").Return(nil).Times(1)

	// First apply, the chart should be installed.
	r.delete(&as)
}
