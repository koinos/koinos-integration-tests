package name_service

import (
	"koinos-integration-tests/integration"
	"testing"

	nameUtil "koinos-integration-tests/integration/name_service"

	"github.com/koinos/koinos-proto-golang/koinos/chain"
	kjsonrpc "github.com/koinos/koinos-util-golang/rpc"
)

func TestNameService(t *testing.T) {
	client := kjsonrpc.NewKoinosRPCClient("http://localhost:8080/")

	genesisKey, err := integration.GetKey(integration.Genesis)
	integration.NoError(t, err)

	koinKey, err := integration.GetKey(integration.Koin)
	integration.NoError(t, err)

	vhpKey, err := integration.GetKey(integration.Vhp)
	integration.NoError(t, err)

	nameServiceKey, err := integration.GetKey(integration.NameService)
	integration.NoError(t, err)

	integration.AwaitChain(t, client)

	t.Logf("Uploading Name Service contract")
	err = integration.UploadSystemContract(client, "../../contracts/name_service.wasm", nameServiceKey)
	integration.NoError(t, err)

	t.Logf("Overriding get_contract_name")
	err = integration.SetSystemCallOverride(client, nameServiceKey, nameUtil.GetNameEntry, uint32(chain.SystemCallId_get_contract_name))
	integration.NoError(t, err)

	t.Logf("Overriding get_contract_address")
	err = integration.SetSystemCallOverride(client, nameServiceKey, nameUtil.GetAddressEntry, uint32(chain.SystemCallId_get_contract_name))
	integration.NoError(t, err)

	t.Logf("Setting record")
	ns := nameUtil.GetNameService(client)

	receipt, err := ns.SetRecord(t, genesisKey, "koin", koinKey.AddressBytes())
	integration.NoError(t, err)
	integration.LogBlockReceipt(t, receipt)

	receipt, err = ns.SetRecord(t, genesisKey, "vhp", vhpKey.AddressBytes())
	integration.NoError(t, err)
	integration.LogBlockReceipt(t, receipt)
}
