module koinos-integration-tests

go 1.18

require (
	github.com/btcsuite/btcd v0.20.1-beta
	github.com/btcsuite/btcutil v1.0.2
	github.com/ethereum/go-ethereum v1.10.9 // indirect
	github.com/koinos/koinos-mq-golang v0.0.0-20220319044422-57bccec4eb07
	github.com/koinos/koinos-proto-golang v0.3.1-0.20220404211729-f0b34183b37c
	github.com/koinos/koinos-util-golang v0.0.0-20220406201011-5df580ccdd47
	github.com/mr-tron/base58 v1.2.0 // indirect
	github.com/multiformats/go-multihash v0.1.0
	github.com/stretchr/testify v1.7.1
	github.com/ybbus/jsonrpc/v2 v2.1.6
	golang.org/x/crypto v0.0.0-20210711020723-a769d52b0f97 // indirect
	google.golang.org/protobuf v1.27.1
)

replace google.golang.org/protobuf => github.com/koinos/protobuf-go v1.27.2-0.20211026185306-2456c83214fe
