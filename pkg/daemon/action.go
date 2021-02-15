package daemon

import (
	"fmt"

	"github.com/JAORMX/selinuxd/pkg/datastore"
	"github.com/JAORMX/selinuxd/pkg/semodule"
	"github.com/JAORMX/selinuxd/pkg/utils"
)

type PolicyAction interface {
	String() string
	do(modulePath string, sh semodule.Handler, ds datastore.DataStore) (string, error)
}

// Defines an action to be taken on a policy file on the specified path
type policyInstall struct {
	path string
}

// newInstallAction will execute the "install" action for a policy.
func newInstallAction(path string) PolicyAction {
	return &policyInstall{path}
}

func (pi *policyInstall) String() string {
	return "install - " + pi.path
}

func (pi *policyInstall) do(modulePath string, sh semodule.Handler, ds datastore.DataStore) (string, error) {
	policyName, err := utils.PolicyNameFromPath(pi.path)
	if err != nil {
		return "", fmt.Errorf("installing policy: %w", err)
	}
	policyPath, err := utils.GetSafePath(modulePath, pi.path)
	if err != nil {
		return "", fmt.Errorf("failed getting a safe path for policy: %w", err)
	}

	installErr := sh.Install(policyPath)
	status := datastore.InstalledStatus
	var msg string

	if installErr != nil {
		status = datastore.FailedStatus
		msg = installErr.Error()
	}

	ps := datastore.PolicyStatus{
		Policy:  policyName,
		Status:  status,
		Message: msg,
	}
	puterr := ds.Put(ps)
	if puterr != nil {
		return "", fmt.Errorf("failed persisting status in datastore: %w", puterr)
	}

	if installErr != nil {
		return "", fmt.Errorf("failed executing install action: %w", installErr)
	}
	return "", nil
}

type policyRemove struct {
	path string
}

// newInstallAction will execute the "remove" action for a policy.
func newRemoveAction(path string) PolicyAction {
	return &policyRemove{path}
}

func (pi *policyRemove) String() string {
	return "remove - " + pi.path
}

func (pi *policyRemove) do(modulePath string, sh semodule.Handler, ds datastore.DataStore) (string, error) {
	var policyArg string
	policyArg, err := utils.PolicyNameFromPath(pi.path)
	if err != nil {
		return "", fmt.Errorf("removing policy: %w", err)
	}

	if !pi.moduleInstalled(sh, policyArg) {
		if err := ds.Remove(policyArg); err != nil {
			return "Module is not in the system", fmt.Errorf("failed removing policy from datastore: %w", err)
		}
		return "No action needed; Module is not in the system", nil
	}

	if err := sh.Remove(policyArg); err != nil {
		return "", fmt.Errorf("failed executing remove action: %w", err)
	}

	if err := ds.Remove(policyArg); err != nil {
		return "", fmt.Errorf("failed removing policy from datastore: %w", err)
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
