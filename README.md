# Koinos Integration Tests

This repo contains Koinos integration tests.

To run an individual test:

1. Run `source .env`. This enables default microservice image tags. Optionally, export any feature branch tags that you want to test for a microservice (e.g. `export MEMPOOL_TAG="9-bump-submodules`)
2. CD in to the test directory.
3. Run `docker-compose up -d`.
4. Run `go test -v ./...` with an optional `--timeout` (Tests should have internal timeouts)
5. Cleanup with `docker-compose down`
