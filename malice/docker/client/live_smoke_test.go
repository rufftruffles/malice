package client

import (
	"context"
	"testing"

	mobyclient "github.com/moby/moby/client"
)

// TestDockerClientLive is a smoke test against a running Docker daemon:
// verifies the migrated client + API version negotiation end-to-end.
// Skipped when no daemon is available (e.g. CI without Docker).
func TestDockerClientLive(t *testing.T) {
	d, err := mobyclient.NewClientWithOpts(
		mobyclient.WithHost("unix:///var/run/docker.sock"),
		mobyclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		t.Skipf("failed to create docker client: %v", err)
	}
	info, err := d.Info(context.Background(), mobyclient.InfoOptions{})
	if err != nil {
		t.Skipf("docker daemon not available: %v", err)
	}
	t.Logf("connected: server %s, %d containers, %d images", info.Info.ServerVersion, info.Info.Containers, info.Info.Images)
}
