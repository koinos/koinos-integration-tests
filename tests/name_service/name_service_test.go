package name_service

import (
	"koinos-integration-tests/integration"
	"testing"

	nameUtil "koinos-integration-tests/integration/name_service"

	"github.com/koinos/koinos-proto-golang/v2/koinos/chain"
	kjsonrpc "github.com/koinos/koinos-util-golang/v2/rpc"
	"github.com/stretchr/testify/require"
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
	_, err = integration.UploadSystemContract(client, "../../contracts/name_service.wasm", nameServiceKey, "name_service")
	integration.NoError(t, err)

	t.Logf("Overriding get_contract_name")
	err = integration.SetSystemCallOverride(client, nameServiceKey, nameUtil.GetNameEntry, uint32(chain.SystemCallId_get_contract_name))
	integration.NoError(t, err)

	t.Logf("Overriding get_contract_address")
	err = integration.SetSystemCallOverride(client, nameServiceKey, nameUtil.GetAddressEntry, uint32(chain.SystemCallId_get_contract_address))
	integration.NoError(t, err)

	t.Logf("Setting record")
	ns := nameUtil.GetNameService(client)

	// Setting records

	receipt, err := ns.SetRecord(t, genesisKey, "vhp", vhpKey.AddressBytes())
	integration.NoError(t, err)
	integration.LogBlockReceipt(t, receipt)

	require.EqualValues(t, 1, len(receipt.GetTransactionReceipts()), "Expected 1 transaction receipt")
	require.EqualValues(t, 1, len(receipt.TransactionReceipts[0].Events), "Expected 1 transaction event")
	require.EqualValues(t, "koinos.contracts.record_update_event", receipt.TransactionReceipts[0].Events[0].Name, "Unexpected event name")
	require.EqualValues(t, 1, len(receipt.TransactionReceipts[0].Events[0].Impacted), "Expected 1 impacted account")
	require.EqualValues(t, vhpKey.AddressBytes(), receipt.TransactionReceipts[0].Events[0].Impacted[0], "Unexpected impacted account")

	receipt, err = ns.SetRecord(t, genesisKey, "koin", koinKey.AddressBytes())
	integration.NoError(t, err)
	integration.LogBlockReceipt(t, receipt)

	require.EqualValues(t, 1, len(receipt.GetTransactionReceipts()), "Expected 1 transaction receipt")
	require.EqualValues(t, 1, len(receipt.TransactionReceipts[0].Events), "Expected 1 transaction event")
	require.EqualValues(t, "koinos.contracts.record_update_event", receipt.TransactionReceipts[0].Events[0].Name, "Unexpected event name")
	require.EqualValues(t, 1, len(receipt.TransactionReceipts[0].Events[0].Impacted), "Expected 1 impacted account")
	require.EqualValues(t, koinKey.AddressBytes(), receipt.TransactionReceipts[0].Events[0].Impacted[0], "Unexpected impacted account")

	aRecord, err := ns.GetAddress(t, "koin")
	integration.NoError(t, err)
	require.EqualValues(t, koinKey.AddressBytes(), aRecord.GetAddress(), "Record address mismatch")

	aRecord, err = ns.GetAddress(t, "vhp")
	integration.NoError(t, err)
	require.EqualValues(t, vhpKey.AddressBytes(), aRecord.GetAddress(), "Record address mismatch")

	nRecord, err := ns.GetName(t, koinKey.AddressBytes())
	integration.NoError(t, err)
	require.EqualValues(t, "koin", nRecord.GetName(), "Record name mismatch")

	nRecord, err = ns.GetName(t, vhpKey.AddressBytes())
	integration.NoError(t, err)
	require.EqualValues(t, "vhp", nRecord.GetName(), "Record name mismatch")

	// Updating records

	newVhpAddress := []byte{0x01, 0x02, 0x03}
	newKoinAddress := []byte{0x04, 0x05, 0x06}

	receipt, err = ns.SetRecord(t, genesisKey, "vhp", newVhpAddress)
	integration.NoError(t, err)
	integration.LogBlockReceipt(t, receipt)

	require.EqualValues(t, 1, len(receipt.GetTransactionReceipts()), "Expected 1 transaction receipt")
	require.EqualValues(t, 1, len(receipt.TransactionReceipts[0].Events), "Expected 1 transaction event")
	require.EqualValues(t, "koinos.contracts.record_update_event", receipt.TransactionReceipts[0].Events[0].Name, "Unexpected event name")
	require.EqualValues(t, 2, len(receipt.TransactionReceipts[0].Events[0].Impacted), "Expected 2 impacted accounts")
	require.EqualValues(t, newVhpAddress, receipt.TransactionReceipts[0].Events[0].Impacted[0], "Unexpected impacted account")
	require.EqualValues(t, vhpKey.AddressBytes(), receipt.TransactionReceipts[0].Events[0].Impacted[1], "Unexpected impacted account")

	receipt, err = ns.SetRecord(t, genesisKey, "koin", newKoinAddress)
	integration.NoError(t, err)
	integration.LogBlockReceipt(t, receipt)

	require.EqualValues(t, 1, len(receipt.GetTransactionReceipts()), "Expected 1 transaction receipt")
	require.EqualValues(t, 1, len(receipt.TransactionReceipts[0].Events), "Expected 1 transaction event")
	require.EqualValues(t, "koinos.contracts.record_update_event", receipt.TransactionReceipts[0].Events[0].Name, "Unexpected event name")
	require.EqualValues(t, 2, len(receipt.TransactionReceipts[0].Events[0].Impacted), "Expected 2 impacted accounts")
	require.EqualValues(t, newKoinAddress, receipt.TransactionReceipts[0].Events[0].Impacted[0], "Unexpected impacted account")
	require.EqualValues(t, koinKey.AddressBytes(), receipt.TransactionReceipts[0].Events[0].Impacted[1], "Unexpected impacted account")

	aRecord, err = ns.GetAddress(t, "koin")
	integration.NoError(t, err)
	require.EqualValues(t, newKoinAddress, aRecord.GetAddress(), "Record address mismatch")

	aRecord, err = ns.GetAddress(t, "vhp")
	integration.NoError(t, err)
	require.EqualValues(t, newVhpAddress, aRecord.GetAddress(), "Record address mismatch")

	nRecord, err = ns.GetName(t, newKoinAddress)
	integration.NoError(t, err)
	require.EqualValues(t, "koin", nRecord.GetName(), "Record name mismatch")

	nRecord, err = ns.GetName(t, newVhpAddress)
	integration.NoError(t, err)
	require.EqualValues(t, "vhp", nRecord.GetName(), "Record name mismatch")
}
