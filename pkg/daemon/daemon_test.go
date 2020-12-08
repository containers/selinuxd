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

	backoff "github.com/cenkalti/backoff/v4"
	"go.uber.org/zap"

	"github.com/JAORMX/selinuxd/pkg/semodule/test"
	"github.com/go-logr/zapr"
)

const oneSec = 1 * time.Second

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

func TestDaemon(t *testing.T) {
	done := make(chan bool)
	logger, _ := zap.NewDevelopment()

	moddir, err := ioutil.TempDir("", "semodtest")
	if err != nil {
		t.Fatalf("Error creating temporary directory: %s", err)
	}
	defer os.RemoveAll(moddir) // clean up

	sockdir, err := ioutil.TempDir("", "selinuxd-sockdir")
	if err != nil {
		t.Fatalf("Error creating temporary directory: %s", err)
	}
	sockpath := sockdir + "/selinuxd.sock"
	defer os.RemoveAll(sockdir) // clean up

	config := SelinuxdOptions{
		StatusServerConfig: StatusServerConfig{
			Path: sockpath,
			Uid: os.Getuid(),
			Gid: os.Getuid(),
		},
	}

	sh := test.NewSEModuleTestHandler()
	go Daemon(&config, moddir, sh, done, zapr.NewLogger(logger))

	t.Run("Module should install a policy", func(t *testing.T) {
		moduleName := "test"
		f := installPolicy(moduleName, moddir, t)
		defer f.Close()

		// Module has to be installed... eventually
		err := backoff.Retry(func() error {
			if !sh.IsModuleInstalled(moduleName) {
				return fmt.Errorf("The module wasn't installed")
			}
			return nil
		}, backoff.WithMaxRetries(backoff.NewConstantBackOff(oneSec), 5))
		if err != nil {
			t.Fatalf("%s", err)
		}
	})


	t.Run("Sending a GET to the socket's /policies/ path should list modules", func(t *testing.T) {
		httpc := http.Client{
			Transport: &http.Transport{
				DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", sockpath)
				},
			},
		}

		response, err := httpc.Get("http://unix/policies/")
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
		moduleName := "test"
		removePolicy(moduleName, moddir, t)

		// Module has to be installed... eventually
		err := backoff.Retry(func() error {
			if sh.IsModuleInstalled(moduleName) {
				return fmt.Errorf("The module was installed when it shouldn't")
			}
			return nil
		}, backoff.WithMaxRetries(backoff.NewConstantBackOff(oneSec), 5))
		if err != nil {
			t.Fatalf("%s", err)
		}
	})

	t.Run("Deamon should create a socket with correct permissions", func(t *testing.T) {
		fi, err := os.Stat(sockpath)
		if err != nil {
			t.Fatal(err)
		}

		if fi.Mode() & os.ModeSocket == 0 {
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
