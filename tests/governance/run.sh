#!/bin/bash

go build ./...
docker-compose up -d
success=$?||$success
go test -v ./...
success=$?||$success
docker-compose down

