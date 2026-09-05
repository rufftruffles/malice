package container

import (
	"context"
	"strings"

	"github.com/maliceio/malice/config"
	"github.com/maliceio/malice/malice/docker/client"
	er "github.com/maliceio/malice/malice/errors"
	"github.com/moby/moby/api/types/container"
	apiclient "github.com/moby/moby/client"
	log "github.com/sirupsen/logrus"
)

// List returns array of container.Summary and error
func List(docker *client.Docker, all bool) ([]container.Summary, error) {
	options := apiclient.ContainerListOptions{
		All: true,
	}
	result, err := docker.Client.ContainerList(context.Background(), options)
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// Inspect returns container.InspectResponse from Container ID
// if the container name exists, otherwise false.
func Inspect(docker *client.Docker, id string) (container.InspectResponse, error) {
	result, err := docker.Client.ContainerInspect(context.Background(), id, apiclient.ContainerInspectOptions{})
	if err != nil {
		return container.InspectResponse{}, err
	}
	return result.Container, nil
}

// Exists returns container.Summary and true
// if the container name exists, otherwise false.
func Exists(docker *client.Docker, name string) (container.Summary, bool, error) {
	return parseContainers(docker, strings.TrimLeft(name, "/"), true)
}

// Running returns container.Summary and true
// if the container name exists and is running, otherwise false.
func Running(docker *client.Docker, name string) (container.Summary, bool, error) {
	return parseContainers(docker, strings.TrimLeft(name, "/"), false)
}

func parseContainers(docker *client.Docker, name string, all bool) (container.Summary, bool, error) {
	// list containers
	log.WithFields(log.Fields{"env": config.Conf.Environment.Run}).Debug("Searching for container: ", name)
	containers, err := List(docker, all)
	if err != nil {
		return container.Summary{}, false, err
	}
	// locate docker container that matches name
	if len(containers) != 0 {
		for _, cont := range containers {
			contJSON, err := Inspect(docker, cont.ID)
			er.CheckError(err)

			log.Debugln("name: ", name, " ", "container.Name: ", strings.TrimLeft(contJSON.Name, "/"))
			log.Debugln("MATCH: ", strings.EqualFold(strings.TrimLeft(contJSON.Name, "/"), name))

			if strings.EqualFold(strings.TrimLeft(contJSON.Name, "/"), name) {
				log.WithFields(log.Fields{"env": config.Conf.Environment.Run}).Debug("Container FOUND: ", name)
				return cont, true, nil
			}
		}
	}
	log.WithFields(log.Fields{"env": config.Conf.Environment.Run}).Debug("Container NOT Found: ", name)
	return container.Summary{}, false, nil
}
