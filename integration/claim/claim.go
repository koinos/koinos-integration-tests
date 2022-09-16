package claim

import (
	"encoding/hex"
	"fmt"
	"koinos-integration-tests/integration"
	"testing"

	"github.com/btcsuite/btcd/btcec"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/koinos/koinos-proto-golang/koinos/contracts/claim"
	"github.com/koinos/koinos-proto-golang/koinos/protocol"
	util "github.com/koinos/koinos-util-golang"
	"github.com/mr-tron/base58"
	"google.golang.org/protobuf/proto"
)

const (
	claimEntry      uint32 = 0xdd1b3c31
	getInfoEntry    uint32 = 0xbd7f6850
	checkClaimEntry uint32 = 0x2ac66b4c
)

// Claim is a wrapper around the claim contract
type Claim struct {
	key    *util.KoinosKey
	client integration.Client
}

// SubmitClaim to the claim contract
func (c *Claim) SubmitClaim(t *testing.T, ethAddress []byte, privateKey []byte, payer *util.KoinosKey) (*protocol.BlockReceipt, error) {
	pk, _ := btcec.PrivKeyFromBytes(btcec.S256(), privateKey)

	messageStr := "claim koins 0x" + hex.EncodeToString(ethAddress) + ":" + base58.Encode(payer.AddressBytes())
	fullMessageStr := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(messageStr), messageStr)

	h := crypto.Keccak256Hash([]byte(fullMessageStr))

	sig, err := btcec.SignCompact(btcec.S256(), pk, h.Bytes(), true)
	if err != nil {
		return nil, err
	}

	claimArgs := &claim.ClaimArguments{
		EthAddress:  ethAddress,
		KoinAddress: payer.AddressBytes(),
		Signature:   sig,
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

// CheckClaim checks the status of a claim
func (c *Claim) CheckClaim(ethAddress []byte) (*claim.ClaimStatus, error) {
	checkClaimArgs := &claim.CheckClaimArguments{EthAddress: ethAddress}

	args, err := proto.Marshal(checkClaimArgs)
	if err != nil {
		return nil, err
	}

	resp, err := integration.ReadContract(c.client, args, c.key.AddressBytes(), checkClaimEntry)
	if err != nil {
		return nil, err
	}

	check := &claim.CheckClaimResult{}
	err = proto.Unmarshal(resp.GetResult(), check)
	if err != nil {
		return nil, err
	}

	if check.GetValue() == nil {
		return &claim.ClaimStatus{}, nil
	}

	return check.GetValue(), nil
}

// NewClaim creates a new Claim wrapper
func NewClaim(client integration.Client) *Claim {
	claimKey, _ := integration.GetKey(integration.Claim)
	return &Claim{key: claimKey, client: client}
}
