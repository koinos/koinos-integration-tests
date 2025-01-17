package pendingTransactionLimit

import (
	"context"
	"koinos-integration-tests/integration"
	"testing"

	"github.com/koinos/koinos-proto-golang/v2/koinos/chain"
	"github.com/koinos/koinos-proto-golang/v2/koinos/protocol"
	util "github.com/koinos/koinos-util-golang/v2"
	kjsonrpc "github.com/koinos/koinos-util-golang/v2/rpc"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func createTransactionWithNonce(client integration.Client, key *util.KoinosKey, nonce uint64) (*protocol.Transaction, error) {
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
			t.Header.RcLimit = 100000

			return nil
		})
}

func TestPublishTransaction(t *testing.T) {
	client := kjsonrpc.NewKoinosRPCClient("http://localhost:28080/")

	integration.AwaitChain(t, client)

	genesisKey, err := integration.GetKey(integration.Genesis)
	integration.NoError(t, err)

	_, err = client.GetAccountNonce(context.Background(), genesisKey.AddressBytes())
	integration.NoError(t, err)

	// Submit transactions up to the limit
	trx1, err := createTransactionWithNonce(client, genesisKey, 1)
	integration.NoError(t, err)
	_, err = integration.SubmitTransaction(client, trx1)
	integration.NoError(t, err)

	trx2, err := createTransactionWithNonce(client, genesisKey, 2)
	integration.NoError(t, err)
	_, err = integration.SubmitTransaction(client, trx2)
	integration.NoError(t, err)

	trx3, err := createTransactionWithNonce(client, genesisKey, 3)
	integration.NoError(t, err)
	_, err = integration.SubmitTransaction(client, trx3)
	integration.NoError(t, err)

	trx4, err := createTransactionWithNonce(client, genesisKey, 4)
	integration.NoError(t, err)
	_, err = integration.SubmitTransaction(client, trx4)
	integration.NoError(t, err)

	trx5, err := createTransactionWithNonce(client, genesisKey, 5)
	integration.NoError(t, err)
	_, err = integration.SubmitTransaction(client, trx5)
	integration.NoError(t, err)

	// Transaction 6 should fail because it passes the limit
	trx6, err := createTransactionWithNonce(client, genesisKey, 6)
	integration.NoError(t, err)
	_, err = integration.SubmitTransaction(client, trx6)
	require.Error(t, err)

	// Create a block containing transaction 1. This will bump us below the limit
	// Transaction 6 should work, but now 7 fails.
	_, err = integration.CreateBlock(client, []*protocol.Transaction{trx1})
	integration.NoError(t, err)

	_, err = integration.SubmitTransaction(client, trx6)
	integration.NoError(t, err)

	trx7, err := createTransactionWithNonce(client, genesisKey, 7)
	integration.NoError(t, err)
	_, err = integration.SubmitTransaction(client, trx7)
	require.Error(t, err)
}
