package add_thunk

import (
	"koinos-integration-tests/integration"
	"testing"

	"github.com/koinos/koinos-proto-golang/v2/koinos/chain"
	"github.com/koinos/koinos-proto-golang/v2/koinos/protocol"
	util "github.com/koinos/koinos-util-golang/v2"
	kjsonrpc "github.com/koinos/koinos-util-golang/v2/rpc"
	"github.com/stretchr/testify/require"
)

func TestAddThunk(t *testing.T) {
	client := kjsonrpc.NewKoinosRPCClient("http://localhost:8080/")

	genesisKey, err := integration.GetKey(integration.Genesis)
	integration.NoError(t, err)

	t.Logf("Generating key for add_thunk contract")
	addThunkKey, err := util.GenerateKoinosKey()
	integration.NoError(t, err)

	t.Logf("Generating key for call_nop contract")
	callNopKey, err := util.GenerateKoinosKey()
	integration.NoError(t, err)

	require.NotEqualValues(t, addThunkKey, callNopKey)

	integration.AwaitChain(t, client)

	maxRc, err := integration.GetAccountRc(client, genesisKey.AddressBytes())
	integration.NoError(t, err)

	t.Logf("Uploading call_nop contract")
	err = integration.UploadContract(client, "../../contracts/call_nop.wasm", callNopKey)
	integration.NoError(t, err)

	t.Logf("Calling call_nop")

	callContract := &protocol.Operation{
		Op: &protocol.Operation_CallContract{
			CallContract: &protocol.CallContractOperation{
				ContractId: callNopKey.AddressBytes(),
			},
		},
	}

	tx, err := integration.CreateTransaction(client, []*protocol.Operation{callContract}, genesisKey)
	integration.NoError(t, err)

	_, err = integration.SubmitTransaction(client, tx)
	require.Error(t, err)

	t.Logf("Uploading add_thunk contract")
	err = integration.UploadContract(client, "../../contracts/add_thunk.wasm", addThunkKey)
	integration.NoError(t, err)

	t.Logf("Check add_thunk fails when not a system contract")

	callAddThunk := &protocol.Operation{
		Op: &protocol.Operation_CallContract{
			CallContract: &protocol.CallContractOperation{
				ContractId: addThunkKey.AddressBytes(),
			},
		},
	}

	tx, err = integration.CreateTransaction(client, []*protocol.Operation{callAddThunk}, genesisKey)
	integration.NoError(t, err)

	_, err = integration.SubmitTransaction(client, tx)
	require.Error(t, err)

	t.Logf("Setting thunk")

	setSystemContract := &protocol.Operation{
		Op: &protocol.Operation_SetSystemContract{
			SetSystemContract: &protocol.SetSystemContractOperation{
				ContractId:     addThunkKey.AddressBytes(),
				SystemContract: true,
			},
		},
	}

	overrideNop := &protocol.Operation{
		Op: &protocol.Operation_SetSystemCall{
			SetSystemCall: &protocol.SetSystemCallOperation{
				CallId: uint32(chain.SystemCallId_nop),
				Target: &protocol.SystemCallTarget{
					Target: &protocol.SystemCallTarget_ThunkId{
						ThunkId: uint32(chain.SystemCallId_nop),
					},
				},
			},
		},
	}

	modRcLimit := func(t *protocol.Transaction) error {
		t.Header.RcLimit = maxRc / 4
		return nil
	}

	tx, err = integration.CreateTransaction(client, []*protocol.Operation{setSystemContract, callAddThunk, overrideNop}, genesisKey, modRcLimit)
	integration.NoError(t, err)

	_, err = integration.SubmitTransaction(client, tx)
	require.NoError(t, err)

	_, err = integration.CreateBlock(client, []*protocol.Transaction{tx}, genesisKey)
	integration.NoError(t, err)

	t.Logf("Calling call_nop again")

	tx, err = integration.CreateTransaction(client, []*protocol.Operation{callContract}, genesisKey, modRcLimit)
	integration.NoError(t, err)

	_, err = integration.SubmitTransaction(client, tx)
	require.NoError(t, err)

	t.Logf("Check error when running add_thunk again")

	tx, err = integration.CreateTransaction(client, []*protocol.Operation{callAddThunk}, genesisKey, modRcLimit)
	integration.NoError(t, err)

	_, err = integration.SubmitTransaction(client, tx)
	require.Error(t, err)
}
