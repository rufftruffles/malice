package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/malice-plugins/pkgs/utils"
	"github.com/maliceio/malice/config"
)

var (
	esURL   string
	esIndex string
	esHTTP  = &http.Client{Timeout: 30 * time.Second}
)

// InitES configures the Elasticsearch endpoint from env or config.
func InitES() {
	esURL = utils.Getopt("MALICE_ELASTICSEARCH_URL", config.Conf.DB.URL)
	esIndex = utils.Getopt("MALICE_ELASTICSEARCH_INDEX", "malice")
}

// ESIndex returns the configured index name.
func ESIndex() string { return esIndex }

func esRequest(method, path string, body []byte) (int, []byte, error) {
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, esURL+path, rd)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := esHTTP.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	return res.StatusCode, data, nil
}

// ESHealthy reports whether the Elasticsearch cluster answers.
func ESHealthy() bool {
	code, _, err := esRequest("GET", "/", nil)
	return err == nil && code == 200
}

// ScanHit pairs a scan doc id with its raw source.
type ScanHit struct {
	ID     string
	Source json.RawMessage
}

// SearchScans returns raw scan docs (newest first) plus the total count.
func SearchScans(from, size int) ([]ScanHit, int64, error) {
	q := fmt.Sprintf(`{"from":%d,"size":%d,"sort":[{"scan_date":"desc"}]}`, from, size)
	code, data, err := esRequest("POST", "/"+esIndex+"/_search", []byte(q))
	if err != nil {
		return nil, 0, err
	}
	if code != http.StatusOK {
		return nil, 0, fmt.Errorf("elasticsearch search returned %d: %s", code, truncate(data, 300))
	}
	var out struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				ID     string          `json:"_id"`
				Source json.RawMessage `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, 0, err
	}
	scans := make([]ScanHit, 0, len(out.Hits.Hits))
	for _, h := range out.Hits.Hits {
		scans = append(scans, ScanHit{ID: h.ID, Source: h.Source})
	}
	return scans, out.Hits.Total.Value, nil
}

// GetScan returns the raw source of a single scan doc by id.
func GetScan(id string) (json.RawMessage, error) {
	code, data, err := esRequest("GET", "/"+esIndex+"/_doc/"+id, nil)
	if err != nil {
		return nil, err
	}
	if code == http.StatusNotFound {
		return nil, fmt.Errorf("scan %q not found", id)
	}
	if code != http.StatusOK {
		return nil, fmt.Errorf("elasticsearch get returned %d: %s", code, truncate(data, 300))
	}
	var out struct {
		Source json.RawMessage `json:"_source"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out.Source, nil
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}
