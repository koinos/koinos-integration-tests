package governance

import (
	"fmt"
	"koinos-integration-tests/integration"

	"github.com/koinos/koinos-proto-golang/koinos/contracts/governance"
	"github.com/koinos/koinos-proto-golang/koinos/protocol"
	util "github.com/koinos/koinos-util-golang"
	"google.golang.org/protobuf/proto"
)

const (
	submitProposalEntry       uint32 = 0xe74b785c
	getProposalByIdEntry      uint32 = 0xc66013ad
	getProposalsEntry         uint32 = 0xd44caa11
	getProposalsByStatusEntry uint32 = 0x66206f76
)

// A wrapper around the governance contract
type Governance struct {
	key    *util.KoinosKey
	client integration.Client
}

// GetGoverance returns the goverance contract object
func GetGovernance(client integration.Client) *Governance {
	goverancneKey, _ := integration.GetKey(integration.Governance)

	return &Governance{key: goverancneKey, client: client}
}

// SubmitProposal to the goverance contract
func (g *Governance) SubmitProposal(payer *util.KoinosKey, mroot []byte, ops []*protocol.Operation, fee uint64) (*protocol.BlockReceipt, error) {
	submitProposalArgs := &governance.SubmitProposalArguments{
		Operations:          ops,
		OperationMerkleRoot: mroot,
		Fee:                 fee,
	}

	args, err := proto.Marshal(submitProposalArgs)
	if err != nil {
		return nil, err
	}

	op := &protocol.Operation{
		Op: &protocol.Operation_CallContract{
			CallContract: &protocol.CallContractOperation{
				ContractId: g.key.AddressBytes(),
				EntryPoint: submitProposalEntry,
				Args:       args,
			},
		},
	}

	transaction, err := integration.CreateTransaction(g.client, []*protocol.Operation{op}, payer)
	if err != nil {
		return nil, err
	}

	return integration.CreateBlock(g.client, []*protocol.Transaction{transaction})
}

// GetProposalById
func (g *Governance) GetProposalById(id []byte) (*governance.ProposalRecord, error) {
	getProposalArgs := &governance.GetProposalByIdArguments{
		ProposalId: id,
	}

	args, err := proto.Marshal(getProposalArgs)
	if err != nil {
		return nil, err
	}

	resp, err := integration.ReadContract(g.client, args, g.key.AddressBytes(), getProposalByIdEntry)
	if err != nil {
		return nil, err
	}

	proposal := &governance.GetProposalByIdResult{}
	err = proto.Unmarshal(resp.GetResult(), proposal)
	if err != nil {
		return nil, err
	}

	return proposal.GetValue(), nil
}

// GetProposals
// Variadic arguments can be:
//    start []byte - proposal id to start at
//    limit uint64 - limit of proposals to return at once
func (g *Governance) GetProposals(vars ...interface{}) ([]*governance.ProposalRecord, error) {
	var start []byte = make([]byte, 0)
	var limit uint64 = 0

	if len(vars) > 0 {
		for _, v := range vars {
			switch t := v.(type) {
			case []byte:
				start = t
			case uint64:
				limit = t
			default:
				return nil, fmt.Errorf("unexpected argument")
			}
		}
	}

	getProposalsArgs := &governance.GetProposalsArguments{
		StartProposal: start,
		Limit:         limit,
	}

	args, err := proto.Marshal(getProposalsArgs)
	if err != nil {
		return nil, err
	}

	resp, err := integration.ReadContract(g.client, args, g.key.AddressBytes(), getProposalsEntry)
	if err != nil {
		return nil, err
	}

	proposals := &governance.GetProposalsResult{}
	err = proto.Unmarshal(resp.GetResult(), proposals)
	if err != nil {
		return nil, err
	}

	return proposals.GetValue(), nil
}

// GetProposalsByStatus
// Variadic arguments can be:
//    start []byte - proposal id to start at
//    limit uint64 - limit of proposals to return at once
func (g *Governance) GetProposalsByStatus(status governance.ProposalStatus, vars ...interface{}) ([]*governance.ProposalRecord, error) {
	var start []byte = make([]byte, 0)
	var limit uint64 = 0

	if len(vars) > 0 {
		for _, v := range vars {
			switch t := v.(type) {
			case []byte:
				start = t
			case uint64:
				limit = t
			default:
				return nil, fmt.Errorf("unexpected argument")
			}
		}
	}

	getProposalsArgs := &governance.GetProposalsByStatusArguments{
		StartProposal: start,
		Limit:         limit,
		Status:        status,
	}

	args, err := proto.Marshal(getProposalsArgs)
	if err != nil {
		return nil, err
	}

	resp, err := integration.ReadContract(g.client, args, g.key.AddressBytes(), getProposalsByStatusEntry)
	if err != nil {
		return nil, err
	}

	proposals := &governance.GetProposalsByStatusResult{}
	err = proto.Unmarshal(resp.GetResult(), proposals)
	if err != nil {
		return nil, err
	}

	return proposals.GetValue(), nil
}
