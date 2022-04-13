package governance

import (
	"koinos-integration-tests/integration"
	"testing"

	"github.com/koinos/koinos-proto-golang/koinos/chain"
	"github.com/koinos/koinos-proto-golang/koinos/protocol"
	chainrpc "github.com/koinos/koinos-proto-golang/koinos/rpc/chain"
	kjsonrpc "github.com/koinos/koinos-util-golang/rpc"
	"github.com/stretchr/testify/assert"
)

func TestGovernance(t *testing.T) {
	client := kjsonrpc.NewKoinosRPCClient("http://localhost:8080/")

	genesisKey, err := integration.GetKey(integration.Genesis)
	integration.NoError(t, err)

	governanceKey, err := integration.GetKey(integration.Governance)
	integration.NoError(t, err)

	integration.AwaitChain(t, client)

	t.Logf("Uploading governance contract")
	err = integration.UploadSystemContract(client, "../../contracts/governance.wasm", governanceKey)
	integration.NoError(t, err)

	t.Logf("Overriding pre_block system call")
	err = integration.SetSystemCallOverride(client, governanceKey, uint32(0x531d5d4e), uint32(chain.SystemCallId_pre_block_callback))
	integration.NoError(t, err)

	t.Logf("Pushing block to ensure pre_block system call does not halt chain")
	block, err := integration.CreateBlock(client, []*protocol.Transaction{}, genesisKey)
	integration.NoError(t, err)

	err = integration.SignBlock(block, genesisKey)
	integration.NoError(t, err)

	var submitBlockResp chainrpc.SubmitBlockResponse
	err = client.Call("chain.submit_block", &chainrpc.SubmitBlockRequest{Block: block}, &submitBlockResp)
	integration.NoError(t, err)

	integration.LogBlockReceipt(t, submitBlockResp.Receipt)

	t.Logf("Querying proposals")
	proposals, err := integration.GovernanceGetProposals(client)
	integration.NoError(t, err)

	assert.EqualValues(t, 0, len(proposals), "Expected no proposals when querying governance contract")

	t.Logf("Submitting proposal")
	proposal := &protocol.Transaction{}
	proposal.Header = &protocol.TransactionHeader{}
	proposal.Id = []byte{0x01, 0x02, 0x03}
	proposal.Header.RcLimit = 10

	blockReceipt, err := integration.GovernanceSubmitProposal(client, governanceKey, proposal, 100)
	integration.NoError(t, err)

	integration.LogBlockReceipt(t, blockReceipt)

	blockEvents := integration.EventsFromBlockReceipt(blockReceipt)

	assert.EqualValues(t, 1, len(blockEvents), "Expected 1 event within the block receipt")
	assert.EqualValues(t, "proposal.submission", blockEvents[0].Name, "Expected 'proposal.submission' event in block receipt")

	t.Logf("Querying proposals")
	proposals, err = integration.GovernanceGetProposals(client)
	integration.NoError(t, err)

	assert.EqualValues(t, 1, len(proposals), "Expected 1 proposal when querying governance contract")
}
