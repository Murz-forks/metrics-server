// Copyright 2018 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package summary

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/golang/glog"
	stats "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"
)

// KubeletInterface knows how to fetch metrics from the Kubelet
type KubeletInterface interface {
	// GetSummary fetches summary metrics from the given Kubelet
	GetSummary(host string) (*stats.Summary, error)
}

type kubeletClient struct {
	portIsInsecure bool
	port           uint
	client         *http.Client
}

type ErrNotFound struct {
	endpoint string
}

func (err *ErrNotFound) Error() string {
	return fmt.Sprintf("%q not found", err.endpoint)
}

func IsNotFoundError(err error) bool {
	_, isNotFound := err.(*ErrNotFound)
	return isNotFound
}

func (kc *kubeletClient) postRequestAndGetValue(client *http.Client, req *http.Request, value interface{}) error {
	// TODO(directxman12): support validating certs by hostname
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body - %v", err)
	}
	if response.StatusCode == http.StatusNotFound {
		return &ErrNotFound{req.URL.String()}
	} else if response.StatusCode != http.StatusOK {
		return fmt.Errorf("request failed - %q, response: %q", response.Status, string(body))
	}

	kubeletAddr := "[unknown]"
	if req.URL != nil {
		kubeletAddr = req.URL.Host
	}
	glog.V(10).Infof("Raw response from Kubelet at %s: %s", kubeletAddr, string(body))

	err = json.Unmarshal(body, value)
	if err != nil {
		return fmt.Errorf("failed to parse output. Response: %q. Error: %v", string(body), err)
	}
	return nil
}

func (kc *kubeletClient) GetSummary(host string) (*stats.Summary, error) {
	url := url.URL{
		Scheme: "https",
		Host:   fmt.Sprintf("%s:%d", host, kc.port),
		Path:   "/stats/summary/",
	}
	if kc.portIsInsecure {
		url.Scheme = "http"
	}

	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return nil, err
	}
	summary := &stats.Summary{}
	client := kc.client
	if client == nil {
		client = http.DefaultClient
	}
	err = kc.postRequestAndGetValue(client, req, summary)
	return summary, err
}

func NewKubeletClient(transport http.RoundTripper, port uint, portIsInsecure bool) (KubeletInterface, error) {
	c := &http.Client{
		Transport: transport,
	}
	return &kubeletClient{
		portIsInsecure: portIsInsecure,
		port:           port,
		client:         c,
	}, nil
}
