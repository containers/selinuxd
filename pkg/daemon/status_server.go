package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"path/filepath"
	"time"

	"github.com/containers/selinuxd/pkg/datastore"
	seiface "github.com/containers/selinuxd/pkg/semodule/interface"
	"github.com/containers/selinuxd/pkg/utils"
	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
)

const (
	DefaultUnixSockAddr = "/var/run/selinuxd.sock"
	unixSockMode        = 0o660
	readTimeout         = 5 * time.Second
)

type StatusServerConfig struct {
	Path            string
	UID             int
	GID             int
	EnableProfiling bool
}

type statusServer struct {
	cfg   StatusServerConfig
	ds    datastore.ReadOnlyDataStore
	sh    seiface.Handler
	l     logr.Logger
	mPath string
	lst   net.Listener
	ready bool
}

type policyStatus struct {
	Policy   string `json:"-"`
	Status   string `json:"status"`
	Message  string `json:"msg"`
	Checksum string `json:"-"`
}

func initStatusServer(
	cfg StatusServerConfig,
	ds datastore.ReadOnlyDataStore,
	sh seiface.Handler,
	l logr.Logger,
	mPath string,
) (*statusServer, error) {
	if cfg.Path == "" {
		cfg.Path = DefaultUnixSockAddr
	}

	lst, err := createSocket(cfg.Path, cfg.UID, cfg.GID)
	if err != nil {
		l.Error(err, "error setting up socket")
		// TODO: jhrozek: signal exit
		return nil, fmt.Errorf("setting up socket: %w", err)
	}

	ss := &statusServer{cfg, ds, sh, l, mPath, lst, false}
	return ss, nil
}

func (ss *statusServer) Serve(readychan <-chan bool) error {
	r := mux.NewRouter()
	ss.initializeRoutes(r)

	server := &http.Server{
		Handler:     r,
		ReadTimeout: readTimeout,
	}

	go ss.waitForReady(readychan)

	if err := server.Serve(ss.lst); err != nil {
		ss.l.Info("Server shutting down: %s", err)
	}
	return nil
}

func (ss *statusServer) waitForReady(readychan <-chan bool) {
	ready := <-readychan
	ss.ready = ready
	ss.l.Info("Status Server got READY signal")
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
	r.HandleFunc("/ready", ss.readyStatusHandler)
	r.HandleFunc("/ready/", ss.readyStatusHandler)
	r.HandleFunc("/", ss.catchAllHandler)

	if ss.cfg.EnableProfiling {
		r.HandleFunc("/debug/pprof/", pprof.Index)
		r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		r.HandleFunc("/debug/pprof/profile", pprof.Profile)
		r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		r.HandleFunc("/debug/pprof/trace", pprof.Trace)

		// Manually add support for paths linked to by index page at /debug/pprof/
		r.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
		r.Handle("/debug/pprof/heap", pprof.Handler("heap"))
		r.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
		r.Handle("/debug/pprof/block", pprof.Handler("block"))
	}
}

func (ss *statusServer) listPoliciesHandler(w http.ResponseWriter, r *http.Request) {
	modules, err := ss.sh.List()
	if err != nil {
		http.Error(w, "Cannot list modules", http.StatusInternalServerError)
		return
	}

	moduleList := []string{}
	for _, module := range modules {
		moduleList = append(moduleList, module.Name)
	}
	err = json.NewEncoder(w).Encode(moduleList)
	if err != nil {
		ss.l.Error(err, "error writing list response")
		http.Error(w, "Cannot list modules", http.StatusInternalServerError)
	}
}

func (ss *statusServer) getPolicyStatusHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	policy := vars["policy"]
	module, err := ss.sh.GetPolicyModule(policy)
	if errors.Is(err, seiface.ErrPolicyNotFound) {
		http.Error(w, "policy is not installed", http.StatusNotFound)
		return
	} else if err != nil {
		ss.l.Error(err, "error getting status")
		http.Error(w, "Cannot get status", http.StatusInternalServerError)
		return
	}

	var policyFile string
	err = filepath.Walk(ss.mPath, func(path string, info os.FileInfo, err error) error {
		if info == nil {
			return nil
		}
		if !info.IsDir() && (filepath.Base(path) == policy+".cil" || filepath.Base(path) == policy+".pp") {
			policyFile = path
			return nil
		}
		return nil
	})
	if err != nil {
		ss.l.Error(err, "error getting status")
		http.Error(w, "Cannot get status", http.StatusInternalServerError)
		return
	}

	cs, csErr := utils.Checksum(policyFile)

	if csErr != nil {
		http.Error(w, "cannot find policy file "+policyFile, http.StatusNotFound)
		return
	}

	if cs != module.Checksum {
		http.Error(w, "Installed policy "+module.Name+" does not much policy file "+policyFile, http.StatusNotFound)
		return
	}
	err = json.NewEncoder(w).Encode(policyStatus{
		Policy:   policy,
		Status:   "Installed",
		Message:  "",
		Checksum: module.Checksum,
	})
	if err != nil {
		ss.l.Error(err, "error writing status response")
		http.Error(w, "Cannot get status", http.StatusInternalServerError)
	}
}

func (ss *statusServer) readyStatusHandler(w http.ResponseWriter, r *http.Request) {
	output := map[string]bool{
		"ready": ss.ready,
	}

	if err := json.NewEncoder(w).Encode(output); err != nil {
		ss.l.Error(err, "error writing ready response")
		http.Error(w, "Cannot get ready status", http.StatusInternalServerError)
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

func serveState(server *statusServer, readychan <-chan bool, logger logr.Logger) {
	slog := logger.WithName("state-server")

	slog.Info("Serving status", "path", server.cfg.Path, "uid", server.cfg.UID, "gid", server.cfg.GID)

	if err := server.Serve(readychan); err != nil {
		slog.Error(err, "Error starting status server")
	}
}
