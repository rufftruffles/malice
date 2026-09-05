package commands

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/fatih/structs"
	"github.com/malice-plugins/pkgs/database/elasticsearch"
	"github.com/malice-plugins/pkgs/utils"
	"github.com/maliceio/malice/config"
	"github.com/maliceio/malice/malice/database"
	"github.com/maliceio/malice/malice/docker/client"
	"github.com/maliceio/malice/malice/docker/client/container"
	"github.com/maliceio/malice/malice/persist"
	"github.com/maliceio/malice/plugins"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// cmdScan scans a sample with all appropriate malice plugins. When interactive
// is false (the REST API path) it never prompts on stdin and never log.Fatals —
// it returns an error instead, so a bad input or a missing image can't hang or
// kill the web server.
type scanCtx struct {
	path   string
	scanID string
	file   *persist.File
	docker *client.Docker
	es     *elasticsearch.Database
}

// scanSetup does the fast setup: docker client, ES database, plugin check, file
// init (hashes), and copy to the malice volume. It does NOT store the file info
// or run any engine, so it is safe to call more than once for the same file.
func scanSetup(path string, logs, interactive bool) (*scanCtx, error) {
	if len(path) == 0 {
		return nil, errors.New("please supply a valid file to scan")
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if interactive {
			log.Fatal(path + ": no such file or directory")
		}
		return nil, errors.New(path + ": no such file or directory")
	}

	docker := client.NewDockerClient()

	// NOTE: the old "clean stale containers" loop here matched the literal
	// name "malice" against Docker names (which carry a leading "/"), so it
	// never matched anything. It is also unsafe under concurrent scans. Helper
	// and plugin containers are uniquely named per scan and removed by their own
	// defer, so no startup sweep is needed.
	es := elasticsearch.Database{
		Index:    utils.Getopt("MALICE_ELASTICSEARCH_INDEX", "malice"),
		URL:      utils.Getopt("MALICE_ELASTICSEARCH_URL", config.Conf.DB.URL),
		Username: utils.Getopt("MALICE_ELASTICSEARCH_USERNAME", config.Conf.DB.Username),
		Password: utils.Getopt("MALICE_ELASTICSEARCH_PASSWORD", config.Conf.DB.Password),
	}

	// This assumes you haven't set up an elasticsearch instance and that malice should create one
	if strings.EqualFold(es.URL, "http://localhost:9200") {
		// Check that database is running
		if _, running, _ := container.Running(docker, config.Conf.DB.Name); !running {
			log.Error("database is NOT running, starting now...")
			err := database.Start(docker, es, logs)
			if err != nil {
				return nil, errors.Wrap(err, "failed to start database")
			}
		}
	}

	// Initialize the malice database
	es.Init()

	// Check Plugin Status
	if plugins.InstalledPluginsCheck(docker) {
		log.Debug("All enabled plugins are installed.")
	} else if !interactive {
		// Never prompt on stdin in the API path - a missing image must
		// surface as an error, not a hang or a log.Fatal that kills the server.
		return nil, errors.New("one or more enabled plugin images are not installed; run 'malice plugin install' first")
	} else {
		// Prompt user to install all plugins?
		fmt.Println("All enabled plugins not installed would you like to install all of them? (yes/no)")
		fmt.Println("[Warning] This can take a while if this is the first time you have ran Malice.")
		if utils.AskForConfirmation() {
			plugins.UpdateEnabledPlugins(docker)
		}
	}

	es.Plugins = database.GetPluginsByCategory()

	file := persist.File{Path: path}
	if err := file.Init(); err != nil {
		return nil, errors.Wrap(err, "failed to initialize file")
	}

	// Output File Hashes
	file.ToMarkdownTable()

	// Copy file to malice volume
	container.CopyToVolume(docker, file)

	// Record the file's MIME type on the doc so the API can report how many
	// engines apply to this file type (the progress denominator). Best effort:
	// a failed detection leaves the field empty and the client falls back to
	// the total engine count.
	if mime, err := persist.GetMimeType(docker, file.SHA256); err == nil {
		file.MimeType = mime
	} else {
		log.Warnf("mime detection failed: %v", err)
	}

	return &scanCtx{path: path, file: &file, docker: docker, es: &es}, nil
}

// scanStore writes the file info to the database and records the returned scan
// id on the context. It is fast (a single ES index call).
func scanStore(ctx *scanCtx) error {
	resp, err := ctx.es.StoreFileInfo(structs.Map(*ctx.file))
	if err != nil {
		return errors.Wrap(err, "scan cmd failed to store file info")
	}
	ctx.scanID = resp.Id
	return nil
}

// scanRun fans out to every applicable engine and blocks until they all report.
// It is slow (tens of seconds to minutes) and must run in the background on the
// API path.
func scanRun(ctx *scanCtx) error {
	elasticsearchInDocker := strings.EqualFold(ctx.es.URL, "http://localhost:9200")

	// Run all Intel Plugins on the md5 hash associated with the file
	plugins.RunIntelPlugins(ctx.docker, ctx.file.SHA1, ctx.scanID, true, elasticsearchInDocker)

	// Get file's mime type (already detected in scanSetup; re-detect only
	// if the field is empty)
	mimeType := ctx.file.MimeType
	if mimeType == "" {
		var err error
		mimeType, err = persist.GetMimeType(ctx.docker, ctx.file.SHA256)
		if err != nil {
			return errors.Wrap(err, "failed to get file's mime type")
		}
	}

	log.Debug("looking for plugins that will run on: ", mimeType)
	// Iterate over all applicable installed plugins
	pluginsForMime := plugins.GetPluginsForMime(mimeType, true)
	log.Debug("found these plugins: ")
	for _, plugin := range pluginsForMime {
		log.Debugf(" - %v", plugin.Name)
	}

	var wg sync.WaitGroup
	wg.Add(len(pluginsForMime))

	for _, plugin := range pluginsForMime {
		log.Debugf(">>>>> RUNNING Plugin: %s >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>", plugin.Name)
		go plugin.StartPlugin(ctx.docker, ctx.file.SHA256, ctx.scanID, true, elasticsearchInDocker, &wg)
	}

	wg.Wait() // this waits for the counter to be 0
	log.Debug("Done with plugins.")
	return nil
}

// cmdScan scans a sample with all appropriate malice plugins. When interactive
// is false (the REST API path) it never prompts on stdin and never log.Fatals -
// it returns an error instead, so a bad input or a missing image can't hang or
// kill the web server.
func cmdScan(path string, logs, interactive bool) error {
	ctx, err := scanSetup(path, logs, interactive)
	if err != nil {
		return err
	}
	if err := scanStore(ctx); err != nil {
		return err
	}
	return scanRun(ctx)
}

// APIScanInit does the fast setup + file store and returns the scan id. The API
// calls it synchronously so the id can be returned in the upload response; the
// web client keys its in-flight set by scan id (not SHA) so re-uploading a file
// with a SHA that already has completed scans does not mask those verdicts.
func APIScanInit(path string) (string, error) {
	ctx, err := scanSetup(path, false, false)
	if err != nil {
		return "", err
	}
	if err := scanStore(ctx); err != nil {
		return "", err
	}
	return ctx.scanID, nil
}

// APIScanRun re-runs the fast setup (to rebuild the docker/ES/file context) and
// then fans out to the engines in the background, using the scan id that
// APIScanInit already stored. It must not store the file info again, or it would
// create a second scan doc for the same upload.
func APIScanRun(path string, scanID string) error {
	ctx, err := scanSetup(path, false, false)
	if err != nil {
		return err
	}
	ctx.scanID = scanID
	return scanRun(ctx)
}

// APIScan is a non-interactive API wrapper for cmdScan.
func APIScan(file string) error {
	return cmdScan(file, false, false)
}
