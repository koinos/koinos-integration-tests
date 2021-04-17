
#!/bin/bash

set -e

function run_tests() {
   success=0
   for test_dir in */ ; do
      pushd $test_dir
      docker-compose up -d
      success=$?||$success
      go test -v ./...
      success=$?||$success
      docker-compose down
      popd
   done

   return $success
}

run_tests