// Copyright 2019 The Cloud Robotics Authors
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

package setup

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/SAP/cloud-robotics/src/go/pkg/setup/util"

	"golang.org/x/crypto/ssh/terminal"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

// GetRobotName returns a valid robot name or an error. If the robotName parameter
// is non-empty, it checks if it is valid. If it is an empty string, the user is
// prompted to select a robot.
func GetRobotName(ctx context.Context, f util.Factory, client dynamic.ResourceInterface, robotName string) (string, error) {
	if robotName == "" {
		exitIfNotRunningInTerminal("ERROR: --robot-name not specified")

		robots, err := client.List(ctx, metav1.ListOptions{})
		if err != nil {
			return "", err
		}

		robotName, err := selectRobot(f, robots.Items)
		if err != nil {
			return "", err
		}
		return robotName, nil
	}
	_, err := client.Get(ctx, robotName, metav1.GetOptions{})
	if err != nil {
		if s, ok := err.(*apierrors.StatusError); ok && s.ErrStatus.Reason == metav1.StatusReasonNotFound {
			return "", fmt.Errorf("robot %v not found", robotName)
		}
		return "", err
	}
	return robotName, nil
}

// exitIfNotRunningInTerminal checks if stdin is connected to a terminal. If
// not, it prints the given message and exits.
func exitIfNotRunningInTerminal(message ...interface{}) {
	if !terminal.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintln(os.Stderr, message...)
		os.Exit(1)
	}
}

// Ask the user to select the robot from a list. Saves name to disk after
// selection.
func selectRobot(f util.Factory, robots []unstructured.Unstructured) (string, error) {
	fmt.Printf("  # %-20v %-10v %-16v\n", "Name", "Type", "Create Time")
	for i, robot := range robots {
		spec, ok := robot.Object["spec"].(map[string]interface{})
		if !ok {
			log.Print("unmarshaling robot failed: spec is not a map")
			continue
		}
		fmt.Printf("%3v %-20v %-10v %v\n", i+1, robot.GetName(), spec["type"], robot.GetCreationTimestamp().String())
	}

	fmt.Print("Select robot: ")
	var ix int
	for {
		var err error
		ix, err = f.ScanInt()
		if err == nil && 1 <= ix && ix <= len(robots) {
			break
		}
		fmt.Printf("Please enter a number (1-%v): ", len(robots))
	}
	return robots[ix-1].GetName(), nil
}
