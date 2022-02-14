package seiface

import (
	"errors"
	"fmt"
)

// errors
var (
	// ErrHandleCreate is an error when getting a handle to semanage
	ErrHandleCreate = errors.New("could not create handle")
	// ErrSELinuxDBConnect is an error to connect to the SELinux database
	ErrSELinuxDBConnect = errors.New("could not connect to the SELinux DB")
	// ErrNilHandle would happen if you initialized the Handler without
	// the using the `NewSemanageHandler` function or without initializing
	// the underlying semanage handler
	ErrNilHandle = errors.New("nil semanage handle")
	// ErrList is an error listing the SELinux modules
	ErrList = errors.New("cannot list")
	// ErrCannotRemoveModule is an error removing a SELinux module
	ErrCannotRemoveModule = errors.New("cannot remove module")
	// ErrCannotInstallModule is an error installing a SELinux module
	ErrCannotInstallModule = errors.New("cannot install module")
	// ErrCommit is an error when committing the changes to the SELinux policy
	ErrCommit = errors.New("cannot commit changes to policy")
)

func NewErrCannotRemoveModule(mName string) error {
	return fmt.Errorf("%w: %s", ErrCannotRemoveModule, mName)
}

func NewErrCannotInstallModule(mName string) error {
	return fmt.Errorf("%w: %s", ErrCannotInstallModule, mName)
}

func NewErrCommit(origErrVal int, msg string) error {
	return fmt.Errorf("%w - error code: %d. message: %s", ErrCommit, origErrVal, msg)
}

// Handler implements an interface to interact
// with SELinux modules.
type Handler interface {
	SetAutoCommit(bool)
	Install(string) error
	List() ([]string, error)
	Remove(string) error
	Commit() error
	Close() error
}
