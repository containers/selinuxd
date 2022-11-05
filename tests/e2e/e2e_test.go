package e2e

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/containers/selinuxd/pkg/datastore"
	. "github.com/onsi/ginkgo/v2"
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

		When("Installing policy in sub-directory", func() {
			var (
				policy     = "subdirtestport"
				subdirPath = filepath.Join(selinuxdDir, "my-subdir")
				policyPath = filepath.Join(subdirPath, fmt.Sprintf("%s.cil", policy))
			)
			BeforeEach(func() {
				By("Creating subdir")
				mkdirErr := os.Mkdir(subdirPath, 0o700)
				Expect(mkdirErr).ToNot(HaveOccurred())
				installPolicyFromReference("../data/testport.cil", policyPath)
			})

			AfterEach(func() {
				removePolicyIfPossible(policyPath)
				By("Deleting subdir")
				rmdirErr := os.Remove(subdirPath)
				Expect(rmdirErr).ToNot(HaveOccurred())
			})

			It("Reports an installed status", func() {
				By("Waiting for the policy to be installed")
				policyEventually(policy).Should(MatchRegexp(`status.*Installed`))
			})
		})

		When("Installing multiple policies", func() {
			policies := map[string]string{
				"testport":               string(datastore.InstalledStatus),
				"badtestport":            string(datastore.FailedStatus),
				"errorlogger":            string(datastore.InstalledStatus),
				"selinuxd":               string(datastore.InstalledStatus),
				"test_append_avc":        string(datastore.InstalledStatus),
				"test_basic":             string(datastore.InstalledStatus),
				"test_default":           string(datastore.InstalledStatus),
				"test_devices":           string(datastore.InstalledStatus),
				"test_fullnetworkaccess": string(datastore.InstalledStatus),
				"test_nocontext":         string(datastore.InstalledStatus),
				"test_ports":             string(datastore.InstalledStatus),
				"test_ttyaccess":         string(datastore.InstalledStatus),
				"test_virtaccess":        string(datastore.InstalledStatus),
				"test_xaccess":           string(datastore.InstalledStatus),
			}
			BeforeEach(func() {
				for pol := range policies {
					pname := fmt.Sprintf("%s.cil", pol)
					installPolicyFromReference(
						filepath.Join("../data/", pname),
						filepath.Join(selinuxdDir, pname),
					)
				}
			})

			AfterEach(func() {
				for pol := range policies {
					pname := fmt.Sprintf("%s.cil", pol)
					removePolicyIfPossible(filepath.Join(selinuxdDir, pname))
				}
			})

			It("Installs all the policies", func() {
				By("Waiting policies to be installed")
				for pol, status := range policies {
					policyEventually(pol).Should(MatchRegexp(fmt.Sprintf(`status.*%s`, status)))
				}
				By("Listing all policies to ensure they're all there")
				pollist := selinuxdctl("status")
				for pol := range policies {
					Expect(pollist).Should(ContainSubstring(pol))
				}
			})
		})
	})
})
