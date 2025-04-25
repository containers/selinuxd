package e2e

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	//nolint:staticcheck
	. "github.com/onsi/ginkgo/v2"
	//nolint:staticcheck
	. "github.com/onsi/gomega"
)

var (
	selinuxdInAContainer  bool
	selinuxdContainerName string
)

const (
	// timeout for selinuxd to report it's ready
	selinuxdReadyTimeout float64 = 320
	// default time to wait for selinuxd do an operation
	selinuxdTimeout = 10 * time.Minute
	// default interval between operations
	defaultInterval = 2 * time.Second
	selinuxdDir     = "/etc/selinux.d"
)

func initVars() {
	if strings.EqualFold(os.Getenv("SELINUXD_IS_CONTAINER"), "yes") ||
		strings.EqualFold(os.Getenv("SELINUXD_IS_CONTAINER"), "true") {
		selinuxdInAContainer = true
		selinuxdContainerName = os.Getenv("SELINUXD_CONTAINER_NAME")
		if selinuxdContainerName == "" {
			fmt.Println("You must specify $SELINUXD_CONTAINER_NAME if running in a container")
			os.Exit(1)
		}
	}
}

// Wrapper for Gomega's Eventually function. Targeted at checking
// That the policy status will eventually reach a certain state.
func policyEventually(policy string) AsyncAssertion {
	return Eventually(func() string {
		return selinuxdctl("status", policy)
	}, selinuxdTimeout, defaultInterval)
}

func do(cmd string, args ...string) string {
	execcmd := exec.Command(cmd, args...)
	output, err := execcmd.CombinedOutput()
	Expect(err).ShouldNot(HaveOccurred(),
		"The command '%s' shouldn't fail.\n- Arguments: %v\n- Output: %s", cmd, args, output)
	return strings.Trim(string(output), "\n")
}

func selinuxdctl(args ...string) string {
	if !selinuxdInAContainer {
		return do("selinuxdctl", args...)
	}
	return do("podman", append([]string{"exec", selinuxdContainerName, "selinuxdctl"}, args...)...)
}

func waitForSelinuxdToBeReady(done Done) {
	for {
		isReady := selinuxdctl("is-ready")
		if isReady == "yes" {
			close(done)
			return
		}
		time.Sleep(defaultInterval)
	}
}

func installPolicyFromReference(refPath, destPath string) {
	By(fmt.Sprintf("Installing policy from %s to %s", refPath, destPath))
	Expect(refPath).Should(BeAnExistingFile())
	ref, openErr := os.Open(refPath)
	Expect(openErr).ShouldNot(HaveOccurred())
	dest, createErr := os.Create(destPath)
	Expect(createErr).ShouldNot(HaveOccurred())
	_, copyErr := io.Copy(dest, ref)
	Expect(copyErr).ShouldNot(HaveOccurred())
}

func removePolicyIfPossible(policy string) {
	if !CurrentGinkgoTestDescription().Failed {
		By(fmt.Sprintf("Removing policy: %s", policy))
		os.Remove(policy)
	}
}
