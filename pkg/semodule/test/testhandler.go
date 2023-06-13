package test

import (
	"path/filepath"
	"sync"

	seiface "github.com/containers/selinuxd/pkg/semodule/interface"
	"github.com/containers/selinuxd/pkg/utils"
)

type SEModuleTestHandler struct {
	modules []string
	mu      sync.Mutex
}

// Ensure that the test handler implements the Handler interface
var _ seiface.Handler = &SEModuleTestHandler{}

func NewSEModuleTestHandler() *SEModuleTestHandler {
	return &SEModuleTestHandler{}
}

func (smt *SEModuleTestHandler) SetAutoCommit(bool) {
}

func (smt *SEModuleTestHandler) Install(modulePath string) error {
	baseFile := filepath.Base(modulePath)
	module := utils.GetFileWithoutExtension(baseFile)
	// Only install module if it's not already there.
	if smt.IsModuleInstalled(module) {
		return nil
	}
	smt.mu.Lock()
	defer smt.mu.Unlock()
	smt.modules = append(smt.modules, module)
	return nil
}

func (smt *SEModuleTestHandler) IsModuleInstalled(module string) bool {
	smt.mu.Lock()
	defer smt.mu.Unlock()
	for _, mod := range smt.modules {
		// The module had already been installed.
		// Nothing to do
		if mod == module {
			return true
		}
	}
	return false
}

func (smt *SEModuleTestHandler) List() ([]seiface.PolicyModule, error) {
	// Return a copy
	smt.mu.Lock()
	defer smt.mu.Unlock()
	policyModules := []seiface.PolicyModule{}
	for _, module := range smt.modules {
		m, err := smt.GetPolicyModule(module)
		if err != nil {
			return policyModules, err
		}
		policyModules = append(policyModules, m)
	}
	return policyModules, nil
}

func (smt *SEModuleTestHandler) GetPolicyModule(moduleName string) (seiface.PolicyModule, error) {
	for _, mod := range smt.modules {
		// The module had already been installed.
		// Nothing to do
		if mod == moduleName {
			return seiface.PolicyModule{
				Name:     moduleName,
				Ext:      "cil",
				Checksum: "sha256:e6c4d9c235af5d3ca50f660fc2283ecc330ebea73ec35241cc9cc47878dab68c",
			}, nil
		}
	}
	return seiface.PolicyModule{}, seiface.ErrPolicyNotFound
}

func (smt *SEModuleTestHandler) Remove(modToRemove string) error {
	idToRemove := -1
	smt.mu.Lock()
	defer smt.mu.Unlock()
	for id, mod := range smt.modules {
		if mod == modToRemove {
			idToRemove = id
			break
		}
	}
	smt.modules = append(smt.modules[:idToRemove], smt.modules[idToRemove+1:]...)
	return nil
}

func (smt *SEModuleTestHandler) Close() error {
	return nil
}

func (smt *SEModuleTestHandler) Commit() error {
	return nil
}
