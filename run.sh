#!/bin/bash

function run_test() {
   local code=0

   pushd $1
   go build ./...
   if [ $? -ne 0 ];
   then
      echo "Failed to build integration test: ${1}"
      code=1
      popd
      continue
   fi

   docker-compose up -d
   if [ $? -ne 0 ];
   then
      echo "Failed to start cluster: ${1}"
      code=1
      docker-compose logs
      docker-compose down
      popd
      continue
   fi

   go test -timeout 5m -v ./...
   if [ $? -ne 0 ];
   then
      echo "Failed during integration test: ${1}"
      docker-compose logs
      code=1
   fi

   docker-compose down -v
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
   run_test tests/$1
fi

exit $?
