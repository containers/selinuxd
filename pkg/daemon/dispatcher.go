package daemon

import (
	"os"

	"gopkg.in/fsnotify.v1"
)

type fileOperationDispatch uint8

const (
	dispatchFileAddition fileOperationDispatch = iota
	dispatchDirectoryAddition
	dispatchRemoval
	dispatchSymlink
	dispatchUnkown
)

func dispatch(e fsnotify.Event) fileOperationDispatch {
	// Since the file was removed, we can't stat
	// the file or directory, so we have a generic removal
	// dispatcher
	if e.Op&fsnotify.Remove != 0 {
		return dispatchRemoval
	}

	finfo, err := os.Stat(e.Name)
	if err != nil {
		return dispatchUnkown
	}

	if finfo.Mode()&os.ModeSymlink == os.ModeSymlink {
		return dispatchSymlink
	}

	if e.Op&(fsnotify.Write|fsnotify.Create) != 0 {
		if finfo.IsDir() {
			return dispatchDirectoryAddition
		}
		return dispatchFileAddition
	}
	return dispatchUnkown
}
