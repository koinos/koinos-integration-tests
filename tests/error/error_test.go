package error

import (
	"github.com/koinos/koinos-proto-golang/koinos/canonical"
	"github.com/koinos/koinos-proto-golang/koinos/chain"
	"github.com/koinos/koinos-proto-golang/koinos/protocol"
	util "github.com/koinos/koinos-util-golang"
	kjsonrpc "github.com/koinos/koinos-util-golang/rpc"
	"github.com/stretchr/testify/require"

	"koinos-integration-tests/integration"
	"testing"
)

func TestGovernance(t *testing.T) {
	client := kjsonrpc.NewKoinosRPCClient("http://localhost:8080/")

	genesisKey, err := integration.GetKey(integration.Genesis)
	integration.NoError(t, err)

	t.Logf("Generating key for exit contract")
	exitKey, err := util.GenerateKoinosKey()
	integration.NoError(t, err)

	integration.AwaitChain(t, client)

	t.Logf("Uploading exit contract")
	err = integration.UploadSystemContract(client, "../../contracts/exit.wasm", exitKey)
	integration.NoError(t, err)

	t.Logf("Calling exit contract with reversion")

	c := chain.Result{Code: 1, Value: []byte("reversion")}
	b, err := canonical.Marshal(&c)
	require.NoError(t, err)

	callContract := &protocol.Operation{
		Op: &protocol.Operation_CallContract{
			CallContract: &protocol.CallContractOperation{
				ContractId: exitKey.AddressBytes(),
				EntryPoint: uint32(0x00),
				Args:       b,
			},
		},
	}

	tx, err := integration.CreateTransaction(client, []*protocol.Operation{callContract}, genesisKey)
	integration.NoError(t, err)

	_, err = integration.CreateBlock(client, []*protocol.Transaction{tx})
	require.Error(t, err)

	t.Logf("Calling Exit contract with failure")

	c = chain.Result{Code: -1, Value: []byte("failure")}
	b, err = canonical.Marshal(&c)
	require.NoError(t, err)

	callContract = &protocol.Operation{
		Op: &protocol.Operation_CallContract{
			CallContract: &protocol.CallContractOperation{
				ContractId: exitKey.AddressBytes(),
				EntryPoint: uint32(0x00),
				Args:       b,
			},
		},
	}

	tx, err = integration.CreateTransaction(client, []*protocol.Operation{callContract}, genesisKey)
	integration.NoError(t, err)

	_, err = integration.CreateBlock(client, []*protocol.Transaction{tx})
	require.Error(t, err)
}
