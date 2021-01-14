package daemon

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"

	"github.com/JAORMX/selinuxd/pkg/datastore"
	"github.com/go-logr/logr"
)

const (
	DefaultUnixSockAddr = "/var/run/selinuxd.sock"
	unixSockMode        = 0660
)

type StatusServerConfig struct {
	Path            string
	UID             int
	GID             int
	EnableProfiling bool
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

func serveState(config StatusServerConfig, ds datastore.ReadOnlyDataStore, logger logr.Logger) {
	slog := logger.WithName("state-server")

	if config.Path == "" {
		config.Path = DefaultUnixSockAddr
	}

	slog.Info("Serving status", "path", config.Path, "uid", config.UID, "gid", config.GID)

	listener, err := createSocket(config.Path, config.UID, config.GID)
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

		modules, err := ds.List()
		if err != nil {
			http.Error(w, "Cannot list modules", http.StatusInternalServerError)
			return
		}

		err = json.NewEncoder(w).Encode(modules)
		if err != nil {
			slog.Error(err, "error writing list response")
		}
	}

	mux.HandleFunc("/policies/", policiesHandler)
	if config.EnableProfiling {
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}
	server := &http.Server{
		Handler: mux,
	}
	if err := server.Serve(listener); err != nil {
		slog.Info("Server shutting down: %s", err)
	}
}
