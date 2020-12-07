package daemon

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
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

	sh := test.NewSEModuleTestHandler()
	go Daemon(moddir, sh, done, zapr.NewLogger(logger))

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
}
