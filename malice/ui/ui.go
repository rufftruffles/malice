package ui

import (
	"errors"
	"net/netip"

	log "github.com/sirupsen/logrus"
	contapi "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/maliceio/malice/config"
	"github.com/maliceio/malice/malice/docker/client"
	"github.com/maliceio/malice/malice/docker/client/container"
)

// Start creates an Kibana container from the image blacktop/kibana:malice
func Start(docker *client.Docker, logs bool) (contapi.InspectResponse, error) {

	portBindings := network.PortMap{
		network.MustParsePort("5601/tcp"): {{HostIP: netip.MustParseAddr("0.0.0.0"), HostPort: "80"}},
	}

	if docker.Ping() {
		contJSON, err := container.Start(
			docker,                             // docker *client.Docker,
			nil,                                // cmd strslice.StrSlice,
			config.Conf.UI.Name,                // name string,
			config.Conf.UI.Image,               // image string,
			logs,                               // logs bool,
			nil,                                // binds []string,
			portBindings,                       // portBindings network.PortMap,
			[]string{config.Conf.Docker.Links}, // links []string,
			nil,                                // env []string,
		)
		log.WithFields(log.Fields{
			"ip":   docker.GetIP(),
			"port": config.Conf.UI.Ports,
			"name": contJSON.Name,
			"env":  config.Conf.Environment.Run,
		}).Info("Kibana Container Started")

		return contJSON, err
	}
	return contapi.InspectResponse{}, errors.New("Cannot connect to the Docker daemon. Is the docker daemon running on this host?")
}

// Init initalizes Kibana for use with malice
func Init(addr string) error {

	var err error

	return err
}
