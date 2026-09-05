package container

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	log "github.com/sirupsen/logrus"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/events"
	apiclient "github.com/moby/moby/client"
	"github.com/moby/moby/client/pkg/versions"
	er "github.com/maliceio/malice/malice/errors"

	"github.com/maliceio/malice/config"
	"github.com/maliceio/malice/malice/docker/client"
	"github.com/maliceio/malice/malice/docker/client/image"
	"github.com/maliceio/malice/malice/docker/client/network"
	"github.com/maliceio/malice/malice/docker/client/volume"
)

func noNetHostConfig() *container.HostConfig {
	return &container.HostConfig{
		Privileged:  false,
		NetworkMode: "none",
	}
}

func getResources() container.Resources {
	return container.Resources{
		Memory:   config.Conf.Docker.Memory, // Memory    int64 // Memory limit (in bytes)
		NanoCPUs: config.Conf.Docker.CPU,    // NanoCPUs  int64 `json:"NanoCpus"` // CPU quota in units of 10<sup>-9</sup> CPUs.
	}
}

func waitExitOrRemoved(ctx context.Context, docker *client.Docker, containerID string, waitRemove bool) chan int {
	if len(containerID) == 0 {
		// containerID can never be empty
		panic("Internal Error: waitExitOrRemoved needs a containerID as parameter")
	}

	var removeErr error
	statusChan := make(chan int)
	exitCode := 125

	// Get events via Events API
	f := make(apiclient.Filters)
	f.Add("type", "container")
	f.Add("container", containerID)
	options := apiclient.EventsListOptions{
		Filters: f,
	}
	eventCtx, cancel := context.WithCancel(ctx)
	eventResult := docker.Client.Events(eventCtx, options)
	eventq := eventResult.Messages
	errq := eventResult.Err

	eventProcessor := func(e events.Message) bool {
		stopProcessing := false
		switch e.Action {
		case events.ActionDie:
			if v, ok := e.Actor.Attributes["exitCode"]; ok {
				code, cerr := strconv.Atoi(v)
				if cerr != nil {
					log.Errorf("failed to convert exitcode '%q' to int: %v", v, cerr)
				} else {
					exitCode = code
				}
			}
			if !waitRemove {
				stopProcessing = true
			} else {
				// If we are talking to an older daemon, `AutoRemove` is not supported.
				// We need to fall back to the old behavior, which is client-side removal
				if versions.LessThan(docker.Client.ClientVersion(), "1.25") {
					go func() {
						_, removeErr = docker.Client.ContainerRemove(ctx, containerID, apiclient.ContainerRemoveOptions{RemoveVolumes: true})
						if removeErr != nil {
							log.Errorf("error removing container: %v", removeErr)
							cancel() // cancel the event Q
						}
					}()
				}
			}
		case events.ActionDetach:
			exitCode = 0
			stopProcessing = true
		case events.ActionDestroy:
			stopProcessing = true
		}
		return stopProcessing
	}

	go func() {
		defer func() {
			statusChan <- exitCode // must always send an exit code or the caller will block
			cancel()
		}()

		for {
			select {
			case <-eventCtx.Done():
				if removeErr != nil {
					return
				}
			case evt := <-eventq:
				if eventProcessor(evt) {
					return
				}
			case err := <-errq:
				log.Errorf("error getting events from daemon: %v", err)
				return
			}
		}
	}()

	return statusChan
}

func checkContainerRequirements(docker *client.Docker, containerName, img string) bool {
	// Check for existance of malice network
	if _, exists, _ := network.Exists(docker, "malice"); !exists {
		log.WithFields(log.Fields{
			"network": "malice",
			"exisits": exists,
			"env":     config.Conf.Environment.Run,
		}).Error("Network malice does not exist, creating now...")
		_, err := network.Create(docker, "malice")
		er.CheckError(err)
	}
	// Check for existance of malice volume
	if _, exists, _ := volume.Exists(docker, "malice"); !exists {
		log.Debug("Volume malice not found.")
		er.CheckError(volume.Create(docker, "malice", "local", nil))
	}
	log.Debug("Volume malice found.")
	// Check that the container isn't already running
	if _, exists, _ := Exists(docker, containerName); exists {
		log.WithFields(log.Fields{
			"exisits": exists,
			"name":    containerName,
			"env":     config.Conf.Environment.Run,
		}).Error("Container is already running...")
		return false
	}
	// Check that we have already pulled the image
	if _, exists, _ := image.Exists(docker, img); exists {
		log.WithFields(log.Fields{
			"exisits": exists,
			"env":     config.Conf.Environment.Run,
		}).Debugf("Image `%s` already pulled.", img)
	} else {
		log.WithFields(log.Fields{
			"exisits": exists,
			"env":     config.Conf.Environment.Run}).Debugf("Pulling Image `%s`", img)
		image.Pull(docker, img, "latest")
	}
	return true
}

// ErrConnectionFailed is an error raised when the connection between the client and the server failed.
var ErrConnectionFailed = errors.New("Cannot connect to the Docker daemon. Is the docker daemon running on this host?")

// ErrorConnectionFailed returns an error with host in the error message when connection to docker daemon failed.
func ErrorConnectionFailed(host string) error {
	return fmt.Errorf("Cannot connect to the Docker daemon at %s. Is the docker daemon running?", host)
}

// getExitCode performs an inspect on the container. It returns
// the running state and the exit code.
func getExitCode(docker *client.Docker, ctx context.Context, containerID string) (bool, int, error) {
	c, err := Inspect(docker, containerID)
	if err != nil {
		// If we can't connect, then the daemon probably died.
		if err != ErrConnectionFailed {
			return false, -1, err
		}
		return false, -1, nil
	}
	return c.State.Running, c.State.ExitCode, nil
}
