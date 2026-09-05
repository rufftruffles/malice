module github.com/maliceio/malice

go 1.26.3

require (
	github.com/BurntSushi/toml v1.5.0
	github.com/cloudflare/cfssl v1.6.5
	github.com/crackcomm/go-clitable v0.0.0-20221128132333-2488a876f62c
	github.com/docker/distribution v2.8.3+incompatible
	github.com/docker/docker v17.10.0-ce+incompatible
	github.com/docker/go-connections v0.4.0
	github.com/docker/go-units v0.5.0
	github.com/docker/machine v0.16.2
	github.com/dustin/go-jsonpointer v0.0.0-20160814072949-ba0abeacc3dc
	github.com/fatih/structs v1.1.0
	github.com/fsnotify/fsnotify v1.9.0
	github.com/gorilla/mux v1.8.0
	github.com/jordan-wright/email v4.0.1-0.20210109023952-943e75fe5223+incompatible
	github.com/malice-plugins/pkgs v1.1.7
	github.com/mattn/go-runewidth v0.0.9
	github.com/parnurzeal/gorequest v0.3.0
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.10.1
	github.com/spf13/cobra v1.8.0
	github.com/spf13/viper v1.18.2
	github.com/urfave/cli v1.22.17
	golang.org/x/net v0.58.0
	gopkg.in/natefinch/lumberjack.v2 v2.2.1
)

require (
	cyphar.com/go-pathrs v0.2.5 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20250102033503-faa5f7b0171c // indirect
	github.com/Microsoft/go-winio v0.6.3-0.20251027160822-ad3df93bed29 // indirect
	github.com/Nvveen/Gotty v0.0.0-20120604004816-cd527374f1e5 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/containerd/continuity v0.5.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/cyphar/filepath-securejoin v0.6.1 // indirect
	github.com/distribution/reference v0.5.0 // indirect
	github.com/docker/go-metrics v0.1.0 // indirect
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7 // indirect
	github.com/dustin/gojson v0.0.0-20160307161227-2e71ec9dd5ad // indirect
	github.com/elazarl/goproxy v1.9.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/klauspost/compress v1.19.2 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-shellwords v1.0.14 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/moul/http2curl v1.0.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/olivere/elastic/v7 v7.0.29 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/opencontainers/runc v0.1.1 // indirect
	github.com/opencontainers/selinux v1.15.1 // indirect
	github.com/pelletier/go-toml/v2 v2.4.3 // indirect
	github.com/prometheus/client_golang v1.24.1 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.70.1 // indirect
	github.com/prometheus/procfs v0.21.1 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sagikazarmark/locafero v0.4.0 // indirect
	github.com/sagikazarmark/slog-shim v0.1.0 // indirect
	github.com/samalba/dockerclient v0.0.0-00010101000000-000000000000 // indirect
	github.com/smartystreets/goconvey v1.8.1 // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/spf13/afero v1.11.0 // indirect
	github.com/spf13/cast v1.6.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/stretchr/testify v1.12.1 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	go.opentelemetry.io/otel v1.45.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.45.0 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.9.0 // indirect
	golang.org/x/crypto v0.55.0 // indirect
	golang.org/x/exp v0.0.0-20260603202125-055de637280b // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/term v0.45.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260803160001-6ac0973c030d // indirect
	google.golang.org/grpc v1.83.2 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/samalba/dockerclient => ./third_party/samalba-dockerclient

replace github.com/docker/docker => github.com/docker/docker v17.10.0-ce+incompatible
