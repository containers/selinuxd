package daemon

import (
	"fmt"

	"github.com/JAORMX/selinuxd/pkg/semodule"
	"github.com/JAORMX/selinuxd/pkg/utils"
)

type policyAction interface {
	String() string
	do(modulePath string, sh semodule.Handler) (string, error)
}

// Defines an action to be taken on a policy file on the specified path
type policyInstall struct {
	path string
}

// newInstallAction will execute the "install" action for a policy.
func newInstallAction(path string) policyAction {
	return &policyInstall{path}
}

func (pi *policyInstall) String() string {
	return "install - " + pi.path
}

func (pi *policyInstall) do(modulePath string, sh semodule.Handler) (string, error) {
	policyPath, err := utils.GetSafePath(modulePath, pi.path)
	if err != nil {
		return "", fmt.Errorf("failed getting a safe path for policy: %w", err)
	}
	if err := sh.Install(policyPath); err != nil {
		return "", fmt.Errorf("failed executing install action: %w", err)
	}
	return "", nil
}

type policyRemove struct {
	path string
}

// newInstallAction will execute the "remove" action for a policy.
func newRemoveAction(path string) policyAction {
	return &policyRemove{path}
}

func (pi *policyRemove) String() string {
	return "remove - " + pi.path
}

func (pi *policyRemove) do(modulePath string, sh semodule.Handler) (string, error) {
	var policyArg string
	baseFile, err := utils.GetCleanBase(pi.path)
	if err != nil {
		return "", fmt.Errorf("failed getting clean base name for policy: %w", err)
	}
	policyArg = utils.GetFileWithoutExtension(baseFile)

	if !pi.moduleInstalled(sh, policyArg) {
		return "No action needed; Module is not in the system", nil
	}

	if err := sh.Remove(policyArg); err != nil {
		return "", fmt.Errorf("failed executing remove action: %w", err)
	}
	return "", nil
}

func (pi *policyRemove) moduleInstalled(sh semodule.Handler, policy string) bool {
	currentModules, err := sh.List()
	if err != nil {
		return false
	}

	for _, mod := range currentModules {
		if policy == mod {
			return true
		}
	}

	return false
}
