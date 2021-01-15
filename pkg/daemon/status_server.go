package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"

	"github.com/JAORMX/selinuxd/pkg/datastore"
	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
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

type statusServer struct {
	cfg StatusServerConfig
	ds  datastore.ReadOnlyDataStore
	l   logr.Logger
}

func newStatusServer(cfg StatusServerConfig, ds datastore.ReadOnlyDataStore, l logr.Logger) *statusServer {
	if cfg.Path == "" {
		cfg.Path = DefaultUnixSockAddr
	}

	ss := &statusServer{cfg, ds, l}
	return ss
}

func (ss *statusServer) Serve() error {
	lst, err := createSocket(ss.cfg.Path, ss.cfg.UID, ss.cfg.GID)
	if err != nil {
		ss.l.Error(err, "error setting up socket")
		// TODO: jhrozek: signal exit
		return fmt.Errorf("setting up socket: %w", err)
	}

	r := mux.NewRouter()
	ss.initializeRoutes(r)

	server := &http.Server{
		Handler: r,
	}
	if err := server.Serve(lst); err != nil {
		ss.l.Info("Server shutting down: %s", err)
	}
	return nil
}

func (ss *statusServer) initializeRoutes(r *mux.Router) {
	// /policies/
	s := r.PathPrefix("/policies").Subrouter()
	s.HandleFunc("/", ss.listPoliciesHandler).
		Methods("GET")
	s.HandleFunc("/", ss.catchAllNotGetHandler)
	// IMPORTANT(jaosorior): We should better restrict what characters
	// does this handler accept
	s.HandleFunc("/{policy}", ss.getPolicyStatusHandler).
		Methods("GET")
	s.HandleFunc("/{policy}", ss.catchAllNotGetHandler)

	// /policies -- without the trailing /
	r.HandleFunc("/policies", ss.listPoliciesHandler).
		Methods("GET")
	r.HandleFunc("/policies", ss.catchAllNotGetHandler)
	r.HandleFunc("/", ss.catchAllHandler)

	if ss.cfg.EnableProfiling {
		r.HandleFunc("/debug/pprof/", pprof.Index)
		r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		r.HandleFunc("/debug/pprof/profile", pprof.Profile)
		r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		r.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}
}

func (ss *statusServer) listPoliciesHandler(w http.ResponseWriter, r *http.Request) {
	modules, err := ss.ds.List()
	if err != nil {
		http.Error(w, "Cannot list modules", http.StatusInternalServerError)
		return
	}

	err = json.NewEncoder(w).Encode(modules)
	if err != nil {
		ss.l.Error(err, "error writing list response")
		http.Error(w, "Cannot list modules", http.StatusInternalServerError)
	}
}

func (ss *statusServer) getPolicyStatusHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	policy := vars["policy"]
	status, msg, err := ss.ds.GetStatus(policy)
	if errors.Is(err, datastore.ErrPolicyNotFound) {
		http.Error(w, "couldn't find requested policy", http.StatusNotFound)
		return
	} else if err != nil {
		ss.l.Error(err, "error getting status")
		http.Error(w, "Cannot get status", http.StatusInternalServerError)
		return
	}

	output := map[string]string{
		"status": string(status),
		"msg":    msg,
	}
	err = json.NewEncoder(w).Encode(output)
	if err != nil {
		ss.l.Error(err, "error writing status response")
		http.Error(w, "Cannot get status", http.StatusInternalServerError)
	}
}

func (ss *statusServer) catchAllHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Invalid path", http.StatusBadRequest)
}

func (ss *statusServer) catchAllNotGetHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Only GET is allowed", http.StatusBadRequest)
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

	slog.Info("Serving status", "path", config.Path, "uid", config.UID, "gid", config.GID)

	server := newStatusServer(config, ds, logger)

	if err := server.Serve(); err != nil {
		slog.Error(err, "Error starting status server")
	}
}
