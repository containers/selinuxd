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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"syscall"

	"github.com/containers/selinuxd/pkg/daemon"
	"github.com/spf13/cobra"
)

// isreadyCmd represents the is-ready command
var isreadyCmd = &cobra.Command{
	Use:   "is-ready [policy]",
	Short: "probe ready endpoint",
	Long:  `Tells whether selinuxd is ready or not.`,
	Run:   isreadyCmdFunc,
}

//nolint:gochecknoinits
func init() {
	rootCmd.AddCommand(isreadyCmd)
	defineIsReadyFlags(isreadyCmd)
}

func defineIsReadyFlags(rootCmd *cobra.Command) {
	rootCmd.Flags().String("socket-path", daemon.DefaultUnixSockAddr, "the path where the selinuxd socket is listening at")
}

func parseIsReadyFlags(rootCmd *cobra.Command) (*daemon.SelinuxdOptions, error) {
	var config daemon.SelinuxdOptions
	var err error

	config.Path, err = rootCmd.Flags().GetString("socket-path")
	if err != nil {
		return nil, fmt.Errorf("failed getting socket-path flag: %w", err)
	}

	return &config, nil
}

func isreadyCmdFunc(rootCmd *cobra.Command, args []string) {
	opts, err := parseIsReadyFlags(rootCmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Parsing flags: %s", err)
		syscall.Exit(1)
	}

	httpc := getHTTPClient(opts.Path)

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	readyurl := baseStatusServerURL + "/ready/"

	req, err := http.NewRequestWithContext(ctx, "GET", readyurl, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Forming ready query: %s", err)
		syscall.Exit(1)
	}

	response, err := httpc.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Querying ready endpoint: %s", err)
		syscall.Exit(1)
	}
	defer response.Body.Close()

	var status map[string]bool
	err = json.NewDecoder(response.Body).Decode(&status)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Decoding ready endpoint response: %s", err)
		syscall.Exit(1)
	}

	if status["ready"] {
		fmt.Fprint(os.Stdout, "yes")
	} else {
		fmt.Fprint(os.Stdout, "no")
	}
}
