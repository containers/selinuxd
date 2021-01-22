package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/JAORMX/selinuxd/pkg/semodule/test"
	backoff "github.com/cenkalti/backoff/v4"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
)

const (
	defaultPollBackOff = 1 * time.Second
	defaultTimeout     = 10 * time.Second
)

var (
	errModuleNotInstalled = fmt.Errorf("the module wasn't installed")
	errModuleInstalled    = fmt.Errorf("the module was installed when it shouldn't")
)

func installPolicy(module, path string, t *testing.T) io.Closer {
	moduleFileName := module + ".cil"
	modPath := filepath.Join(path, moduleFileName)
	f, err := os.Create(modPath)
	if err != nil {
		t.Fatalf("Couldn't open module file %s: %s", modPath, err)
	}
	return f
}

func removePolicy(module, path string, t *testing.T) {
	moduleFileName := module + ".cil"
	modPath := filepath.Join(path, moduleFileName)
	err := os.Remove(modPath)
	if err != nil {
		t.Fatalf("Couldn't remove module file %s: %s", modPath, err)
	}
}

func getHTTPClient(sockpath string) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", sockpath)
			},
		},
	}
}

func getReadyRequest(ctx context.Context, t *testing.T) *http.Request {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://unix/ready", nil)
	if err != nil {
		t.Fatalf("failed getting request: %s", err)
	}
	return req
}

// nolint:gocognit
func TestDaemon(t *testing.T) {
	done := make(chan bool)
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("Couldn't initialize logger: %s", err)
	}

	moddir, err := ioutil.TempDir("", "semodtest")
	if err != nil {
		t.Fatalf("Error creating temporary directory: %s", err)
	}
	defer os.RemoveAll(moddir) // clean up

	dir, err := ioutil.TempDir("", "selinuxd")
	if err != nil {
		t.Fatalf("Error creating temporary directory: %s", err)
	}
	sockpath := filepath.Join(dir, "selinuxd.sock")
	dbpath := filepath.Join(dir, "selinuxd.db")
	defer os.RemoveAll(dir) // clean up
	httpc := getHTTPClient(sockpath)

	config := SelinuxdOptions{
		StatusServerConfig: StatusServerConfig{
			Path: sockpath,
			UID:  os.Getuid(),
			GID:  os.Getuid(),
		},
		StatusDBPath: dbpath,
	}

	moduleName := "test"
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	sh := test.NewSEModuleTestHandler()
	go Daemon(&config, moddir, sh, done, zapr.NewLogger(logger))

	t.Run("Module should install a policy", func(t *testing.T) {
		f := installPolicy(moduleName, moddir, t)
		defer f.Close()

		// Module has to be installed... eventually
		err := backoff.Retry(func() error {
			if !sh.IsModuleInstalled(moduleName) {
				return errModuleNotInstalled
			}
			return nil
		}, backoff.WithMaxRetries(backoff.NewConstantBackOff(defaultPollBackOff), 5))
		if err != nil {
			t.Fatalf("%s", err)
		}
	})

	t.Run("Sending a GET to the socket's /ready/ path should return the ready status", func(t *testing.T) {
		req := getReadyRequest(ctx, t)
		response, err := httpc.Do(req)
		if err != nil {
			t.Fatalf("GET error on the socket: %s", err)
		}
		defer response.Body.Close()

		var status map[string]bool
		err = json.NewDecoder(response.Body).Decode(&status)
		if err != nil {
			t.Fatalf("cannot decode response: %s", err)
		}

		if status["ready"] != true {
			t.Fatalf("expected 'test' module, got: %t", status["ready"])
		}
	})

	t.Run("Sending a GET to the socket's /policies/ path should list modules", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, "GET", "http://unix/policies/", nil)
		if err != nil {
			t.Fatalf("failed getting request: %s", err)
		}

		response, err := httpc.Do(req)
		if err != nil {
			t.Fatalf("GET error on the socket: %s", err)
		}
		defer response.Body.Close()

		var moduleList []string
		err = json.NewDecoder(response.Body).Decode(&moduleList)
		if err != nil {
			t.Fatalf("cannot decode response: %s", err)
		}

		if len(moduleList) != 1 {
			t.Fatalf("expected one module, got: %d", len(moduleList))
		}

		if moduleList[0] != "test" {
			t.Fatalf("expected 'test' module, got: %s", moduleList[0])
		}
	})

	t.Run("Module should remove a policy", func(t *testing.T) {
		// We use the previously installed module
		removePolicy(moduleName, moddir, t)

		// Module has to be installed... eventually
		err := backoff.Retry(func() error {
			if sh.IsModuleInstalled(moduleName) {
				return errModuleInstalled
			}
			return nil
		}, backoff.WithMaxRetries(backoff.NewConstantBackOff(defaultPollBackOff), 5))
		if err != nil {
			t.Fatalf("%s", err)
		}
	})

	t.Run("Sending a GET to the socket's /policies/ path now not show the policy", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, "GET", "http://unix/policies/", nil)
		if err != nil {
			t.Fatalf("failed getting request: %s", err)
		}

		response, err := httpc.Do(req)
		if err != nil {
			t.Fatalf("GET error on the socket: %s", err)
		}
		defer response.Body.Close()

		var moduleList []string
		err = json.NewDecoder(response.Body).Decode(&moduleList)
		if err != nil {
			t.Fatalf("cannot decode response: %s", err)
		}

		if len(moduleList) != 0 {
			t.Fatalf("expected zero modules, got: %d", len(moduleList))
		}
	})

	t.Run("Deamon should create a socket with correct permissions", func(t *testing.T) {
		fi, err := os.Stat(sockpath)
		if err != nil {
			t.Fatal(err)
		}

		if fi.Mode()&os.ModeSocket == 0 {
			t.Fatal("not a socket")
		}

		perms := fi.Mode().Perm()
		if perms != 0660 {
			t.Fatalf("wrong perms, got %#o expected 0660", perms)
		}

		stat, ok := fi.Sys().(*syscall.Stat_t)
		if !ok {
			t.Fatal("stat error")
		}

		if int(stat.Uid) != os.Getuid() {
			t.Fatalf("wrong UID, got %d expected %d", int(stat.Uid), os.Getuid())
		}
		if int(stat.Gid) != os.Getgid() {
			t.Fatalf("wrong GID, got %d expected %d", int(stat.Gid), os.Getgid())
		}
	})
}
