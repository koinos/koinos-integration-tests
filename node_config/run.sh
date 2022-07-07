#!/bin/bash

set -e
set -x

mkdir -p /koinos/chain
rsync -a -v --ignore-existing /koinos-config/genesis_data.json /koinos/chain/genesis_data.json

mkdir -p /koinos/block_producer
mkdir -p /koinos/chain
rsync -a -v --ignore-existing /koinos-config/private.key /koinos/block_producer/private.key
rsync -a -v --ignore-existing /koinos-config/pob.key /koinos/block_producer/pob.key

mkdir -p /koinos/jsonrpc/descriptors
pushd /koinos/jsonrpc/descriptors

wget https://github.com/koinos/koinos-proto-descriptors/raw/master/koinos_descriptors.pb

popd
