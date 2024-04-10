package error

import (
	"github.com/koinos/koinos-proto-golang/v2/koinos/canonical"
	"github.com/koinos/koinos-proto-golang/v2/koinos/chain"
	"github.com/koinos/koinos-proto-golang/v2/koinos/protocol"
	util "github.com/koinos/koinos-util-golang/v2"
	kjsonrpc "github.com/koinos/koinos-util-golang/v2/rpc"
	"github.com/stretchr/testify/require"

	"koinos-integration-tests/integration"
	"testing"
)

func TestError(t *testing.T) {
	client := kjsonrpc.NewKoinosRPCClient("http://localhost:28080/")

	genesisKey, err := integration.GetKey(integration.Genesis)
	integration.NoError(t, err)

	t.Logf("Generating key for exit contract")
	exitKey, err := util.GenerateKoinosKey()
	integration.NoError(t, err)

	integration.AwaitChain(t, client)

	integration.InitNameService(t, client)

	t.Logf("Uploading exit contract")
	_, err = integration.UploadSystemContract(client, "../../contracts/exit.wasm", exitKey, "exit")
	integration.NoError(t, err)

	t.Logf("Calling exit contract with reversion")

	c := chain.ExitArguments{
		Code: 1,
		Res: &chain.Result{
			Value: &chain.Result_Error{
				Error: &chain.ErrorData{
					Message: "a reversion has occurred",
				},
			},
		},
	}
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
	require.EqualValues(t, "a reversion has occurred", err.Error(), "Unexpected error message")

	t.Logf("Calling exit contract with failure")

	c = chain.ExitArguments{
		Code: 1,
		Res: &chain.Result{
			Value: &chain.Result_Error{
				Error: &chain.ErrorData{
					Message: "a failure has occurred",
				},
			},
		},
	}
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
	require.EqualValues(t, "a failure has occurred", err.Error(), "Unexpected error message")
}
