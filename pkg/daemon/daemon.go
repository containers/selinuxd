package daemon

import (
	"log"
	"os"
	"path/filepath"

	"github.com/JAORMX/selinuxd/pkg/semodule"
	"github.com/go-logr/logr"
	"gopkg.in/fsnotify.v1"
)

func Daemon(modulePath string, sh semodule.SEModuleHandler, done chan bool, logger logr.Logger) {
	policyops := make(chan policyAction)

	logger.Info("Started daemon")

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	// TODO(jaosorior): Enable multiple watchers
	go watchFiles(watcher, policyops, logger)

	go installPolicies(modulePath, sh, policyops, logger)

	// NOTE(jaosorior): We do this before adding the path to the notification
	// watcher so all the policies are installed already when we start watching
	// for events.
	installPoliciesInDir(modulePath, policyops)

	err = watcher.Add(modulePath)
	if err != nil {
		logger.Error(err, "Could not create an fsnotify watcher")
	}

	<-done
}

func watchFiles(watcher *fsnotify.Watcher, policyops chan policyAction, logger logr.Logger) {
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
				policyops <- NewRemoveAction(event.Name)
			} else if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				fwlog.Info("Installing policy", "file", event.Name)
				policyops <- NewInstallAction(event.Name)
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

func installPolicies(modulePath string, sh semodule.SEModuleHandler, policyops chan policyAction, logger logr.Logger) {
	ilog := logger.WithName("policy-installer")
	for {
		action, ok := <-policyops
		if !ok {
			ilog.Info("WARNING: the actions channel has been closed or is empty")
			return // TODO(jaosorior): Actually signal exit
		}
		if actionOut, err := action.do(modulePath, sh); err != nil {
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

func installPoliciesInDir(mpath string, policyops chan policyAction) {
	filepath.Walk(mpath, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		policyops <- NewInstallAction(path)
		return nil
	})
}
