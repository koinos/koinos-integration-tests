
#!/bin/bash

function run_tests() {
   success=0
   for test_dir in */ ; do
      pushd $test_dir
      go build ./...
      docker-compose up -d
      success=$?||$success
      go test -v ./...
      success=$?||$success
      docker-compose logs
      docker-compose down
      popd
   done

   return $success
}

run_tests
