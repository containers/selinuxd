package test

import (
	"github.com/JAORMX/selinuxd/pkg/semodule"
	"github.com/JAORMX/selinuxd/pkg/utils"
)

type SEModuleTestHandler struct {
	modules []string
}

// Ensure that the test handler implements the SEModuleHandler interface
var _ semodule.SEModuleHandler = &SEModuleTestHandler{}

func NewSEModuleTestHandler() *SEModuleTestHandler {
	return &SEModuleTestHandler{}
}

func (smt *SEModuleTestHandler) Install(modulePath string) error {
	baseFile, _ := utils.GetCleanBase(modulePath)
	module := utils.GetFileWithoutExtension(baseFile)
	// Only install module if it's not already there.
	if smt.IsModuleInstalled(module) {
		return nil
	}
	smt.modules = append(smt.modules, module)
	return nil
}

func (smt *SEModuleTestHandler) IsModuleInstalled(module string) bool {
	for _, mod := range smt.modules {
		// The module had already been installed.
		// Nothing to do
		if mod == module {
			return true
		}
	}
	return false
}

func (smt *SEModuleTestHandler) List() ([]string, error) {
	// Return a copy
	return append([]string(nil), smt.modules...), nil
}
func (smt *SEModuleTestHandler) Remove(modToRemove string) error {
	idToRemove := -1
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
