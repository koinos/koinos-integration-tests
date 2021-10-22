echo "$DOCKER_PASSWORD" | docker login -u $DOCKER_USERNAME --password-stdin

$TRAVIS_BUILD_DIR/tests/run.sh
