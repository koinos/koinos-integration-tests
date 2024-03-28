package propose_block

import (
	"context"
	"koinos-integration-tests/integration"
	"koinos-integration-tests/integration/token"
	"testing"

	mq "github.com/koinos/koinos-mq-golang"
	broadcast "github.com/koinos/koinos-proto-golang/v2/koinos/broadcast"
	"github.com/koinos/koinos-proto-golang/v2/koinos/protocol"
	kjsonrpc "github.com/koinos/koinos-util-golang/v2/rpc"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func broadcastGossipStatus(t *testing.T, c *mq.Client, flag bool) {
	gossipStatusBytes, err := proto.Marshal(&broadcast.GossipStatus{Enabled: flag})
	integration.NoError(t, err)

	c.Broadcast(context.Background(), "application/octet-stream", "koinos.gossip.status", gossipStatusBytes)
}

func broadcastTransactionAccepted(t *testing.T, c *mq.Client, transaction *protocol.Transaction) {
	transactionedAcceptedBytes, err := proto.Marshal(&broadcast.TransactionAccepted{Transaction: transaction})
	integration.NoError(t, err)

	c.Broadcast(context.Background(), "application/octet-stream", "koinos.transaction.accepted", transactionedAcceptedBytes)
}

func TestProposeBlock(t *testing.T) {
	client := kjsonrpc.NewKoinosRPCClient("http://localhost:8080/")
	mqClient := mq.NewClient("amqp://guest:guest@localhost:5672/", mq.NoRetry)
	mqClient.Start(context.Background())

	koinKey, err := integration.GetKey(integration.Koin)
	integration.NoError(t, err)

	producerKey, err := integration.GetKey(integration.PobProducer)
	integration.NoError(t, err)

	koin := token.GetKoinToken(client)

	integration.AwaitChain(t, client)

	integration.InitNameService(t, client)

	t.Logf("Uploading KOIN contract")
	_, err = integration.UploadSystemContract(client, "../../contracts/koin.wasm", koinKey, "koin")
	integration.NoError(t, err)

	t.Logf("Minting KOIN")
	koin.Mint(producerKey.AddressBytes(), 100000000000000) // 1,000,000.00000000 KOIN

	producerBalance, err := koin.Balance(producerKey.AddressBytes())
	integration.NoError(t, err)
	require.EqualValues(t, uint64(100000000000000), producerBalance)

	// Broadcast transactions

	// Turn on block production with gossip broadcast
}
