#!/bin/bash

function run_tests() {
   local code=0

   for test_dir in */ ; do
      pushd $test_dir
      go build ./...
      if [ $? -ne 0 ];
      then
        echo "Failed to build integration test: ${test_dir}"
        code=1
        popd
        continue
      fi

      docker-compose up -d
      if [ $? -ne 0 ];
      then
        echo "Failed to start cluster: ${test_dir}"
        code=1
        docker-compose logs
        docker-compose down
        popd
        continue
      fi

      go test -v ./...
      if [ $? -ne 0 ];
      then
        echo "Failed during integration test: ${test_dir}"
        docker-compose logs
        code=1
      fi

      docker-compose down
      popd
   done

   return $success
}

run_tests
exit $?
