# Malice

> A free, open-source, multi-engine malware scanner. Malice orchestrates a fleet of
> antivirus and analysis engines in Docker, runs them all against a single file, and
> stores the combined results in Elasticsearch — a self-hosted VirusTotal.

[![License: Apache-2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

## Why

Malice is a free, open-source alternative to VirusTotal that anyone can run at any
scale — from an independent researcher to a security team — without sending files to a
third party.

## How it works

Malice is a thin Go client. It does not run any AV engine itself. For each file it:

1. Hashes the file (MD5 / SHA-1 / SHA-256 / SHA-512).
2. Copies it into a Docker volume.
3. Runs every enabled engine that supports the file MIME type as an isolated Docker
   container, plus a set of hash-lookup "intel" engines.
4. Collects each engine JSON result and stores one scan document in Elasticsearch.

The result is one document per file: file metadata + hashes, and a per-engine result tree.

## Engines

17 engines across 6 categories:

| Category   | Engine        | What it does |
|------------|---------------|--------------|
| av         | capa          | CAPA — capability detection (mandiant/capa) |
| av         | clamav        | ClamAV |
| av         | eset          | ESET Endpoint Antivirus (EEA) on-demand scan |
| av         | kvrt          | Kaspersky Virus Removal Tool (Linux) |
| av         | lmd           | Linux Malware Detect (signature-based) |
| av         | yara          | YARA rule scan |
| document   | office        | Triage OLE/RTF documents |
| document   | pdf           | Triage PDF documents |
| exe        | diec          | Detect-It-Easy — file type + packer/protector ID |
| exe        | floss         | FireEye FLOSS — obfuscated string solver |
| exe        | pescan        | Triage portable executables (PE) |
| exe        | rizin         | rizin (rz-bin) — headers, sections, imports, exports, strings |
| intel      | hashlookup    | CIRCL hashlookup — NSRL/known-file lookup |
| intel      | nsrl          | NSRL database hash search |
| intel      | shadow-server | ShadowServer hash lookup |
| intel      | virustotal    | VirusTotal file scan + hash lookup (needs API key) |
| metadata   | fileinfo      | ssdeep / TRiD / exiftool |

AV engines report a positive detection; `intel` engines report whether the hash is known;
`document` / `exe` / `metadata` engines report analysis, not a threat verdict.

## Quick start (Docker Compose)

```bash
docker compose up -d
# UI + API: http://localhost:3993
```

This starts Elasticsearch 8 and the Malice client. Malice pulls and runs the engine
images on demand (the first scan of a new engine type is slower while the image downloads).

> The Malice client talks to the Docker daemon to orchestrate the engine containers, so
> the compose file mounts `/var/run/docker.sock`.

## CLI

```bash
malice scan <file>          # scan a file with all enabled engines
malice plugin list          # list engines
malice plugin install <e>   # pull an engine image
malice lookup <hash>        # run the intel engines against a hash
malice serve --port 3993    # start the web UI + REST API
```

## REST API

| Method | Path                  | Description |
|--------|-----------------------|-------------|
| GET    | /api/health           | Service + ES health, engine count |
| GET    | /api/scans?from&size  | List scans (newest first) |
| GET    | /api/scans/:id        | Full scan document + per-engine results |
| POST   | /api/scans            | Upload a file (multipart `file`) and start a scan |
| GET    | /api/plugins          | List engines |

## Web UI

A dependency-free single-page app served at `/`:

- **Scans** — list, upload, and drill into any scan (verdict, file + hashes, per-engine grid).
- **Engines** — the full engine roster grouped by category.

## Development

```bash
make build   # embed config + build malice-bin
make test    # run tests
make lint    # go vet + gofmt
make ci      # lint + test
```

### Building the binary

Malice depends on `github.com/malice-plugins/pkgs` (shared Elasticsearch + Docker
helpers). This repository pins it to a local sibling checkout via a `go.mod` `replace`:

```
replace github.com/malice-plugins/pkgs => ../malice-plugins
```

So a build expects a `malice-plugins/` directory next to `malice/`. For a published
release this `replace` is dropped in favor of the tagged `pkgs` module.

## License

Apache 2.0 — see [LICENSE](LICENSE).
