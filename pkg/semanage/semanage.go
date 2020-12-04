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
)

var globLogger logr.Logger

//export LogWrapper
func LogWrapper(cmsg *C.char, level C.int) {
	msg := C.GoString(cmsg)

	// swtich on the level and do err/fail/info
	globLogger.Info(msg)
}

type SeHandler struct {
	handle *C.semanage_handle_t
}

func SmInit(logger logr.Logger) (*SeHandler, error) {
	globLogger = logger
	handle := C.semanage_handle_create()
	if handle == nil {
		return nil, errors.New("could not create handle")
	}

	C.wrap_set_cb(handle, nil)

	rv := C.semanage_connect(handle)
	if rv < 0 {
		return nil, errors.New("could not connect to the SELinux DB")
	}

	return &SeHandler{
		handle,
	}, nil
}

func (sm *SeHandler) getNthModName(n int, modInfoList *C.semanage_module_info_t) (string, func()) {
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

func (sm *SeHandler) SmList() ([]string, error) {
	var modInfoList *C.semanage_module_info_t
	var cNmod C.int

	if sm.handle == nil {
		return nil, errors.New("nil handle")
	}

	rv := C.semanage_module_list(sm.handle, &modInfoList, &cNmod)
	if rv < 0 {
		return nil, errors.New("cannot list")
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

func (sm *SeHandler) SmRemove(moduleName string) error {
	if sm.handle == nil {
		return errors.New("nil handle")
	}

	cModName := C.CString(moduleName)
	defer C.free(unsafe.Pointer(cModName))

	rv := C.semanage_module_remove(sm.handle, cModName)
	if rv < 0 {
		return fmt.Errorf("cannot remove module %s", moduleName)
	}

	return sm.commit()
}

func (sm *SeHandler) SmInstallFile(moduleFile string) error {
	if sm.handle == nil {
		return errors.New("nil handle")
	}

	cModFile := C.CString(moduleFile)
	defer C.free(unsafe.Pointer(cModFile))

	rv := C.semanage_module_install_file(sm.handle, cModFile)
	if rv < 0 {
		return fmt.Errorf("cannot install module %s", moduleFile)
	}

	return sm.commit()
}

func (sm *SeHandler) commit() error {
	if sm.handle == nil {
		return errors.New("nil handle")
	}

	rv := C.semanage_commit(sm.handle)
	if rv < 0 {
		return fmt.Errorf("Couldn't commit changes to policy. Error: %d", rv)
	}

	return nil
}

func (sm *SeHandler) SmDone() {
	if sm.handle == nil {
		// semanage uses asserts and just crashes when the pointer is NULL
		return
	}

	rv := C.semanage_is_connected(sm.handle)
	if rv == 1 {
		C.semanage_disconnect(sm.handle)
	}

	C.semanage_handle_destroy(sm.handle)
	sm.handle = nil
}
