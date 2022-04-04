echo "$DOCKER_PASSWORD" | docker login -u $DOCKER_USERNAME --password-stdin

cd $TRAVIS_BUILD_DIR
./run.sh
