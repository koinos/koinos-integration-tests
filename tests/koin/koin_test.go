package koin

import (
	"koinos-integration-tests/integration"
	"testing"

	util "github.com/koinos/koinos-util-golang"
	kjsonrpc "github.com/koinos/koinos-util-golang/rpc"
	"github.com/stretchr/testify/assert"
)

func TestKoin(t *testing.T) {
	client := kjsonrpc.NewKoinosRPCClient("http://localhost:8080/")

	t.Logf("Generating key for alice")
	aliceKey, err := util.GenerateKoinosKey()
	integration.NoError(t, err)

	t.Logf("Generating key for bob")
	bobKey, err := util.GenerateKoinosKey()
	integration.NoError(t, err)

	assert.NotEqualValues(t, aliceKey, bobKey)

	koinKey, err := integration.GetKey(integration.Koin)
	integration.NoError(t, err)

	integration.AwaitChain(t, client)

	t.Logf("Uploading KOIN contract")
	err = integration.UploadSystemContract(client, "../../contracts/koin.wasm", koinKey)
	integration.NoError(t, err)

	t.Logf("Minting 1000 satoshis to alice")
	err = integration.KoinMint(client, aliceKey.AddressBytes(), uint64(1000))
	integration.NoError(t, err)

	supply, err := integration.KoinTotalSupply(client)
	integration.NoError(t, err)

	assert.EqualValues(t, uint64(1000), supply)

	t.Logf("Transferring 500 satoshi from alice to bob")
	err = integration.KoinTransfer(client, aliceKey, bobKey.AddressBytes(), uint64(500))
	integration.NoError(t, err)

	t.Logf("Ensuring total supply remains unchanged")
	supply, err = integration.KoinTotalSupply(client)
	integration.NoError(t, err)

	assert.EqualValues(t, uint64(1000), supply)

	t.Logf("Minting 500 satoshis to bob")
	err = integration.KoinMint(client, bobKey.AddressBytes(), uint64(500))
	integration.NoError(t, err)

	supply, err = integration.KoinTotalSupply(client)
	integration.NoError(t, err)

	t.Logf("Ensuring total supply is 1500")
	assert.EqualValues(t, uint64(1500), supply)

	t.Logf("Asserting alice's balance is 500")
	aliceBalance, err := integration.KoinBalance(client, aliceKey.AddressBytes())
	integration.NoError(t, err)

	assert.EqualValues(t, uint64(500), aliceBalance)

	t.Logf("Asserting bob's balance is 1000")
	bobBalance, err := integration.KoinBalance(client, bobKey.AddressBytes())
	integration.NoError(t, err)

	assert.EqualValues(t, uint64(1000), bobBalance)
}
