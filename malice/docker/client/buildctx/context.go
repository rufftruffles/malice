// Package buildctx provides build-context helpers for image builds.
//
// Vendored from github.com/docker/cli v20.10.27, cli/command/image/build
// (Apache License 2.0, Copyright Docker, Inc. and contributors), because
// docker/cli cannot compile against docker/docker v17.10 (it imports
// docker/docker/errdefs, which does not exist in that version).
package buildctx

import (
	"archive/tar"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/docker/docker/builder/remotecontext/git"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/fileutils"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/progress"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/pkg/errors"
)

const (
	// DefaultDockerfileName is the default filename with Docker commands, read by docker build
	DefaultDockerfileName = "Dockerfile"
	// archiveHeaderSize is the number of bytes in an archive header
	archiveHeaderSize = 512
)

// ValidateContextDirectory checks if all the contents of the directory
// can be read and returns an error if some files can't be read.
// Symlinks which point to non-existing files don't trigger an error.
func ValidateContextDirectory(srcPath string, excludes []string) error {
	contextRoot := filepath.Join(srcPath, ".")

	pm, err := fileutils.NewPatternMatcher(excludes)
	if err != nil {
		return err
	}

	return filepath.Walk(contextRoot, func(filePath string, f os.FileInfo, err error) error {
		if err != nil {
			if os.IsPermission(err) {
				return errors.Errorf("can't stat '%s'", filePath)
			}
			if os.IsNotExist(err) {
				return errors.Errorf("file ('%s') not found or excluded by .dockerignore", filePath)
			}
			return err
		}

		// skip this directory/file if it's not in the path, it won't get added to the context
		if relFilePath, err := filepath.Rel(contextRoot, filePath); err != nil {
			return err
		} else if skip, err := filepathMatches(pm, relFilePath); err != nil {
			return err
		} else if skip {
			if f.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// skip checking if symlinks point to non-existing files, such symlinks can be useful
		// also skip named pipes, because they hang on open
		if f.Mode()&(os.ModeSymlink|os.ModeNamedPipe) != 0 {
			return nil
		}

		if !f.IsDir() {
			currentFile, err := os.Open(filePath)
			if err != nil && os.IsPermission(err) {
				return errors.Errorf("no permission to read from '%s'", filePath)
			}
			currentFile.Close()
		}
		return nil
	})
}

func filepathMatches(matcher *fileutils.PatternMatcher, file string) (bool, error) {
	file = filepath.Clean(file)
	if file == "." {
		// Don't let them exclude everything, kind of silly.
		return false, nil
	}
	return matcher.Matches(file)
}

// DetectArchiveReader detects whether the input stream is an archive or a
// Dockerfile and returns a buffered version of input, safe to consume in lieu
// of input.
func DetectArchiveReader(input io.ReadCloser) (rc io.ReadCloser, isArchive bool, err error) {
	buf := bufio.NewReader(input)

	magic, err := buf.Peek(archiveHeaderSize * 2)
	if err != nil && err != io.EOF {
		return nil, false, errors.Errorf("failed to peek context header from STDIN: %v", err)
	}

	return ioutils.NewReadCloserWrapper(buf, func() error { return input.Close() }), IsArchive(magic), nil
}

// WriteTempDockerfile writes a Dockerfile stream to a temporary file with a
// name specified by DefaultDockerfileName and returns the path to the
// temporary directory containing the Dockerfile.
func WriteTempDockerfile(rc io.ReadCloser) (dockerfileDir string, err error) {
	dockerfileDir, err = os.MkdirTemp("", "docker-build-tempdockerfile-")
	if err != nil {
		return "", errors.Errorf("unable to create temporary context directory: %v", err)
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(dockerfileDir)
		}
	}()

	f, err := os.Create(filepath.Join(dockerfileDir, DefaultDockerfileName))
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, rc); err != nil {
		return "", err
	}
	return dockerfileDir, rc.Close()
}

// GetContextFromReader will read the contents of the given reader as either a
// Dockerfile or tar archive. Returns a tar archive used as a context and a
// path to the Dockerfile inside the tar.
func GetContextFromReader(rc io.ReadCloser, dockerfileName string) (out io.ReadCloser, relDockerfile string, err error) {
	rc, isArchive, err := DetectArchiveReader(rc)
	if err != nil {
		return nil, "", err
	}

	if isArchive {
		return rc, dockerfileName, nil
	}

	if dockerfileName == "-" {
		return nil, "", errors.New("build context is not an archive")
	}

	dockerfileDir, err := WriteTempDockerfile(rc)
	if err != nil {
		return nil, "", err
	}

	tar, err := archive.Tar(dockerfileDir, archive.Uncompressed)
	if err != nil {
		return nil, "", err
	}

	return ioutils.NewReadCloserWrapper(tar, func() error {
		err := tar.Close()
		os.RemoveAll(dockerfileDir)
		return err
	}), DefaultDockerfileName, nil
}

// IsArchive checks for the magic bytes of a tar or any supported compression
// algorithm.
func IsArchive(header []byte) bool {
	compression := archive.DetectCompression(header)
	if compression != archive.Uncompressed {
		return true
	}
	r := tar.NewReader(bytes.NewBuffer(header))
	_, err := r.Next()
	return err == nil
}

// GetContextFromGitURL uses a Git URL as context for a build. The git repo is
// cloned into a temporary directory used as the context directory.
func GetContextFromGitURL(gitURL, dockerfileName string) (string, string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", "", errors.Wrapf(err, "unable to find 'git'")
	}
	absContextDir, err := git.Clone(gitURL)
	if err != nil {
		return "", "", errors.Wrapf(err, "unable to 'git clone' to temporary context directory")
	}

	absContextDir, err = ResolveAndValidateContextPath(absContextDir)
	if err != nil {
		return "", "", err
	}
	relDockerfile, err := getDockerfileRelPath(absContextDir, dockerfileName)
	if err == nil && strings.HasPrefix(relDockerfile, ".."+string(filepath.Separator)) {
		return "", "", errors.Errorf("the Dockerfile (%s) must be within the build context", dockerfileName)
	}

	return absContextDir, relDockerfile, err
}

// GetContextFromURL uses a remote URL as context for a build. The remote
// resource is downloaded as either a Dockerfile or a tar archive.
func GetContextFromURL(out io.Writer, remoteURL, dockerfileName string) (io.ReadCloser, string, error) {
	response, err := getWithStatusError(remoteURL)
	if err != nil {
		return nil, "", errors.Errorf("unable to download remote context %s: %v", remoteURL, err)
	}
	progressOutput := streamformatter.NewProgressOutput(out)

	progReader := progress.NewProgressReader(response.Body, progressOutput, response.ContentLength, "", fmt.Sprintf("Downloading build context from remote url: %s", remoteURL))

	return GetContextFromReader(ioutils.NewReadCloserWrapper(progReader, func() error { return response.Body.Close() }), dockerfileName)
}

func getWithStatusError(url string) (resp *http.Response, err error) {
	if resp, err = http.Get(url); err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusBadRequest {
		return resp, nil
	}
	msg := fmt.Sprintf("failed to GET %s with status %s", url, resp.Status)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, errors.Wrapf(err, "%s: error reading body", msg)
	}
	return nil, errors.Errorf("%s: %s", msg, bytes.TrimSpace(body))
}

// GetContextFromLocalDir uses the given local directory as context for a
// build. Returns the absolute path to the local context directory, the
// relative path of the dockerfile in that context directory, and a non-nil
// error on failure.
func GetContextFromLocalDir(localDir, dockerfileName string) (string, string, error) {
	localDir, err := ResolveAndValidateContextPath(localDir)
	if err != nil {
		return "", "", err
	}

	if dockerfileName != "" && dockerfileName != "-" {
		if dockerfileName, err = filepath.Abs(dockerfileName); err != nil {
			return "", "", errors.Errorf("unable to get absolute path to Dockerfile: %v", err)
		}
	}

	relDockerfile, err := getDockerfileRelPath(localDir, dockerfileName)
	return localDir, relDockerfile, err
}

// ResolveAndValidateContextPath uses the given context directory for a build
// and returns the absolute path to the context directory.
func ResolveAndValidateContextPath(givenContextDir string) (string, error) {
	absContextDir, err := filepath.Abs(givenContextDir)
	if err != nil {
		return "", errors.Errorf("unable to get absolute context directory of given context directory %q: %v", givenContextDir, err)
	}

	if !isUNC(absContextDir) {
		absContextDir, err = filepath.EvalSymlinks(absContextDir)
		if err != nil {
			return "", errors.Errorf("unable to evaluate symlinks in context path: %v", err)
		}
	}

	stat, err := os.Lstat(absContextDir)
	if err != nil {
		return "", errors.Errorf("unable to stat context directory %q: %v", absContextDir, err)
	}

	if !stat.IsDir() {
		return "", errors.Errorf("context must be a directory: %s", absContextDir)
	}
	return absContextDir, err
}

func getDockerfileRelPath(absContextDir, givenDockerfile string) (string, error) {
	var err error

	if givenDockerfile == "-" {
		return givenDockerfile, nil
	}

	absDockerfile := givenDockerfile
	if absDockerfile == "" {
		absDockerfile = filepath.Join(absContextDir, DefaultDockerfileName)

		// Just to be nice ;-) look for 'dockerfile' too but only
		// use it if we found it, otherwise ignore this check
		if _, err = os.Lstat(absDockerfile); os.IsNotExist(err) {
			altPath := filepath.Join(absContextDir, strings.ToLower(DefaultDockerfileName))
			if _, err = os.Lstat(altPath); err == nil {
				absDockerfile = altPath
			}
		}
	}

	if !filepath.IsAbs(absDockerfile) {
		absDockerfile = filepath.Join(absContextDir, absDockerfile)
	}

	if !isUNC(absDockerfile) {
		absDockerfile, err = filepath.EvalSymlinks(absDockerfile)
		if err != nil {
			return "", errors.Errorf("unable to evaluate symlinks in Dockerfile path: %v", err)
		}
	}

	if _, err := os.Lstat(absDockerfile); err != nil {
		if os.IsNotExist(err) {
			return "", errors.Errorf("Cannot locate Dockerfile: %q", absDockerfile)
		}
		return "", errors.Errorf("unable to stat Dockerfile: %v", err)
	}

	relDockerfile, err := filepath.Rel(absContextDir, absDockerfile)
	if err != nil {
		return "", errors.Errorf("unable to get relative Dockerfile path: %v", err)
	}

	return relDockerfile, nil
}

func isUNC(path string) bool {
	return runtime.GOOS == "windows" && strings.HasPrefix(path, `\\`)
}
