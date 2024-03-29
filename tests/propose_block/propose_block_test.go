package propose_block

import (
	"context"
	"koinos-integration-tests/integration"
	xtoken "koinos-integration-tests/integration/token"
	"testing"
	"time"

	mq "github.com/koinos/koinos-mq-golang"
	broadcast "github.com/koinos/koinos-proto-golang/v2/koinos/broadcast"
	"github.com/koinos/koinos-proto-golang/v2/koinos/contracts/token"
	"github.com/koinos/koinos-proto-golang/v2/koinos/protocol"
	util "github.com/koinos/koinos-util-golang/v2"
	kjsonrpc "github.com/koinos/koinos-util-golang/v2/rpc"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func broadcastGossipStatus(t *testing.T, c *mq.Client, flag bool) {
	gossipStatusBytes, err := proto.Marshal(&broadcast.GossipStatus{Enabled: flag})
	integration.NoError(t, err)

	c.Broadcast(context.Background(), "application/octet-stream", "koinos.gossip.status", gossipStatusBytes)
}

func broadcastTransactionAccepted(t *testing.T, c *mq.Client, height uint64, transaction *protocol.Transaction) {
	bogusReceipt := protocol.TransactionReceipt{MaxPayerRc: 10000000000, DiskStorageUsed: 1, NetworkBandwidthUsed: 1, ComputeBandwidthUsed: 1}
	transactionedAcceptedBytes, err := proto.Marshal(&broadcast.TransactionAccepted{Transaction: transaction, Receipt: &bogusReceipt, Height: height})
	integration.NoError(t, err)

	c.Broadcast(context.Background(), "application/octet-stream", "koinos.transaction.accept", transactionedAcceptedBytes)
}

func transferTransaction(client integration.Client, from *util.KoinosKey, to []byte, value uint64, mod func(*protocol.Transaction) error) (*protocol.Transaction, error) {
	transferArgs := &token.TransferArguments{
		From:  from.AddressBytes(),
		To:    to,
		Value: value,
	}

	args, err := proto.Marshal(transferArgs)
	if err != nil {
		return nil, err
	}

	koinKey, err := integration.GetKey(integration.Koin)
	if err != nil {
		return nil, err
	}

	op := &protocol.Operation{
		Op: &protocol.Operation_CallContract{
			CallContract: &protocol.CallContractOperation{
				ContractId: koinKey.AddressBytes(),
				EntryPoint: 0x27f576ca,
				Args:       args,
			},
		},
	}

	transaction, err := integration.CreateTransaction(client, []*protocol.Operation{op}, from, mod)
	if err != nil {
		return nil, err
	}

	return transaction, nil
}

func TestProposeBlock(t *testing.T) {
	panicTime := time.NewTimer(time.Minute)
	go func() {
		<-panicTime.C
		panic("Timer expired")
	}()

	client := kjsonrpc.NewKoinosRPCClient("http://localhost:8080/")
	mqClient := mq.NewClient("amqp://guest:guest@localhost:5672/", mq.NoRetry)
	mqClient.Start(context.Background())

	koinKey, err := integration.GetKey(integration.Koin)
	integration.NoError(t, err)

	t.Logf("Generating key for alice")
	aliceKey, err := util.GenerateKoinosKey()
	integration.NoError(t, err)

	t.Logf("Generating key for bob")
	bobKey, err := util.GenerateKoinosKey()
	integration.NoError(t, err)

	koin := xtoken.GetKoinToken(client)

	integration.AwaitChain(t, client)

	integration.InitNameService(t, client)

	t.Logf("Uploading KOIN contract")
	_, err = integration.UploadSystemContract(client, "../../contracts/koin.wasm", koinKey, "koin")
	integration.NoError(t, err)

	originalMint := uint64(100000000000000)

	t.Logf("Minting KOIN to Alice")
	koin.Mint(aliceKey.AddressBytes(), originalMint) // 1,000,000.00000000 KOIN

	aliceBalance, err := koin.Balance(aliceKey.AddressBytes())
	integration.NoError(t, err)
	require.EqualValues(t, originalMint, aliceBalance)

	t.Logf("Minting KOIN to Bob")
	koin.Mint(bobKey.AddressBytes(), originalMint) // 1,000,000.00000000 KOIN

	bobBalance, err := koin.Balance(bobKey.AddressBytes())
	integration.NoError(t, err)
	require.EqualValues(t, originalMint, bobBalance)

	numTransactions := 100
	koinPerTransaction := uint64(1000)
	aliceNonce := uint64(0)
	bobNonce := uint64(0)

	beforeHeadInfo, err := integration.GetHeadInfo(client)
	integration.NoError(t, err)

	t.Logf("Current head block height: %d", beforeHeadInfo.HeadTopology.Height)

	t.Logf("Broadcasting %d accepted transactions", numTransactions)
	for i := 0; i < numTransactions; i++ {
		mod := func(transaction *protocol.Transaction) error {
			if i%2 == 0 {
				aliceNonce += 1
				nonceBytes, err := util.UInt64ToNonceBytes(aliceNonce)
				if err != nil {
					return err
				}
				transaction.Header.Nonce = nonceBytes
			} else {
				bobNonce += 1
				nonceBytes, err := util.UInt64ToNonceBytes(bobNonce)
				if err != nil {
					return err
				}
				transaction.Header.Payer = bobKey.AddressBytes()
				transaction.Header.Nonce = nonceBytes
			}
			transaction.Header.RcLimit = 10000000

			return nil
		}

		transaction, err := transferTransaction(client, aliceKey, bobKey.AddressBytes(), koinPerTransaction, mod)
		integration.NoError(t, err)

		broadcastTransactionAccepted(t, mqClient, beforeHeadInfo.GetHeadTopology().Height, transaction)
	}

	for {
		pendingTrans, err := integration.GetPendingTransactions(client, 100)
		integration.NoError(t, err)

		if len(pendingTrans) == 100 {
			t.Logf("Mempool contains 100 pending transactions")
			break
		}

		time.Sleep(time.Millisecond * 100)
	}

	broadcastGossipStatus(t, mqClient, true)

	var block *protocol.Block = nil

	for {
		afterHeadInfo, err := integration.GetHeadInfo(client)
		integration.NoError(t, err)

		blocksByHeight, err := integration.GetBlocksByHeight(client, afterHeadInfo.HeadTopology.Id, beforeHeadInfo.HeadTopology.Height, 2, true, false)
		integration.NoError(t, err)

		for _, blockItem := range blocksByHeight.BlockItems {
			if blockItem.BlockHeight == beforeHeadInfo.HeadTopology.Height+1 {
				t.Logf("Found block with height: %d", beforeHeadInfo.HeadTopology.Height+1)
				block = blockItem.Block
				break
			}
		}

		if block != nil {
			break
		}

		time.Sleep(time.Millisecond * 100)
	}

	t.Logf("Ensure subsequent block with height %d contains %d transactions", block.Header.Height, numTransactions/2)
	require.EqualValues(t, numTransactions/2, len(block.Transactions))

	t.Logf("Checking balance of Alice")
	aliceBalance, err = koin.Balance(aliceKey.AddressBytes())
	integration.NoError(t, err)
	require.EqualValues(t, originalMint-(uint64(numTransactions/2)*koinPerTransaction), aliceBalance)
	t.Logf("Alice has %d KOIN satoshis", aliceBalance)

	t.Logf("Checking balance of Bob")
	bobBalance, err = koin.Balance(bobKey.AddressBytes())
	integration.NoError(t, err)
	require.EqualValues(t, originalMint+(uint64(numTransactions/2)*koinPerTransaction), bobBalance)
	t.Logf("Bob has %d KOIN satoshis", bobBalance)
}
