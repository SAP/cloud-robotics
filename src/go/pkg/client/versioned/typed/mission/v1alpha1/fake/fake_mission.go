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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1alpha1 "github.com/SAP/cloud-robotics/src/go/pkg/apis/mission/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeMissions implements MissionInterface
type FakeMissions struct {
	Fake *FakeMissionV1alpha1
	ns   string
}

var missionsResource = schema.GroupVersionResource{Group: "mission.cloudrobotics.com", Version: "v1alpha1", Resource: "missions"}

var missionsKind = schema.GroupVersionKind{Group: "mission.cloudrobotics.com", Version: "v1alpha1", Kind: "Mission"}

// Get takes name of the mission, and returns the corresponding mission object, and an error if there is any.
func (c *FakeMissions) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.Mission, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(missionsResource, c.ns, name), &v1alpha1.Mission{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Mission), err
}

// List takes label and field selectors, and returns the list of Missions that match those selectors.
func (c *FakeMissions) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.MissionList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(missionsResource, missionsKind, c.ns, opts), &v1alpha1.MissionList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.MissionList{ListMeta: obj.(*v1alpha1.MissionList).ListMeta}
	for _, item := range obj.(*v1alpha1.MissionList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested missions.
func (c *FakeMissions) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(missionsResource, c.ns, opts))

}

// Create takes the representation of a mission and creates it.  Returns the server's representation of the mission, and an error, if there is any.
func (c *FakeMissions) Create(ctx context.Context, mission *v1alpha1.Mission, opts v1.CreateOptions) (result *v1alpha1.Mission, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(missionsResource, c.ns, mission), &v1alpha1.Mission{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Mission), err
}

// Update takes the representation of a mission and updates it. Returns the server's representation of the mission, and an error, if there is any.
func (c *FakeMissions) Update(ctx context.Context, mission *v1alpha1.Mission, opts v1.UpdateOptions) (result *v1alpha1.Mission, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(missionsResource, c.ns, mission), &v1alpha1.Mission{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Mission), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeMissions) UpdateStatus(ctx context.Context, mission *v1alpha1.Mission, opts v1.UpdateOptions) (*v1alpha1.Mission, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(missionsResource, "status", c.ns, mission), &v1alpha1.Mission{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Mission), err
}

// Delete takes name of the mission and deletes it. Returns an error if one occurs.
func (c *FakeMissions) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(missionsResource, c.ns, name), &v1alpha1.Mission{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeMissions) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(missionsResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.MissionList{})
	return err
}

// Patch applies the patch and returns the patched mission.
func (c *FakeMissions) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.Mission, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(missionsResource, c.ns, name, pt, data, subresources...), &v1alpha1.Mission{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Mission), err
}