package commands

import (
	"fmt"
	"os"
	"strconv"

	"github.com/maliceio/malice/api"
	"github.com/urfave/cli/v2"
)

// cmdServe starts the Malice web UI + REST API.
func cmdServe(c *cli.Context) error {
	port := c.Int("port")
	if p := os.Getenv("MALICE_WEB_PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			port = v
		}
	}
	addr := fmt.Sprintf("0.0.0.0:%d", port)

	api.Init()
	api.SetScanFunc(APIScan)
	return api.Start(addr)
}

var _ = cli.StringFlag{}
