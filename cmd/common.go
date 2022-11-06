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
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
)

const (
	defaultModulePath   = "/etc/selinux.d"
	defaultTimeout      = 10 * time.Second
	baseStatusServerURL = "http://unix"
)

func getLogger() (logr.Logger, error) {
	logger, err := zap.NewProduction()
	if err != nil {
		return logr.Logger{}, fmt.Errorf("error creating logger: %w", err)
	}
	logIf := zapr.NewLogger(logger)
	// NOTE(jaosorior): While this may return errors, they're mostly
	// harmless and handling them is more work than its worth
	//nolint:errcheck
	defer logger.Sync() // flushes buffer, if any
	return logIf, nil
}

func getHTTPClient(sockpath string) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				conn, err := net.Dial("unix", sockpath)
				if err != nil {
					return nil, fmt.Errorf("error dialing unix socket: %w", err)
				}
				return conn, nil
			},
		},
	}
}
