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
	"syscall"

	"github.com/containers/selinuxd/pkg/daemon"
	"github.com/containers/selinuxd/pkg/datastore"
	"github.com/containers/selinuxd/pkg/semodule"
	seiface "github.com/containers/selinuxd/pkg/semodule/interface"
	"github.com/containers/selinuxd/pkg/version"
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
)

// oneshotCmd represents the oneshot command
var oneshotCmd = &cobra.Command{
	Use:   "oneshot",
	Short: "install SELinux policies in the designated directory",
	Long:  `This does a one-shot installation of SELinux policies.`,
	Run:   oneshotCmdFunc,
}

//nolint:gochecknoinits
func init() {
	rootCmd.AddCommand(oneshotCmd)
	defineOneShotFlags(oneshotCmd)
}

func defineOneShotFlags(rootCmd *cobra.Command) {
	rootCmd.Flags().String("datastore-path", datastore.DefaultDataStorePath, "The path to the policy data store")
}

func parseOneShotFlags(rootCmd *cobra.Command) (*daemon.SelinuxdOptions, error) {
	var config daemon.SelinuxdOptions
	var err error

	config.StatusDBPath, err = rootCmd.Flags().GetString("datastore-path")
	if err != nil {
		return nil, fmt.Errorf("failed getting datastore-path flag: %w", err)
	}

	return &config, nil
}

func tryInstallAllPolicies(sh seiface.Handler, ds datastore.DataStore, logger logr.Logger) {
	policyops := make(chan daemon.PolicyAction)

	go func() {
		if err := daemon.InstallPoliciesInDir(defaultModulePath, policyops, nil); err != nil {
			logger.Error(err, "Installing policies in module directory")
		}
		close(policyops)
	}()

	daemon.InstallPolicies(defaultModulePath, sh, ds, policyops, logger)
}

func oneshotCmdFunc(rootCmd *cobra.Command, _ []string) {
	logger, err := getLogger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err)
		syscall.Exit(1)
	}

	version.PrintInfoPermissive(logger)

	opts, err := parseOneShotFlags(rootCmd)
	if err != nil {
		logger.Error(err, "Parsing flags")
		syscall.Exit(1)
	}

	sh, err := semodule.NewSemoduleHandler(false, logger)
	if err != nil {
		logger.Error(err, "Creating semodule handler")
	}
	defer sh.Close()

	ds, err := datastore.New(opts.StatusDBPath)
	if err != nil {
		logger.Error(err, "Unable to get R/W datastore")
	}
	defer ds.Close()

	logger.Info("Running oneshot command")

	tryInstallAllPolicies(sh, ds, logger)

	if err := sh.Commit(); err != nil {
		logger.Info("Unable to install policies in one commit. " +
			"This is most likely due to a policy being wrongly formatted. " +
			"Will attempt to install each policy individually.")
		// Do longer policy-per-policy install
		sh.SetAutoCommit(true)
		tryInstallAllPolicies(sh, ds, logger)
	}

	logger.Info("Done installing policies in directory")
}
