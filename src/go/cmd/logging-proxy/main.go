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

/*
This proxy forwards (fluent-bit) client requests to the remote endpoints defined through FLUENTD_HOST and FLUENTD_PORT
environment variables.
It attaches an oauth2 token (obtained through Google's metadata service) to the request. This proxy is designed to run
on the same pod as the client (fluent-bit) in order to remove the need for TLS encrypted traffic.
The outbound request (to the remote endpoint [fluentd]) will be TLS encrypted, of course.
In order to (re)load environment variables, this proxy has to be restarted.
ENABLE_DEBUG (true, 1) logs **all** client requests and remote responses. DO NOT USE IN PRODUCTION.
*/

package main

import (
	"bytes"
	"fmt"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const oauth2Scope = "https://www.googleapis.com/auth/cloud-platform.read-only"

var (
	listeningAddr   string
	remoteHost      string
	tenantNamespace string
	debug           bool
	debugRequestID  = 0
)

type proxy struct {
	client *http.Client
	m      *sync.Mutex
}

var hopByHopHeaders = []string{
	"Upgrade",
	"Proxy-Authorization",
	"Connection",
	"Proxy-Authenticate",
	"Keep-Alive",
	"Transfer-Encoding",
	"Trailers",
	"Te",
}

// reroute replaces the current request url with the url to relay to (e.g. fluentd)
func reroute(id int, r *http.Request) {
	original := r.URL.String()

	host := remoteHost
	query := ""
	if r.URL.RawQuery != "" {
		query = fmt.Sprintf("?%s", r.URL.RawQuery)
	}
	fragment := ""
	if r.URL.Fragment != "" {
		fragment = fmt.Sprintf("#%s", r.URL.Fragment)
	}

	r.URL, _ = url.Parse(fmt.Sprintf("https://%s%s%s%s", host, r.URL.Path, query, fragment))

	debugLog(id, "Changing Request URL: from: %s \n\t to: %s", original, r.URL.String())
}

func cpHeader(dst, src http.Header) {
	for h, vArrSrc := range src {
		for _, v := range vArrSrc {
			dst.Add(h, v)
		}
	}
}

func rmHopByHopHeaders(h http.Header) {
	for _, v := range hopByHopHeaders {
		h.Del(v)
	}
}

func debugLog(id int, format string, values ...interface{}) {
	if !debug {
		return
	}
	log.Printf("DEBUG: REQUEST "+fmt.Sprintf("%d", id)+":"+format+"\n", values...)
}

func debugLogBody(id int, body io.ReadCloser, format string, values ...interface{}) io.ReadCloser {
	bodyByte, _ := ioutil.ReadAll(body)
	body.Close()
	debugLog(id, format, string(bodyByte), values)
	body = ioutil.NopCloser(bytes.NewBuffer(bodyByte))
	return body
}

func (p proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var id int
	if debug {
		p.m.Lock()
		id = debugRequestID
		debugRequestID++
		p.m.Unlock()
	}
	debugLog(id, "Request Header: %v", r.Header)
	if debug {
		r.Body = debugLogBody(id, r.Body, "Request Body: %s")
	}
	defer r.Body.Close()

	// it is an error to set this field in client request (as per Go documentation)
	r.RequestURI = ""

	rmHopByHopHeaders(r.Header)

	reroute(id, r)

	debugLog(id, "Setting Host to: %v", remoteHost)
	r.Host = remoteHost
	// Add query parameter with tenant namespace for authentification
	debugLog(id, "Incoming query paramter: %v", r.URL.RawQuery)
	q := r.URL.Query()
	q.Add("tenant-namespace", tenantNamespace)
	r.URL.RawQuery = q.Encode()
	debugLog(id, "Outgoing query paramter: %v", r.URL.RawQuery)

	resp, err := p.client.Do(r)
	if err != nil {
		http.Error(w, "error: proxy: check proxy logs for details", http.StatusBadGateway)
		log.Println("error: proxy: ServeHTTP: Do:", err)
		return
	}

	debugLog(id, "Response Status: %s", resp.Status)
	debugLog(id, "Response Header: %v", resp.Header)
	if debug {
		resp.Body = debugLogBody(id, resp.Body, "Response Body: %s")
	}
	defer resp.Body.Close()

	rmHopByHopHeaders(resp.Header)

	cpHeader(w.Header(), resp.Header)

	w.WriteHeader(resp.StatusCode)
	if n, err := io.Copy(w, resp.Body); err != nil {
		// this error shouldn't occur
		log.Println("critical error: proxy: ServeHTTP: Copy: copying response body failed: copied bytes:", n, err)
		if _, err = w.Write([]byte("CRITICAL ERROR: PROXY: COPYING RESPONSE BODY FAILED: SEE PROXY LOGS FOR MORE DETAILS")); err != nil {
			log.Println("critical error: proxy: ServeHTTP: Write:", err)
		}
		return
	}
}

// initClient creates an oauth2 client with the metadata service token source which will be used to relay requests with bearer auth
func initClient() *http.Client {
	ctx := context.Background()
	client, err := google.DefaultClient(ctx, oauth2Scope)
	if err != nil {
		log.Fatal("fatal error: main: DefaultClient:", err)
	}
	client.Timeout = 60 * time.Second

	return client
}

func main() {
	if debug {
		log.Println("DEBUG MODE ENABLED: LOGGING REQUESTS AND RESPONSES.")
	}
	log.Println("Proxy starting. For further documentation see the source code files 'main.go' or others.")
	fmt.Println("---")

	remoteHost = os.Getenv("FLUENTD_HOST")
	listeningAddr = os.Getenv("LISTENING_ADDR")
	tenantNamespace = os.Getenv("TENANT_NAMESPACE")

	if remoteHost == "" {
		log.Fatal("Environment variable FLUENTD_HOST is not set")
	}
	if listeningAddr == "" {
		log.Fatal("Environment variable LISTENING_ADDR is not set")
	}

	if tenantNamespace == "" {
		tenantNamespace = "default"
	}

	client := initClient()

	handler := proxy{client: client, m: &sync.Mutex{}}

	log.Println("proxy server listening on", listeningAddr)
	if err := http.ListenAndServe(listeningAddr, handler); err != nil {
		log.Fatal("fatal error: main: ListenAndServe:", err)
	}
}

func init() {
	d := os.Getenv("ENABLE_DEBUG")
	debug = d == "1" || strings.ToLower(d) == "true"
}
