// Copyright 2017 Istio Authors. All Rights Reserved.
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

package quota

import (
	"fmt"
	"testing"

	rpc "github.com/gogo/googleapis/google/rpc"

	"istio.io/istio/mixer/test/client/env"
)

const (
	mixerQuotaFailMessage = "Quota is exhausted for: RequestCount"
)

// Stats in Envoy proxy.
var expectedStats = map[string]int{
	"http_mixer_filter.total_blocking_remote_check_calls": 2,
	"http_mixer_filter.total_blocking_remote_quota_calls": 2,
	"http_mixer_filter.total_check_calls":                 2,
	"http_mixer_filter.total_quota_calls":                 2,
	"http_mixer_filter.total_remote_check_calls":          2,
	"http_mixer_filter.total_remote_quota_calls":          2,
	"http_mixer_filter.total_report_calls":                2,
}

func TestQuotaCall(t *testing.T) {
	s := env.NewTestSetup(env.QuotaCallTest, t)
	env.SetStatsUpdateInterval(s.MfConfig(), 1)

	// Add v2 quota config for all requests.
	env.AddHTTPQuota(s.MfConfig(), "RequestCount", 5)
	if err := s.SetUp(); err != nil {
		t.Fatalf("Failed to setup test: %v", err)
	}
	defer s.TearDown()

	url := fmt.Sprintf("http://localhost:%d/echo", s.Ports().ClientProxyPort)

	// Issues a GET echo request with 0 size body
	tag := "OKGet"
	if _, _, err := env.HTTPGet(url); err != nil {
		t.Errorf("Failed in request %s: %v", tag, err)
	}
	s.VerifyQuota(tag, "RequestCount", 5)

	// Issues a failed POST request caused by Mixer Quota
	tag = "QuotaFail"
	s.SetMixerQuotaStatus(rpc.Status{
		Code:    int32(rpc.RESOURCE_EXHAUSTED),
		Message: mixerQuotaFailMessage,
	})
	code, respBody, err := env.HTTPPost(url, "text/plain", "Hello World!")
	// Make sure to restore r_status for next request.
	s.SetMixerQuotaStatus(rpc.Status{})
	if err != nil {
		t.Errorf("Failed in request %s: %v", tag, err)
	}
	if code != 429 {
		t.Errorf("Status code 429 is expected.")
	}
	if respBody != "RESOURCE_EXHAUSTED:"+mixerQuotaFailMessage {
		t.Errorf("Error response body is not expected.")
	}
	s.VerifyQuota(tag, "RequestCount", 5)

	// Check stats for Check, Quota and report calls.
	s.VerifyStats(expectedStats)
}
