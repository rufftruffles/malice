package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/maliceio/malice/config"
	"github.com/maliceio/malice/plugins"
	"github.com/maliceio/malice/web"
	log "github.com/sirupsen/logrus"
)

// scanFunc is injected by the serve command (avoids an api->commands import cycle).
var scanFunc func(path string) error

// SetScanFunc wires the scan entry point (commands.APIScan) into the API.
func SetScanFunc(fn func(path string) error) { scanFunc = fn }

// Init loads plugins and configures the ES endpoint.
func Init() {
	InitES()
	plugins.Load()
}

// Start runs the HTTP server (REST API + embedded UI) on addr.
func Start(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", handleHealth)
	mux.HandleFunc("/api/scans", handleScans)
	mux.HandleFunc("/api/scans/", handleScanDetail)
	mux.HandleFunc("/api/plugins", handlePlugins)
	uiFS := http.FileServerFS(web.FS())
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		uiFS.ServeHTTP(w, r)
	}))
	log.Infof("malice web UI + REST API listening on %s", addr)
	return http.ListenAndServe(addr, mux)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		log.Errorf("api: encode response: %v", err)
	}
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
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
			writeErr(w, http.StatusBadGateway, err.Error())
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
		writeErr(w, http.StatusNotFound, err.Error())
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
	if scanFunc == nil {
		writeErr(w, http.StatusInternalServerError, "scan backend not wired")
		return
	}
	if err := r.ParseMultipartForm(512 << 20); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid multipart form: "+err.Error())
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "missing 'file' field: "+err.Error())
		return
	}
	defer file.Close()

	name := filepath.Base(header.Filename)
	if name == "" || name == "." || name == "/" {
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

	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.Errorf("scan goroutine panic: %v", rec)
			}
			os.RemoveAll(dir)
		}()
		if err := scanFunc(dstPath); err != nil {
			log.Errorf("scan of %s failed: %v", name, err)
		}
	}()

	writeJSON(w, map[string]interface{}{
		"status":  "queued",
		"sha256":  sha,
		"name":    name,
		"message": "scan started; poll /api/scans to follow progress",
	})
}

// foundIsThreat lists engines where found=true is a positive detection.
// For eset/diec/rizin, found=true only means "processed/analyzed", so they
// are deliberately excluded — their real signals are detections/matches.
var foundIsThreat = map[string]bool{
	"kvrt": true, "lmd": true, "hashlookup": true, "nsrl": true, "shadow-server": true,
}

// isDetection reports whether a single engine result is a positive hit.
func isDetection(name string, res json.RawMessage) bool {
	var r struct {
		Found      bool          `json:"found"`
		Infected   bool          `json:"infected"`
		Status     string        `json:"status"`
		Matches    []interface{} `json:"matches"`
		Detections []interface{} `json:"detections"`
	}
	_ = json.Unmarshal(res, &r)
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
	return map[string]interface{}{
		"file":             doc.File,
		"scan_date":        doc.ScanDate,
		"verdict":          verdict,
		"engines_reported": reported,
		"detections":       detections,
	}
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
