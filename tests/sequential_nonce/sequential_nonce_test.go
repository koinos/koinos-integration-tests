package sequentialNonce_test

import (
	"context"
	"koinos-integration-tests/integration"
	"koinos-integration-tests/integration/token"
	"testing"
	"time"

	"github.com/koinos/koinos-proto-golang/v2/koinos/chain"
	"github.com/koinos/koinos-proto-golang/v2/koinos/protocol"
	"github.com/koinos/koinos-proto-golang/v2/koinos/standards/kcs4"
	util "github.com/koinos/koinos-util-golang/v2"
	kjsonrpc "github.com/koinos/koinos-util-golang/v2/rpc"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func createTransactionWithNonce(client integration.Client, key *util.KoinosKey, nonce uint64, op *protocol.Operation) (*protocol.Transaction, error) {
	var nonceValue chain.ValueType
	nonceValue.Kind = &chain.ValueType_Uint64Value{Uint64Value: nonce}
	nonceBytes, err := proto.Marshal(&nonceValue)
	if err != nil {
		return nil, err
	}

	return integration.CreateTransaction(
		client,
		[]*protocol.Operation{},
		key,
		func(t *protocol.Transaction) error {
			t.Header.Nonce = nonceBytes
			t.Header.RcLimit = 1000000

			return nil
		})
}

func TestPublishTransaction(t *testing.T) {
	producer := kjsonrpc.NewKoinosRPCClient("http://localhost:28080/")
	api := kjsonrpc.NewKoinosRPCClient("http://localhost:28081/")

	genesisKey, err := integration.GetKey(integration.Genesis)
	integration.NoError(t, err)

	koinKey, err := integration.GetKey(integration.Koin)
	integration.NoError(t, err)

	govKey, err := integration.GetKey(integration.Governance)
	integration.NoError(t, err)

	userKey, err := util.GenerateKoinosKey()
	integration.NoError(t, err)

	koin := token.GetKoinToken(producer)

	integration.AwaitChain(t, producer)
	integration.AwaitChain(t, api)

	integration.InitNameService(t, producer)
	integration.InitGetContractMetadata(t, producer)

	t.Logf("Uploading KOIN contract")
	_, err = integration.UploadSystemContract(producer, "../../contracts/koin.wasm", koinKey, "koin")
	integration.NoError(t, err)

	t.Logf("Uploading governance contract")
	_, err = integration.UploadSystemContract(producer, "../../contracts/governance.wasm", govKey, "governance")
	integration.NoError(t, err)

	t.Logf("Minting KOIN")
	koin.Mint(userKey.AddressBytes(), 100000000000000) // 1,000,000.00000000 KOIN
	koin.Mint(genesisKey.AddressBytes(), 100000000000) // 1,000.00000000 KOIN

	integration.InitResources(t, producer)

	startingNonce := uint64(0)

	nonce, err := api.GetAccountNonce(context.Background(), userKey.AddressBytes())
	integration.NoError(t, err)
	require.Equal(t, startingNonce, nonce)

	nonce, err = producer.GetAccountNonce(context.Background(), userKey.AddressBytes())
	integration.NoError(t, err)
	require.Equal(t, startingNonce, nonce)

	producerHeadInfo, err := integration.GetHeadInfo(producer)
	integration.NoError(t, err)

	t.Logf("Producer head: %v", producerHeadInfo.HeadTopology.Height)

	t.Logf("Waiting for API node to be at head...")

	for {
		apiHeadInfo, err := integration.GetHeadInfo(api)
		integration.NoError(t, err)

		if apiHeadInfo.HeadTopology.Height >= producerHeadInfo.HeadTopology.Height {
			break
		}

		time.Sleep(100 * time.Millisecond)
	}

	transferArgs := &kcs4.TransferArguments{
		From:  userKey.AddressBytes(),
		To:    koinKey.AddressBytes(),
		Value: 1,
	}

	args, err := proto.Marshal(transferArgs)
	integration.NoError(t, err)

	op := &protocol.Operation{
		Op: &protocol.Operation_CallContract{
			CallContract: &protocol.CallContractOperation{
				ContractId: koinKey.AddressBytes(),
				EntryPoint: 0x27f576ca,
				Args:       args,
			},
		},
	}

	t.Logf("Sending transactions...")

	for nonce < 5000+startingNonce {
		nonce += 1

		trx, err := createTransactionWithNonce(api, userKey, nonce, op)
		integration.NoError(t, err)
		_, err = integration.SubmitTransaction(api, trx)
		if err != nil {
			// Some times we can submit a transaction too soon. This will allow us retry at the same nonce.
			nonce -= 1
		}

		if nonce%100 == startingNonce {
			t.Logf("Sent %v transactions...", nonce)
		}
	}

	test_timer := time.NewTimer(60 * time.Second)
	go func() {
		<-test_timer.C
		panic("Timer expired")
	}()

	for {
		apiNonce, err := api.GetPendingNonce(context.Background(), userKey.AddressBytes())
		integration.NoError(t, err)

		t.Logf("API Pending Nonce: %v\n", apiNonce)

		if nonce == apiNonce {
			break
		}

		time.Sleep(time.Second)
	}

	for {
		producerNonce, err := producer.GetAccountNonce(context.Background(), userKey.AddressBytes())
		integration.NoError(t, err)

		t.Logf("Producer Account Nonce: %v\n", producerNonce)

		if nonce == producerNonce {
			break
		}

		time.Sleep(time.Second)
	}
}
