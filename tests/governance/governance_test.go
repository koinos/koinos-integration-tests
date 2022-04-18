package governance

import (
	"koinos-integration-tests/integration"
	govUtil "koinos-integration-tests/integration/governance"
	"strconv"
	"testing"

	"github.com/koinos/koinos-proto-golang/koinos/chain"
	"github.com/koinos/koinos-proto-golang/koinos/contracts/governance"
	"github.com/koinos/koinos-proto-golang/koinos/protocol"
	util "github.com/koinos/koinos-util-golang"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

const (
	Standard   int = 0
	Governance int = 1
)

const (
	BlocksPerWeek          = 604800 / 10
	ReviewPeriod           = BlocksPerWeek
	VotePeriod             = BlocksPerWeek * 2
	ApplicationDelay       = BlocksPerWeek
	GovernanceThreshold    = 75
	StandardThreshold      = 60
	MinProposalDenominator = 1000000
	MaxProposalMultiplier  = 10
)

func TestGovernance(t *testing.T) {
	client := integration.NewKoinosMQClient("amqp://guest:guest@localhost:5672/")

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
	receipt, err := integration.CreateBlock(client, []*protocol.Transaction{}, genesisKey)
	integration.NoError(t, err)

	integration.LogBlockReceipt(t, receipt)

	testFailedProposal(t, client, CreateLogOverrideProposal, Standard)
	testSuccessfulProposal(t, client, CreateLogOverrideProposal, Standard)
}

func testSuccessfulProposal(t *testing.T, client integration.Client, proposalFactory func(client integration.Client) (*protocol.Transaction, error), proposalType int) {
	threshold := StandardThreshold
	if proposalType == Governance {
		threshold = GovernanceThreshold
	}

	genesisKey, err := integration.GetKey(integration.Genesis)
	integration.NoError(t, err)

	governanceKey, err := integration.GetKey(integration.Governance)
	integration.NoError(t, err)

	t.Logf("Generating key for hello contract")
	helloKey, err := util.GenerateKoinosKey()
	integration.NoError(t, err)

	t.Logf("Creating and uploading hello contract")
	uploadTransaction, err := integration.UploadContractTransaction(client, "../../contracts/hello.wasm", helloKey)
	integration.NoError(t, err)

	receipt, err := integration.CreateBlock(client, []*protocol.Transaction{uploadTransaction}, genesisKey)
	integration.NoError(t, err)

	integration.LogBlockReceipt(t, receipt)

	t.Logf("Querying proposals")
	gov := govUtil.GetGovernance(client)
	proposals, err := gov.GetProposals()
	integration.NoError(t, err)

	assert.EqualValues(t, 0, len(proposals), "Expected no proposals when querying governance contract")

	t.Logf("Submitting proposal")
	proposal, err := proposalFactory(client)
	integration.NoError(t, err)

	receipt, err = gov.SubmitProposal(governanceKey, proposal, 100)
	integration.NoError(t, err)

	integration.LogBlockReceipt(t, receipt)

	blockEvents := integration.EventsFromBlockReceipt(receipt)

	assert.EqualValues(t, 1, len(blockEvents), "Expected 1 event within the block receipt")
	assert.EqualValues(t, "proposal.submission", blockEvents[0].Name, "Expected 'proposal.submission' event in block receipt")

	t.Logf("Querying proposals")
	proposals, err = gov.GetProposals()
	integration.NoError(t, err)

	assert.EqualValues(t, 1, len(proposals), "Expected 1 proposal when querying governance contract")
	assert.EqualValues(t, proposals[0].Proposal.Id, proposal.Id, "Proposal ID mismatch")

	t.Logf("Querying proposals by status")
	proposals, err = gov.GetProposalsByStatus(governance.ProposalStatus_pending)
	integration.NoError(t, err)

	assert.EqualValues(t, 1, len(proposals), "Expected 1 proposal when querying governance contract")
	assert.EqualValues(t, proposals[0].Proposal.Id, proposal.Id, "Proposal ID mismatch")

	t.Logf("Querying proposals by ID")
	prec, err := gov.GetProposalById(proposal.Id)
	integration.NoError(t, err)

	assert.NotNil(t, prec, "Expected proposal from query")
	assert.EqualValues(t, prec.Proposal.Id, proposal.Id, "Proposal ID mismatch")

	t.Logf("Pushing blocks to enter the voting period")

	logPerK := func(b *protocol.Block) error {
		if b.Header.Height%1000 == 0 {
			t.Logf("Created block at height: " + strconv.FormatUint(b.Header.Height, 10))
		}
		return nil
	}
	_, err = integration.CreateBlocks(client, ReviewPeriod-1, logPerK, genesisKey)
	integration.NoError(t, err)

	receipt, err = integration.CreateBlock(client, []*protocol.Transaction{}, genesisKey)
	integration.NoError(t, err)

	integration.LogBlockReceipt(t, receipt)

	blockEvents = integration.EventsFromBlockReceipt(receipt)
	assert.EqualValues(t, 1, len(blockEvents), "Expected 1 event within the block receipt")
	assert.EqualValues(t, "proposal.status", blockEvents[0].Name, "Expected 'proposal.status' event in block receipt")

	var statusEvent governance.ProposalStatusEvent
	err = proto.Unmarshal(blockEvents[0].Data, &statusEvent)
	integration.NoError(t, err)

	t.Logf("Ensuring the correct proposal status was emitted")
	assert.EqualValues(t, statusEvent.Id, proposal.Id, "Proposal ID mismatch")
	assert.EqualValues(t, statusEvent.Status, governance.ProposalStatus_active, "Proposal status mismatch")

	t.Logf("Querying proposals")
	proposals, err = gov.GetProposals()
	integration.NoError(t, err)

	assert.EqualValues(t, 1, len(proposals), "Expected 1 proposal when querying governance contract")
	assert.EqualValues(t, proposals[0].Proposal.Id, proposal.Id, "Proposal ID mismatch")

	t.Logf("Querying proposals by status")
	proposals, err = gov.GetProposalsByStatus(governance.ProposalStatus_active)
	integration.NoError(t, err)

	assert.EqualValues(t, 1, len(proposals), "Expected 1 proposal when querying governance contract")
	assert.EqualValues(t, proposals[0].Proposal.Id, proposal.Id, "Proposal ID mismatch")

	t.Logf("Querying proposals by ID")
	prec, err = gov.GetProposalById(proposal.Id)
	integration.NoError(t, err)

	assert.NotNil(t, prec, "Expected proposal from query")
	assert.EqualValues(t, prec.Proposal.Id, proposal.Id, "Proposal ID mismatch")

	logAndVote := func(b *protocol.Block) error {
		b.Header.ApprovedProposals = append(b.Header.ApprovedProposals, proposal.Id)
		if b.Header.Height%1000 == 0 {
			t.Logf("Created block at height: " + strconv.FormatUint(b.Header.Height, 10))
		}
		return nil
	}

	receipts, err := integration.CreateBlocks(client, (VotePeriod * threshold / 100), logAndVote, genesisKey)
	integration.NoError(t, err)

	for _, receipt := range receipts {
		blockEvents = integration.EventsFromBlockReceipt(receipt)
		assert.EqualValues(t, 1, len(blockEvents), "Expected 1 event within the block receipt")
		assert.EqualValues(t, "proposal.vote", blockEvents[0].Name, "Expected 'proposal.vote' event in block receipt")
	}

	receipts, err = integration.CreateBlocks(client, (VotePeriod*(100-threshold)/100)-1, logPerK, genesisKey)
	integration.NoError(t, err)

	for _, receipt := range receipts {
		blockEvents = integration.EventsFromBlockReceipt(receipt)
		assert.EqualValues(t, 0, len(blockEvents), "Expected no events within the block receipt")
	}

	receipt, err = integration.CreateBlock(client, []*protocol.Transaction{}, genesisKey)
	integration.NoError(t, err)

	integration.LogBlockReceipt(t, receipt)

	blockEvents = integration.EventsFromBlockReceipt(receipt)
	assert.EqualValues(t, 1, len(blockEvents), "Expected 1 event within the block receipt")
	assert.EqualValues(t, "proposal.status", blockEvents[0].Name, "Expected 'proposal.status' event in block receipt")

	err = proto.Unmarshal(blockEvents[0].Data, &statusEvent)
	integration.NoError(t, err)

	t.Logf("Ensuring the correct proposal status was emitted")
	assert.EqualValues(t, statusEvent.Id, proposal.Id, "Proposal ID mismatch")
	assert.EqualValues(t, statusEvent.Status, governance.ProposalStatus_approved, "Proposal status mismatch")

	t.Logf("Querying proposals")
	proposals, err = gov.GetProposals()
	integration.NoError(t, err)

	assert.EqualValues(t, 0, len(proposals), "Expected no proposals when querying governance contract")

	t.Logf("Querying proposals by status")
	proposals, err = gov.GetProposalsByStatus(governance.ProposalStatus_approved)
	integration.NoError(t, err)

	assert.EqualValues(t, 1, len(proposals), "Expected no proposals when querying governance contract")

	t.Logf("Querying proposals by ID")
	prec, err = gov.GetProposalById(proposal.Id)
	integration.NoError(t, err)

	assert.NotNil(t, prec, "Expected no proposal from query")
}

func testFailedProposal(t *testing.T, client integration.Client, proposalFactory func(client integration.Client) (*protocol.Transaction, error), proposalType int) {
	threshold := StandardThreshold
	if proposalType == Governance {
		threshold = GovernanceThreshold
	}

	genesisKey, err := integration.GetKey(integration.Genesis)
	integration.NoError(t, err)

	governanceKey, err := integration.GetKey(integration.Governance)
	integration.NoError(t, err)

	t.Logf("Querying proposals")
	gov := govUtil.GetGovernance(client)
	proposals, err := gov.GetProposals()
	integration.NoError(t, err)

	assert.EqualValues(t, 0, len(proposals), "Expected no proposals when querying governance contract")

	t.Logf("Submitting proposal")
	proposal, err := proposalFactory(client)
	integration.NoError(t, err)

	receipt, err := gov.SubmitProposal(governanceKey, proposal, 100)
	integration.NoError(t, err)

	integration.LogBlockReceipt(t, receipt)

	blockEvents := integration.EventsFromBlockReceipt(receipt)

	assert.EqualValues(t, 1, len(blockEvents), "Expected 1 event within the block receipt")
	assert.EqualValues(t, "proposal.submission", blockEvents[0].Name, "Expected 'proposal.submission' event in block receipt")

	t.Logf("Querying proposals")
	proposals, err = gov.GetProposals()
	integration.NoError(t, err)

	assert.EqualValues(t, 1, len(proposals), "Expected 1 proposal when querying governance contract")
	assert.EqualValues(t, proposals[0].Proposal.Id, proposal.Id, "Proposal ID mismatch")

	t.Logf("Querying proposals by status")
	proposals, err = gov.GetProposalsByStatus(governance.ProposalStatus_pending)
	integration.NoError(t, err)

	assert.EqualValues(t, 1, len(proposals), "Expected 1 proposal when querying governance contract")
	assert.EqualValues(t, proposals[0].Proposal.Id, proposal.Id, "Proposal ID mismatch")

	t.Logf("Querying proposals by ID")
	prec, err := gov.GetProposalById(proposal.Id)
	integration.NoError(t, err)

	assert.NotNil(t, prec, "Expected proposal from query")
	assert.EqualValues(t, prec.Proposal.Id, proposal.Id, "Proposal ID mismatch")

	t.Logf("Pushing blocks to enter the voting period")

	logPerK := func(b *protocol.Block) error {
		if b.Header.Height%1000 == 0 {
			t.Logf("Created block at height: " + strconv.FormatUint(b.Header.Height, 10))
		}
		return nil
	}
	_, err = integration.CreateBlocks(client, ReviewPeriod-1, logPerK, genesisKey)
	integration.NoError(t, err)

	receipt, err = integration.CreateBlock(client, []*protocol.Transaction{}, genesisKey)
	integration.NoError(t, err)

	integration.LogBlockReceipt(t, receipt)

	blockEvents = integration.EventsFromBlockReceipt(receipt)
	assert.EqualValues(t, 1, len(blockEvents), "Expected 1 event within the block receipt")
	assert.EqualValues(t, "proposal.status", blockEvents[0].Name, "Expected 'proposal.status' event in block receipt")

	var statusEvent governance.ProposalStatusEvent
	err = proto.Unmarshal(blockEvents[0].Data, &statusEvent)
	integration.NoError(t, err)

	t.Logf("Ensuring the correct proposal status was emitted")
	assert.EqualValues(t, statusEvent.Id, proposal.Id, "Proposal ID mismatch")
	assert.EqualValues(t, statusEvent.Status, governance.ProposalStatus_active, "Proposal status mismatch")

	t.Logf("Querying proposals")
	proposals, err = gov.GetProposals()
	integration.NoError(t, err)

	assert.EqualValues(t, 1, len(proposals), "Expected 1 proposal when querying governance contract")
	assert.EqualValues(t, proposals[0].Proposal.Id, proposal.Id, "Proposal ID mismatch")

	t.Logf("Querying proposals by status")
	proposals, err = gov.GetProposalsByStatus(governance.ProposalStatus_active)
	integration.NoError(t, err)

	assert.EqualValues(t, 1, len(proposals), "Expected 1 proposal when querying governance contract")
	assert.EqualValues(t, proposals[0].Proposal.Id, proposal.Id, "Proposal ID mismatch")

	t.Logf("Querying proposals by ID")
	prec, err = gov.GetProposalById(proposal.Id)
	integration.NoError(t, err)

	assert.NotNil(t, prec, "Expected proposal from query")
	assert.EqualValues(t, prec.Proposal.Id, proposal.Id, "Proposal ID mismatch")

	logAndVote := func(b *protocol.Block) error {
		b.Header.ApprovedProposals = append(b.Header.ApprovedProposals, proposal.Id)
		if b.Header.Height%1000 == 0 {
			t.Logf("Created block at height: " + strconv.FormatUint(b.Header.Height, 10))
		}
		return nil
	}

	receipts, err := integration.CreateBlocks(client, (VotePeriod*threshold/100)-1, logAndVote, genesisKey)
	integration.NoError(t, err)

	for _, receipt := range receipts {
		blockEvents = integration.EventsFromBlockReceipt(receipt)
		assert.EqualValues(t, 1, len(blockEvents), "Expected 1 event within the block receipt")
		assert.EqualValues(t, "proposal.vote", blockEvents[0].Name, "Expected 'proposal.vote' event in block receipt")
	}

	receipts, err = integration.CreateBlocks(client, (VotePeriod * (threshold - 100) / 100), logPerK, genesisKey)
	integration.NoError(t, err)

	for _, receipt := range receipts {
		blockEvents = integration.EventsFromBlockReceipt(receipt)
		assert.EqualValues(t, 0, len(blockEvents), "Expected no events within the block receipt")
	}

	receipt, err = integration.CreateBlock(client, []*protocol.Transaction{}, genesisKey)
	integration.NoError(t, err)

	integration.LogBlockReceipt(t, receipt)

	blockEvents = integration.EventsFromBlockReceipt(receipt)
	assert.EqualValues(t, 1, len(blockEvents), "Expected 1 event within the block receipt")
	assert.EqualValues(t, "proposal.status", blockEvents[0].Name, "Expected 'proposal.status' event in block receipt")

	err = proto.Unmarshal(blockEvents[0].Data, &statusEvent)
	integration.NoError(t, err)

	t.Logf("Ensuring the correct proposal status was emitted")
	assert.EqualValues(t, statusEvent.Id, proposal.Id, "Proposal ID mismatch")
	assert.EqualValues(t, statusEvent.Status, governance.ProposalStatus_expired, "Proposal status mismatch")

	t.Logf("Querying proposals")
	proposals, err = gov.GetProposals()
	integration.NoError(t, err)

	assert.EqualValues(t, 0, len(proposals), "Expected no proposals when querying governance contract")

	t.Logf("Querying proposals by status")
	proposals, err = gov.GetProposalsByStatus(governance.ProposalStatus_expired)
	integration.NoError(t, err)

	assert.EqualValues(t, 0, len(proposals), "Expected no proposals when querying governance contract")

	t.Logf("Querying proposals by ID")
	prec, err = gov.GetProposalById(proposal.Id)
	integration.NoError(t, err)

	assert.Nil(t, prec, "Expected no proposal from query")
}

func CreateLogOverrideProposal(client integration.Client) (*protocol.Transaction, error) {
	governanceKey, err := integration.GetKey(integration.Governance)
	if err != nil {
		return nil, err
	}

	syscallOverrideKey, err := util.GenerateKoinosKey()
	if err != nil {
		return nil, err
	}

	wasm, err := integration.BytesFromFile("../../contracts/syscall_override.wasm", 512000)
	if err != nil {
		return nil, err
	}

	uco := protocol.UploadContractOperation{}
	uco.ContractId = syscallOverrideKey.AddressBytes()
	uco.Bytecode = wasm

	uploadOperation := &protocol.Operation{
		Op: &protocol.Operation_UploadContract{
			UploadContract: &uco,
		},
	}

	overrideOperation := &protocol.Operation{
		Op: &protocol.Operation_SetSystemCall{
			SetSystemCall: &protocol.SetSystemCallOperation{
				CallId: uint32(chain.SystemCallId_log),
				Target: &protocol.SystemCallTarget{
					Target: &protocol.SystemCallTarget_SystemCallBundle{
						SystemCallBundle: &protocol.ContractCallBundle{
							ContractId: syscallOverrideKey.AddressBytes(),
							EntryPoint: uint32(0x00),
						},
					},
				},
			},
		},
	}

	return integration.CreateTransaction(client, []*protocol.Operation{uploadOperation, overrideOperation}, governanceKey)
}
