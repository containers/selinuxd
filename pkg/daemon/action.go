package daemon

import (
	"fmt"

	"github.com/JAORMX/selinuxd/pkg/semodule"
	"github.com/JAORMX/selinuxd/pkg/utils"
)

// Defines an action to be taken on a policy file on the specified path
type policyAction struct {
	path      string
	operation policyOp
}

// defines the operation that an action will take on the file
type policyOp int16

const (
	install policyOp = iota
	remove  policyOp = iota
)

func (po policyOp) String() string {
	switch po {
	case install:
		return "install"
	case remove:
		return "remove"
	default:
		return "unknown"
	}
}

func (pa policyAction) do(modulePath string, sh semodule.SEModuleHandler) (string, error) {
	var policyArg string
	var err error

	switch pa.operation {
	case install:
		policyPath, err := utils.GetSafePath(modulePath, pa.path)
		if err != nil {
			return "", err
		}
		err = sh.Install(policyPath)
	case remove:
		baseFile, err := utils.GetCleanBase(pa.path)
		if err != nil {
			return "", err
		}
		policyArg = utils.GetFileWithoutExtension(baseFile)

		if !pa.moduleInstalled(sh, policyArg) {
			return "No action needed; Module is not in the system", nil
		}

		err = sh.Remove(policyArg)
	default:
		return "", fmt.Errorf("Unkown operation for policy %s. This shouldn't happen", pa.path)
	}

	return "", err
}

func (pa policyAction) moduleInstalled(sh semodule.SEModuleHandler, policy string) bool {
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
