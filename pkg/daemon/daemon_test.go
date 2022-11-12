package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	backoff "github.com/cenkalti/backoff/v4"
	"github.com/containers/selinuxd/pkg/datastore"
	"github.com/containers/selinuxd/pkg/semodule/test"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
)

const (
	defaultPollBackOff = 1 * time.Second
	defaultTimeout     = 10 * time.Second
)

var (
	errModuleNotInstalled    = fmt.Errorf("the module wasn't installed")
	errModuleInstalled       = fmt.Errorf("the module was installed when it shouldn't")
	errInstallNotPerfomedYet = fmt.Errorf("install action not performed yet")
)

func getPolicyPath(module, path string) string {
	moduleFileName := module + ".cil"
	return filepath.Join(path, moduleFileName)
}

func installPolicy(module, path string, t *testing.T) {
	modPath := getPolicyPath(module, path)
	message := []byte("Hello, Gophers!")
	err := os.WriteFile(modPath, message, 0o600)
	if err != nil {
		t.Fatal(err)
	}
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
				//nolint: wrapcheck // let's not complicate the test
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

//nolint:gocognit,gocyclo
func TestDaemon(t *testing.T) {
	done := make(chan bool)
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("Couldn't initialize logger: %s", err)
	}

	moddir := filepath.Join(os.TempDir(), "semodtest")
	err = os.Mkdir(moddir, 0o755)
	if err != nil {
		t.Fatalf("Error creating temporary directory: %s", err)
	}
	defer os.RemoveAll(moddir) // clean up

	dir := filepath.Join(os.TempDir(), "selinuxd")
	err = os.Mkdir(dir, 0o755)
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

	ds, err := datastore.NewTestCountedDS(config.StatusDBPath)
	if err != nil {
		t.Fatalf("Unable to get R/W datastore: %s", err)
	}
	defer ds.Close()

	go Daemon(&config, moddir, sh, ds, done, zapr.NewLogger(logger))
	defer close(done)

	t.Run("Should install a policy", func(t *testing.T) {
		installPolicy(moduleName, moddir, t)

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

	t.Run("Should skip policy installation if it's already installed", func(t *testing.T) {
		initPolicyGets := ds.GetCalls()
		initPolicyPuts := ds.PutCalls()
		time.Sleep(1 * time.Second)
		// Overwritting a policy with the same contents should not
		// trigger another PUT
		installPolicy(moduleName, moddir, t)

		var currentGetCalls, currentPutCalls int32

		// Module has to be installed... eventually
		err := backoff.Retry(func() error {
			// "touching" the policy will trigger an inotify
			// event which will attempt to install it again.
			// The action interface will "get" the policy
			// and compare the checksum
			currentGetCalls = ds.GetCalls()
			currentPutCalls = ds.PutCalls()
			if initPolicyGets == currentGetCalls {
				return errInstallNotPerfomedYet
			}
			return nil
		}, backoff.WithMaxRetries(backoff.NewConstantBackOff(defaultPollBackOff), 5))
		if err != nil {
			t.Fatalf("%s. Got GET calls %d - Started with %d", err, currentGetCalls, initPolicyGets)
		}

		if currentPutCalls != initPolicyPuts {
			t.Fatalf("The policy was updated unexpectedly. Got put calls: %d - Expected: %d",
				currentGetCalls, initPolicyPuts)
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

	t.Run("Sending a GET to the socket's /policies/<policy name path should show the policy's status", func(t *testing.T) {
		ppath := fmt.Sprintf("http://unix/policies/%s", moduleName)
		req, err := http.NewRequestWithContext(ctx, "GET", ppath, nil)
		if err != nil {
			t.Fatalf("failed getting request: %s", err)
		}

		response, err := httpc.Do(req)
		if err != nil {
			t.Fatalf("GET error on the socket: %s", err)
		}
		defer response.Body.Close()

		var moduleStatus map[string]string
		err = json.NewDecoder(response.Body).Decode(&moduleStatus)
		if err != nil {
			t.Fatalf("cannot decode response: %s", err)
		}

		_, hasMsg := moduleStatus["msg"]

		if !hasMsg {
			t.Fatalf("expected status to contain message")
		}

		if moduleStatus["status"] != string(datastore.InstalledStatus) {
			t.Fatalf("expected module's status to be installed, got: %s", moduleStatus["status"])
		}
	})

	t.Run("Module should remove a policy", func(t *testing.T) {
		// We use the previously installed module
		removePolicy(moduleName, moddir, t)

		// Module has to be removed... eventually
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
		if perms != 0o660 {
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

	subdirPath := filepath.Join(moddir, "sub-dir")
	subdirPolicy := "subdirpolicy"

	t.Run("Module should track a policy in sub-directory", func(t *testing.T) {
		if err := os.Mkdir(subdirPath, 0o700); err != nil {
			t.Fatalf("Unable to create sub-directory: %s", err)
		}
		installPolicy(subdirPolicy, subdirPath, t)

		// Module has to be installed... eventually
		err := backoff.Retry(func() error {
			if !sh.IsModuleInstalled(subdirPolicy) {
				return errModuleNotInstalled
			}
			return nil
		}, backoff.WithMaxRetries(backoff.NewConstantBackOff(defaultPollBackOff), 5))
		if err != nil {
			t.Fatalf("%s", err)
		}
	})

	t.Run("Module should stop tracking a policy in sub-directory", func(t *testing.T) {
		os.RemoveAll(subdirPath)

		// Module has to be removed... eventually
		err := backoff.Retry(func() error {
			if sh.IsModuleInstalled(subdirPolicy) {
				return errModuleInstalled
			}
			return nil
		}, backoff.WithMaxRetries(backoff.NewConstantBackOff(defaultPollBackOff), 5))
		if err != nil {
			t.Fatalf("%s", err)
		}
	})
}

func TestDaemonWithSubdir(t *testing.T) {
	done := make(chan bool)
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("Couldn't initialize logger: %s", err)
	}

	moddir := filepath.Join(os.TempDir(), "semodtest")
	err = os.Mkdir(moddir, 0o755)
	if err != nil {
		t.Fatalf("Error creating temporary directory: %s", err)
	}
	defer os.RemoveAll(moddir) // clean up

	dir := filepath.Join(os.TempDir(), "selinuxd")
	err = os.Mkdir(dir, 0o755)
	if err != nil {
		t.Fatalf("Error creating temporary directory: %s", err)
	}
	sockpath := filepath.Join(dir, "selinuxd.sock")
	dbpath := filepath.Join(dir, "selinuxd.db")
	defer os.RemoveAll(dir) // clean up

	config := SelinuxdOptions{
		StatusServerConfig: StatusServerConfig{
			Path: sockpath,
			UID:  os.Getuid(),
			GID:  os.Getuid(),
		},
		StatusDBPath: dbpath,
	}

	sh := test.NewSEModuleTestHandler()

	ds, err := datastore.NewTestCountedDS(config.StatusDBPath)
	if err != nil {
		t.Fatalf("Unable to get R/W datastore: %s", err)
	}
	defer ds.Close()

	subdirPath := filepath.Join(moddir, "sub-dir")
	subdirPolicy := "subdirpolicy"

	t.Run("Install policy before daemon runs", func(t *testing.T) {
		if err := os.Mkdir(subdirPath, 0o700); err != nil {
			t.Fatalf("Unable to create sub-directory: %s", err)
		}
		installPolicy(subdirPolicy, subdirPath, t)
	})

	go Daemon(&config, moddir, sh, ds, done, zapr.NewLogger(logger))
	defer close(done)

	t.Run("Module should track a policy in pre-existing sub-directory", func(t *testing.T) {
		// Module has to be installed... eventually
		err := backoff.Retry(func() error {
			if !sh.IsModuleInstalled(subdirPolicy) {
				return errModuleNotInstalled
			}
			return nil
		}, backoff.WithMaxRetries(backoff.NewConstantBackOff(defaultPollBackOff), 5))
		if err != nil {
			t.Fatalf("%s", err)
		}
	})

	t.Run("Module should stop tracking a policy in sub-directory", func(t *testing.T) {
		os.RemoveAll(subdirPath)

		// Module has to be removed... eventually
		err := backoff.Retry(func() error {
			if sh.IsModuleInstalled(subdirPolicy) {
				return errModuleInstalled
			}
			return nil
		}, backoff.WithMaxRetries(backoff.NewConstantBackOff(defaultPollBackOff), 5))
		if err != nil {
			t.Fatalf("%s", err)
		}
	})
}
