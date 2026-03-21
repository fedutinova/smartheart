#!/bin/bash
set -euo pipefail

export DEBIAN_FRONTEND=noninteractive

apt-get update -q
apt-get upgrade -yq
apt-get install -yq curl git ca-certificates gnupg rsync

install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg \
  | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
chmod a+r /etc/apt/keyrings/docker.gpg

echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
  https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
  | tee /etc/apt/sources.list.d/docker.list > /dev/null

apt-get update -q
apt-get install -yq docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

systemctl enable docker
systemctl start docker

mkdir -p /opt/smartheart/uploads
mkdir -p /opt/smartheart/caddy/data
mkdir -p /opt/smartheart/caddy/config
mkdir -p /opt/smartheart/frontend/dist
mkdir -p /opt/smartheart/rag_pipeline/documents
mkdir -p /opt/smartheart/rag_pipeline/chroma_db_4
mkdir -p /opt/smartheart/migrations

touch /opt/smartheart/.env

echo "smartheart bootstrap complete" > /var/log/smartheart-bootstrap.log
