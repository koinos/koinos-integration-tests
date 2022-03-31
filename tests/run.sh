#!/bin/bash

function run_tests() {
   local success=0

   for test_dir in */ ; do
      pushd $test_dir
      go build ./...
      docker-compose up -d
      if [ $? -ne 0 ] || [ $success -ne 0 ];
      then
        success=1
      fi
      go test -v ./...
      if [ $? -ne 0 ] || [ $success -ne 0 ];
      then
        success=1
      fi
      docker-compose logs
      docker-compose down
      popd
   done

   return $success
}

run_tests
exit $?

