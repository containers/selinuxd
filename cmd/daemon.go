/*
Copyright © 2020 Red Hat, Inc.

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
	"os"
	"os/signal"
	"syscall"

	"github.com/containers/selinuxd/pkg/daemon"
	"github.com/containers/selinuxd/pkg/datastore"
	"github.com/containers/selinuxd/pkg/semodule"
	"github.com/containers/selinuxd/pkg/version"
	"github.com/spf13/cobra"
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

//nolint:gochecknoinits
func init() {
	rootCmd.AddCommand(daemonCmd)
	defineFlags(daemonCmd)
}

func defineFlags(rootCmd *cobra.Command) {
	rootCmd.Flags().String("socket-path", daemon.DefaultUnixSockAddr, "The path to the socket to listen at")
	rootCmd.Flags().Int("socket-uid", 0, "The user owner of the status HTTP socket")
	rootCmd.Flags().Int("socket-gid", 0, "The group owner of the status HTTP socket")
	rootCmd.Flags().String("datastore-path", datastore.DefaultDataStorePath, "The path to the policy data store")
	rootCmd.Flags().Bool("enable-profiling", false, "whether to enable or not profiling endpoints in the status server.")
}

func parseFlags(rootCmd *cobra.Command) (*daemon.SelinuxdOptions, error) {
	var config daemon.SelinuxdOptions
	var err error

	config.UID, err = rootCmd.Flags().GetInt("socket-uid")
	if err != nil {
		return nil, fmt.Errorf("failed getting socket-uid flag: %w", err)
	}

	config.GID, err = rootCmd.Flags().GetInt("socket-gid")
	if err != nil {
		return nil, fmt.Errorf("failed getting socket-gid flag: %w", err)
	}

	config.Path, err = rootCmd.Flags().GetString("socket-path")
	if err != nil {
		return nil, fmt.Errorf("failed getting socket-path flag: %w", err)
	}

	config.StatusDBPath, err = rootCmd.Flags().GetString("datastore-path")
	if err != nil {
		return nil, fmt.Errorf("failed getting datastore-path flag: %w", err)
	}

	config.EnableProfiling, err = rootCmd.Flags().GetBool("enable-profiling")
	if err != nil {
		return nil, fmt.Errorf("failed getting enable-profiling flag: %w", err)
	}

	return &config, nil
}

func daemonCmdFunc(rootCmd *cobra.Command, _ []string) {
	logger, err := getLogger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err)
		syscall.Exit(1)
	}

	options, err := parseFlags(rootCmd)
	if err != nil {
		logger.Error(err, "Parsing flags")
		syscall.Exit(1)
	}

	version.PrintInfoPermissive(logger)

	exitSignal := make(chan os.Signal, 1)
	done := make(chan bool)
	signal.Notify(exitSignal, syscall.SIGINT, syscall.SIGTERM)

	sh, err := semodule.NewSemoduleHandler(true, logger)
	if err != nil {
		logger.Error(err, "Creating semodule handler")
	}
	defer sh.Close()

	go daemon.Daemon(options, defaultModulePath, sh, nil, done, logger)

	<-exitSignal
	logger.Info("Exit signal received")
	done <- true
}
