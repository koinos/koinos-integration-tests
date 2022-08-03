package claim

import (
	"koinos-integration-tests/integration"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/koinos/koinos-proto-golang/koinos/contracts/claim"
	"github.com/koinos/koinos-proto-golang/koinos/protocol"
	util "github.com/koinos/koinos-util-golang"
	"google.golang.org/protobuf/proto"
)

const (
	claimEntry   uint32 = 0xdd1b3c31
	getInfoEntry uint32 = 0xbd7f6850
)

// Claim is a wrapper around the claim contract
type Claim struct {
	key    *util.KoinosKey
	client integration.Client
}

// SubmitClaim to the claim contract
func (c *Claim) SubmitClaim(t *testing.T, publicKey []byte, privateKey []byte, payer *util.KoinosKey) (*protocol.BlockReceipt, error) {
	claimArgs := &claim.ClaimArguments{
		EthAddress:  publicKey,
		KoinAddress: payer.PublicBytes(),
	}

	args, err := proto.Marshal(claimArgs)
	if err != nil {
		return nil, err
	}

	op := &protocol.Operation{
		Op: &protocol.Operation_CallContract{
			CallContract: &protocol.CallContractOperation{
				ContractId: c.key.AddressBytes(),
				EntryPoint: claimEntry,
				Args:       args,
			},
		},
	}

	transaction, err := integration.CreateTransaction(c.client, []*protocol.Operation{op}, payer)
	if err != nil {
		return nil, err
	}

	headerBytes, err := proto.Marshal(transaction.GetHeader())
	if err != nil {
		return nil, err
	}

	pk, err := crypto.ToECDSA(privateKey)
	if err != nil {
		return nil, err
	}

	sig, err := crypto.Sign(headerBytes, pk)
	if err != nil {
		return nil, err
	}

	transaction.Signatures = append(transaction.Signatures, sig)

	return integration.CreateBlock(c.client, []*protocol.Transaction{transaction})
}

// GetInfo from the claim contract
func (c *Claim) GetInfo() (*claim.ClaimInfo, error) {
	getInfoArgs := &claim.GetInfoArguments{}

	args, err := proto.Marshal(getInfoArgs)
	if err != nil {
		return nil, err
	}

	resp, err := integration.ReadContract(c.client, args, c.key.AddressBytes(), getInfoEntry)
	if err != nil {
		return nil, err
	}

	info := &claim.GetInfoResult{}
	err = proto.Unmarshal(resp.GetResult(), info)
	if err != nil {
		return nil, err
	}

	return info.GetValue(), nil
}

// NewClaim creates a new Claim wrapper
func NewClaim(client integration.Client) *Claim {
	claimKey, _ := integration.GetKey(integration.Claim)
	return &Claim{key: claimKey, client: client}
}
