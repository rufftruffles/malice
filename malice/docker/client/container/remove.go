package container

import (
	"context"

	log "github.com/sirupsen/logrus"
	apiclient "github.com/moby/moby/client"
	"github.com/maliceio/malice/malice/docker/client"
)

// Remove removes the `cont` container unforcedly.
// If volumes is true, the associated volumes are removed with container.
// If links is true, the associated links are removed with container.
// If force is true, the container will be destroyed with extreme prejudice.
func Remove(docker *client.Docker, contID string, volumes bool, links bool, force bool) error {
	log.Debug("Removing container: ", contID)
	return removeContainer(docker, context.Background(), contID, volumes, links, force)
}

// removeContainer
func removeContainer(docker *client.Docker, ctx context.Context, container string, removeVolumes, removeLinks, force bool) error {
	options := apiclient.ContainerRemoveOptions{
		RemoveVolumes: removeVolumes,
		RemoveLinks:   removeLinks,
		Force:         force,
	}
	if _, err := docker.Client.ContainerRemove(ctx, container, options); err != nil {
		return err
	}
	return nil
}
