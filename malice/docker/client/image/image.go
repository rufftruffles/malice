package image

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/docker/cli/cli/command/image/build"
	"github.com/moby/moby/api/types/jsonstream"
	imagetypes "github.com/moby/moby/api/types/image"
	registrytypes "github.com/moby/moby/api/types/registry"
	apiclient "github.com/moby/moby/client"
	"github.com/moby/moby/client/pkg/jsonmessage"
	"github.com/moby/moby/client/pkg/progress"
	"github.com/moby/moby/client/pkg/streamformatter"
	"github.com/moby/go-archive"
	"github.com/moby/go-archive/compression"
	"github.com/moby/patternmatcher"
	"github.com/maliceio/malice/config"
	"github.com/maliceio/malice/malice/docker/client"
	er "github.com/maliceio/malice/malice/errors"

	log "github.com/sirupsen/logrus"
)

// defaultSearchLimit is the default number of results to return from a search
const defaultSearchLimit = 25

// Pull pulls docker image:tag
// TODO: add trusted pull for offcial malice plugins
func Pull(docker *client.Docker, id string, tag string) {

	responseBody, err := docker.Client.ImagePull(context.Background(), id, apiclient.ImagePullOptions{})
	defer responseBody.Close()
	er.CheckError(err)

	jsonmessage.DisplayJSONMessagesStream(responseBody, os.Stdout, os.Stdout.Fd(), true, nil)
}

// Build builds docker image from git repository
func Build(docker *client.Docker, repository string, tags []string, buildArgs map[string]*string, labels map[string]string, quiet bool) {

	var (
		buildCtx io.ReadCloser
		err      error
	)

	var (
		contextDir    string
		tempDir       string
		relDockerfile string
		progBuff      io.Writer
		buildBuff     io.Writer
	)

	progBuff = os.Stdout
	buildBuff = os.Stdout

	ctxType, err := build.DetectContextType(repository)
	if err != nil {
		er.CheckError(err)
	}

	switch ctxType {
	case build.ContextTypeStdin:
		buildCtx, relDockerfile, err = build.GetContextFromReader(os.Stdin, "")
	case build.ContextTypeGit:
		tempDir, relDockerfile, err = build.GetContextFromGitURL(repository, "")
	case build.ContextTypeRemote:
		buildCtx, relDockerfile, err = build.GetContextFromURL(progBuff, repository, "")
	default:
		_, relDockerfile, err = build.GetContextFromLocalDir(repository, "")
	}

	if tempDir != "" {
		defer os.RemoveAll(tempDir)
		contextDir = tempDir
	}

	if buildCtx == nil {
		// And canonicalize dockerfile name to a platform-independent one
		relDockerfile = filepath.ToSlash(relDockerfile)

		var excludes []string
		if excludes, err = build.ReadDockerignore(contextDir); err != nil {
			er.CheckError(err)
		}

		if err := build.ValidateContextDirectory(contextDir, excludes); err != nil {
			log.Fatalf("Error checking context: '%s'.", err)
		}

		// If .dockerignore mentions .dockerignore or the Dockerfile
		// then make sure we send both files over to the daemon
		// because Dockerfile is, obviously, needed no matter what, and
		// .dockerignore is needed to know if either one needs to be
		// removed. The daemon will remove them for us, if needed, after it
		// parses the Dockerfile. Ignore errors here, as they will have been
		// caught by validateContextDirectory above.
		var includes = []string{"."}
		keepThem1, _ := patternmatcher.Matches(".dockerignore", excludes)
		keepThem2, _ := patternmatcher.Matches(relDockerfile, excludes)
		if keepThem1 || keepThem2 {
			includes = append(includes, ".dockerignore", relDockerfile)
		}

		buildCtx, err = archive.TarWithOptions(contextDir, &archive.TarOptions{
			Compression:     compression.None,
			ExcludePatterns: excludes,
			IncludeFiles:    includes,
		})
		er.CheckError(err)
	}

	// Setup an upload progress bar
	progressOutput := streamformatter.NewProgressOutput(progBuff)

	var body io.Reader = progress.NewProgressReader(buildCtx, progressOutput, 0, "", "Sending build context to Docker daemon")

	buildOptions := apiclient.ImageBuildOptions{
		Tags:           tags,
		SuppressOutput: quiet,
		NoCache:        true,
		Dockerfile:     relDockerfile,
		BuildArgs:      buildArgs,
		Labels:         labels,
	}
	response, err := docker.Client.ImageBuild(context.Background(), body, buildOptions)
	defer response.Body.Close()
	er.CheckError(err)

	err = jsonmessage.DisplayJSONMessagesStream(response.Body, buildBuff, os.Stdout.Fd(), true, nil)
	if err != nil {
		if jerr, ok := err.(*jsonstream.Error); ok {
			// If no error code is set, default to 1
			if jerr.Code == 0 {
				jerr.Code = 1
			}
			if quiet {
				fmt.Fprintf(os.Stderr, "%s%s", progBuff, buildBuff)
			}
		}
	}
}

// Exists returns APIImages images list and true
// if the image name exists, otherwise false.
func Exists(docker *client.Docker, name string) (imagetypes.Summary, bool, error) {
	log.WithFields(log.Fields{"env": config.Conf.Environment.Run}).Debug("Searching for image: ", name)
	images, err := List(docker, name, false)
	if err != nil {
		return imagetypes.Summary{}, false, err
	}

	r := regexp.MustCompile(name)
	if len(images) != 0 {
		for _, image := range images {
			for _, tag := range image.RepoTags {
				if r.MatchString(tag) {
					log.WithFields(log.Fields{"env": config.Conf.Environment.Run}).Debug("Image FOUND: ", name)
					return image, true, nil
				}
			}
		}
	}

	log.WithFields(log.Fields{"env": config.Conf.Environment.Run}).Debug("Image NOT Found: ", name)
	return imagetypes.Summary{}, false, nil
}

// List lists all images
func List(docker *client.Docker, name string, all bool) ([]imagetypes.Summary, error) {

	options := apiclient.ImageListOptions{
		All: all,
	}
	result, err := docker.Client.ImageList(context.Background(), options)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	return result.Items, nil
}

type searchOptions struct {
	term    string
	noTrunc bool
	limit   int
	filter  []string

	// Deprecated
	stars     uint
	automated bool
}

// Search searches for malice images
func Search(docker *client.Docker, term string) error {

	opts := searchOptions{
		term:  term,
		limit: defaultSearchLimit,
	}
	options := apiclient.ImageSearchOptions{
		Limit: opts.limit,
	}

	searchResult, err := docker.Client.ImageSearch(context.Background(), opts.term, options)
	if err != nil {
		return err
	}

	results := searchResultsByStars(searchResult.Items)
	sort.Sort(results)

	w := tabwriter.NewWriter(os.Stdout, 10, 1, 3, ' ', 0)
	fmt.Fprintf(w, "NAME\tDESCRIPTION\tSTARS\tOFFICIAL\tAUTOMATED\n")
	for _, res := range results {
		// --automated and -s, --stars are deprecated since Docker 1.12
		if (opts.automated && !res.IsAutomated) || (int(opts.stars) > res.StarCount) {
			continue
		}
		desc := strings.Replace(res.Description, "\n", " ", -1)
		desc = strings.Replace(desc, "\r", " ", -1)
		if !opts.noTrunc && len(desc) > 45 {
			desc = desc[:42] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t", res.Name, desc, res.StarCount)
		if res.IsOfficial {
			fmt.Fprint(w, "[OK]")

		}
		fmt.Fprint(w, "\t")
		if res.IsAutomated {
			fmt.Fprint(w, "[OK]")
		}
		fmt.Fprint(w, "\n")
	}
	w.Flush()
	return nil
}

// SearchResultsByStars sorts search results in descending order by number of stars.
type searchResultsByStars []registrytypes.SearchResult

func (r searchResultsByStars) Len() int           { return len(r) }
func (r searchResultsByStars) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r searchResultsByStars) Less(i, j int) bool { return r[j].StarCount < r[i].StarCount }
