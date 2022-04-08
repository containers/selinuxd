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
	fullArgs := []string{"-v", opFlag}
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
		smt.logger.Error(err, "Installing policy", "modulePath", modulePath)
		return seiface.NewErrCannotInstallModule(modulePath)
	}

	smt.logger.Info("Installing policy", "modulePath", modulePath, "out", out)
	return nil
}

func (smt *SEModulePcuHandler) List() ([]string, error) {
	out, err := runSemodule("-lfull")
	if err != nil {
		smt.logger.Error(err, "Listing policies")
		return nil, seiface.ErrList
	}
	modules := make([]string, 0)
	for _, line := range strings.Split(string(out), "\n") {
		module := strings.Split(line, " ")
		if module[0] == "350" {
			modules = append(modules, module[1])
		}
	}
	return modules, nil
}

func (smt *SEModulePcuHandler) Remove(modToRemove string) error {
	out, err := runSemodule("-X", "350", "-r", modToRemove)
	if err != nil {
		smt.logger.Error(err, "Removing a policy", "modToRemove", modToRemove)
		return seiface.NewErrCannotRemoveModule(modToRemove)
	}

	smt.logger.Info("Removing a policy", "out", out)
	return nil
}

func (smt *SEModulePcuHandler) Close() error {
	return nil
}

func (smt *SEModulePcuHandler) Commit() error {
	return nil
}
