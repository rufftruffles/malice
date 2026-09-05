package container

import (
	"context"
	"fmt"
	"os"

	"github.com/cloudflare/cfssl/log"
	"github.com/docker/cli/cli"
	"github.com/maliceio/malice/malice/docker/client"
	er "github.com/maliceio/malice/malice/errors"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/api/types/strslice"
	apiclient "github.com/moby/moby/client"
)

type runOptions struct {
	autoRemove bool
	detach     bool
	sigProxy   bool
	name       string
	detachKeys string
}

// Run performs a docker run command
func Run(
	docker *client.Docker,
	cmd strslice.StrSlice,
	name string,
	image string,
	logs bool,
	binds []string,
	portBindings network.PortMap,
	links []string,
	env []string,
) error {
	var (
		waitDisplayID chan struct{}
		errCh         chan error
		err           error
	)

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
		AutoRemove:   true,
	}

	networkingConfig := &network.NetworkingConfig{}

	ctx, cancelFun := context.WithCancel(context.Background())
	defer cancelFun()

	createResponse, err := createContainer(docker, ctx, createContConf, hostConfig, networkingConfig, hostConfig.ContainerIDFile, name)
	er.CheckError(err)

	// Make this asynchronous to allow the client to write to stdin before having to read the ID
	waitDisplayID = make(chan struct{})
	go func() {
		defer close(waitDisplayID)
		fmt.Fprintf(os.Stdout, "%s\n", createResponse.ID)
	}()

	attach := false
	statusChan := waitExitOrRemoved(ctx, docker, createResponse.ID, true)
	//start the container
	log.Debugf("Starting containter: %s", createResponse.ID)
	if _, err = docker.Client.ContainerStart(ctx, createResponse.ID, apiclient.ContainerStartOptions{}); err != nil {
		// If we have holdHijackedConnection, we should notify
		// holdHijackedConnection we are going to exit and wait
		// to avoid the terminal are not restored.
		if attach {
			cancelFun()
			<-errCh
		}

		<-statusChan

		er.CheckError(err)
	}

	if errCh != nil {
		if err = <-errCh; err != nil {
			log.Debugf("Error hijack: %s", err)
			return err
		}
	}

	// Detached mode: wait for the id to be displayed and return.
	<-waitDisplayID

	status := <-statusChan
	if status != 0 {
		return cli.StatusError{StatusCode: status}
	}
	return nil
}
