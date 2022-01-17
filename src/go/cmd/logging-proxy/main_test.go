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

package main

import (
	"bytes"
	"fmt"
	"gopkg.in/h2non/gock.v1"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProxy_ServeHTTP(t *testing.T) {
	tt := []struct {
		remHost string
		path    string
		status  int
		noMock  bool
	}{
		{
			remHost: "fluentd.default.svc.some.cluster.com",
			path:    "some-tag",
			status:  http.StatusOK,
		},
		{
			remHost: "fluentd.default.svc.some.cluster.com",
			path:    "some-tag",
			status:  http.StatusOK,
		},
		{
			remHost: "abc.com",
			path:    "some-tag",
			noMock:  true,
			status:  http.StatusBadGateway,
		},
	}

	for _, tc := range tt {
		remoteHost = tc.remHost
		endpoint := remoteHost

		// create new (fluent-bit) client request
		r := httptest.NewRequest("POST", "http://127.0.0.1:8080/"+tc.path, bytes.NewBuffer([]byte(rawKymaSystemRecord)))
		r.Header.Set("Connection", "keep-alive")

		rec := httptest.NewRecorder()
		p := proxy{client: &http.Client{}} // default client instead of google.DefaultClient

		// create mock (fluentd remote server)
		gock.New("https://"+endpoint).
			Post("/"+tc.path).
			Filter(func(req *http.Request) bool {
				// test if hop by hop was removed
				if r.Header.Get("Connection") != "" {
					t.Fatal("hop by hop header was not removed on the way from proxy to remote")
				}
				// only "activate" (match) mock if specified
				return !tc.noMock
			}).Reply(tc.status).
			BodyString(rawKymaSystemRecord).
			SetHeader("Connection", "close").
			SetHeader("X-My-Custom-Header", "some-value") // test correct proxying of response headers

		// method to be tested
		p.ServeHTTP(rec, r)

		if !tc.noMock && !gock.IsDone() {
			t.Error("gock mock not triggered for", tc)
		}
		gock.Off()

		if tc.status != rec.Code {
			t.Errorf("incorrect status code: want: %d, got: %d", tc.status, rec.Code)
			continue
		}

		// guards further checks that are only expected to pass in a success case (code = 2xx)
		if fmt.Sprintf("%d", rec.Code)[0] != '2' {
			continue
		}

		got := rec.Body.String()
		if got != rawKymaSystemRecord {
			t.Error("body not proxied correctly. Got:", got)
		}
		if rec.Header().Get("Connection") != "" {
			t.Error("hop by hop header was not removed on the way back from proxy to client")
		}
		if rec.Header().Get("X-My-Custom-Header") == "" {
			t.Errorf("response header '%s' not copied correctly from remote response to client response", "X-My-Custom-Header")
		}
	}
}

func TestReroute(t *testing.T) {
	remoteHost = fluentdHost

	tt := []struct {
		r    *http.Request
		want string
	}{{
		r:    httptest.NewRequest("POST", "http://127.0.0.1/", bytes.NewBuffer([]byte(rawKymaSystemRecord))),
		want: "https://" + fluentdHost + "/",
	},
		{
			r:    httptest.NewRequest("POST", "http://127.0.0.1:1337/", bytes.NewBuffer([]byte(rawKymaSystemRecord))),
			want: "https://" + fluentdHost + "/",
		},
		{
			r:    httptest.NewRequest("POST", "http://127.0.0.1/kube", bytes.NewBuffer([]byte(rawKymaSystemRecord))),
			want: "https://" + fluentdHost + "/kube",
		},
		{
			r:    httptest.NewRequest("POST", "http://127.0.0.1/kube?abc=dfg", bytes.NewBuffer([]byte(rawKymaSystemRecord))),
			want: "https://" + fluentdHost + "/kube?abc=dfg",
		},
	}

	for _, tc := range tt {
		reroute(1, tc.r)
		got := tc.r.URL.String()
		if got != tc.want {
			t.Errorf("got: %s , want: %s", got, tc.want)
		}
	}
}

const fluentdHost = "host.local"
const rawKymaSystemRecord = `[{"date":1632315578.793905,"source_type":"kyma-system","stream":"stderr","time":"2021-09-22T12:59:38.793904564Z","cluster_identifier":"api.dome-robot.whr-eval.internal.canary.k8s.ondemand.com","kubernetes":{"annotations":{"reference_resources_gardener_cloud_configmap-179b5e9b":"kube-proxy-cleanup-script-d61416a9","reference_resources_gardener_cloud_configmap-384e2626":"kube-proxy-config-e6acf503","reference_resources_gardener_cloud_secret-19adee2c":"kube-proxy-40051c6a","checksum_secret-kube-proxy":"39e2b201a92c701ff5d519ca5a3665b7442c554d2568d82f84dbe9e06441370c","reference_resources_gardener_cloud_configmap-f80ee5d2":"kube-proxy-conntrack-fix-script-40051c6a","scheduler_alpha_kubernetes_io_critical-pod":"","vpaUpdates":"Pod resources updated by kube-proxy: container 0: memory request, cpu request, cpu limit, memory limit; container 1: cpu request, memory request","vpaObservedContainers":"kube-proxy, conntrack-fix","kubernetes_io_psp":"gardener.kube-system.kube-proxy"},"labels":{"app":"kubernetes","shoot_gardener_cloud_no-cleanup":"true","origin":"gardener","pod-template-generation":"2","role":"proxy","gardener_cloud_role":"system-component","controller-revision-hash":"8557d78c57"},"pod_name":"kube-proxy-cggbh","host":"shoot--whr-eval--dome-robot-worker-bzsj9-z1-644f8-gkgdz","container_image":"sha256:447e2dca739209f708fe2bcfed4eb13aa25680bb878b534ee7cf020a7e2a9b01","pod_id":"0a90b338-d3e3-4055-94fe-742fbe22017e","docker_id":"59e1a639ca2acce0620804fc00c97c86860a6b7bad4fabd0350eebf2cd0dff3a","container_name":"kube-proxy","namespace_name":"kube-system","container_hash":"eu.gcr.io/sap-se-gcr-k8s-public/k8s_gcr_io/kube-proxy@sha256:d5a5cd17f3b1e63c77cb077c91897dff5c35b86765749a929b6382cecf2e6f98"},"log":"I0922 12:59:38.793585 1 proxier.go:871] Syncing iptables rules\n"}]`
const rawFluentBitMetricsRecord = `[{"date":1632315578.573239,"cluster_identifier":"api.dome-robot.whr-eval.internal.canary.k8s.ondemand.com","kubernetes.labels.app":"fluent-bit-metrics","kubernetes.pod_name":"fluent-bit-w9fqd","kubernetes.namespace_name":"kyma-system","Mem.total":7642328,"Mem.used":3902884,"Mem.free":3739444,"source_type":"metrics"}]`
