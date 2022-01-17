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

// The robotauth package contains the class for reading and writing the
// robot-id.json file. This file contains the id & private key of a robot
// that's connected to a Cloud project.
package robotauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/SAP/cloud-robotics/src/go/pkg/kubeutils"
	"golang.org/x/oauth2"
)

const (
	// TODO(ensonic): setup-dev creates a key and stores it, only for the ssh-app to read it
	credentialsFile = "~/.config/cloud-robotics/robot-id.json"
)

// Object containing ID, as stored in robot-id.json.
type RobotAuth struct {
	RobotName         string `json:"id"`
	Domain            string `json:"domain"`
	UpstreamAuthToken string `json:"upstream_auth_token"`
}

func filename() string {
	return kubeutils.ExpandUser(credentialsFile)
}

// LoadFromFile loads key from json file. If keyfile is "", it tries to load
// from the default location.
func LoadFromFile(keyfile string) (*RobotAuth, error) {
	if keyfile == "" {
		keyfile = filename()
	}
	raw, err := ioutil.ReadFile(keyfile)
	if err != nil {
		return nil, fmt.Errorf("failed to read %v: %v", credentialsFile, err)
	}

	var robotAuth RobotAuth
	err = json.Unmarshal(raw, &robotAuth)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %v: %v", credentialsFile, err)
	}

	return &robotAuth, nil
}

// StoreInFile writes a newly-chosen ID to disk.
func (r *RobotAuth) StoreInFile() error {
	raw, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("failed to serialize ID: %v", err)
	}

	file := filename()
	if err := os.MkdirAll(kubeutils.ExpandUser(filepath.Dir(file)), 0700); err != nil {
		return err
	}

	err = ioutil.WriteFile(file, raw, 0600)
	if err != nil {
		return fmt.Errorf("failed to write %v: %v", credentialsFile, err)
	}

	return nil
}

// CreateRobotTokenSource creates an OAuth2 token source for robot service account token.
// It provides access to the robot-service service account in upstream Kyma cluster.
func (r *RobotAuth) CreateRobotTokenSource(ctx context.Context) oauth2.TokenSource {
	// Use service account token source. Currently the token does not expire
	s := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: r.UpstreamAuthToken,
		TokenType:   "Bearer",
		Expiry:      time.Date(10000, time.January, 1, 0, 0, 0, 0, time.UTC),
	})
	return s
}
