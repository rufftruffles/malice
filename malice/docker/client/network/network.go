package network

import (
	"context"
	"regexp"

	log "github.com/sirupsen/logrus"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	apiclient "github.com/moby/moby/client"
	"github.com/maliceio/malice/config"
	"github.com/maliceio/malice/malice/docker/client"
)

// Exists returns network.Summary and true
// if the network name exists, otherwise false.
func Exists(docker *client.Docker, name string) (network.Summary, bool, error) {
	return parseNetworks(docker, name, true)
}

// Create creates a docker Network with the given name
// returns: apiclient.NetworkCreateResult, error
func Create(docker *client.Docker, name string) (apiclient.NetworkCreateResult, error) {
	options := apiclient.NetworkCreateOptions{}
	net, err := docker.Client.NetworkCreate(context.Background(), name, options)
	log.WithFields(log.Fields{
		"name": name,
		"env":  config.Conf.Environment.Run,
	}).Info("Created Network: ", name)
	return net, err
}

// Connect connects a container to a network
func Connect(docker *client.Docker, net network.Summary, container container.InspectResponse) error {
	netConfig := network.EndpointSettings{}
	log.WithFields(log.Fields{
		"env": config.Conf.Environment.Run,
	}).Debugf("Connecting container %s to network %s", container.Name, net.Name)
	_, err := docker.Client.NetworkConnect(context.Background(), net.ID, apiclient.NetworkConnectOptions{
		Container:      container.ID,
		EndpointConfig: &netConfig,
	})
	return err
}

// parseNetworks parses the networks
func parseNetworks(docker *client.Docker, name string, all bool) (network.Summary, bool, error) {
	// list networks
	log.WithFields(log.Fields{"env": config.Conf.Environment.Run}).Debug("Searching for Network: ", name)
	networks, err := List(docker, all)
	if err != nil {
		return network.Summary{}, false, err
	}
	// locate docker Network that matches name
	r := regexp.MustCompile(name)
	if len(networks) != 0 {
		for _, net := range networks {
			if r.MatchString(net.Name) {
				log.WithFields(log.Fields{"env": config.Conf.Environment.Run}).Debug("Network FOUND: ", name)
				return net, true, nil
			}
		}
	}
	log.WithFields(log.Fields{"env": config.Conf.Environment.Run}).Debug("Network NOT Found: ", name)
	return network.Summary{}, false, nil
}

// List returns array of network.Summary and error
func List(docker *client.Docker, all bool) ([]network.Summary, error) {

	options := apiclient.NetworkListOptions{}
	result, err := docker.Client.NetworkList(context.Background(), options)
	if err != nil {
		return nil, err
	}

	return result.Items, nil
}
