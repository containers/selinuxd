//go:build !policycoreutils && !semanage
// +build !policycoreutils,!semanage

package semodule

import (
	"errors"

	seiface "github.com/containers/selinuxd/pkg/semodule/interface"
	"github.com/go-logr/logr"
)

// ErrNoSemodule is an error when no usable semodule back end is selected
var ErrNoSemodule = errors.New("no semodule back end built")

func NewSemoduleHandler(_ bool, logger logr.Logger) (seiface.Handler, error) {
	return nil, ErrNoSemodule
}
