echo "$DOCKER_PASSWORD" | docker login -u $DOCKER_USERNAME --password-stdin

../tests/run.sh
