package container

import (
	"context"
	"errors"
	"os"

	"github.com/maliceio/malice/config"
	"github.com/maliceio/malice/malice/docker/client"
	er "github.com/maliceio/malice/malice/errors"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/api/types/strslice"
	apiclient "github.com/moby/moby/client"
	log "github.com/sirupsen/logrus"
)

// Start starts a malice docker container
func Start(
	docker *client.Docker,
	cmd strslice.StrSlice,
	name string,
	image string,
	logs bool,
	binds []string,
	portBindings network.PortMap,
	links []string,
	env []string,
) (container.InspectResponse, error) {

	if docker.Ping() {
		// Check that all requirements for the container to run are ready
		if !checkContainerRequirements(docker, name, image) {
			return container.InspectResponse{}, errors.New("container is already running")
		}

		createContConf := &container.Config{
			Image: image,
			Cmd:   cmd,
			Env:   env,
		}
		hostConfig := &container.HostConfig{
			Binds:        binds,
			PortBindings: portBindings,
			Links:        links,
			Privileged:   false,
		}
		networkingConfig := &network.NetworkingConfig{}
		if net := os.Getenv("MALICE_DOCKER_NETWORK"); net != "" {
			networkingConfig.EndpointsConfig = map[string]*network.EndpointSettings{net: {}}
		}

		contResponse, err := docker.Client.ContainerCreate(context.Background(), apiclient.ContainerCreateOptions{
			Config:           createContConf,
			HostConfig:       hostConfig,
			NetworkingConfig: networkingConfig,
			Name:             name,
		})
		if err != nil {
			log.WithFields(log.Fields{"env": config.Conf.Environment.Run}).Errorf("CreateContainer error = %s\n", err)
		}

		_, err = docker.Client.ContainerStart(context.Background(), contResponse.ID, apiclient.ContainerStartOptions{})
		if err != nil {
			log.WithFields(log.Fields{"env": config.Conf.Environment.Run}).Errorf("StartContainer error = %s\n", err)
		}

		if logs {
			LogContainer(docker, contResponse.ID)
		}

		contJSON, err := Inspect(docker, contResponse.ID)
		return contJSON, err
	}
	return container.InspectResponse{}, errors.New("Cannot connect to the Docker daemon. Is the docker daemon running on this host?")
}

// LogContainer tails container logs to terminal
func LogContainer(docker *client.Docker, contID string) {

	options := apiclient.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	}

	logs, err := docker.Client.ContainerLogs(context.Background(), contID, options)
	defer logs.Close()
	er.CheckError(err)

	_, err = stdcopy.StdCopy(os.Stdout, os.Stderr, logs)
	er.CheckError(err)
}
