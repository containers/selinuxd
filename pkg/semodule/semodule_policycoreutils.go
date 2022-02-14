//go:build policycoreutils
// +build policycoreutils

package semodule

import (
	"github.com/containers/selinuxd/pkg/semodule/interface"
	"github.com/containers/selinuxd/pkg/semodule/policycoreutils"
	"github.com/go-logr/logr"
)

func NewSemoduleHandler(_ bool, logger logr.Logger) (seiface.Handler, error) {
	return policycoreutils.NewSEModulePcuHandler(logger)
}
