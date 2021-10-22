module koinos-integration-tests

go 1.15

require (
	github.com/koinos/koinos-proto-golang v0.0.0-20211021190958-179635a3c9bd
	github.com/koinos/koinos-types-golang v0.1.1-0.20210415002848-a15a03ff0a10
	github.com/koinos/koinos-util-golang v0.0.0-20210415214934-d87402bdd6bb // indirect
	github.com/ybbus/jsonrpc/v2 v2.1.6
	google.golang.org/protobuf v1.27.1
)

replace google.golang.org/protobuf => github.com/koinos/protobuf-go v1.27.2-0.20211016005428-adb3d63afc5e
