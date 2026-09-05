package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/maliceio/malice/config"
	"github.com/maliceio/malice/malice/maldirs"
	"github.com/maliceio/malice/plugins"
	"github.com/maliceio/malice/secrets"
	"github.com/maliceio/malice/web"
	log "github.com/sirupsen/logrus"
)

// maxUploadBytes caps the total size of an uploaded sample. Without it,
// ParseMultipartForm only bounds the in-memory portion and spools the rest to
// a temp file with no total limit, so a single large upload could fill the disk.
const maxUploadBytes = 512 << 20

// scanSem caps concurrent scans. Each scan fans out to ~17 engine containers,
// so unbounded concurrency is a resource-exhaustion DoS on the docker daemon
// and Elasticsearch.
var scanSem = make(chan struct{}, 2)

// scanInitFunc and scanRunFunc are injected by the serve command (avoids an
// api->commands import cycle). scanInitFunc does the fast setup + file store and
// returns the scan id synchronously; scanRunFunc fans out to the engines in the
// background.
var scanInitFunc func(path string) (string, error)
var scanRunFunc func(path, scanID string) error

// SetScanFunc wires the scan entry points (commands.APIScanInit /
// commands.APIScanRun) into the API.
func SetScanFunc(initFn func(path string) (string, error), runFn func(path, scanID string) error) {
	scanInitFunc = initFn
	scanRunFunc = runFn
}

// Init loads plugins, configures the ES endpoint, and opens the
// file-backed secrets store (engine credentials entered via the web UI).
func Init() {
	InitES()
	plugins.Load()
	if err := secrets.Init(filepath.Join(maldirs.GetBaseDir(), "secrets.env")); err != nil {
		log.Errorf("secrets: init: %v", err)
	}
}

// Start runs the HTTP server (REST API + embedded UI) on addr.
func Start(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", handleHealth)
	mux.HandleFunc("/api/scans", handleScans)
	mux.HandleFunc("/api/scans/", handleScanDetail)
	mux.HandleFunc("/api/plugins", handlePlugins)
	mux.HandleFunc("/api/settings", handleSettings)
	uiFS := http.FileServerFS(web.FS())
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		uiFS.ServeHTTP(w, r)
	}))
	log.Infof("malice web UI + REST API listening on %s", addr)
	// Timeouts guard against slowloris / pinned idle connections.
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	return srv.ListenAndServe()
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		log.Errorf("api: encode response: %v", err)
	}
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	engines := len(plugins.Plugs.Plugins)
	enabled := 0
	for _, p := range plugins.Plugs.Plugins {
		if p.Enabled {
			enabled++
		}
	}
	writeJSON(w, map[string]interface{}{
		"status":          "ok",
		"version":         config.Conf.Version,
		"elasticsearch":   ESHealthy(),
		"engines":         engines,
		"engines_enabled": enabled,
		"index":           ESIndex(),
	})
}

func handleScans(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		from, _ := strconv.Atoi(r.URL.Query().Get("from"))
		size, _ := strconv.Atoi(r.URL.Query().Get("size"))
		if size <= 0 || size > 100 {
			size = 20
		}
		if from < 0 {
			from = 0
		}
		scans, total, err := SearchScans(from, size)
		if err != nil {
			log.Errorf("api: search scans: %v", err)
			writeErr(w, http.StatusBadGateway, "failed to query scans")
			return
		}
		list := make([]map[string]interface{}, 0, len(scans))
		for _, s := range scans {
			sum := summarizeScan(s.Source)
			sum["id"] = s.ID
			list = append(list, sum)
		}
		writeJSON(w, map[string]interface{}{"total": total, "from": from, "size": size, "scans": list})
	case http.MethodPost:
		handleUpload(w, r)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func handleScanDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	id := filepath.Base(r.URL.Path) // strip /api/scans/
	if id == "" || id == "." || id == "/" {
		writeErr(w, http.StatusNotFound, "scan id required")
		return
	}
	src, err := GetScan(id)
	if err != nil {
		log.Errorf("api: get scan %s: %v", id, err)
		writeErr(w, http.StatusNotFound, "scan not found")
		return
	}
	sum := summarizeScan(src)
	sum["id"] = id
	sum["plugins"] = pluginsOf(src)
	writeJSON(w, sum)
}

func handlePlugins(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	out := make([]map[string]interface{}, 0, len(plugins.Plugs.Plugins))
	for _, p := range plugins.Plugs.Plugins {
		out = append(out, map[string]interface{}{
			"name":        p.Name,
			"category":    p.Category,
			"enabled":     p.Enabled,
			"description": p.Description,
			"image":       p.Image,
		})
	}
	writeJSON(w, map[string]interface{}{"engines": out, "total": len(out)})
}

// handleUpload accepts a multipart file, stores it in a temp dir, and kicks off a scan.
func handleUpload(w http.ResponseWriter, r *http.Request) {
	if scanInitFunc == nil || scanRunFunc == nil {
		writeErr(w, http.StatusInternalServerError, "scan backend not wired")
		return
	}
	// Cap the total request body before parsing; ParseMultipartForm alone only
	// bounds the in-memory portion and would spool an unbounded body to /tmp.
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid or oversized upload")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "missing 'file' field")
		return
	}
	defer file.Close()

	name := filepath.Base(header.Filename)
	if name == "" || name == "." || name == ".." || name == "/" {
		name = "upload"
	}
	dir, err := os.MkdirTemp("", "malice-upload-")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "temp dir: "+err.Error())
		return
	}
	dstPath := filepath.Join(dir, name)
	dst, err := os.Create(dstPath)
	if err != nil {
		os.RemoveAll(dir)
		writeErr(w, http.StatusInternalServerError, "create temp file: "+err.Error())
		return
	}
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(dst, h), file); err != nil {
		dst.Close()
		os.RemoveAll(dir)
		writeErr(w, http.StatusInternalServerError, "store upload: "+err.Error())
		return
	}
	dst.Close()
	sha := hex.EncodeToString(h.Sum(nil))

	// Admit the scan only if a slot is free; otherwise reject with 429 rather
	// than queueing unbounded concurrent 17-engine scans.
	select {
	case scanSem <- struct{}{}:
	default:
		os.RemoveAll(dir)
		writeErr(w, http.StatusTooManyRequests, "too many scans in progress; try again shortly")
		return
	}

	// Generate the scan id synchronously (fast setup + file store) so it can be
	// returned in the response. The web client keys its in-flight set by scan id
	// (not SHA) so re-uploading a file whose SHA already has completed scans does
	// not mask those verdicts behind a "Scanning" badge.
	scanID, err := scanInitFunc(dstPath)
	if err != nil {
		<-scanSem
		os.RemoveAll(dir)
		writeErr(w, http.StatusInternalServerError, "start scan: "+err.Error())
		return
	}

	go func() {
		defer func() {
			<-scanSem
			if rec := recover(); rec != nil {
				log.Errorf("scan goroutine panic: %v", rec)
			}
			os.RemoveAll(dir)
		}()
		if err := scanRunFunc(dstPath, scanID); err != nil {
			log.Errorf("scan of %s failed: %v", name, err)
		}
	}()

	writeJSON(w, map[string]interface{}{
		"status":  "queued",
		"id":      scanID,
		"sha256":  sha,
		"name":    name,
		"message": "scan started; poll /api/scans to follow progress",
	})
}

// foundIsThreat lists engines where found=true is itself a positive detection
// (a threat-intel or AV hit). nsrl and shadow-server are handled specially in
// isDetection because their found=true also fires on known-GOOD matches.
var foundIsThreat = map[string]bool{
	"kvrt": true, "lmd": true, "hashlookup": true,
}

// isDetection reports whether a single engine result is a positive hit.
// Semantics differ per engine, so the generic found/infected/matches checks
// are not enough:
//   - nsrl: found=true means the hash IS in the NIST NSRL (known-GOOD software) — never a threat
//   - shadow-server: found=true also fires on a whitelist (known-good) match; only a
//     sandbox antivirus/metadata hit is a real detection
//   - virustotal: positives are reported as a `positives` int, not found/detections
//   - eset/diec/rizin: found=true only means "processed/analyzed"; real signals are detections/matches
func isDetection(name string, res json.RawMessage) bool {
	var r struct {
		Found      bool          `json:"found"`
		Infected   bool          `json:"infected"`
		Status     string        `json:"status"`
		Matches    []interface{} `json:"matches"`
		Detections []interface{} `json:"detections"`
		Positives  int           `json:"positives"`
		SandBox    struct {
			MetaData  map[string]string `json:"metadata"`
			Antivirus map[string]string `json:"antivirus"`
		} `json:"sandbox"`
	}
	_ = json.Unmarshal(res, &r)

	switch name {
	case "nsrl":
		// An NSRL hit is NIST's catalog of known-legitimate software — a known-GOOD signal.
		return false
	case "shadow-server":
		// A whitelist match is known-good; only a sandbox AV/metadata hit is a detection.
		return len(r.SandBox.Antivirus) > 0 || len(r.SandBox.MetaData) > 0
	case "virustotal":
		return r.Positives > 0
	}

	if r.Infected {
		return true
	}
	if r.Status == "infected" || r.Status == "threat" || r.Status == "malicious" {
		return true
	}
	if len(r.Matches) > 0 || len(r.Detections) > 0 {
		return true
	}
	return r.Found && foundIsThreat[name]
}

// summarizeScan derives a verdict + engine counts from a raw scan doc.
func summarizeScan(raw json.RawMessage) map[string]interface{} {
	var doc struct {
		File     map[string]interface{}                `json:"file"`
		Plugins  map[string]map[string]json.RawMessage `json:"plugins"`
		ScanDate string                                `json:"scan_date"`
	}
	_ = json.Unmarshal(raw, &doc)

	reported := 0
	threat := false
	detections := []string{}
	for _, cat := range doc.Plugins {
		for name, res := range cat {
			// The doc is pre-seeded with a null slot per enabled engine; the slot
			// is filled when that engine reports. Count only filled slots so
			// engines_reported climbs 0→N as the scan progresses (a slot count
			// would be stuck at the total from the first poll).
			if isNullResult(res) {
				continue
			}
			reported++
			if isDetection(name, res) {
				threat = true
				detections = append(detections, name)
			}
		}
	}
	verdict := "clean"
	if threat {
		verdict = "threat"
	}
	// engines_expected is how many of the enabled engines apply to this
	// file's MIME type - the correct denominator for progress. Older docs
	// have no mime_type; for those the client falls back to the total.
	expected := 0
	if mime, _ := doc.File["mime_type"].(string); mime != "" {
		expected = len(plugins.ScanPlugins(mime, true))
	}
	return map[string]interface{}{
		"file":             doc.File,
		"scan_date":        doc.ScanDate,
		"verdict":          verdict,
		"engines_reported": reported,
		"engines_expected": expected,
		"detections":       detections,
	}
}

// isNullResult reports whether a raw plugin slot is still empty (a JSON null or
// blank). A slot is null until its engine reports a result; engines that do not
// apply to a file type (e.g. capa/pescan on a text file) stay null.
func isNullResult(res json.RawMessage) bool {
	t := bytes.TrimSpace(res)
	return len(t) == 0 || string(t) == "null"
}

// pluginsOf returns the raw plugins tree for the detail view.
func pluginsOf(raw json.RawMessage) map[string]map[string]json.RawMessage {
	var doc struct {
		Plugins map[string]map[string]json.RawMessage `json:"plugins"`
	}
	_ = json.Unmarshal(raw, &doc)
	return doc.Plugins
}

var _ = fmt.Sprintf

// settingsKeys is the allowlist of engine credentials the web UI may set.
// Only these keys are accepted by POST; anything else is rejected so a client
// cannot write arbitrary keys into the secrets store.
var settingsKeys = []string{"VIRUSTOTAL_API_KEY", "ESET_LICENSE_KEY"}

func settingsKeyAllowed(k string) bool {
	for _, a := range settingsKeys {
		if k == a {
			return true
		}
	}
	return false
}

// handleSettings serves the engine-credential settings page.
//
//	GET  -> { keys: { NAME: "****abcd" | "", admin_token_required: bool } }
//	POST -> { NAME: value, ... } (allowlist + validation; values never logged)
//
// GET is unauthenticated (it only returns masked values, safe to expose). POST
// is a state-changing route, so if MALICE_ADMIN_TOKEN is configured it is
// required (Authorization: Bearer <token>). The rest of the API is unauthenticated
// (single-user trusted-network tool), so the token is opt-in hardening, not a
// hard requirement.
func handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		keys := map[string]string{}
		for _, k := range settingsKeys {
			keys[k] = secrets.Mask(k)
		}
		writeJSON(w, map[string]interface{}{
			"keys":                 keys,
			"admin_token_required": os.Getenv("MALICE_ADMIN_TOKEN") != "",
		})
	case http.MethodPost:
		if tok := os.Getenv("MALICE_ADMIN_TOKEN"); tok != "" {
			if r.Header.Get("Authorization") != "Bearer "+tok {
				writeErr(w, http.StatusUnauthorized, "admin token required")
				return
			}
		}
		// Cap the body so a client cannot send an unbounded payload.
		r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		updated := []string{}
		for k, v := range payload {
			if !settingsKeyAllowed(k) {
				writeErr(w, http.StatusBadRequest, "unknown key: "+k)
				return
			}
			// secrets.Set validates the key name and value (length/charset) and
			// its error never contains the value, so it is safe to surface.
			if err := secrets.Set(k, v); err != nil {
				writeErr(w, http.StatusBadRequest, err.Error())
				return
			}
			updated = append(updated, k)
		}
		sort.Strings(updated)
		// Log only which keys changed, never the values.
		log.Infof("settings: updated %d credential key(s): %v", len(updated), updated)
		writeJSON(w, map[string]interface{}{
			"status":  "ok",
			"updated": updated,
		})
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
