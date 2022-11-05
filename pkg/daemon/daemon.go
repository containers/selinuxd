package daemon

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/containers/selinuxd/pkg/datastore"
	seiface "github.com/containers/selinuxd/pkg/semodule/interface"
	"github.com/fsnotify/fsnotify"
	"github.com/go-logr/logr"
)

type SelinuxdOptions struct {
	StatusServerConfig
	StatusDBPath string
}

// Daemon takes the following parameters:
// * `opts`: are the options to run status server.
// * `mPath`: is the path to install and read modules from.
// * `sh`: is the SELinux module handler interface.
// * `ds`: is the DataStore interface.
// * `l`: is a logger interface.
func Daemon(opts *SelinuxdOptions, mPath string, sh seiface.Handler, ds datastore.DataStore, done chan bool,
	l logr.Logger,
) {
	policyops := make(chan PolicyAction)
	readychan := make(chan bool)

	l.Info("Started daemon")
	if ds == nil {
		var err error
		ds, err = datastore.New(opts.StatusDBPath)
		if err != nil {
			l.Error(err, "Unable to get R/W datastore")
			panic(err)
		}
		defer ds.Close()
	}

	ss, err := initStatusServer(opts.StatusServerConfig, ds.GetReadOnly(), l)
	if err != nil {
		l.Error(err, "Unable initialize status server")
		panic(err)
	}

	go serveState(ss, readychan, l)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		l.Error(err, "Unable to get fsnotify watcher")
		panic(err)
	}
	defer watcher.Close()

	// TODO(jaosorior): Enable multiple watchers
	go watchFiles(watcher, policyops, l)

	go InstallPolicies(mPath, sh, ds, policyops, l)

	// NOTE(jaosorior): We do this before adding the path to the notification
	// watcher so all the policies are installed already when we start watching
	// for events.
	if err := InstallPoliciesInDir(mPath, policyops, watcher); err != nil {
		l.Error(err, "Installing policies in module directory")
	}

	err = watcher.Add(mPath)
	if err != nil {
		l.Error(err, "Could not create an fsnotify watcher")
	}

	readychan <- true
	close(readychan)

	<-done
}

func watchFiles(watcher *fsnotify.Watcher, policyops chan PolicyAction, logger logr.Logger) {
	fwlog := logger.WithName("file-watcher")
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				fwlog.Info("WARNING: the fsnotify channel has been closed or is empty")
				return // TODO(jaosorior): Actually signal exit
			}
			switch dispatch(event) {
			case dispatchRemoval:
				fwlog.Info("Removing policy", "file", event.Name)
				policyops <- newRemoveAction(event.Name)
			case dispatchFileAddition:
				fwlog.Info("Installing policy", "file", event.Name)
				policyops <- newInstallAction(event.Name)
			case dispatchDirectoryAddition:
				fwlog.Info("Tracking sub-directory", "directory", event.Name)
				if addErr := watcher.Add(event.Name); addErr != nil {
					fwlog.Error(addErr, "Unable to watch sub-directory")
				}
				fwlog.Info("Installing policies in sub-directory", "directory", event.Name)
				if instErr := InstallPoliciesInDir(event.Name, policyops, watcher); instErr != nil {
					fwlog.Error(instErr, "Error installing policies in sub-directory")
				}
			case dispatchSymlink:
				fwlog.Info("Ignoring symlink", "symlink", event.Name)
			case dispatchUnkown:
				fwlog.Info("Ignoring file due to unknown state", "file", event.Name)
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				fwlog.Info("WARNING: the fsnotify channel has been closed or is empty")
				return // TODO(jaosorior): Actually signal exit
			}
			fwlog.Error(err, "Error watching for event")
		}
	}
}

// InstallPolicies installs the policies found in the `modulePath` directory
func InstallPolicies(modulePath string, sh seiface.Handler, ds datastore.DataStore, policyops chan PolicyAction, logger logr.Logger) { //nolint:lll
	ilog := logger.WithName("policy-installer")
	for action := range policyops {
		if actionOut, err := action.do(modulePath, sh, ds); err != nil {
			ilog.Error(err, "Failed applying operation on policy", "operation", action, "output", actionOut)
		} else {
			// TODO(jaosorior): Replace this log with proper tracking of the installation status
			if actionOut == "" {
				actionOut = "The operation was successful"
			}
			ilog.Info(actionOut, "operation", action)
		}
	}
	ilog.Info("The policy operations channel is now closed")
}

func InstallPoliciesInDir(mpath string, policyops chan PolicyAction, watcher *fsnotify.Watcher) error {
	err := filepath.Walk(mpath, func(path string, info os.FileInfo, err error) error {
		if info == nil {
			return nil
		}
		if watcher != nil && info.IsDir() {
			err := watcher.Add(path)
			if err != nil {
				return fmt.Errorf("unable to watch directory %s: %w", path, err)
			}
			return nil
		} else if info.IsDir() {
			// ignore directories
			return nil
		}

		policyops <- newInstallAction(path)
		return nil
	})
	if err != nil {
		return fmt.Errorf("unable to walk module directory: %w", err)
	}
	return nil
}
