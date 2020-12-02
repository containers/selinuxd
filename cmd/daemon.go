/*
Copyright © 2020 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"gopkg.in/fsnotify.v1"
)

// daemonCmd represents the daemon command
var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Start the selinuxd daemon",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: daemonCmdFunc,
}

func init() {
	rootCmd.AddCommand(daemonCmd)
}

// Defines an action to be taken on a policy file on the specified path
type policyAction struct {
	path      string
	operation policyOp
}

// defines the operation that an action will take on the file
type policyOp int16

const (
	install policyOp = iota
	remove  policyOp = iota
)

func (po policyOp) String() string {
	switch po {
	case install:
		return "install"
	case remove:
		return "remove"
	default:
		return "unknown"
	}
}

const modulePath = "/etc/selinux.d"

func (pa policyAction) do() (string, error) {
	var opFlag string
	var policyArg string
	switch pa.operation {
	case install:
		opFlag = "-i"
		policyPath, err := getSafePath(pa.path)
		if err != nil {
			return "", err
		}
		policyArg = policyPath
	case remove:
		opFlag = "-r"
		baseFile, err := getCleanBase(pa.path)
		if err != nil {
			return "", err
		}
		policyArg = getFileWithoutExtension(baseFile)

		if !pa.moduleInstalled(policyArg) {
			return "No action needed; Module is not in the system", nil
		}
	default:
		return "", fmt.Errorf("Unkown operation for policy %s. This shouldn't happen", pa.path)
	}

	// TODO(jaosorior): Replace commands to semodule with an actual
	// library call to semanage
	cmd := exec.Command("/usr/sbin/semodule", opFlag, policyArg)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (pa policyAction) moduleInstalled(policy string) bool {
	// TODO(jaosorior): Replace commands to semodule with an actual
	// library call to semanage
	grep := exec.Command("/usr/bin/grep", policy)
	ps := exec.Command("/usr/sbin/semodule", "-l")
	pipe, _ := ps.StdoutPipe()
	grep.Stdin = pipe
	defer pipe.Close()
	ps.Start()
	// Run and get the output of grep.
	res, _ := grep.Output()
	// We have this policy installed if it appeared in the grep's output
	return strings.Trim(string(res), "\n") != ""
}

func getFileWithoutExtension(filename string) string {
	var extension = filepath.Ext(filename)
	return filename[0 : len(filename)-len(extension)]
}

func getCleanBase(path string) (string, error) {
	// NOTE: don't trust the path even if it came from fsnotify
	cleanPath := filepath.Clean(path)
	if cleanPath == "" {
		return "", fmt.Errorf("Invalid path: %s", path)
	}

	// NOTE: Still not trusting that path. Let's just use the base
	// and use our configured base path
	return filepath.Base(cleanPath), nil
}

func getSafePath(path string) (string, error) {
	policyFileBase, err := getCleanBase(path)
	if err != nil {
		return "", err
	}
	policyPath := filepath.Join(modulePath, policyFileBase)
	return policyPath, nil
}

func getLogger() logr.Logger {
	logger, _ := zap.NewProduction()
	defer logger.Sync() // flushes buffer, if any
	return zapr.NewLogger(logger)
}

func daemonCmdFunc(cmd *cobra.Command, args []string) {
	logger := getLogger()
	exitSignal := make(chan os.Signal, 1)
	done := make(chan bool)
	signal.Notify(exitSignal, syscall.SIGINT, syscall.SIGTERM)

	go Daemon(done, logger)

	<-exitSignal
	logger.Info("Exit signal received")
	done <- true
}

func Daemon(done chan bool, logger logr.Logger) {
	policyops := make(chan policyAction)

	logger.Info("Started daemon")

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	// TODO(jaosorior): Enable multiple watchers
	go watchFiles(watcher, policyops, logger)

	go installPolicies(policyops, logger)

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
				policyops <- policyAction{path: event.Name, operation: remove}
			} else if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				fwlog.Info("Installing policy", "file", event.Name)
				policyops <- policyAction{path: event.Name, operation: install}
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

func installPolicies(policyops chan policyAction, logger logr.Logger) {
	ilog := logger.WithName("policy-installer")
	for {
		action, ok := <-policyops
		if !ok {
			ilog.Info("WARNING: the actions channel has been closed or is empty")
			return // TODO(jaosorior): Actually signal exit
		}
		if actionOut, err := action.do(); err != nil {
			ilog.Error(err, "Failed applying operation on policy", "operation", action.operation, "policy", action.path, "output", actionOut)
		} else {
			// TODO(jaosorior): Replace this log with proper tracking of the installation status
			if actionOut == "" {
				actionOut = "The operation was successful"
			}
			logger.Info(string(actionOut))
		}
	}
}
