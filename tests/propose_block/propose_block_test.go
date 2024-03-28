package propose_block

import (
	"koinos-integration-tests/integration"
	"koinos-integration-tests/integration/token"
	"testing"

	kjsonrpc "github.com/koinos/koinos-util-golang/v2/rpc"
	"github.com/stretchr/testify/require"
)

func TestProposeBlock(t *testing.T) {
	client := kjsonrpc.NewKoinosRPCClient("http://localhost:8080/")

	koinKey, err := integration.GetKey(integration.Koin)
	integration.NoError(t, err)

	producerKey, err := integration.GetKey(integration.PobProducer)
	integration.NoError(t, err)

	//genesisKey, err := integration.GetKey(integration.Genesis)
	integration.NoError(t, err)

	koin := token.GetKoinToken(client)

	integration.AwaitChain(t, client)

	integration.InitNameService(t, client)

	t.Logf("Uploading KOIN contract")
	_, err = integration.UploadSystemContract(client, "../../contracts/koin.wasm", koinKey, "koin")
	integration.NoError(t, err)

	t.Logf("Minting KOIN")
	koin.Mint(producerKey.AddressBytes(), 100000000000000) // 1,000,000.00000000 KOIN

	producerBalance, err := koin.Balance(producerKey.AddressBytes())
	integration.NoError(t, err)
	require.EqualValues(t, uint64(100000000000000), producerBalance)
}
