package client

import (
	"os"
	"os/exec"
	"runtime"

	log "github.com/sirupsen/logrus"
	"github.com/malice-plugins/pkgs/utils"
	"github.com/maliceio/malice/config"
	"github.com/moby/moby/client"
	"context"
)

// NOTE: https://github.com/eris-ltd/eris-cli/blob/master/perform/docker_run.go

// Docker is the Malice docker client
type Docker struct {
	Client *client.Client
	ip     string
	port   string
}

// NewDockerClient creates a new Docker Client
func NewDockerClient() *Docker {
	var docker *client.Client
	var ip, port string
	var err error

	switch os := runtime.GOOS; os {
	case "linux":
		log.Debug("Running inside Docker...")
		docker, err = client.NewClientWithOpts(client.WithHost("unix:///var/run/docker.sock"), client.WithAPIVersionNegotiation())
		ip = "localhost"
		port = "2375"
	case "darwin":
		log.Debug("Running on Docker for Mac...")
		docker, err = client.NewClientWithOpts(client.WithHost("unix:///var/run/docker.sock"), client.WithAPIVersionNegotiation())
		ip = "localhost"
		port = "2375"
	case "windows":
		log.Debug("Running on Docker for Windows...")
		docker, err = client.NewClientWithOpts(client.WithHostFromEnv(), client.WithAPIVersionNegotiation())
		if err != nil {
			log.Fatal(err)
		}
		ip, port, err = parseDockerEndoint(utils.Getopt("DOCKER_HOST", config.Conf.Docker.EndPoint))
	default:
		log.Debug("Creating NewEnvClient...")
		docker, err = client.NewClientWithOpts(client.WithHostFromEnv(), client.WithAPIVersionNegotiation())
		if err != nil {
			log.Fatal(err)
		}
		ip, port, err = parseDockerEndoint(utils.Getopt("DOCKER_HOST", config.Conf.Docker.EndPoint))
	}
	if err != nil {
		log.Fatal(err)
	}
	// Check if client can connect
	if _, err = docker.Info(context.Background(), client.InfoOptions{}); err != nil {
		handleClientError(err)
	} else {
		log.WithFields(log.Fields{"ip": ip, "port": port}).Debug("Connected to docker daemon client")
	}

	return &Docker{
		Client: docker,
		ip:     ip,
		port:   port,
	}
}

// GetIP returns IP of docker client
func (docker *Docker) GetIP() string {
	return docker.ip
}

// TODO: Make this betta MUCHO betta
func handleClientError(dockerError error) {
	if dockerError != nil {
		log.WithFields(log.Fields{"env": config.Conf.Environment.Run}).Error("Unable to connect to docker client")
		switch runtime.GOOS {
		case "darwin":
			if _, err := os.Stat("/Applications/Docker.app"); os.IsNotExist(err) {
				log.Info("Please install Docker for Mac - https://docs.docker.com/docker-for-mac/")
			} else {
				log.Info("Please start Docker for Mac.")
			}
		case "linux":
			log.Info("Please start the docker daemon. `sudo service docker start`")
		case "windows":
			if _, err := exec.LookPath("/Applications/Docker.app"); err != nil {
				log.Info("Please install Docker for Windows - https://docs.docker.com/docker-for-windows/")
			} else {
				log.Info("Please start Docker for Windows.")
			}
		}
		os.Exit(2)
	}
}
