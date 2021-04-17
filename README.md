# Koinos Integration Tests

This repo contains Koinos integration tests.

To run the test suite:

1. Optionally, export any feature branch tags that you want to test for a microservice (e.g. `export MEMPOOL_TAG="9-bump-submodules`)
2. CD in to tests
3. Run `run.sh`

To run an individual test:

1. Optionally, export any feature branch tags that you want to test for a microservice (e.g. `export MEMPOOL_TAG="9-bump-submodules`)
2. CD in to the test directory.
3. Run `docker-compose up -d`.
4. Run `go test -v ./...` with an optional `--timeout` (Tests should already have internal timeouts)
5. Cleanup with `docker-compose down`
