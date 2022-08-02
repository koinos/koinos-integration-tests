package failures

import (
	"koinos-integration-tests/integration"
	"strconv"
	"testing"

	"github.com/koinos/koinos-proto-golang/koinos/protocol"
	util "github.com/koinos/koinos-util-golang"
	kjsonrpc "github.com/koinos/koinos-util-golang/rpc"
	"github.com/stretchr/testify/require"
)

func TestFailures(t *testing.T) {
	client := kjsonrpc.NewKoinosRPCClient("http://localhost:8080/")

	t.Logf("Generating key for failures")
	failuresKey, err := util.GenerateKoinosKey()
	integration.NoError(t, err)

	integration.AwaitChain(t, client)

	t.Logf("Uploading failures contract")
	err = integration.UploadContract(client, "../../contracts/failures.wasm", failuresKey)
	integration.NoError(t, err)

	integration.CreateBlock(client, []*protocol.Transaction{})

	for i := uint32(1); i < 6; i++ {
		_, err := testEntryPoint(t, client, failuresKey, i)
		t.Logf("Entry: " + strconv.FormatUint(uint64(i), 10))
		require.Error(t, err)
		t.Logf("Error: " + err.Error())
	}
}

func testEntryPoint(t *testing.T, client integration.Client, key *util.KoinosKey, entryPoint uint32) (*protocol.TransactionReceipt, error) {
	aliceKey, err := util.GenerateKoinosKey()
	integration.NoError(t, err)

	op := &protocol.Operation{
		Op: &protocol.Operation_CallContract{
			CallContract: &protocol.CallContractOperation{
				ContractId: key.AddressBytes(),
				EntryPoint: entryPoint,
			},
		},
	}

	tx, err := integration.CreateTransaction(client, []*protocol.Operation{op}, aliceKey)
	integration.NoError(t, err)
	return integration.SubmitTransaction(client, tx)
}
