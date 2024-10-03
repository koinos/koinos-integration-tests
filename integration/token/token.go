package token

import (
	"fmt"
	"koinos-integration-tests/integration"

	"github.com/koinos/koinos-proto-golang/v2/koinos/protocol"
	"github.com/koinos/koinos-proto-golang/v2/koinos/standards/kcs4"
	util "github.com/koinos/koinos-util-golang/v2"
	"google.golang.org/protobuf/proto"
)

const (
	mintEntry        uint32 = 0xdc6f17bb
	balanceOfEntry   uint32 = 0x5c721497
	totalSupplyEntry uint32 = 0xb0da3934
	transferEntry    uint32 = 0x27f576ca
	burnEntry        uint32 = 0x859facc5
	approveEntry     uint32 = 0x74e21680
)

// Token interfaces with a token contract
type Token struct {
	key             *util.KoinosKey
	contractAddress []byte
	client          integration.Client
}

// NewToken returns a Token object using the contractAddress
func NewToken(contractAddress []byte, client integration.Client) *Token {
	return &Token{contractAddress: contractAddress, client: client}
}

// GetKoinToken returns the KOIN Token object
func GetKoinToken(client integration.Client) *Token {
	koinKey, _ := integration.GetKey(integration.Koin)
	return &Token{key: koinKey, contractAddress: koinKey.AddressBytes(), client: client}
}

// GetVhpToken returns the VHP Token object
func GetVhpToken(client integration.Client) *Token {
	vhpKey, _ := integration.GetKey(integration.Vhp)
	return &Token{key: vhpKey, contractAddress: vhpKey.AddressBytes(), client: client}
}

// Mint tokens to an address
func (t *Token) Mint(to []byte, value uint64) error {
	if t.key == nil {
		return fmt.Errorf("token must know key to mint tokens")
	}

	mintArgs := &kcs4.MintArguments{
		To:    to,
		Value: value,
	}

	args, err := proto.Marshal(mintArgs)
	if err != nil {
		return err
	}

	op := &protocol.Operation{
		Op: &protocol.Operation_CallContract{
			CallContract: &protocol.CallContractOperation{
				ContractId: t.contractAddress,
				EntryPoint: mintEntry,
				Args:       args,
			},
		},
	}

	transaction, err := integration.CreateTransaction(t.client, []*protocol.Operation{op}, t.key)
	if err != nil {
		return err
	}

	_, err = integration.CreateBlock(t.client, []*protocol.Transaction{transaction})
	return err
}

// Balance of an address
func (t *Token) Balance(address []byte) (uint64, error) {
	balance, err := integration.GetAccountBalance(t.client, address, t.contractAddress, balanceOfEntry)
	if err != nil {
		return 0, err
	}

	return balance, nil
}

// TotalSupply of the token
func (t *Token) TotalSupply() (uint64, error) {
	resp, err := integration.ReadContract(t.client, make([]byte, 0), t.contractAddress, totalSupplyEntry)
	if err != nil {
		return 0, err
	}

	totalSupply := &kcs4.TotalSupplyResult{}
	err = proto.Unmarshal(resp.GetResult(), totalSupply)
	if err != nil {
		return 0, err
	}

	return totalSupply.GetValue(), nil
}

// Transfer tokens from one address to another
func (t *Token) Transfer(from *util.KoinosKey, to []byte, value uint64) error {
	transferArgs := &kcs4.TransferArguments{
		From:  from.AddressBytes(),
		To:    to,
		Value: value,
	}

	args, err := proto.Marshal(transferArgs)
	if err != nil {
		return err
	}

	op := &protocol.Operation{
		Op: &protocol.Operation_CallContract{
			CallContract: &protocol.CallContractOperation{
				ContractId: t.contractAddress,
				EntryPoint: transferEntry,
				Args:       args,
			},
		},
	}

	transaction, err := integration.CreateTransaction(t.client, []*protocol.Operation{op}, from)
	if err != nil {
		return err
	}

	_, err = integration.CreateBlock(t.client, []*protocol.Transaction{transaction})
	return err
}

// Burn tokens from an address
func (t *Token) Burn(from *util.KoinosKey, value uint64) error {
	burnArgs := &kcs4.BurnArguments{
		From:  from.AddressBytes(),
		Value: value,
	}

	args, err := proto.Marshal(burnArgs)
	if err != nil {
		return err
	}

	op := &protocol.Operation{
		Op: &protocol.Operation_CallContract{
			CallContract: &protocol.CallContractOperation{
				ContractId: t.contractAddress,
				EntryPoint: burnEntry,
				Args:       args,
			},
		},
	}

	transaction, err := integration.CreateTransaction(t.client, []*protocol.Operation{op}, from)
	if err != nil {
		return err
	}

	_, err = integration.CreateBlock(t.client, []*protocol.Transaction{transaction})
	return err
}

// Approve creates an allowance for the token
func (t *Token) Approve(owner *util.KoinosKey, to []byte, value uint64) error {
	approveArgs := &kcs4.ApproveArguments{
		Owner:   owner.AddressBytes(),
		Spender: to,
		Value:   value,
	}

	args, err := proto.Marshal(approveArgs)
	if err != nil {
		return err
	}

	op := &protocol.Operation{
		Op: &protocol.Operation_CallContract{
			CallContract: &protocol.CallContractOperation{
				ContractId: t.contractAddress,
				EntryPoint: approveEntry,
				Args:       args,
			},
		},
	}

	transaction, err := integration.CreateTransaction(t.client, []*protocol.Operation{op}, owner)
	if err != nil {
		return err
	}

	_, err = integration.CreateBlock(t.client, []*protocol.Transaction{transaction})
	return err
}
