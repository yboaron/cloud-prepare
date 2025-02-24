/*
SPDX-License-Identifier: Apache-2.0

Copyright Contributors to the Submariner project.

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

package gcp_test

import (
	"errors"
	"net/http"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/submariner-io/admiral/pkg/reporter"
	"github.com/submariner-io/cloud-prepare/pkg/api"
	"github.com/submariner-io/cloud-prepare/pkg/gcp"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
)

const ingressRuleName = "test-infraID-submariner-internal-ports-ingress"

var _ = Describe("Cloud", func() {
	Describe("PrepareForSubmariner", testPrepareForSubmariner)
	Describe("CleanupAfterSubmariner", testCleanupAfterSubmariner)
})

func testPrepareForSubmariner() {
	t := newCloudTestDriver()

	var retError error

	JustBeforeEach(func() {
		retError = t.cloud.PrepareForSubmariner(api.PrepareForSubmarinerInput{
			InternalPorts: []api.PortSpec{
				{
					Port:     100,
					Protocol: "TCP",
				},
				{
					Port:     200,
					Protocol: "UDP",
				},
			},
		}, reporter.Stdout())
	})

	When("the firewall rule doesn't exist", func() {
		BeforeEach(func() {
			t.gcpClient.EXPECT().GetFirewallRule(projectID, ingressRuleName).Return(nil, &googleapi.Error{Code: http.StatusNotFound})
		})

		Context("", func() {
			var actualRule *compute.Firewall

			BeforeEach(func() {
				t.gcpClient.EXPECT().InsertFirewallRule(projectID, gomock.Any()).DoAndReturn(func(_ string, rule *compute.Firewall) error {
					actualRule = rule
					return nil
				})
			})

			It("should correctly insert it", func() {
				Expect(retError).To(Succeed())

				Expect(actualRule).ToNot(BeNil(), "InsertFirewallRule was not called")
				assertIngressRule(actualRule)
			})
		})

		Context("and insertion fails", func() {
			BeforeEach(func() {
				t.gcpClient.EXPECT().InsertFirewallRule(projectID, gomock.Any()).Return(errors.New("fake insert error"))
			})

			It("should return an error", func() {
				Expect(retError).ToNot(Succeed())
			})
		})
	})

	When("the firewall rule already exists", func() {
		BeforeEach(func() {
			t.gcpClient.EXPECT().GetFirewallRule(projectID, ingressRuleName).DoAndReturn(func(_, ruleName string) (*compute.Firewall, error) {
				return &compute.Firewall{Name: ruleName}, nil
			})
		})

		Context("", func() {
			var actualRule *compute.Firewall

			BeforeEach(func() {
				t.gcpClient.EXPECT().UpdateFirewallRule(projectID, ingressRuleName, gomock.Any()).DoAndReturn(
					func(_, _ string, rule *compute.Firewall) error {
						actualRule = rule
						return nil
					})
			})

			It("should update it", func() {
				Expect(retError).To(Succeed())

				Expect(actualRule).ToNot(BeNil(), "UpdateFirewallRule was not called")
				assertIngressRule(actualRule)
			})
		})

		Context("and update fails", func() {
			BeforeEach(func() {
				t.gcpClient.EXPECT().UpdateFirewallRule(projectID, ingressRuleName, gomock.Any()).Return(errors.New("fake update error"))
			})

			It("should return an error", func() {
				Expect(retError).ToNot(Succeed())
			})
		})
	})

	When("retrieval of the firewall rule fails", func() {
		BeforeEach(func() {
			t.gcpClient.EXPECT().GetFirewallRule(projectID, ingressRuleName).Return(nil, errors.New("fake get error"))
		})

		It("should return an error", func() {
			Expect(retError).ToNot(Succeed())
		})
	})
}

func testCleanupAfterSubmariner() {
	t := newCloudTestDriver()

	var retError error

	JustBeforeEach(func() {
		retError = t.cloud.CleanupAfterSubmariner(reporter.Stdout())
	})

	Context("on success", func() {
		BeforeEach(func() {
			t.gcpClient.EXPECT().DeleteFirewallRule(projectID, ingressRuleName).Return(nil)
		})

		It("should delete the firewall rule", func() {
			Expect(retError).To(Succeed())
		})
	})

	When("the firewall rule doesn't exist", func() {
		BeforeEach(func() {
			t.gcpClient.EXPECT().DeleteFirewallRule(projectID, ingressRuleName).Return(&googleapi.Error{Code: http.StatusNotFound})
		})

		It("should succeed", func() {
			Expect(retError).To(Succeed())
		})
	})

	When("deletion fails", func() {
		BeforeEach(func() {
			t.gcpClient.EXPECT().DeleteFirewallRule(projectID, ingressRuleName).Return(errors.New("fake delete error"))
		})

		It("should return an error", func() {
			Expect(retError).ToNot(Succeed())
		})
	})
}

type cloudTestDriver struct {
	fakeGCPClientBase
	cloud api.Cloud
}

func newCloudTestDriver() *cloudTestDriver {
	t := &cloudTestDriver{}

	BeforeEach(func() {
		t.beforeEach()

		t.cloud = gcp.NewCloud(gcp.CloudInfo{
			InfraID:   infraID,
			Region:    region,
			ProjectID: projectID,
			Client:    t.gcpClient,
		})
	})

	AfterEach(t.afterEach)

	return t
}

func assertIngressRule(rule *compute.Firewall) {
	Expect(rule.Name).To(Equal(ingressRuleName))
	Expect(rule.Direction).To(Equal("INGRESS"))
	Expect(rule.Allowed).To(HaveLen(2))
	Expect(rule.Allowed[0]).To(Equal(&compute.FirewallAllowed{
		IPProtocol: "TCP",
		Ports:      []string{"100"},
	}))
	Expect(rule.Allowed[1]).To(Equal(&compute.FirewallAllowed{
		IPProtocol: "UDP",
		Ports:      []string{"200"},
	}))
}
