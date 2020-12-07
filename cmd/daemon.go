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
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/JAORMX/selinuxd/pkg/daemon"
	"github.com/JAORMX/selinuxd/pkg/semodule/semanage"
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

func getLogger() logr.Logger {
	logger, _ := zap.NewProduction()
	defer logger.Sync() // flushes buffer, if any
	return zapr.NewLogger(logger)
}

func daemonCmdFunc(cmd *cobra.Command, args []string) {
	const modulePath = "/etc/selinux.d"

	logger := getLogger()
	exitSignal := make(chan os.Signal, 1)
	done := make(chan bool)
	signal.Notify(exitSignal, syscall.SIGINT, syscall.SIGTERM)

	sh, err := semanage.NewSemanageHandler(logger)
	if err != nil {
		log.Fatal(err)
	}
	defer sh.Close()

	go daemon.Daemon(modulePath, sh, done, logger)

	<-exitSignal
	logger.Info("Exit signal received")
	done <- true
}
