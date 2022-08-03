package claim

import (
	claimUtil "koinos-integration-tests/integration/claim"

	"github.com/koinos/koinos-proto-golang/koinos/protocol"
	kjsonrpc "github.com/koinos/koinos-util-golang/rpc"
	"github.com/stretchr/testify/require"

	"koinos-integration-tests/integration"
	"testing"
)

func TestClaim(t *testing.T) {
	client := kjsonrpc.NewKoinosRPCClient("http://localhost:8080/")

	//genesisKey, err := integration.GetKey(integration.Genesis)
	//integration.NoError(t, err)

	claimKey, err := integration.GetKey(integration.Claim)
	integration.NoError(t, err)

	koinKey, err := integration.GetKey(integration.Koin)
	integration.NoError(t, err)

	integration.AwaitChain(t, client)

	t.Logf("Uploading KOIN contract")
	err = integration.UploadSystemContract(client, "../../contracts/koin.wasm", koinKey)
	integration.NoError(t, err)

	t.Logf("Uploading claim contract")
	err = integration.UploadSystemContract(client, "../../contracts/claim.wasm", claimKey, func(op *protocol.UploadContractOperation) error {
		op.AuthorizesTransactionApplication = true
		return nil
	})
	integration.NoError(t, err)

	cl := claimUtil.NewClaim(client)

	t.Logf("Checking initial info")
	testInfo(t, cl, 0, 0, 4, 4682918988467)
}

func testInfo(t *testing.T, cl *claimUtil.Claim, eth_claimed uint32, koin_claimed uint64, total_eth uint32, total_koin uint64) {
	info, err := cl.GetInfo()
	integration.NoError(t, err)

	require.EqualValues(t, 4, info.TotalEthAccounts)
	require.EqualValues(t, 4682918988467, info.TotalKoin)
	require.EqualValues(t, 0, info.KoinClaimed)
	require.EqualValues(t, 0, info.EthAccountsClaimed)
}
