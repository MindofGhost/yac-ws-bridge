#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
docker build -t yac-ws-adapter:local .
docker build -f bridge-cloud/build/Dockerfile --output type=local,dest=deploy/dist .
python3 scripts/package-cloud.py
docker build --target helper-artifacts --output type=local,dest=deploy/dist/android .
