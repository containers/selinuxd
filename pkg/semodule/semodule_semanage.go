//go:build semanage
// +build semanage

package semodule

import (
	"github.com/containers/selinuxd/pkg/semodule/interface"
	"github.com/containers/selinuxd/pkg/semodule/semanage"
	"github.com/go-logr/logr"
)

func NewSemoduleHandler(autoCommit bool, logger logr.Logger) (seiface.Handler, error) {
	return semanage.NewSemanageHandler(autoCommit, logger)
}
