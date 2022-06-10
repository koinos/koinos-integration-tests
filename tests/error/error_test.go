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

func TestError(t *testing.T) {
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

	einfo := chain.ErrorInfo{Message: "a reversion has occurred"}
	einfoBytes, err := canonical.Marshal(&einfo)
	integration.NoError(t, err)

	c := chain.Result{Code: 1, Value: einfoBytes}
	b, err := canonical.Marshal(&c)
	integration.NoError(t, err)

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

	_, err = integration.SubmitTransaction(client, tx)
	require.Error(t, err)

	t.Logf("Ensuring the error message was propagated to the response")
	require.EqualValues(t, err.Error(), "a reversion has occurred", "Unexpected error message")

	t.Logf("Calling exit contract with failure")

	einfo.Message = "a failure has occurred"
	einfoBytes, err = canonical.Marshal(&einfo)
	integration.NoError(t, err)

	c.Value = einfoBytes
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

	_, err = integration.SubmitTransaction(client, tx)
	require.Error(t, err)

	t.Logf("Ensuring the error message was propagated to the response")
	require.EqualValues(t, err.Error(), "a failure has occurred", "Unexpected error message")
}
