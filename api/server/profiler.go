package server

import (
	"net/http"
	"net/http/pprof"

	"github.com/gorilla/mux"
)

// profilerSetup registers the Go pprof endpoints on the router.
func profilerSetup(m *mux.Router) {
	m.Path("/debug/vars").Handler(http.HandlerFunc(pprof.Index))
	m.Path("/debug/pprof/").Handler(http.HandlerFunc(pprof.Index))
	m.Path("/debug/pprof/cmdline").Handler(http.HandlerFunc(pprof.Cmdline))
	m.Path("/debug/pprof/profile").Handler(http.HandlerFunc(pprof.Profile))
	m.Path("/debug/pprof/symbol").Handler(http.HandlerFunc(pprof.Symbol))
	m.Path("/debug/pprof/trace").Handler(http.HandlerFunc(pprof.Trace))
}
