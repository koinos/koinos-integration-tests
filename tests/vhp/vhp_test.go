package vhp

import (
	"koinos-integration-tests/integration"
	"koinos-integration-tests/integration/token"
	"testing"

	util "github.com/koinos/koinos-util-golang"
	"github.com/stretchr/testify/require"
)

func TestVhp(t *testing.T) {
	client := integration.NewKoinosMQClient("amqp://guest:guest@localhost:5672/")

	t.Logf("Generating key for alice")
	aliceKey, err := util.GenerateKoinosKey()
	integration.NoError(t, err)

	t.Logf("Generating key for bob")
	bobKey, err := util.GenerateKoinosKey()
	integration.NoError(t, err)

	require.NotEqualValues(t, aliceKey, bobKey)

	vhpKey, err := integration.GetKey(integration.Vhp)
	integration.NoError(t, err)

	integration.AwaitChain(t, client)

	t.Logf("Uploading VHP contract")
	err = integration.UploadSystemContract(client, "../../contracts/vhp.wasm", vhpKey)
	integration.NoError(t, err)

	t.Logf("Minting 1000 satoshis to alice")
	vhp := token.GetVhpToken(client)
	err = vhp.Mint(aliceKey.AddressBytes(), uint64(1000))
	integration.NoError(t, err)

	supply, err := vhp.TotalSupply()
	integration.NoError(t, err)

	require.EqualValues(t, uint64(1000), supply)

	t.Logf("Transferring 500 satoshi from alice to bob")
	err = vhp.Transfer(aliceKey, bobKey.AddressBytes(), uint64(500))
	integration.NoError(t, err)

	t.Logf("Ensuring total supply remains unchanged")
	supply, err = vhp.TotalSupply()
	integration.NoError(t, err)

	require.EqualValues(t, uint64(1000), supply)

	t.Logf("Minting 500 satoshis to bob")
	err = vhp.Mint(bobKey.AddressBytes(), uint64(500))
	integration.NoError(t, err)

	supply, err = vhp.TotalSupply()
	integration.NoError(t, err)

	t.Logf("Ensuring total supply is 1500")
	require.EqualValues(t, uint64(1500), supply)

	t.Logf("Asserting alice's balance is 500")
	aliceBalance, err := vhp.Balance(aliceKey.AddressBytes())
	integration.NoError(t, err)

	require.EqualValues(t, uint64(500), aliceBalance)

	t.Logf("Asserting bob's balance is 1000")
	bobBalance, err := vhp.Balance(bobKey.AddressBytes())
	integration.NoError(t, err)

	require.EqualValues(t, uint64(1000), bobBalance)

	t.Logf("Burning 100 satoshi from bob's balance")

	err = vhp.Burn(bobKey, uint64(100))
	integration.NoError(t, err)

	t.Logf("Ensuring total supply is 1400")
	supply, err = vhp.TotalSupply()
	integration.NoError(t, err)

	require.EqualValues(t, uint64(1400), supply)

	t.Logf("Asserting bob's balance is 900")
	bobBalance, err = vhp.Balance(bobKey.AddressBytes())
	integration.NoError(t, err)

	require.EqualValues(t, uint64(900), bobBalance)
}
