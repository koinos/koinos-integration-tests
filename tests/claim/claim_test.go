package claim

import (
	"encoding/hex"
	claimUtil "koinos-integration-tests/integration/claim"

	"github.com/koinos/koinos-proto-golang/koinos/protocol"
	util "github.com/koinos/koinos-util-golang"
	kjsonrpc "github.com/koinos/koinos-util-golang/rpc"
	"github.com/stretchr/testify/require"

	"koinos-integration-tests/integration"
	"testing"
)

const (
	claimAPubKey  = "AAAA1a60fec04ff912D673ab974A5b847A950f8F"
	claimAPrivKey = "d2c45a63fb20d400c1ed986f0946cc8367c38da278ee090fd430df26627e9a29"
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
	err = integration.UploadSystemContract(client, "../../contracts/claim.wasm", claimKey)
	integration.NoError(t, err)

	cl := claimUtil.NewClaim(client)

	t.Logf("Checking initial info")
	testInfo(t, cl, 0, 0, 4, 4682918988467)

	// aliceKey, err := util.GenerateKoinosKey()
	// integration.NoError(t, err)

	// koin := token.GetKoinToken(client)

	// totalSupply, err := koin.TotalSupply()
	// integration.NoError(t, err)

	// t.Logf("KOIN supply: %d", totalSupply)

	// t.Logf("Minting to Alice")
	// koin.Mint(aliceKey.AddressBytes(), 200000000)

	// totalSupply, err = koin.TotalSupply()
	// integration.NoError(t, err)

	// t.Logf("Submitting claim")

	// receipt, err := submitClaim(t, cl, claimAPubKey, claimAPrivKey, aliceKey)
	// integration.NoError(t, err)
	// integration.LogBlockReceipt(t, receipt)
}

func submitClaim(t *testing.T, cl *claimUtil.Claim, pubKey string, privKey string, koinosKey *util.KoinosKey) (*protocol.BlockReceipt, error) {
	claimPubKey, err := hex.DecodeString(pubKey)
	integration.NoError(t, err)

	claimPrivKey, err := hex.DecodeString(privKey)
	integration.NoError(t, err)

	return cl.SubmitClaim(t, claimPubKey, claimPrivKey, koinosKey)
}

func testInfo(t *testing.T, cl *claimUtil.Claim, eth_claimed uint32, koin_claimed uint64, total_eth uint32, total_koin uint64) {
	info, err := cl.GetInfo()
	integration.NoError(t, err)

	require.EqualValues(t, 4, info.TotalEthAccounts)
	require.EqualValues(t, 4682918988467, info.TotalKoin)
	require.EqualValues(t, 0, info.KoinClaimed)
	require.EqualValues(t, 0, info.EthAccountsClaimed)
}
