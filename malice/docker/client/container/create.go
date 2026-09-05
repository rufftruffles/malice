package container

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/containerd/errdefs"
	"github.com/distribution/reference"
	"github.com/maliceio/malice/malice/docker/client"
	er "github.com/maliceio/malice/malice/errors"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	apiclient "github.com/moby/moby/client"
	"github.com/moby/moby/client/pkg/jsonmessage"
)

func pullImage(ctx context.Context, docker *client.Docker, image string, out io.Writer) error {
	// Validate the image reference.
	if _, err := reference.ParseNormalizedNamed(image); err != nil {
		return err
	}

	responseBody, err := docker.Client.ImagePull(ctx, image, apiclient.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer responseBody.Close()

	return jsonmessage.DisplayJSONMessagesStream(
		responseBody,
		out,
		os.Stdout.Fd(),
		true,
		nil)
}

type cidFile struct {
	path    string
	file    *os.File
	written bool
}

func (cid *cidFile) Close() error {
	cid.file.Close()

	if !cid.written {
		if err := os.Remove(cid.path); err != nil {
			return fmt.Errorf("failed to remove the CID file '%s': %s \n", cid.path, err)
		}
	}

	return nil
}

func (cid *cidFile) Write(id string) error {
	if _, err := cid.file.Write([]byte(id)); err != nil {
		return fmt.Errorf("Failed to write the container ID to the file: %s", err)
	}
	cid.written = true
	return nil
}

func newCIDFile(path string) (*cidFile, error) {
	if _, err := os.Stat(path); err == nil {
		return nil, fmt.Errorf("Container ID file found, make sure the other container isn't running or delete %s", path)
	}

	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("Failed to create the container ID file: %s", err)
	}

	return &cidFile{path: path, file: f}, nil
}

// createContainer
func createContainer(docker *client.Docker, ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, cidfile, name string) (apiclient.ContainerCreateResult, error) {
	stderr := os.Stderr
	var containerIDFile *cidFile
	if cidfile != "" {
		var err error
		if containerIDFile, err = newCIDFile(cidfile); err != nil {
			er.CheckError(err)
			return apiclient.ContainerCreateResult{}, err
		}
		defer containerIDFile.Close()
	}

	//create the container
	response, err := docker.Client.ContainerCreate(ctx, apiclient.ContainerCreateOptions{
		Config:           config,
		HostConfig:       hostConfig,
		NetworkingConfig: networkingConfig,
		Name:             name,
	})
	er.CheckError(err)
	//if image not found try to pull it
	if err != nil {
		if errdefs.IsNotFound(err) {
			// we don't want to write to stdout anything apart from container.ID
			if err = pullImage(ctx, docker, config.Image, stderr); err != nil {
				return apiclient.ContainerCreateResult{}, err
			}
			// Retry
			var retryErr error
			response, retryErr = docker.Client.ContainerCreate(ctx, apiclient.ContainerCreateOptions{
				Config:           config,
				HostConfig:       hostConfig,
				NetworkingConfig: networkingConfig,
				Name:             name,
			})
			if retryErr != nil {
				return apiclient.ContainerCreateResult{}, retryErr
			}
		} else {
			return apiclient.ContainerCreateResult{}, err
		}
	}

	for _, warning := range response.Warnings {
		fmt.Fprintf(stderr, "WARNING: %s\n", warning)
	}
	if containerIDFile != nil {
		if err = containerIDFile.Write(response.ID); err != nil {
			return apiclient.ContainerCreateResult{}, err
		}
	}
	return response, nil
}
