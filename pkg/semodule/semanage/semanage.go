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
	"bytes"
	"errors"
	"fmt"
	"sync"
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

func NewErrCommit(origErrVal int, msg string) error {
	return fmt.Errorf("%w - error code: %d. message: %s", ErrCommit, origErrVal, msg)
}

type globalErrorFlusher struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

// This is not ideal... but we need this since it's the only way to get
// the error messages from libsemanage. If libsemanage had a way to specify
// an error handler per call, this would not be needed.
var errflush *globalErrorFlusher = &globalErrorFlusher{}

func (f *globalErrorFlusher) write(msg string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.buf.WriteString(msg)
}

func (f *globalErrorFlusher) flush() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	defer f.buf.Reset()
	return f.buf.String()
}

//export LogWrapper
func LogWrapper(cmsg *C.char, level C.int) {
	msg := C.GoString(cmsg)

	// swtich on the level and do err/fail/info
	globLogger.Info(msg)
	errflush.write(msg)
}

type SeHandler struct {
	handle     *C.semanage_handle_t
	autoCommit bool
}

// NewSemanageHandler creates a new instance of a semodule.Handler that
// handles SELinux module interactions through the semanage interface
//
// `autoCommit` tells the handler to always issue a commit when
// installing/removing policies. If this is set to `off` You would
// need to commit explicitly.
func NewSemanageHandler(autoCommit bool, logger logr.Logger) (semodule.Handler, error) {
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
		autoCommit,
	}, nil
}

// SetAutoCommit set's the `autoCommit` property in the handler
func (sm *SeHandler) SetAutoCommit(autoCommit bool) {
	sm.autoCommit = autoCommit
}

func (sm *SeHandler) getNthModName(n int, modInfoList *C.semanage_module_info_t) string {
	modInfo := C.semanage_module_list_nth(modInfoList, C.int(n))
	if modInfo == nil {
		return ""
	}
	defer C.semanage_module_info_destroy(sm.handle, modInfo)

	// no free seems to be required, this returns a const char
	cName := C.semanage_module_get_name(modInfo)
	if cName == nil {
		return ""
	}

	return C.GoString(cName)
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
		name := sm.getNthModName(n, modInfoList)
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

	if sm.autoCommit {
		return sm.Commit()
	}
	return nil
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

	if sm.autoCommit {
		return sm.Commit()
	}
	return nil
}

func (sm *SeHandler) Commit() error {
	if sm.handle == nil {
		return ErrNilHandle
	}

	rv := C.semanage_commit(sm.handle)
	// This ensures that we always flush after commit
	msg := errflush.flush()
	if rv < 0 {
		return NewErrCommit(int(rv), msg)
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
