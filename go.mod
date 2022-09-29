module koinos-integration-tests

go 1.18

require (
	github.com/btcsuite/btcd v0.20.1-beta
	github.com/btcsuite/btcutil v1.0.2
	github.com/ethereum/go-ethereum v1.10.21
	github.com/koinos/koinos-mq-golang v0.0.0-20220319044422-57bccec4eb07
	github.com/koinos/koinos-proto-golang v0.4.1-0.20220929173025-ac76a8266abe
	github.com/koinos/koinos-util-golang v0.0.0-20220809223732-ae4a6f8736ca
	github.com/mr-tron/base58 v1.2.0
	github.com/multiformats/go-multihash v0.2.0
	github.com/stretchr/testify v1.7.2
	github.com/ybbus/jsonrpc/v3 v3.1.1
	google.golang.org/protobuf v1.27.1
)

require (
	github.com/btcsuite/btcd/btcec/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.0.1 // indirect
	github.com/klauspost/cpuid/v2 v2.0.9 // indirect
	github.com/koinos/koinos-log-golang v0.0.0-20210413225320-69e5d4a4c6c2 // indirect
	github.com/minio/sha256-simd v1.0.0 // indirect
	github.com/multiformats/go-varint v0.0.6 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/shopspring/decimal v1.3.1 // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	github.com/streadway/amqp v1.0.0 // indirect
	github.com/ybbus/jsonrpc/v2 v2.1.6 // indirect
	go.uber.org/atomic v1.6.0 // indirect
	go.uber.org/multierr v1.5.0 // indirect
	go.uber.org/zap v1.16.0 // indirect
	golang.org/x/crypto v0.0.0-20220525230936-793ad666bf5e // indirect
	golang.org/x/sys v0.0.0-20220520151302-bc2c85ada10a // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	lukechampine.com/blake3 v1.1.6 // indirect
)

replace google.golang.org/protobuf => github.com/koinos/protobuf-go v1.27.2-0.20211026185306-2456c83214fe
