package daemon

import (
	"github.com/JAORMX/selinuxd/pkg/semodule"
	"github.com/JAORMX/selinuxd/pkg/utils"
)

type policyAction interface {
	String() string
	do(modulePath string, sh semodule.SEModuleHandler) (string, error)
}

// Defines an action to be taken on a policy file on the specified path
type policyInstall struct {
	path string
}

func NewInstallAction(path string) policyAction {
	return &policyInstall{path}
}

func (pi *policyInstall) String() string {
	return "install - " + pi.path
}

func (pi *policyInstall) do(modulePath string, sh semodule.SEModuleHandler) (string, error) {
	policyPath, err := utils.GetSafePath(modulePath, pi.path)
	if err != nil {
		return "", err
	}
	err = sh.Install(policyPath)
	return "", err
}

type policyRemove struct {
	path string
}

func NewRemoveAction(path string) policyAction {
	return &policyRemove{path}
}

func (pi *policyRemove) String() string {
	return "remove - " + pi.path
}

func (pi *policyRemove) do(modulePath string, sh semodule.SEModuleHandler) (string, error) {
	var policyArg string
	baseFile, err := utils.GetCleanBase(pi.path)
	if err != nil {
		return "", err
	}
	policyArg = utils.GetFileWithoutExtension(baseFile)

	if !pi.moduleInstalled(sh, policyArg) {
		return "No action needed; Module is not in the system", nil
	}

	err = sh.Remove(policyArg)
	return "", err
}

func (pi *policyRemove) moduleInstalled(sh semodule.SEModuleHandler, policy string) bool {
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
