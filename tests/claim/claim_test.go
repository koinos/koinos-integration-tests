package claim

import (
	"encoding/hex"
	claimUtil "koinos-integration-tests/integration/claim"
	"koinos-integration-tests/integration/token"

	"github.com/koinos/koinos-proto-golang/v2/koinos/contracts/claim"
	"github.com/koinos/koinos-proto-golang/v2/koinos/protocol"
	util "github.com/koinos/koinos-util-golang/v2"
	kjsonrpc "github.com/koinos/koinos-util-golang/v2/rpc"
	"github.com/stretchr/testify/require"

	"koinos-integration-tests/integration"
	"testing"
)

const (
	claimAAddress = "AAAA1a60fec04ff912D673ab974A5b847A950f8F"
	claimAPrivKey = "d2c45a63fb20d400c1ed986f0946cc8367c38da278ee090fd430df26627e9a29"
	claimAValue   = 9900000000
	claimBAddress = "BBBB49877B724A6cDd5DdF22A11c739Fe0BD8625"
	claimBPrivKey = "1c3855b94a9ec214ba5eeedc1d7bea8c2b710562069cd6078774b57442fe90ce"
	claimBValue   = 50000000000
	claimCAddress = "CCCC60CE8B8FDfdC97d13619790986e64D02Ba3F"
	claimCPrivKey = "8e96108ba841651d30f5539485aee74e692437ff8b2ba80600102c43ad62ae22"
	claimCValue   = 4598962988467
	claimDAddress = "DDDDAA294C11235eB8B129B9b474B89e38eB685E"
	claimDPrivKey = "18f2a960234f5ea1ea09d6d05d77fd7fd45e7d6ede4d941d652002e0fa5a8141"
	claimDValue   = 24056000000
	bogusAddress  = "DEADBEEFDEADBEEFDEADBEEFDEADBEEFDEADBEEF"
)

func TestClaim(t *testing.T) {
	client := kjsonrpc.NewKoinosRPCClient("http://localhost:28080/")

	claimKey, err := integration.GetKey(integration.Claim)
	integration.NoError(t, err)

	koinKey, err := integration.GetKey(integration.Koin)
	integration.NoError(t, err)

	integration.AwaitChain(t, client)

	integration.InitNameService(t, client)

	t.Logf("Uploading KOIN contract")
	_, err = integration.UploadSystemContract(client, "../../contracts/koin.wasm", koinKey, "koin")
	integration.NoError(t, err)

	t.Logf("Uploading claim contract")
	_, err = integration.UploadSystemContract(client, "../../contracts/claim.wasm", claimKey, "claim")
	integration.NoError(t, err)

	cl := claimUtil.NewClaim(client)

	t.Logf("Checking initial info")
	info := &claim.ClaimInfo{
		TotalEthAccounts:   4,
		TotalKoin:          4682918988467,
		KoinClaimed:        0,
		EthAccountsClaimed: 0,
	}
	testInfo(t, cl, info)

	aliceKey, err := util.GenerateKoinosKey()
	integration.NoError(t, err)
	aliceAddress := aliceKey.AddressBytes()

	bobKey, err := util.GenerateKoinosKey()
	integration.NoError(t, err)
	bobAddress := bobKey.AddressBytes()

	koin := token.GetKoinToken(client)

	totalSupply, err := koin.TotalSupply()
	integration.NoError(t, err)

	t.Logf("KOIN supply: %d", totalSupply)

	t.Logf("Minting to Alice")
	koin.Mint(aliceKey.AddressBytes(), 200000000)
	expectedSupply := 200000000

	integration.NoError(t, err)

	t.Logf("Submitting claim with missing signature of Koinos address")
	receipt, err := submitClaim(t, cl, claimAAddress, claimAPrivKey, aliceKey, bobKey)
	integration.NoError(t, err)
	require.EqualValues(t, 1, len(receipt.GetTransactionReceipts()), "expected 1 transaction receipt")
	require.EqualValues(t, 1, len(receipt.GetTransactionReceipts()[0].GetLogs()), "expected 1 log entry")
	require.EqualValues(t, true, receipt.GetTransactionReceipts()[0].Reverted, "expected transaction to be reverted")
	require.EqualValues(t, "transaction reverted: transaction was not signed by the destination KOIN address", receipt.GetTransactionReceipts()[0].GetLogs()[0], "expected reversion with missing signature from koinos address")

	t.Logf("Submitting claim")
	receipt, err = submitClaim(t, cl, claimAAddress, claimAPrivKey, aliceKey, aliceKey)
	integration.NoError(t, err)
	integration.LogBlockReceipt(t, receipt)
	expectedSupply += claimAValue
	info.KoinClaimed += claimAValue
	info.EthAccountsClaimed++

	totalSupply, err = koin.TotalSupply()
	integration.NoError(t, err)
	require.EqualValues(t, expectedSupply, totalSupply, "total supply mismatch")

	aliceBalance, err := koin.Balance(aliceAddress)
	integration.NoError(t, err)
	require.EqualValues(t, expectedSupply, aliceBalance, "alice balance mismatch")

	testInfo(t, cl, info)

	claim, err := checkClaim(t, cl, claimAAddress)
	integration.NoError(t, err)
	require.EqualValues(t, claim.TokenAmount, claimAValue)
	require.EqualValues(t, claim.Claimed, true)

	t.Logf("Submitting duplicate claim")
	receipt, err = submitClaim(t, cl, claimAAddress, claimAPrivKey, aliceKey, aliceKey)
	integration.NoError(t, err)
	require.EqualValues(t, len(receipt.TransactionReceipts), 1)
	require.EqualValues(t, receipt.TransactionReceipts[0].Reverted, true)

	testInfo(t, cl, info)

	claim, err = checkClaim(t, cl, claimAAddress)
	integration.NoError(t, err)
	require.EqualValues(t, claim.TokenAmount, claimAValue)
	require.EqualValues(t, claim.Claimed, true)

	t.Logf("Submitting a claim with the wrong signature")
	receipt, err = submitClaim(t, cl, claimBAddress, claimAPrivKey, aliceKey, aliceKey)
	integration.NoError(t, err)
	require.EqualValues(t, len(receipt.TransactionReceipts), 1)
	require.EqualValues(t, receipt.TransactionReceipts[0].Reverted, true)

	testInfo(t, cl, info)

	claim, err = checkClaim(t, cl, claimBAddress)
	integration.NoError(t, err)
	require.EqualValues(t, claim.TokenAmount, claimBValue)
	require.EqualValues(t, claim.Claimed, false)

	t.Logf("Submitting a claim on a non-existent address")
	_, err = submitClaim(t, cl, bogusAddress, claimAPrivKey, aliceKey, aliceKey)
	integration.NoError(t, err)
	require.EqualValues(t, len(receipt.TransactionReceipts), 1)
	require.EqualValues(t, receipt.TransactionReceipts[0].Reverted, true)

	testInfo(t, cl, info)

	claim, err = checkClaim(t, cl, bogusAddress)
	integration.NoError(t, err)
	require.EqualValues(t, claim.TokenAmount, 0)
	require.EqualValues(t, claim.Claimed, false)

	t.Logf("Submit remainder of claims")
	expectedAliceBalance := expectedSupply
	expectedBobBalance := 0

	_, err = submitClaim(t, cl, claimBAddress, claimBPrivKey, bobKey, bobKey)
	integration.NoError(t, err)
	expectedSupply += claimBValue
	expectedBobBalance += claimBValue
	info.KoinClaimed += claimBValue
	info.EthAccountsClaimed++

	totalSupply, err = koin.TotalSupply()
	integration.NoError(t, err)
	require.EqualValues(t, expectedSupply, totalSupply, "total supply mismatch")

	aliceBalance, err = koin.Balance(aliceAddress)
	integration.NoError(t, err)
	require.EqualValues(t, expectedAliceBalance, aliceBalance, "alice balance mismatch")

	bobBalance, err := koin.Balance(bobAddress)
	integration.NoError(t, err)
	require.EqualValues(t, expectedBobBalance, bobBalance, "bob balance mismatch")

	testInfo(t, cl, info)

	claim, err = checkClaim(t, cl, claimBAddress)
	integration.NoError(t, err)
	require.EqualValues(t, claim.TokenAmount, claimBValue)
	require.EqualValues(t, claim.Claimed, true)

	_, err = submitClaim(t, cl, claimCAddress, claimCPrivKey, bobKey, bobKey)
	integration.NoError(t, err)
	expectedSupply += claimCValue
	expectedBobBalance += claimCValue
	info.KoinClaimed += claimCValue
	info.EthAccountsClaimed++

	totalSupply, err = koin.TotalSupply()
	integration.NoError(t, err)
	require.EqualValues(t, expectedSupply, totalSupply, "total supply mismatch")

	aliceBalance, err = koin.Balance(aliceAddress)
	integration.NoError(t, err)
	require.EqualValues(t, expectedAliceBalance, aliceBalance, "alice balance mismatch")

	bobBalance, err = koin.Balance(bobAddress)
	integration.NoError(t, err)
	require.EqualValues(t, expectedBobBalance, bobBalance, "bob balance mismatch")

	testInfo(t, cl, info)

	claim, err = checkClaim(t, cl, claimCAddress)
	integration.NoError(t, err)
	require.EqualValues(t, claim.TokenAmount, claimCValue)
	require.EqualValues(t, claim.Claimed, true)

	_, err = submitClaim(t, cl, claimDAddress, claimDPrivKey, bobKey, bobKey)
	integration.NoError(t, err)
	expectedSupply += claimDValue
	expectedBobBalance += claimDValue
	info.KoinClaimed += claimDValue
	info.EthAccountsClaimed++

	totalSupply, err = koin.TotalSupply()
	integration.NoError(t, err)
	require.EqualValues(t, expectedSupply, totalSupply, "total supply mismatch")

	aliceBalance, err = koin.Balance(aliceAddress)
	integration.NoError(t, err)
	require.EqualValues(t, expectedAliceBalance, aliceBalance, "alice balance mismatch")

	bobBalance, err = koin.Balance(bobAddress)
	integration.NoError(t, err)
	require.EqualValues(t, expectedBobBalance, bobBalance, "bob balance mismatch")

	testInfo(t, cl, info)

	claim, err = checkClaim(t, cl, claimDAddress)
	integration.NoError(t, err)
	require.EqualValues(t, claim.TokenAmount, claimDValue)
	require.EqualValues(t, claim.Claimed, true)
}

func submitClaim(t *testing.T, cl *claimUtil.Claim, pubKey string, privKey string, koinosKey *util.KoinosKey, payerKey *util.KoinosKey) (*protocol.BlockReceipt, error) {
	claimPubKey, err := hex.DecodeString(pubKey)
	integration.NoError(t, err)

	claimPrivKey, err := hex.DecodeString(privKey)
	integration.NoError(t, err)

	return cl.SubmitClaim(t, claimPubKey, claimPrivKey, koinosKey, payerKey)
}

func checkClaim(t *testing.T, cl *claimUtil.Claim, address string) (*claim.ClaimStatus, error) {
	claimAddress, err := hex.DecodeString(address)
	integration.NoError(t, err)

	return cl.CheckClaim(claimAddress)
}

func testInfo(t *testing.T, cl *claimUtil.Claim, expectedInfo *claim.ClaimInfo) {
	info, err := cl.GetInfo()
	integration.NoError(t, err)

	require.EqualValues(t, expectedInfo.TotalEthAccounts, info.TotalEthAccounts)
	require.EqualValues(t, expectedInfo.TotalKoin, info.TotalKoin)
	require.EqualValues(t, expectedInfo.KoinClaimed, info.KoinClaimed)
	require.EqualValues(t, expectedInfo.EthAccountsClaimed, info.EthAccountsClaimed)
}
