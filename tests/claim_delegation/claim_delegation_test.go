package claim_delegation

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	claimUtil "koinos-integration-tests/integration/claim"
	"koinos-integration-tests/integration/name_service"
	"koinos-integration-tests/integration/token"

	"github.com/koinos/koinos-proto-golang/koinos/chain"
	"github.com/koinos/koinos-proto-golang/koinos/contracts/claim"
	tokenproto "github.com/koinos/koinos-proto-golang/koinos/contracts/token"
	"github.com/koinos/koinos-proto-golang/koinos/protocol"
	util "github.com/koinos/koinos-util-golang"
	kjsonrpc "github.com/koinos/koinos-util-golang/rpc"
	"github.com/stretchr/testify/require"

	"koinos-integration-tests/integration"
	"testing"

	"google.golang.org/protobuf/proto"
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

func TestClaimDelegation(t *testing.T) {
	client := kjsonrpc.NewKoinosRPCClient("http://localhost:8080/")

	claimKey, err := integration.GetKey(integration.Claim)
	integration.NoError(t, err)

	koinKey, err := integration.GetKey(integration.Koin)
	integration.NoError(t, err)

	governanceKey, err := integration.GetKey(integration.Governance)
	integration.NoError(t, err)

	claimDelegationKey, err := integration.GetKey(integration.ClaimDelegation)
	integration.NoError(t, err)

	integration.AwaitChain(t, client)

	integration.InitNameService(t, client)

	t.Logf("Uploading KOIN contract")
	_, err = integration.UploadSystemContract(client, "../../contracts/koin.wasm", koinKey, "koin")
	integration.NoError(t, err)

	t.Logf("Uploading claim contract")
	_, err = integration.UploadSystemContract(client, "../../contracts/claim.wasm", claimKey, "claim")
	integration.NoError(t, err)

	ns := name_service.GetNameService(client)
	genesisKey, err := integration.GetKey(integration.Genesis)
	integration.NoError(t, err)
	governanceKey, err := integration.GetKey(integration.Governance)
	integration.NoError(t, err)
	_, err = ns.SetRecord(t, genesisKey, "governance", governanceKey.AddressBytes())
	integration.NoError(t, err)

	fmt.Printf("Claim contract: %v\n", base64.StdEncoding.EncodeToString(claimKey.AddressBytes()))
	fmt.Printf("Claim delegation contract: %v\n", base64.StdEncoding.EncodeToString(claimDelegationKey.AddressBytes()))
	fmt.Printf("Koin contract: %v\n", base64.StdEncoding.EncodeToString(koinKey.AddressBytes()))

	t.Logf("Uploading claim delegation contract")
	err = integration.UploadContract(
		client,
		"../../contracts/claim_delegation.wasm",
		claimDelegationKey,
		func(op *protocol.UploadContractOperation) error {
			op.AuthorizesTransactionApplication = true
			return nil
		},
	)
	integration.NoError(t, err)

	koin := token.GetKoinToken(client)

	aliceKey, err := util.GenerateKoinosKey()
	integration.NoError(t, err)
	aliceAddress := aliceKey.AddressBytes()

	bobKey, err := util.GenerateKoinosKey()
	integration.NoError(t, err)
	bobAddress := bobKey.AddressBytes()

	t.Logf("Minting to claim delegation contract")
	koin.Mint(claimDelegationKey.AddressBytes(), 10000000000)
	delegationTokens := uint64(10000000000)
	expectedSupply := delegationTokens
	checkSupply(t, koin, expectedSupply)

	t.Logf("Minting to Alice")
	koin.Mint(aliceKey.AddressBytes(), 200000000)
	expectedSupply += 200000000

	checkSupply(t, koin, expectedSupply)

	t.Logf("Minting to Bob")
	koin.Mint(bobAddress, 1)
	expectedSupply += 1
	bobBalance, err := koin.Balance(bobAddress)
	integration.NoError(t, err)

	checkSupply(t, koin, expectedSupply)

	err = integration.SetSystemCallOverride(client, koinKey, uint32(0x2d464aab), uint32(chain.SystemCallId_get_account_rc))
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

	totalSupply, err := koin.TotalSupply()
	integration.NoError(t, err)

	t.Logf("KOIN supply: %d", totalSupply)

	t.Logf("Submitting claim")

	receipt, err := submitClaimWithDelegation(t, cl, claimAAddress, claimAPrivKey, aliceKey, claimDelegationKey)
	integration.NoError(t, err)
	integration.LogBlockReceipt(t, receipt)
	expectedSupply += claimAValue
	info.KoinClaimed += claimAValue
	info.EthAccountsClaimed++

	checkSupply(t, koin, expectedSupply)

	aliceBalance, err := koin.Balance(aliceAddress)
	integration.NoError(t, err)
	require.EqualValues(t, expectedSupply-delegationTokens-bobBalance, aliceBalance, "alice balance mismatch")

	testInfo(t, cl, info)

	claim, err := checkClaim(t, cl, claimAAddress)
	integration.NoError(t, err)
	require.EqualValues(t, claim.TokenAmount, claimAValue)
	require.EqualValues(t, claim.Claimed, true)

	t.Logf("Submitting duplicate claim")
	_, err = submitClaimWithDelegation(t, cl, claimAAddress, claimAPrivKey, aliceKey, claimDelegationKey)
	require.Error(t, err)

	testInfo(t, cl, info)

	claim, err = checkClaim(t, cl, claimAAddress)
	integration.NoError(t, err)
	require.EqualValues(t, claim.TokenAmount, claimAValue)
	require.EqualValues(t, claim.Claimed, true)

	t.Logf("Submitting a claim with the wrong signature")
	receipt, err = submitClaimWithDelegation(t, cl, claimBAddress, claimAPrivKey, aliceKey, claimDelegationKey)
	integration.NoError(t, err)
	require.EqualValues(t, len(receipt.TransactionReceipts), 1)
	require.EqualValues(t, receipt.TransactionReceipts[0].Reverted, true)

	testInfo(t, cl, info)

	claim, err = checkClaim(t, cl, claimBAddress)
	integration.NoError(t, err)
	require.EqualValues(t, claim.TokenAmount, claimBValue)
	require.EqualValues(t, claim.Claimed, false)

	t.Logf("Submitting a claim on a non-existent address")
	_, err = submitClaimWithDelegation(t, cl, bogusAddress, claimAPrivKey, aliceKey, claimDelegationKey)
	require.Error(t, err)

	testInfo(t, cl, info)

	claim, err = checkClaim(t, cl, bogusAddress)
	integration.NoError(t, err)
	require.EqualValues(t, claim.TokenAmount, 0)
	require.EqualValues(t, claim.Claimed, false)

	t.Logf("Submit remainder of claims")
	expectedAliceBalance := expectedSupply - delegationTokens - bobBalance
	expectedBobBalance := 1

	_, err = submitClaim(t, cl, claimBAddress, claimBPrivKey, bobKey)
	require.Error(t, err, "should have insufficient rc")

	_, err = submitClaimWithDelegation(t, cl, claimBAddress, claimBPrivKey, bobKey, claimDelegationKey)
	integration.NoError(t, err)
	expectedSupply += claimBValue
	expectedBobBalance += claimBValue
	info.KoinClaimed += claimBValue
	info.EthAccountsClaimed++

	checkSupply(t, koin, expectedSupply)

	aliceBalance, err = koin.Balance(aliceAddress)
	integration.NoError(t, err)
	require.EqualValues(t, expectedAliceBalance, aliceBalance, "alice balance mismatch")

	bobBalance, err = koin.Balance(bobAddress)
	integration.NoError(t, err)
	require.EqualValues(t, expectedBobBalance, bobBalance, "bob balance mismatch")

	testInfo(t, cl, info)

	claim, err = checkClaim(t, cl, claimBAddress)
	integration.NoError(t, err)
	require.EqualValues(t, claim.TokenAmount, claimBValue)
	require.EqualValues(t, claim.Claimed, true)

	_, err = submitClaimWithDelegation(t, cl, claimCAddress, claimCPrivKey, bobKey, claimDelegationKey)
	integration.NoError(t, err)
	expectedSupply += claimCValue
	expectedBobBalance += claimCValue
	info.KoinClaimed += claimCValue
	info.EthAccountsClaimed++

	checkSupply(t, koin, expectedSupply)

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

	_, err = submitClaimWithDelegation(t, cl, claimDAddress, claimDPrivKey, bobKey, claimDelegationKey)
	integration.NoError(t, err)
	expectedSupply += claimDValue
	expectedBobBalance += claimDValue
	info.KoinClaimed += claimDValue
	info.EthAccountsClaimed++

	checkSupply(t, koin, expectedSupply)

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

	t.Log("Ensure that KOIN can be removed from the claim delegation contract")
	err = transferWithWrongKey(client, koinKey, bobKey, claimDelegationKey.AddressBytes(), bobAddress, delegationTokens)
	require.Error(t, err, "bob should not authorize transfer from claim delegation contract")

	err = koin.Transfer(claimDelegationKey, bobAddress, delegationTokens)
	integration.NoError(t, err)

	claimDelegationBalance, err := koin.Balance(claimDelegationKey.AddressBytes())
	integration.NoError(t, err)

	require.EqualValues(t, 0, claimDelegationBalance, "claim delegation contract should have 0 KOIN")

	t.Log("Ensure claim delegation KOIN has been transferred to Bob")
	expectedBobBalance += int(delegationTokens)
	bobBalance, err = koin.Balance(bobAddress)
	integration.NoError(t, err)
	require.EqualValues(t, expectedBobBalance, bobBalance, "bob balance mismatch")

	checkSupply(t, koin, expectedSupply)
}

func submitClaim(t *testing.T, cl *claimUtil.Claim, pubKey string, privKey string, koinosKey *util.KoinosKey) (*protocol.BlockReceipt, error) {
	claimPubKey, err := hex.DecodeString(pubKey)
	integration.NoError(t, err)

	claimPrivKey, err := hex.DecodeString(privKey)
	integration.NoError(t, err)

	return cl.SubmitClaim(t, claimPubKey, claimPrivKey, koinosKey)
}

func submitClaimWithDelegation(t *testing.T, cl *claimUtil.Claim, pubKey string, privKey string, koinKey *util.KoinosKey, payerKey *util.KoinosKey) (*protocol.BlockReceipt, error) {
	claimPubKey, err := hex.DecodeString(pubKey)
	integration.NoError(t, err)

	claimPrivKey, err := hex.DecodeString(privKey)
	integration.NoError(t, err)

	return cl.SubmitClaimWithDelegation(t, claimPubKey, claimPrivKey, koinKey, payerKey)
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

func checkSupply(t *testing.T, koin *token.Token, expected uint64) {
	t.Logf("Ensuring total supply is %d", expected)
	supply, err := koin.TotalSupply()
	t.Logf("Total supply returned is %d", supply)
	integration.NoError(t, err)
	require.EqualValues(t, expected, supply, "total supply mismatch")
}

func transferWithWrongKey(client integration.Client, koinKey *util.KoinosKey, signerKey *util.KoinosKey, from []byte, to []byte, value uint64) error {
	const transferEntry uint32 = 0x27f576ca
	transferArgs := &tokenproto.TransferArguments{
		From:  from,
		To:    to,
		Value: value,
	}

	args, err := proto.Marshal(transferArgs)
	if err != nil {
		return err
	}

	op := &protocol.Operation{
		Op: &protocol.Operation_CallContract{
			CallContract: &protocol.CallContractOperation{
				ContractId: koinKey.AddressBytes(),
				EntryPoint: transferEntry,
				Args:       args,
			},
		},
	}

	transaction, err := integration.CreateTransaction(client, []*protocol.Operation{op}, signerKey)
	if err != nil {
		return err
	}

	_, err = integration.SubmitTransaction(client, transaction)
	return err
}
