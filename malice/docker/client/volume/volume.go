package volume

import (
	"context"
	"regexp"
	"strings"

	"github.com/maliceio/malice/config"
	"github.com/maliceio/malice/malice/docker/client"
	"github.com/moby/moby/api/types/volume"
	apiclient "github.com/moby/moby/client"
	log "github.com/sirupsen/logrus"
)

// Exists returns volume.Volume and true
// if the volume name exists, otherwise false.
func Exists(docker *client.Docker, name string) (*volume.Volume, bool, error) {
	return parseVolumes(docker, name, true)
}

// Create creates a docker volume with the given name
// returns: error
func Create(docker *client.Docker, name, driver string, labels []string) error {
	volReq := apiclient.VolumeCreateOptions{
		Driver: driver,
		Name:   name,
		Labels: convertKVStringsToMap(labels),
	}

	vol, err := docker.Client.VolumeCreate(context.Background(), volReq)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"env": config.Conf.Environment.Run,
	}).Info("Created Volume: ", vol.Volume.Name)

	return nil
}

// ParseVolumes parses the volumes
func parseVolumes(docker *client.Docker, name string, all bool) (*volume.Volume, bool, error) {
	// list volumes
	log.WithFields(log.Fields{"env": config.Conf.Environment.Run}).Debug("Searching for volume: ", name)
	volumes, err := List(docker, all)
	if err != nil {
		return nil, false, err
	}
	// locate docker volume that matches name
	r := regexp.MustCompile(name)
	if len(volumes) != 0 {
		for i := range volumes {
			vol := volumes[i]
			if r.MatchString(vol.Name) {
				log.WithFields(log.Fields{"env": config.Conf.Environment.Run}).Debug("Volume FOUND: ", name)
				return &vol, true, nil
			}
		}
	}
	log.WithFields(log.Fields{"env": config.Conf.Environment.Run}).Debug("Volume NOT Found: ", name)
	return nil, false, nil
}

// List returns array of volume.Volume and error
func List(docker *client.Docker, all bool) ([]volume.Volume, error) {
	result, err := docker.Client.VolumeList(context.Background(), apiclient.VolumeListOptions{})
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// convertKVStringsToMap is vendored from docker's runconfig/opts
// (the package no longer exists in modern docker).
func convertKVStringsToMap(values []string) map[string]string {
	result := make(map[string]string, len(values))
	for _, value := range values {
		key, val, found := strings.Cut(value, "=")
		if !found {
			val = ""
		}
		result[key] = val
	}
	return result
}
