package pob

import (
	"context"
	"koinos-integration-tests/integration"
	"koinos-integration-tests/integration/token"
	"testing"
	"time"

	"github.com/koinos/koinos-proto-golang/koinos/chain"
	"github.com/koinos/koinos-proto-golang/koinos/contracts/pob"
	"github.com/koinos/koinos-proto-golang/koinos/protocol"
	kjsonrpc "github.com/koinos/koinos-util-golang/rpc"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

const (
	burnEntry                  uint32 = 0x859facc5
	registerPublicKeyEntry            = 0x53192be1
	processBlockSignatureEntry        = 0xe0adbeab
)

type void struct{}

func TestPob(t *testing.T) {
	client := kjsonrpc.NewKoinosRPCClient("http://localhost:8080/")

	koinKey, err := integration.GetKey(integration.Koin)
	integration.NoError(t, err)

	vhpKey, err := integration.GetKey(integration.Vhp)
	integration.NoError(t, err)

	pobKey, err := integration.GetKey(integration.Pob)
	integration.NoError(t, err)

	producerKey, err := integration.GetKey(integration.PobProducer)
	integration.NoError(t, err)

	genesisKey, err := integration.GetKey(integration.Genesis)
	integration.NoError(t, err)

	koin := token.GetKoinToken(client)
	vhp := token.GetVhpToken(client)

	integration.AwaitChain(t, client)

	t.Logf("Uploading KOIN contract")
	err = integration.UploadSystemContract(client, "../../contracts/koin.wasm", koinKey)
	integration.NoError(t, err)

	t.Logf("Uploading VHP contract")
	err = integration.UploadSystemContract(client, "../../contracts/vhp.wasm", vhpKey)
	integration.NoError(t, err)

	t.Logf("Uploading PoB contract")
	err = integration.UploadSystemContract(client, "../../contracts/pob.wasm", pobKey)
	integration.NoError(t, err)

	t.Logf("Minting KOIN")
	koin.Mint(producerKey.AddressBytes(), 100000000000000) // 1,000,000.00000000 KOIN

	producerBalance, err := koin.Balance(producerKey.AddressBytes())
	integration.NoError(t, err)
	require.EqualValues(t, uint64(100000000000000), producerBalance)

	t.Logf("Burning KOIN")
	burnArgs := &pob.BurnArguments{
		TokenAmount: 1000000000000, // 10,000.00000000 KOIN
		BurnAddress: producerKey.AddressBytes(),
		VhpAddress:  producerKey.AddressBytes(),
	}

	args, err := proto.Marshal(burnArgs)
	integration.NoError(t, err)

	burnKoin := &protocol.Operation{
		Op: &protocol.Operation_CallContract{
			CallContract: &protocol.CallContractOperation{
				ContractId: pobKey.AddressBytes(),
				EntryPoint: burnEntry,
				Args:       args,
			},
		},
	}

	tx, err := integration.CreateTransaction(client, []*protocol.Operation{burnKoin}, producerKey)
	integration.NoError(t, err)

	receipt, err := integration.CreateBlock(client, []*protocol.Transaction{tx}, genesisKey)
	integration.NoError(t, err)

	require.EqualValues(t, len(receipt.TransactionReceipts), 1, "Expected 1 transaction receipt")
	require.EqualValues(t, len(receipt.TransactionReceipts[0].Events), 2, "Expected 2 events in transaction receipt")

	// TODO: Check events

	producerBalance, err = koin.Balance(producerKey.AddressBytes())
	integration.NoError(t, err)
	require.EqualValues(t, uint64(99000000000000), producerBalance)

	producerVhpBalance, err := vhp.Balance(producerKey.AddressBytes())
	integration.NoError(t, err)
	require.EqualValues(t, uint64(burnArgs.TokenAmount), producerVhpBalance)

	t.Logf("Register Public Key")
	registerArgs := &pob.RegisterPublicKeyArguments{
		Producer:  producerKey.AddressBytes(),
		PublicKey: producerKey.PublicBytes(),
	}

	args, err = proto.Marshal(registerArgs)
	integration.NoError(t, err)

	registerKey := &protocol.Operation{
		Op: &protocol.Operation_CallContract{
			CallContract: &protocol.CallContractOperation{
				ContractId: pobKey.AddressBytes(),
				EntryPoint: registerPublicKeyEntry,
				Args:       args,
			},
		},
	}

	tx, err = integration.CreateTransaction(client, []*protocol.Operation{registerKey}, producerKey)
	integration.NoError(t, err)

	receipt, err = integration.CreateBlock(client, []*protocol.Transaction{tx}, genesisKey)
	integration.NoError(t, err)

	require.EqualValues(t, len(receipt.TransactionReceipts), 1, "Expected 1 transaction receipt")
	require.EqualValues(t, len(receipt.TransactionReceipts[0].Events), 1, "Expected 1 events in transaction receipt")

	// TODO: Check event

	integration.CreateBlocks(client, 20, genesisKey)

	t.Logf("Enabling PoB")

	enablePoB := &protocol.Operation{
		Op: &protocol.Operation_SetSystemCall{
			SetSystemCall: &protocol.SetSystemCallOperation{
				CallId: uint32(chain.SystemCallId_process_block_signature),
				Target: &protocol.SystemCallTarget{
					Target: &protocol.SystemCallTarget_SystemCallBundle{
						SystemCallBundle: &protocol.ContractCallBundle{
							ContractId: pobKey.AddressBytes(),
							EntryPoint: processBlockSignatureEntry,
						},
					},
				},
			},
		},
	}

	tx, err = integration.CreateTransaction(client, []*protocol.Operation{enablePoB}, genesisKey)
	integration.NoError(t, err)

	receipt, err = integration.CreateBlock(client, []*protocol.Transaction{tx}, genesisKey)
	integration.NoError(t, err)

	headInfo, err := integration.GetHeadInfo(client)
	integration.NoError(t, err)

	endBlock := headInfo.HeadTopology.Height + 10

	test_timer := time.NewTimer(30 * time.Second)
	cancelChan := make(chan void)
	go func() {
		select {
		case <-cancelChan:
			return
		case <-test_timer.C:
			panic("Timer expired")
		}
	}()

	for {
		headInfo, err = integration.GetHeadInfo(client)
		integration.NoError(t, err)

		t.Logf("Block Height %d", headInfo.HeadTopology.Height)

		if headInfo.HeadTopology.Height > endBlock {
			cancelChan <- void{}
			break
		}

		time.Sleep(time.Second)
	}

	// Set the public key again. Should trigger key delay
	//tx, err = integration.CreateTransaction(client, []*protocol.Operation{registerKey}, producerKey)
	//integration.NoError(t, err)

	txReceipt, err := client.SubmitTransaction(context.Background(), []*protocol.Operation{registerKey}, producerKey, &kjsonrpc.SubmissionParams{Nonce: 3, RCLimit: 0}, true)
	integration.NoError(t, err)

	require.EqualValues(t, len(txReceipt.Events), 1, "Expected 1 events in transaction receipt")

	headInfo, err = integration.GetHeadInfo(client)
	integration.NoError(t, err)

	headBlock := headInfo.HeadTopology.Height

	for {
		time.Sleep(time.Second)

		headInfo, err = integration.GetHeadInfo(client)
		integration.NoError(t, err)

		if headInfo.HeadTopology.Height == headBlock {
			break
		}

		headBlock = headInfo.HeadTopology.Height
	}

	<-(time.NewTimer(5 * time.Second).C)
	headInfo, err = integration.GetHeadInfo(client)
	integration.NoError(t, err)

	require.EqualValues(t, headBlock, headInfo.HeadTopology.Height, "Blocks erroneously produced")
}
