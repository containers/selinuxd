package daemon

import (
	"os"
	"path/filepath"

	"github.com/JAORMX/selinuxd/pkg/datastore"
	"github.com/JAORMX/selinuxd/pkg/semodule"
	"github.com/go-logr/logr"
	"gopkg.in/fsnotify.v1"
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
func Daemon(opts *SelinuxdOptions, mPath string, sh semodule.Handler, ds datastore.DataStore, done chan bool,
	l logr.Logger) {
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
	if err := InstallPoliciesInDir(mPath, policyops); err != nil {
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
			if event.Op&fsnotify.Remove != 0 {
				fwlog.Info("Removing policy", "file", event.Name)
				policyops <- newRemoveAction(event.Name)
			} else if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				fwlog.Info("Installing policy", "file", event.Name)
				policyops <- newInstallAction(event.Name)
			}
			// TODO(jaosorior): handle rename
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
// nolint: lll
func InstallPolicies(modulePath string, sh semodule.Handler, ds datastore.DataStore, policyops chan PolicyAction, logger logr.Logger) {
	ilog := logger.WithName("policy-installer")
	for {
		action, ok := <-policyops
		if !ok {
			ilog.Info("The policy operations channel is now closed")
			return // TODO(jaosorior): Actually signal exit
		}
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
}

func InstallPoliciesInDir(mpath string, policyops chan PolicyAction) error {
	return filepath.Walk(mpath, func(path string, info os.FileInfo, err error) error {
		if info == nil || info.IsDir() {
			return nil
		}
		policyops <- newInstallAction(path)
		return nil
	})
}
