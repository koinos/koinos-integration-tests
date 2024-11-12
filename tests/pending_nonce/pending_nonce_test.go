package pendingNonce

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

	// Nonce 0 and 2 should fail
	trx, err := createTransactionWithNonce(client, genesisKey, 0)
	integration.NoError(t, err)
	_, err = integration.SubmitTransaction(client, trx)
	require.Error(t, err, "nonce error expected")

	trx, err = createTransactionWithNonce(client, genesisKey, 2)
	integration.NoError(t, err)
	_, err = integration.SubmitTransaction(client, trx)
	require.Error(t, err, "nonce error expected")

	// Submitting nonce 1 should work and then 0 and 1 should then fail
	trx, err = createTransactionWithNonce(client, genesisKey, 1)
	integration.NoError(t, err)
	_, err = integration.SubmitTransaction(client, trx)
	integration.NoError(t, err)

	trx, err = createTransactionWithNonce(client, genesisKey, 0)
	integration.NoError(t, err)
	_, err = integration.SubmitTransaction(client, trx)
	require.Error(t, err, "nonce error expected")

	trx, err = createTransactionWithNonce(client, genesisKey, 1)
	integration.NoError(t, err)
	_, err = integration.SubmitTransaction(client, trx)
	require.Error(t, err, "nonce error expected")

	// Nonce 2 should now work and 0, 1, and 2 should then fail
	trx, err = createTransactionWithNonce(client, genesisKey, 2)
	integration.NoError(t, err)
	_, err = integration.SubmitTransaction(client, trx)
	integration.NoError(t, err)

	trx, err = createTransactionWithNonce(client, genesisKey, 0)
	integration.NoError(t, err)
	_, err = integration.SubmitTransaction(client, trx)
	require.Error(t, err, "nonce error expected")

	trx, err = createTransactionWithNonce(client, genesisKey, 1)
	integration.NoError(t, err)
	_, err = integration.SubmitTransaction(client, trx)
	require.Error(t, err, "nonce error expected")

	trx, err = createTransactionWithNonce(client, genesisKey, 2)
	integration.NoError(t, err)
	_, err = integration.SubmitTransaction(client, trx)
	require.Error(t, err, "nonce error expected")
}
