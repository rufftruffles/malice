#!/bin/sh
# deploy.sh - one-shot deployment of the malice multi-engine scanner.
#
# What it does:
#   1. checks the host meets the minimum spec (4 cores, 8 GB RAM, 80 GB free)
#   2. installs Docker + the compose plugin if missing
#   3. pulls the server, all 17 engine images, and Elasticsearch from GHCR,
#      and retags the engines to the names the server expects (malice/<name>)
#   4. starts the stack with docker compose
#   5. opens port 3993 in ufw if the firewall is active
#   6. prints the URL
#
# It also installs a daily systemd timer that re-pulls the four
# signature-decaying engine images (clamav, kvrt, yara, lmd). Those are
# rebuilt nightly by GitHub Actions; the timer keeps a client current with
# a plain pull. ESET is not in the list: its entrypoint refreshes signatures
# on every scan.
#
# Run it from a clone of the malice repo:  ./deploy.sh
set -eu

REGISTRY=ghcr.io/rufftruffles
VERSION=1.0.0
ES_VERSION=8.19.20
ENGINES="nsrl virustotal shadow-server hashlookup fileinfo yara clamav pescan floss office pdf capa diec lmd rizin eset kvrt"
DECAYING="clamav kvrt yara lmd"
PORT=3993

# ---- 0. root check ---------------------------------------------------------
if [ "$(id -u)" != "0" ]; then
    echo "this script needs root (it installs docker and manages the firewall)."
    echo "re-run with: sudo ./deploy.sh"
    exit 1
fi

# ---- 1. spec check ---------------------------------------------------------
fail=0
cores=$(nproc)
mem_kb=$(awk '/^MemTotal:/ {print $2}' /proc/meminfo)
mem_gb=$((mem_kb / 1024 / 1024))
disk_root=/
[ -d /var/lib/docker ] && disk_root=/var/lib/docker
free_gb=$(df -kBG "$disk_root" | awk 'NR==2 {gsub("G","",$4); print $4}')

echo "== spec check"
echo "   cores: $cores (need 4)"
echo "   ram:   ${mem_gb} GB (need 8)"
echo "   disk:  ${free_gb} GB free on $disk_root (need 80)"
[ "$cores" -ge 4 ] || { echo "   too few cores"; fail=1; }
[ "$mem_gb" -ge 8 ] || { echo "   not enough ram"; fail=1; }
[ "$free_gb" -ge 80 ] || { echo "   not enough disk"; fail=1; }
if [ "$fail" != "0" ]; then
    echo "host does not meet the minimum spec. aborting."
    exit 1
fi
echo "   ok"

# ---- 2. docker -------------------------------------------------------------
if ! command -v docker >/dev/null 2>&1; then
    echo "== installing docker"
    curl -fsSL https://get.docker.com | sh
    systemctl enable --now docker
else
    echo "== docker present: $(docker --version)"
fi
if ! docker compose version >/dev/null 2>&1; then
    echo "docker compose plugin is missing. install docker-compose-plugin and re-run."
    exit 1
fi
systemctl is-active docker >/dev/null || systemctl start docker

# ---- 3. pull images --------------------------------------------------------
pull_and_tag() {
    src=$1
    dst=$2
    echo "   pull $src"
    docker pull "$src"
    docker tag "$src" "$dst"
}

echo "== pulling images ($VERSION)"
pull_and_tag "$REGISTRY/malice:$VERSION" "malice/malice:$VERSION"
for e in $ENGINES; do
    pull_and_tag "$REGISTRY/malice-$e:$VERSION" "malice/$e:latest"
done
echo "   pull elasticsearch:$ES_VERSION"
docker pull "docker.elastic.co/elasticsearch/elasticsearch:$ES_VERSION"

# ---- 4. start the stack ----------------------------------------------------
echo "== starting stack"
docker compose up -d
docker compose ps

# ---- 5. firewall -----------------------------------------------------------
if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | head -1 | grep -q "Status: active"; then
    echo "== opening port $PORT in ufw"
    ufw allow "$PORT/tcp" >/dev/null
else
    echo "== no active ufw firewall, skipping"
fi

# ---- 6. daily image refresh timer ------------------------------------------
echo "== installing daily image refresh timer"
cat > /etc/systemd/system/malice-images-pull.service <<EOF
[Unit]
Description=Pull refreshed malice engine images
After=docker.service
Requires=docker.service

[Service]
Type=oneshot
ExecStart=/bin/sh -c 'for e in $DECAYING; do docker pull $REGISTRY/malice-\$e:$VERSION && docker tag $REGISTRY/malice-\$e:$VERSION malice/\$e:latest; done'
TimeoutStartSec=20m
EOF
cat > /etc/systemd/system/malice-images-pull.timer <<EOF
[Unit]
Description=Daily malice engine image refresh

[Timer]
OnCalendar=*-*-* 04:52:00
Persistent=true

[Install]
WantedBy=timers.target
EOF
systemctl daemon-reload
systemctl enable --now malice-images-pull.timer

# ---- 7. done ----------------------------------------------------------------
ip=$(hostname -I 2>/dev/null | awk '{print $1}')
[ -n "$ip" ] || ip=$(ip -4 -o addr show 2>/dev/null | awk '{print $4}' | cut -d/ -f1 | grep -v '^127\.' | head -1)

echo
echo "deployed."
echo "   url:      http://$ip:$PORT"
echo "   engines:  17 (see the Engines page)"
echo "   updates:  engine images refresh daily at 04:52 (malice-images-pull.timer)"
echo "   stop:     docker compose down"
