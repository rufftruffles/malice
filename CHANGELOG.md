# Changelog

## 1.0.0 (2026-09-05)

Revival of the abandoned maliceio/malice project (last release 2019).

### Core

- Build system: Dep (dead) to Go modules, Go 1.26+
- Docker client: v17.10 SDK to Docker SDK 29 (moby/moby client + docker/cli 29)
- Backend: Elasticsearch 6.5 to Elasticsearch 8 (official Go client; same `malice`
  index and document shape)
- Removed dead code: the 2016 React skeleton, docker/machine, the
  elk command

### Engines

- 17 engines rebuilt from source and verified end to end
- Dropped 14 commercial AV engines from the original roster; none offers a free
  Linux CLI anymore
- Added: CAPA, DIE, rizin, CIRCL hashlookup, Kaspersky KVRT, Linux Malware Detect,
  ESET EEA 13.2
- Rebuilt: ClamAV, YARA (yara-x), FLOSS 3.x, pescan (Authenticode via pefile +
  asn1crypto), office (oletools), pdf (pdfid + pdf-parser), NSRL (RDSv3 SQLite)
- VirusTotal: v3 API, opt-in key (the hardcoded key is gone)
- ESET: on-demand scan (odscan), opt-in license, signatures refreshed on every scan

### Web + API

- New web UI: scans, engines, settings
- REST API: `/api/scans`, `/api/plugins`, `/api/settings`, `/api/health`
- Settings page for the VirusTotal and ESET credentials (0600 file store, masked
  display, values never logged)

### Deploy + updates

- `deploy.sh`: one-command deployment (spec check, Docker install, image pull,
  compose up, firewall, URL)
- `docker-compose.yml`: Elasticsearch 8 + server
- Nightly rebuild of the signature-decaying engines (clamav, kvrt, yara, lmd) with
  push to GHCR; client-side daily image refresh timer
- Engine images published to GHCR as `ghcr.io/rufftruffles/malice-<engine>`
