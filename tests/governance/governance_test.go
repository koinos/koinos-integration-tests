package governance

import (
	"koinos-integration-tests/integration"
	govUtil "koinos-integration-tests/integration/governance"
	"koinos-integration-tests/integration/token"
	"strconv"
	"testing"

	"github.com/koinos/koinos-proto-golang/koinos/chain"
	"github.com/koinos/koinos-proto-golang/koinos/contracts/governance"
	"github.com/koinos/koinos-proto-golang/koinos/protocol"
	util "github.com/koinos/koinos-util-golang"
	kjsonrpc "github.com/koinos/koinos-util-golang/rpc"
	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

const (
	Standard   int = 0
	Governance int = 1
)

const (
	BlocksPerWeek          = 10
	ReviewPeriod           = BlocksPerWeek
	VotePeriod             = BlocksPerWeek * 2
	ApplicationDelay       = BlocksPerWeek
	GovernanceThreshold    = 75
	StandardThreshold      = 60
	MinProposalDenominator = 1000000
	MaxProposalMultiplier  = 10
)

func TestGovernance(t *testing.T) {
	client := kjsonrpc.NewKoinosRPCClient("http://localhost:8080/")

	genesisKey, err := integration.GetKey(integration.Genesis)
	integration.NoError(t, err)

	governanceKey, err := integration.GetKey(integration.Governance)
	integration.NoError(t, err)

	koinKey, err := integration.GetKey(integration.Koin)
	integration.NoError(t, err)

	integration.AwaitChain(t, client)

	t.Logf("Uploading KOIN contract")
	err = integration.UploadSystemContract(client, "../../contracts/koin.wasm", koinKey)
	integration.NoError(t, err)

	t.Logf("Uploading governance contract")
	err = integration.UploadSystemContract(client, "../../contracts/governance.wasm", governanceKey, func(op *protocol.UploadContractOperation) error {
		op.AuthorizesTransactionApplication = true
		return nil
	})
	integration.NoError(t, err)

	t.Logf("Overriding pre_block system call")
	err = integration.SetSystemCallOverride(client, governanceKey, uint32(0x531d5d4e), uint32(chain.SystemCallId_pre_block_callback))
	integration.NoError(t, err)

	t.Logf("Overriding require_system_authority system call")
	err = integration.SetSystemCallOverride(client, governanceKey, uint32(0xa88d06c9), uint32(chain.SystemCallId_check_system_authority))
	integration.NoError(t, err)

	t.Logf("Pushing block to ensure pre_block system call does not halt chain")
	receipt, err := integration.CreateBlock(client, []*protocol.Transaction{}, genesisKey)
	integration.NoError(t, err)

	integration.LogBlockReceipt(t, receipt)

	testFailedProposal(t, client, makeLogOverrideProposal, Standard)
	testSuccessfulProposal(t, client, makeLogOverrideProposal, Standard, testLogOverrideProposal)

	testFailedProposal(t, client, makeGovernanceRemovalProposal, Governance)
	testSuccessfulProposal(t, client, makeGovernanceRemovalProposal, Governance, testGovernanceRemovalProposal)

	testProposalFees(t, client)
}

func testProposalFees(t *testing.T, client integration.Client) {
	koin := token.GetKoinToken(client)
	gov := govUtil.GetGovernance(client)

	aliceKey, err := util.GenerateKoinosKey()
	integration.NoError(t, err)

	t.Logf("Minting to Alice")
	koin.Mint(aliceKey.AddressBytes(), 200000000)

	t.Logf("Sumbitting proposal with insufficient balance")

	op := &protocol.Operation{
		Op: &protocol.Operation_UploadContract{
			UploadContract: &protocol.UploadContractOperation{
				ContractId: aliceKey.AddressBytes(),
			},
		},
	}

	proposal, err := integration.CreateTransaction(client, []*protocol.Operation{op}, aliceKey)
	integration.NoError(t, err)

	receipt, err := gov.SubmitProposal(aliceKey, proposal, 200000001)
	integration.NoError(t, err)

	koinKey, _ := integration.GetKey(integration.Koin)
	t.Logf(base58.Encode(koinKey.AddressBytes()))

	require.EqualValues(t, 1, len(receipt.TransactionReceipts), "Expected 1 transaction within the block")
	require.EqualValues(t, 0, len(receipt.TransactionReceipts[0].Events), "Expected 0 transaction events")

	t.Logf("Submitting proposal with insufficuent fee")

	receipt, err = gov.SubmitProposal(aliceKey, proposal, 9999999)
	integration.NoError(t, err)

	require.EqualValues(t, 1, len(receipt.TransactionReceipts), "Expected 1 transaction within the block")
	require.EqualValues(t, 0, len(receipt.TransactionReceipts[0].Events), "Expected 0 transaction events")

	t.Logf("Submitting proposal with sufficient fee")

	receipt, err = gov.SubmitProposal(aliceKey, proposal, 100000000)
	integration.NoError(t, err)

	require.EqualValues(t, 1, len(receipt.TransactionReceipts), "Expected 1 transaction within the block")
	require.EqualValues(t, 2, len(receipt.TransactionReceipts[0].Events), "Expected 2 transaction events")
	require.EqualValues(t, "koin.burn", receipt.TransactionReceipts[0].Events[0].Name, "Expected KOIN Burn event")
}

func testSuccessfulProposal(t *testing.T, client integration.Client, proposalFactory func(client integration.Client) (*protocol.Transaction, error), proposalType int, onSuccess func(c integration.Client, t *testing.T) error) {
	koin := token.GetKoinToken(client)

	threshold := StandardThreshold
	if proposalType == Governance {
		threshold = GovernanceThreshold
	}

	aliceKey, err := util.GenerateKoinosKey()
	integration.NoError(t, err)

	genesisKey, err := integration.GetKey(integration.Genesis)
	integration.NoError(t, err)

	t.Logf("Querying proposals")
	gov := govUtil.GetGovernance(client)
	proposals, err := gov.GetProposals()
	integration.NoError(t, err)

	require.EqualValues(t, 0, len(proposals), "Expected no proposals when querying governance contract")

	t.Logf("Submitting proposal")
	proposal, err := proposalFactory(client)
	integration.NoError(t, err)

	err = koin.Mint(aliceKey.AddressBytes(), 200000000)
	integration.NoError(t, err)

	receipt, err := gov.SubmitProposal(aliceKey, proposal, 100000000)
	integration.NoError(t, err)

	integration.LogBlockReceipt(t, receipt)

	blockEvents := integration.EventsFromBlockReceipt(receipt)

	require.EqualValues(t, 2, len(blockEvents), "Expected 1 event within the block receipt")
	require.EqualValues(t, "koin.burn", blockEvents[0].Name, "Expected 'koin.burn' event in block receipt")
	require.EqualValues(t, "proposal.submission", blockEvents[1].Name, "Expected 'proposal.submission' event in block receipt")

	t.Logf("Querying proposals")
	proposals, err = gov.GetProposals()
	integration.NoError(t, err)

	require.EqualValues(t, 1, len(proposals), "Expected 1 proposal when querying governance contract")
	require.EqualValues(t, proposals[0].Proposal.Id, proposal.Id, "Proposal ID mismatch")
	if proposalType == Governance {
		require.EqualValues(t, proposals[0].UpdatesGovernance, true, "Governance update mismatch")
	} else {
		require.EqualValues(t, proposals[0].UpdatesGovernance, false, "Governance update mismatch")
	}

	t.Logf("Querying proposals by status")
	proposals, err = gov.GetProposalsByStatus(governance.ProposalStatus_pending)
	integration.NoError(t, err)

	require.EqualValues(t, 1, len(proposals), "Expected 1 proposal when querying governance contract")
	require.EqualValues(t, proposals[0].Proposal.Id, proposal.Id, "Proposal ID mismatch")
	if proposalType == Governance {
		require.EqualValues(t, proposals[0].UpdatesGovernance, true, "Governance update mismatch")
	} else {
		require.EqualValues(t, proposals[0].UpdatesGovernance, false, "Governance update mismatch")
	}

	t.Logf("Querying proposals by ID")
	prec, err := gov.GetProposalById(proposal.Id)
	integration.NoError(t, err)

	require.NotNil(t, prec, "Expected proposal from query")
	require.EqualValues(t, prec.Proposal.Id, proposal.Id, "Proposal ID mismatch")
	if proposalType == Governance {
		require.EqualValues(t, prec.UpdatesGovernance, true, "Governance update mismatch")
	} else {
		require.EqualValues(t, prec.UpdatesGovernance, false, "Governance update mismatch")
	}

	t.Logf("Pushing blocks to enter the voting period")

	logPerK := func(b *protocol.Block) error {
		t.Logf("Created block at height: " + strconv.FormatUint(b.Header.Height, 10))
		return nil
	}
	_, err = integration.CreateBlocks(client, ReviewPeriod-1, logPerK, genesisKey)
	integration.NoError(t, err)

	receipt, err = integration.CreateBlock(client, []*protocol.Transaction{}, genesisKey)
	integration.NoError(t, err)

	integration.LogBlockReceipt(t, receipt)

	blockEvents = integration.EventsFromBlockReceipt(receipt)
	require.EqualValues(t, 1, len(blockEvents), "Expected 1 event within the block receipt")
	require.EqualValues(t, "proposal.status", blockEvents[0].Name, "Expected 'proposal.status' event in block receipt")

	var statusEvent governance.ProposalStatusEvent
	err = proto.Unmarshal(blockEvents[0].Data, &statusEvent)
	integration.NoError(t, err)

	t.Logf("Ensuring the correct proposal status was emitted")
	require.EqualValues(t, statusEvent.Id, proposal.Id, "Proposal ID mismatch")
	require.EqualValues(t, statusEvent.Status, governance.ProposalStatus_active, "Proposal status mismatch")

	t.Logf("Querying proposals")
	proposals, err = gov.GetProposals()
	integration.NoError(t, err)

	require.EqualValues(t, 1, len(proposals), "Expected 1 proposal when querying governance contract")
	require.EqualValues(t, proposals[0].Proposal.Id, proposal.Id, "Proposal ID mismatch")
	if proposalType == Governance {
		require.EqualValues(t, proposals[0].UpdatesGovernance, true, "Governance update mismatch")
	} else {
		require.EqualValues(t, proposals[0].UpdatesGovernance, false, "Governance update mismatch")
	}

	t.Logf("Querying proposals by status")
	proposals, err = gov.GetProposalsByStatus(governance.ProposalStatus_active)
	integration.NoError(t, err)

	require.EqualValues(t, 1, len(proposals), "Expected 1 proposal when querying governance contract")
	require.EqualValues(t, proposals[0].Proposal.Id, proposal.Id, "Proposal ID mismatch")
	if proposalType == Governance {
		require.EqualValues(t, proposals[0].UpdatesGovernance, true, "Governance update mismatch")
	} else {
		require.EqualValues(t, proposals[0].UpdatesGovernance, false, "Governance update mismatch")
	}

	t.Logf("Querying proposals by ID")
	prec, err = gov.GetProposalById(proposal.Id)
	integration.NoError(t, err)

	require.NotNil(t, prec, "Expected proposal from query")
	require.EqualValues(t, prec.Proposal.Id, proposal.Id, "Proposal ID mismatch")
	if proposalType == Governance {
		require.EqualValues(t, prec.UpdatesGovernance, true, "Governance update mismatch")
	} else {
		require.EqualValues(t, prec.UpdatesGovernance, false, "Governance update mismatch")
	}

	logAndVote := func(b *protocol.Block) error {
		b.Header.ApprovedProposals = append(b.Header.ApprovedProposals, proposal.Id)
		t.Logf("Created block at height: " + strconv.FormatUint(b.Header.Height, 10))
		return nil
	}

	receipts, err := integration.CreateBlocks(client, (VotePeriod * threshold / 100), logAndVote, genesisKey)
	integration.NoError(t, err)

	for _, receipt := range receipts {
		blockEvents = integration.EventsFromBlockReceipt(receipt)
		require.EqualValues(t, 1, len(blockEvents), "Expected 1 event within the block receipt")
		require.EqualValues(t, "proposal.vote", blockEvents[0].Name, "Expected 'proposal.vote' event in block receipt")
	}

	receipts, err = integration.CreateBlocks(client, (VotePeriod*(100-threshold)/100)-1, logPerK, genesisKey)
	integration.NoError(t, err)

	for _, receipt := range receipts {
		blockEvents = integration.EventsFromBlockReceipt(receipt)
		require.EqualValues(t, 0, len(blockEvents), "Expected no events within the block receipt")
	}

	receipt, err = integration.CreateBlock(client, []*protocol.Transaction{}, genesisKey)
	integration.NoError(t, err)

	integration.LogBlockReceipt(t, receipt)

	blockEvents = integration.EventsFromBlockReceipt(receipt)
	require.EqualValues(t, 1, len(blockEvents), "Expected 1 event within the block receipt")
	require.EqualValues(t, "proposal.status", blockEvents[0].Name, "Expected 'proposal.status' event in block receipt")

	err = proto.Unmarshal(blockEvents[0].Data, &statusEvent)
	integration.NoError(t, err)

	t.Logf("Ensuring the correct proposal status was emitted")
	require.EqualValues(t, statusEvent.Id, proposal.Id, "Proposal ID mismatch")
	require.EqualValues(t, statusEvent.Status, governance.ProposalStatus_approved, "Proposal status mismatch")

	t.Logf("Querying proposals")
	proposals, err = gov.GetProposals()
	integration.NoError(t, err)

	require.EqualValues(t, 1, len(proposals), "Expected proposal when querying governance contract")
	require.EqualValues(t, prec.Proposal.Id, proposal.Id, "Proposal ID mismatch")
	if proposalType == Governance {
		require.EqualValues(t, proposals[0].UpdatesGovernance, true, "Governance update mismatch")
	} else {
		require.EqualValues(t, proposals[0].UpdatesGovernance, false, "Governance update mismatch")
	}

	t.Logf("Querying proposals by status")
	proposals, err = gov.GetProposalsByStatus(governance.ProposalStatus_approved)
	integration.NoError(t, err)

	require.EqualValues(t, 1, len(proposals), "Expected proposal when querying governance contract")
	require.EqualValues(t, prec.Proposal.Id, proposal.Id, "Proposal ID mismatch")
	if proposalType == Governance {
		require.EqualValues(t, proposals[0].UpdatesGovernance, true, "Governance update mismatch")
	} else {
		require.EqualValues(t, proposals[0].UpdatesGovernance, false, "Governance update mismatch")
	}

	t.Logf("Querying proposals by ID")
	prec, err = gov.GetProposalById(proposal.Id)
	integration.NoError(t, err)

	require.NotNil(t, prec, "Expected proposal from query")
	if proposalType == Governance {
		require.EqualValues(t, prec.UpdatesGovernance, true, "Governance update mismatch")
	} else {
		require.EqualValues(t, prec.UpdatesGovernance, false, "Governance update mismatch")
	}

	receipts, err = integration.CreateBlocks(client, ApplicationDelay-1, logPerK, genesisKey)
	integration.NoError(t, err)

	for _, receipt := range receipts {
		blockEvents = integration.EventsFromBlockReceipt(receipt)
		require.EqualValues(t, 0, len(blockEvents), "Expected no events within the block receipt")
	}

	receipt, err = integration.CreateBlock(client, []*protocol.Transaction{}, genesisKey)
	integration.NoError(t, err)

	integration.LogBlockReceipt(t, receipt)

	require.EqualValues(t, 1, len(receipt.Events), "Expected 3 event within the block receipt")
	require.EqualValues(t, "proposal.status", receipt.Events[0].Name, "Expected 'proposal.status' event in block receipt")

	err = proto.Unmarshal(receipt.Events[0].Data, &statusEvent)
	integration.NoError(t, err)

	t.Logf("Ensuring the correct proposal status was emitted")
	require.EqualValues(t, statusEvent.Id, proposal.Id, "Proposal ID mismatch")
	require.EqualValues(t, statusEvent.Status, governance.ProposalStatus_applied, "Proposal status mismatch")

	err = onSuccess(client, t)
	require.Nil(t, err)
}

func testFailedProposal(t *testing.T, client integration.Client, proposalFactory func(client integration.Client) (*protocol.Transaction, error), proposalType int) {
	koin := token.GetKoinToken(client)

	threshold := StandardThreshold
	if proposalType == Governance {
		threshold = GovernanceThreshold
	}

	aliceKey, err := util.GenerateKoinosKey()
	integration.NoError(t, err)

	genesisKey, err := integration.GetKey(integration.Genesis)
	integration.NoError(t, err)

	t.Logf("Querying proposals")
	gov := govUtil.GetGovernance(client)
	proposals, err := gov.GetProposals()
	integration.NoError(t, err)

	require.EqualValues(t, 0, len(proposals), "Expected no proposals when querying governance contract")

	t.Logf("Submitting proposal")
	proposal, err := proposalFactory(client)
	integration.NoError(t, err)

	err = koin.Mint(aliceKey.AddressBytes(), 200000000)
	integration.NoError(t, err)

	receipt, err := gov.SubmitProposal(aliceKey, proposal, 100000000)
	integration.NoError(t, err)

	integration.LogBlockReceipt(t, receipt)

	blockEvents := integration.EventsFromBlockReceipt(receipt)

	require.EqualValues(t, 2, len(blockEvents), "Expected 2 event within the block receipt")
	require.EqualValues(t, "koin.burn", blockEvents[0].Name, "Expected 'koin.burn' event in block receipt")
	require.EqualValues(t, "proposal.submission", blockEvents[1].Name, "Expected 'proposal.submission' event in block receipt")

	t.Logf("Querying proposals")
	proposals, err = gov.GetProposals()
	integration.NoError(t, err)

	require.EqualValues(t, 1, len(proposals), "Expected 1 proposal when querying governance contract")
	require.EqualValues(t, proposals[0].Proposal.Id, proposal.Id, "Proposal ID mismatch")
	if proposalType == Governance {
		require.EqualValues(t, proposals[0].UpdatesGovernance, true, "Governance update mismatch")
	} else {
		require.EqualValues(t, proposals[0].UpdatesGovernance, false, "Governance update mismatch")
	}

	t.Logf("Querying proposals by status")
	proposals, err = gov.GetProposalsByStatus(governance.ProposalStatus_pending)
	integration.NoError(t, err)

	require.EqualValues(t, 1, len(proposals), "Expected 1 proposal when querying governance contract")
	require.EqualValues(t, proposals[0].Proposal.Id, proposal.Id, "Proposal ID mismatch")
	if proposalType == Governance {
		require.EqualValues(t, proposals[0].UpdatesGovernance, true, "Governance update mismatch")
	} else {
		require.EqualValues(t, proposals[0].UpdatesGovernance, false, "Governance update mismatch")
	}

	t.Logf("Querying proposals by ID")
	prec, err := gov.GetProposalById(proposal.Id)
	integration.NoError(t, err)

	require.NotNil(t, prec, "Expected proposal from query")
	require.EqualValues(t, prec.Proposal.Id, proposal.Id, "Proposal ID mismatch")
	if proposalType == Governance {
		require.EqualValues(t, prec.UpdatesGovernance, true, "Governance update mismatch")
	} else {
		require.EqualValues(t, prec.UpdatesGovernance, false, "Governance update mismatch")
	}

	t.Logf("Pushing blocks to enter the voting period")

	logPerK := func(b *protocol.Block) error {
		t.Logf("Created block at height: " + strconv.FormatUint(b.Header.Height, 10))
		return nil
	}
	_, err = integration.CreateBlocks(client, ReviewPeriod-1, logPerK, genesisKey)
	integration.NoError(t, err)

	receipt, err = integration.CreateBlock(client, []*protocol.Transaction{}, genesisKey)
	integration.NoError(t, err)

	integration.LogBlockReceipt(t, receipt)

	blockEvents = integration.EventsFromBlockReceipt(receipt)
	require.EqualValues(t, 1, len(blockEvents), "Expected 1 event within the block receipt")
	require.EqualValues(t, "proposal.status", blockEvents[0].Name, "Expected 'proposal.status' event in block receipt")

	var statusEvent governance.ProposalStatusEvent
	err = proto.Unmarshal(blockEvents[0].Data, &statusEvent)
	integration.NoError(t, err)

	t.Logf("Ensuring the correct proposal status was emitted")
	require.EqualValues(t, statusEvent.Id, proposal.Id, "Proposal ID mismatch")
	require.EqualValues(t, statusEvent.Status, governance.ProposalStatus_active, "Proposal status mismatch")

	t.Logf("Querying proposals")
	proposals, err = gov.GetProposals()
	integration.NoError(t, err)

	require.EqualValues(t, 1, len(proposals), "Expected 1 proposal when querying governance contract")
	require.EqualValues(t, proposals[0].Proposal.Id, proposal.Id, "Proposal ID mismatch")
	if proposalType == Governance {
		require.EqualValues(t, proposals[0].UpdatesGovernance, true, "Governance update mismatch")
	} else {
		require.EqualValues(t, proposals[0].UpdatesGovernance, false, "Governance update mismatch")
	}

	t.Logf("Querying proposals by status")
	proposals, err = gov.GetProposalsByStatus(governance.ProposalStatus_active)
	integration.NoError(t, err)

	require.EqualValues(t, 1, len(proposals), "Expected 1 proposal when querying governance contract")
	require.EqualValues(t, proposals[0].Proposal.Id, proposal.Id, "Proposal ID mismatch")
	if proposalType == Governance {
		require.EqualValues(t, proposals[0].UpdatesGovernance, true, "Governance update mismatch")
	} else {
		require.EqualValues(t, proposals[0].UpdatesGovernance, false, "Governance update mismatch")
	}

	t.Logf("Querying proposals by ID")
	prec, err = gov.GetProposalById(proposal.Id)
	integration.NoError(t, err)

	require.NotNil(t, prec, "Expected proposal from query")
	require.EqualValues(t, prec.Proposal.Id, proposal.Id, "Proposal ID mismatch")
	if proposalType == Governance {
		require.EqualValues(t, prec.UpdatesGovernance, true, "Governance update mismatch")
	} else {
		require.EqualValues(t, prec.UpdatesGovernance, false, "Governance update mismatch")
	}

	logAndVote := func(b *protocol.Block) error {
		b.Header.ApprovedProposals = append(b.Header.ApprovedProposals, proposal.Id)
		t.Logf("Created block at height: " + strconv.FormatUint(b.Header.Height, 10))
		return nil
	}

	receipts, err := integration.CreateBlocks(client, (VotePeriod*threshold/100)-1, logAndVote, genesisKey)
	integration.NoError(t, err)

	for _, receipt := range receipts {
		integration.LogBlockReceipt(t, receipt)
		blockEvents = integration.EventsFromBlockReceipt(receipt)
		require.EqualValues(t, 1, len(blockEvents), "Expected 1 event within the block receipt")
		require.EqualValues(t, "proposal.vote", blockEvents[0].Name, "Expected 'proposal.vote' event in block receipt")
	}

	receipts, err = integration.CreateBlocks(client, (VotePeriod * (100 - threshold) / 100), logPerK, genesisKey)
	integration.NoError(t, err)

	for _, receipt := range receipts {
		blockEvents = integration.EventsFromBlockReceipt(receipt)
		require.EqualValues(t, 0, len(blockEvents), "Expected no events within the block receipt")
	}

	receipt, err = integration.CreateBlock(client, []*protocol.Transaction{}, genesisKey)
	integration.NoError(t, err)

	integration.LogBlockReceipt(t, receipt)

	blockEvents = integration.EventsFromBlockReceipt(receipt)
	require.EqualValues(t, 1, len(blockEvents), "Expected 1 event within the block receipt")
	require.EqualValues(t, "proposal.status", blockEvents[0].Name, "Expected 'proposal.status' event in block receipt")

	err = proto.Unmarshal(blockEvents[0].Data, &statusEvent)
	integration.NoError(t, err)

	t.Logf("Ensuring the correct proposal status was emitted")
	require.EqualValues(t, statusEvent.Id, proposal.Id, "Proposal ID mismatch")
	require.EqualValues(t, statusEvent.Status, governance.ProposalStatus_expired, "Proposal status mismatch")

	t.Logf("Querying proposals")
	proposals, err = gov.GetProposals()
	integration.NoError(t, err)

	require.EqualValues(t, 0, len(proposals), "Expected no proposals when querying governance contract")

	t.Logf("Querying proposals by status")
	proposals, err = gov.GetProposalsByStatus(governance.ProposalStatus_expired)
	integration.NoError(t, err)

	require.EqualValues(t, 0, len(proposals), "Expected no proposals when querying governance contract")

	t.Logf("Querying proposals by ID")
	prec, err = gov.GetProposalById(proposal.Id)
	integration.NoError(t, err)

	require.Nil(t, prec, "Expected no proposal from query")
}

func testLogOverrideProposal(client integration.Client, t *testing.T) error {
	genesisKey, err := integration.GetKey(integration.Genesis)
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

	t.Logf("Generating key for bob")
	bobKey, err := util.GenerateKoinosKey()
	integration.NoError(t, err)

	callContract := &protocol.Operation{
		Op: &protocol.Operation_CallContract{
			CallContract: &protocol.CallContractOperation{
				ContractId: helloKey.AddressBytes(),
				EntryPoint: uint32(0x00),
				Args:       make([]byte, 0),
			},
		},
	}

	tx, err := integration.CreateTransaction(client, []*protocol.Operation{callContract}, bobKey)
	integration.NoError(t, err)

	receipt, err = integration.CreateBlock(client, []*protocol.Transaction{tx}, genesisKey)
	integration.NoError(t, err)

	integration.LogBlockReceipt(t, receipt)

	t.Logf("Ensuring the log system call has been overridden")
	require.EqualValues(t, len(receipt.TransactionReceipts), 1, "Expected 1 transaction receipt")
	require.EqualValues(t, len(receipt.TransactionReceipts[0].Logs), 1, "Expected 1 log entry in transaction receipt")
	require.EqualValues(t, receipt.TransactionReceipts[0].Logs[0], "test: Greetings from koinos vm", "Log entry mismatch")

	return nil
}

func makeLogOverrideProposal(client integration.Client) (*protocol.Transaction, error) {
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

	setSysContractOperation := &protocol.Operation{
		Op: &protocol.Operation_SetSystemContract{
			SetSystemContract: &protocol.SetSystemContractOperation{
				ContractId:     syscallOverrideKey.AddressBytes(),
				SystemContract: true,
			},
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

	mod := func(tx *protocol.Transaction) error {
		tx.Header.Payer = governanceKey.AddressBytes()
		tx.Header.Payee = syscallOverrideKey.AddressBytes()
		return nil
	}

	return integration.CreateTransaction(client, []*protocol.Operation{uploadOperation, setSysContractOperation, overrideOperation}, mod, syscallOverrideKey)
}

func testGovernanceRemovalProposal(client integration.Client, t *testing.T) error {
	genesisKey, err := integration.GetKey(integration.Genesis)
	integration.NoError(t, err)

	t.Logf("Pushing block to ensure pre_block system no longer emits logs")
	receipt, err := integration.CreateBlock(client, []*protocol.Transaction{}, genesisKey)
	integration.NoError(t, err)

	require.EqualValues(t, len(receipt.Logs), 0, "Expected no log entries")

	return nil
}

func makeGovernanceRemovalProposal(client integration.Client) (*protocol.Transaction, error) {
	governanceKey, err := integration.GetKey(integration.Governance)
	if err != nil {
		return nil, err
	}

	proposerKey, err := util.GenerateKoinosKey()
	if err != nil {
		return nil, err
	}

	overridePreBlockOperation := &protocol.Operation{
		Op: &protocol.Operation_SetSystemCall{
			SetSystemCall: &protocol.SetSystemCallOperation{
				CallId: uint32(chain.SystemCallId_pre_block_callback),
				Target: &protocol.SystemCallTarget{
					Target: &protocol.SystemCallTarget_ThunkId{
						ThunkId: uint32(chain.SystemCallId_pre_block_callback),
					},
				},
			},
		},
	}

	overrideCheckSystemAuthorityOperation := &protocol.Operation{
		Op: &protocol.Operation_SetSystemCall{
			SetSystemCall: &protocol.SetSystemCallOperation{
				CallId: uint32(chain.SystemCallId_check_system_authority),
				Target: &protocol.SystemCallTarget{
					Target: &protocol.SystemCallTarget_ThunkId{
						ThunkId: uint32(chain.SystemCallId_check_system_authority),
					},
				},
			},
		},
	}

	mod := func(tx *protocol.Transaction) error {
		tx.Header.Payer = governanceKey.AddressBytes()
		tx.Header.Payee = proposerKey.AddressBytes()
		return nil
	}

	return integration.CreateTransaction(client, []*protocol.Operation{overridePreBlockOperation, overrideCheckSystemAuthorityOperation}, mod, proposerKey)
}
