package semanage

/*
#cgo CFLAGS: -I/usr/include/semanage
#cgo LDFLAGS: -L/usr/lib64 -lsemanage -lsepol
#include <semanage.h>
#include <stdlib.h>

void wrap_set_cb(semanage_handle_t *handle, void *arg);


*/
import "C"
import (
	"errors"
	"fmt"
	"unsafe"

	"github.com/go-logr/logr"

	"github.com/JAORMX/selinuxd/pkg/semodule"
)

var globLogger logr.Logger

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
	// ErrCommit is an error when commiting the changes to the SELinux policy
	ErrCommit = errors.New("cannot commit changes to policy")
)

func NewErrCannotRemoveModule(mName string) error {
	return fmt.Errorf("%w: %s", ErrCannotRemoveModule, mName)
}

func NewErrCannotInstallModule(mName string) error {
	return fmt.Errorf("%w: %s", ErrCannotInstallModule, mName)
}

func NewErrCommit(origErrVal int) error {
	return fmt.Errorf("%w - error code: %d", ErrCommit, origErrVal)
}

//export LogWrapper
func LogWrapper(cmsg *C.char, level C.int) {
	msg := C.GoString(cmsg)

	// swtich on the level and do err/fail/info
	globLogger.Info(msg)
}

type SeHandler struct {
	handle *C.semanage_handle_t
}

// NewSemanageHandler creates a new instance of a semodule.Handler that
// handles SELinux module interactions through the semanage interface
func NewSemanageHandler(logger logr.Logger) (semodule.Handler, error) {
	globLogger = logger
	handle := C.semanage_handle_create()
	if handle == nil {
		return nil, ErrHandleCreate
	}

	C.wrap_set_cb(handle, nil)

	rv := C.semanage_connect(handle)
	if rv < 0 {
		return nil, ErrSELinuxDBConnect
	}

	return &SeHandler{
		handle,
	}, nil
}

func (sm *SeHandler) getNthModName(n int, modInfoList *C.semanage_module_info_t) (module string, cleanup func()) {
	var modInfo *C.semanage_module_info_t
	free := func() {}

	modInfo = C.semanage_module_list_nth(modInfoList, C.int(n))
	if modInfo == nil {
		return "", free
	}

	free = func() {
		C.semanage_module_info_destroy(sm.handle, modInfo)
	}

	// no free seems to be required, this returns a const char
	cName := C.semanage_module_get_name(modInfo)
	if cName == nil {
		return "", free
	}

	return C.GoString(cName), free
}

func (sm *SeHandler) List() ([]string, error) {
	var modInfoList *C.semanage_module_info_t
	var cNmod C.int

	if sm.handle == nil {
		return nil, ErrNilHandle
	}

	// NOTE(jaosorior): I actually don't understand the warning
	// gocritic is issuing here...
	// nolint:gocritic
	rv := C.semanage_module_list(sm.handle, &modInfoList, &cNmod)
	if rv < 0 {
		return nil, ErrList
	}
	defer C.free(unsafe.Pointer(modInfoList))

	nmod := int(cNmod)
	modNames := make([]string, 0)

	for n := 0; n < nmod; n++ {
		name, freeModInfo := sm.getNthModName(n, modInfoList)
		defer freeModInfo()
		if name == "" {
			continue
		}
		modNames = append(modNames, name)
	}

	return modNames, nil
}

func (sm *SeHandler) Remove(moduleName string) error {
	if sm.handle == nil {
		return ErrNilHandle
	}

	cModName := C.CString(moduleName)
	defer C.free(unsafe.Pointer(cModName))

	rv := C.semanage_module_remove(sm.handle, cModName)
	if rv < 0 {
		return NewErrCannotRemoveModule(moduleName)
	}

	return sm.commit()
}

func (sm *SeHandler) Install(moduleFile string) error {
	if sm.handle == nil {
		return ErrNilHandle
	}

	cModFile := C.CString(moduleFile)
	defer C.free(unsafe.Pointer(cModFile))

	rv := C.semanage_module_install_file(sm.handle, cModFile)
	if rv < 0 {
		return NewErrCannotInstallModule(moduleFile)
	}

	return sm.commit()
}

func (sm *SeHandler) commit() error {
	if sm.handle == nil {
		return ErrNilHandle
	}

	rv := C.semanage_commit(sm.handle)
	if rv < 0 {
		return NewErrCommit(int(rv))
	}

	return nil
}

// Close disconnects the Semanage handler's connection.
// It implements the Closer interface [1]
//
// [1] https://golang.org/pkg/io/#Closer
func (sm *SeHandler) Close() error {
	if sm.handle == nil {
		// semanage uses asserts and just crashes when the pointer is NULL
		return nil
	}

	rv := C.semanage_is_connected(sm.handle)
	if rv == 1 {
		C.semanage_disconnect(sm.handle)
	}

	C.semanage_handle_destroy(sm.handle)
	sm.handle = nil
	return nil
}
