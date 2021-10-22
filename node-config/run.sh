#!/bin/bash

set -e
set -x

mkdir -p /koinos/chain
rsync -a -v --ignore-existing /koinos-config/genesis.pub /koinos/chain/genesis.pub

mkdir -p /koinos/block_producer
mkdir -p /koinos/chain
rsync -a -v --ignore-existing /koinos-config/private.key /koinos/block_producer/private.key

mkdir -p /koinos/jsonrpc/descriptors
pushd /koinos/jsonrpc/descriptors

wget https://github.com/koinos/koinos-proto-descriptors/raw/master/koinos_descriptors.pb

popd
