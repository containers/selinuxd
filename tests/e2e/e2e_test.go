package e2e

import (
	"fmt"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("E2e", func() {
	When("selinuxd is ready", func() {
		BeforeEach(waitForSelinuxdToBeReady, selinuxdReadyTimeout)

		When("Installing a basic policy", func() {
			var (
				policy     = "testport"
				policyPath = filepath.Join(selinuxdDir, fmt.Sprintf("%s.cil", policy))
			)
			BeforeEach(func() {
				installPolicyFromReference("../data/testport.cil", policyPath)
			})

			AfterEach(func() {
				removePolicyIfPossible(policyPath)
			})

			It("Succeeds", func() {
				By("Waiting for the policy to be installed")
				policyEventually(policy).Should(MatchRegexp(`status.*Installed`))
			})
		})

		When("Installing a erroneous policy", func() {
			var (
				policy     = "badtestport"
				policyPath = filepath.Join(selinuxdDir, fmt.Sprintf("%s.cil", policy))
			)
			BeforeEach(func() {
				installPolicyFromReference("../data/badtestport.cil", policyPath)
			})

			AfterEach(func() {
				removePolicyIfPossible(policyPath)
			})

			It("Reports an error status", func() {
				By("Waiting for the policy to be installed")
				policyEventually(policy).Should(MatchRegexp(`status.*Failed`))
			})
		})

		When("Updating an erroneous policy", func() {
			var (
				policy     = "updatetestport"
				policyPath = filepath.Join(selinuxdDir, fmt.Sprintf("%s.cil", policy))
			)
			BeforeEach(func() {
				installPolicyFromReference("../data/badtestport.cil", policyPath)
			})

			AfterEach(func() {
				removePolicyIfPossible(policyPath)
			})

			It("Reports an error status", func() {
				By("Waiting for the policy to be marked as failed")
				policyEventually(policy).Should(MatchRegexp(`status.*Failed`))

				By("Updating policy to a valid one")
				installPolicyFromReference("../data/testport.cil", policyPath)

				By("Waiting for the policy to be installed")
				policyEventually(policy).Should(MatchRegexp(`status.*Installed`))
			})
		})
	})
})
