package daemon

import (
	"encoding/json"
	"fmt"
	"github.com/JAORMX/selinuxd/pkg/semodule"
	"github.com/go-logr/logr"
	"net"
	"net/http"
	"os"
)

const unixSockAddr = "/var/run/selinuxd.sock"
const unixSockMode = 0660

type StatusServerConfig struct {
	Path string
	Uid int
	Gid int
}

func createSocket(path string, uid, gid int) (net.Listener, error) {
	if err := os.RemoveAll(path); err != nil {
		return nil, fmt.Errorf("cannot remove old socket: %w", err)
	}

	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("listen error: %w", err)
	}

	err = os.Chown(path, uid, gid)
	if err != nil {
		return nil, fmt.Errorf("chown error: %w", err)
	}

	err = os.Chmod(path, unixSockMode)
	if err != nil {
		return nil, fmt.Errorf("chmod error: %w", err)
	}

	return listener, nil
}

func serveState(config StatusServerConfig, sh semodule.SEModuleHandler, logger logr.Logger) {
	slog := logger.WithName("state-server")

	if config.Path == "" {
		config.Path = unixSockAddr
	}

	slog.Info("Serving status", "path", config.Path, "uid", config.Uid, "gid", config.Gid)

	listener, err := createSocket(config.Path, config.Uid, config.Gid)
	if err != nil {
		slog.Error(err, "error setting up socket")
		// TODO: jhrozek: signal exit
		return
	}

	mux := http.NewServeMux()
	policiesHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "" && r.Method != "GET" {
			http.Error(w, "Only GET is allowed", http.StatusBadRequest)
			return
		}

		modules, err := sh.List()
		if err != nil {
			http.Error(w, "Cannot list modules", http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(modules)
	}

	mux.HandleFunc("/policies/", policiesHandler)
	server := &http.Server{
		Handler: mux,
	}
    server.Serve(listener)
}
