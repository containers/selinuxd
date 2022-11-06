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
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syscall"

	"github.com/containers/selinuxd/pkg/daemon"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status [policy]",
	Args:  cobra.RangeArgs(0, 1),
	Short: "Get the status of a policy",
	Long:  `This gets the status of the given policy.`,
	Run:   statusCmdFunc,
}

//nolint:gochecknoinits
func init() {
	rootCmd.AddCommand(statusCmd)
	defineStatusFlags(statusCmd)
}

func defineStatusFlags(rootCmd *cobra.Command) {
	rootCmd.Flags().String("socket-path", daemon.DefaultUnixSockAddr, "the path where the selinuxd socket is listening at")
}

func parseStatusFlags(rootCmd *cobra.Command) (*daemon.SelinuxdOptions, error) {
	var config daemon.SelinuxdOptions
	var err error

	config.Path, err = rootCmd.Flags().GetString("socket-path")
	if err != nil {
		return nil, fmt.Errorf("failed getting socket-path flag: %w", err)
	}

	return &config, nil
}

func statusCmdFunc(rootCmd *cobra.Command, args []string) {
	opts, err := parseStatusFlags(rootCmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Parsing flags: %s", err)
		syscall.Exit(1)
	}

	httpc := getHTTPClient(opts.Path)

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	policyurl := baseStatusServerURL + "/policies/"
	if len(args) == 1 {
		policyurl += url.QueryEscape(args[0])
	}

	req, err := http.NewRequestWithContext(ctx, "GET", policyurl, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Forming policy query: %s", err)
		syscall.Exit(1)
	}

	response, err := httpc.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Querying policy status: %s", err)
		syscall.Exit(1)
	}
	defer response.Body.Close()

	table := tablewriter.NewWriter(os.Stdout)

	// multiple status
	if len(args) == 0 {
		handleList(table, response)
	} else {
		handleSinglePolicy(table, response)
	}

	table.Render()
}

func handleList(table *tablewriter.Table, response *http.Response) {
	table.SetHeader([]string{"Name"})
	var moduleList []string
	err := json.NewDecoder(response.Body).Decode(&moduleList)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Decoding policy status list response: %s", err)
		syscall.Exit(1)
	}

	for _, mod := range moduleList {
		table.Append([]string{mod})
	}
}

func handleSinglePolicy(table *tablewriter.Table, response *http.Response) {
	table.SetHeader([]string{"Key", "Value"})
	if response.StatusCode == http.StatusOK {
		var moduleStatus map[string]string
		err := json.NewDecoder(response.Body).Decode(&moduleStatus)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Decoding policy status response: %s", err)
			syscall.Exit(1)
		}

		for key, value := range moduleStatus {
			table.Append([]string{key, value})
		}
		return
	}

	buf := new(strings.Builder)
	_, err := io.Copy(buf, response.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Decoding policy status error response: %s", err)
		syscall.Exit(1)
	}
	table.Append([]string{"error", strings.Trim(buf.String(), "\n")})
}
