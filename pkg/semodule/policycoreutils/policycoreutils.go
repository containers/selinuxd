//go:build policycoreutils
// +build policycoreutils

package policycoreutils

import (
	"context"
	"os/exec"
	"strings"

	"github.com/containers/selinuxd/pkg/semodule/interface"
	"github.com/go-logr/logr"
)

type SEModulePcuHandler struct {
	logger logr.Logger
}

// Ensure that the test handler implements the Handler interface
var _ seiface.Handler = &SEModulePcuHandler{}

func runSemodule(opFlag string, policyArgs ...string) (string, error) {
	fullArgs := []string{opFlag}
	fullArgs = append(fullArgs, policyArgs...)
	cmd := exec.CommandContext(context.TODO(), "/usr/sbin/semodule", fullArgs...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func NewSEModulePcuHandler(logger logr.Logger) (*SEModulePcuHandler, error) {
	return &SEModulePcuHandler{logger: logger}, nil
}

func (smt *SEModulePcuHandler) SetAutoCommit(_ bool) {
	// left to policycoreutils
}

func (smt *SEModulePcuHandler) Install(modulePath string) error {
	out, err := runSemodule("-X", "350", "-i", modulePath)
	if err != nil {
		smt.logger.Error(err, "Installing policy", "modulePath", modulePath, "output", out)
		return seiface.NewErrCannotInstallModule(modulePath)
	}

	smt.logger.Info("Installing policy", "modulePath", modulePath, "output", out)
	return nil
}

func (smt *SEModulePcuHandler) List() ([]seiface.PolicyModule, error) {
	out, err := runSemodule("-lfull", "--checksum")
	if err != nil {
		smt.logger.Error(err, "Listing policies")
		return nil, seiface.ErrList
	}
	modules := make([]seiface.PolicyModule, 0)
	for _, line := range strings.Split(string(out), "\n") {
		module := strings.Fields(line)
		if len(module) != 4 {
			continue
		}
		if module[0] == "350" {
			policyModule := seiface.PolicyModule{module[1], module[2], module[3]}
			modules = append(modules, policyModule)
		}
	}
	return modules, nil
}

func (smt *SEModulePcuHandler) GetPolicyModule(moduleName string) (seiface.PolicyModule, error) {
	modules, err := smt.List()
	if err != nil {
		smt.logger.Error(err, "Getting module checksum")
		return seiface.PolicyModule{}, seiface.ErrList
	}
	for _, module := range modules {
		// 350 module  cil  sha256:dadb16b11a1d298e57cbd965f5fc060b7a9263b8d6b23af7763e68ac22fb5265
		if module.Name == moduleName {
			smt.logger.Info(moduleName, "checksum", module.Checksum)
			return module, nil
		}
	}
	return seiface.PolicyModule{}, seiface.ErrPolicyNotFound
}

func (smt *SEModulePcuHandler) Remove(modToRemove string) error {
	out, err := runSemodule("-X", "350", "-r", modToRemove)
	if err != nil {
		smt.logger.Error(err, "Removing a policy", "modToRemove", modToRemove, "output", out)
		return seiface.NewErrCannotRemoveModule(modToRemove)
	}

	smt.logger.Info("Removing a policy", "output", out)
	return nil
}

func (smt *SEModulePcuHandler) Close() error {
	return nil
}

func (smt *SEModulePcuHandler) Commit() error {
	return nil
}
