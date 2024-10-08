package koin

import (
	"koinos-integration-tests/integration"
	"koinos-integration-tests/integration/token"
	"math"
	"testing"

	util "github.com/koinos/koinos-util-golang/v2"
	"github.com/stretchr/testify/require"
)

func TestKoin(t *testing.T) {
	client := integration.NewKoinosMQClient("amqp://guest:guest@localhost:25673/")

	t.Logf("Generating key for alice")
	aliceKey, err := util.GenerateKoinosKey()
	integration.NoError(t, err)

	t.Logf("Generating key for bob")
	bobKey, err := util.GenerateKoinosKey()
	integration.NoError(t, err)

	require.NotEqualValues(t, aliceKey, bobKey)

	koinKey, err := integration.GetKey(integration.Koin)
	integration.NoError(t, err)

	integration.AwaitChain(t, client)

	integration.InitNameService(t, client)
	integration.InitGetContractMetadata(t, client)

	t.Logf("Uploading KOIN contract")
	_, err = integration.UploadSystemContract(client, "../../contracts/koin.wasm", koinKey, "koin")
	integration.NoError(t, err)

	t.Logf("Minting 1000 satoshis to alice")
	koin := token.GetKoinToken(client)
	err = koin.Mint(aliceKey.AddressBytes(), uint64(1000))
	integration.NoError(t, err)

	supply, err := koin.TotalSupply()
	integration.NoError(t, err)

	require.EqualValues(t, uint64(1000), supply)

	t.Logf("Fail to transfer 1001 satoshi from alice to bob")
	err = koin.Transfer(aliceKey, bobKey.AddressBytes(), uint64(1001))
	integration.NoError(t, err)

	balance, err := koin.Balance(aliceKey.AddressBytes())
	integration.NoError(t, err)
	require.EqualValues(t, uint64(1000), balance)

	balance, err = koin.Balance(bobKey.AddressBytes())
	integration.NoError(t, err)
	require.EqualValues(t, uint64(0), balance)

	supply, err = koin.TotalSupply()
	integration.NoError(t, err)

	require.EqualValues(t, uint64(1000), supply)

	t.Logf("Fail to overflow 64-bit unsigned integer during mint")
	err = koin.Mint(aliceKey.AddressBytes(), (math.MaxUint64-supply)+1)
	integration.NoError(t, err)

	balance, err = koin.Balance(aliceKey.AddressBytes())
	integration.NoError(t, err)
	require.EqualValues(t, uint64(1000), balance)

	balance, err = koin.Balance(bobKey.AddressBytes())
	integration.NoError(t, err)
	require.EqualValues(t, uint64(0), balance)

	supply, err = koin.TotalSupply()
	integration.NoError(t, err)

	require.EqualValues(t, uint64(1000), supply)

	t.Logf("Fail to burn more than balance")
	balance, err = koin.Balance(aliceKey.AddressBytes())
	integration.NoError(t, err)
	require.EqualValues(t, uint64(1000), balance)

	err = koin.Burn(aliceKey, balance+1)
	integration.NoError(t, err)

	balance, err = koin.Balance(aliceKey.AddressBytes())
	integration.NoError(t, err)
	require.EqualValues(t, uint64(1000), balance)

	balance, err = koin.Balance(bobKey.AddressBytes())
	integration.NoError(t, err)
	require.EqualValues(t, uint64(0), balance)

	supply, err = koin.TotalSupply()
	integration.NoError(t, err)

	require.EqualValues(t, uint64(1000), supply)

	t.Logf("Transferring 500 satoshi from alice to bob")
	err = koin.Transfer(aliceKey, bobKey.AddressBytes(), uint64(500))
	integration.NoError(t, err)

	t.Logf("Ensuring total supply remains unchanged")
	supply, err = koin.TotalSupply()
	integration.NoError(t, err)

	require.EqualValues(t, uint64(1000), supply)

	t.Logf("Minting 500 satoshis to bob")
	err = koin.Mint(bobKey.AddressBytes(), uint64(500))
	integration.NoError(t, err)

	supply, err = koin.TotalSupply()
	integration.NoError(t, err)

	t.Logf("Ensuring total supply is 1500")
	require.EqualValues(t, uint64(1500), supply)

	t.Logf("Asserting alice's balance is 500")
	aliceBalance, err := koin.Balance(aliceKey.AddressBytes())
	integration.NoError(t, err)

	require.EqualValues(t, uint64(500), aliceBalance)

	t.Logf("Asserting bob's balance is 1000")
	bobBalance, err := koin.Balance(bobKey.AddressBytes())
	integration.NoError(t, err)

	require.EqualValues(t, uint64(1000), bobBalance)

	t.Logf("Burning 100 satoshi from bob's balance")

	err = koin.Burn(bobKey, uint64(100))
	integration.NoError(t, err)

	t.Logf("Ensuring total supply is 1400")
	supply, err = koin.TotalSupply()
	integration.NoError(t, err)

	require.EqualValues(t, uint64(1400), supply)

	t.Logf("Asserting bob's balance is 900")
	bobBalance, err = koin.Balance(bobKey.AddressBytes())
	integration.NoError(t, err)

	require.EqualValues(t, uint64(900), bobBalance)
}
