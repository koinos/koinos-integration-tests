#!/bin/bash

function run_test() {
   local code=0

   pushd $1

   TEST_NAME=`basename \`pwd\``
   COMPOSE_COMMAND="docker compose -p $TEST_NAME -f ../../node_config/docker-compose.config.yml -f docker-compose.yml"

   which go
   go build ./...
   if [ $? -ne 0 ];
   then
      echo "Failed to build integration test: $TEST_NAME"
      code=1
      popd
      return $code
   fi

   $COMPOSE_COMMAND up -d
   if [ $? -ne 0 ];
   then
      echo "Failed to start cluster: $TEST_NAME"
      code=1
      $COMPOSE_COMMAND logs
      $COMPOSE_COMMAND down
      popd
      return $code
   fi

   which go
   go clean -testcache
   go test -timeout 30m -v ./...
   if [ $? -ne 0 ];
   then
      echo "Failed during integration test: $TEST_NAME"
      $COMPOSE_COMMAND logs
      code=1
   fi

   $COMPOSE_COMMAND down -v
   popd

   return $code
}

function run_tests() {
   local code=0

   for dir in tests/*/ ; do
     run_test $dir
     if [ $? -ne 0 ];
     then
        code=1
     fi
   done

   return $code
}

if [ $# -eq 0 ];
then
   run_tests
else
   for test in "$@"
   do
      run_test tests/$test
   done
fi

exit $?
