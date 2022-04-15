package token

import (
	"fmt"
	"koinos-integration-tests/integration"

	"github.com/koinos/koinos-proto-golang/koinos/contracts/token"
	"github.com/koinos/koinos-proto-golang/koinos/protocol"
	util "github.com/koinos/koinos-util-golang"
	"github.com/koinos/koinos-util-golang/rpc"
	"google.golang.org/protobuf/proto"
)

const (
	mintEntry        uint32 = 0xdc6f17bb
	balanceOfEntry   uint32 = 0x5c721497
	totalSupplyEntry uint32 = 0xb0da3934
	transferEntry    uint32 = 0x27f576ca
)

// Token interfaces with a token contract
type Token struct {
	key             *util.KoinosKey
	contractAddress []byte
	client          *rpc.KoinosRPCClient
}

// NewToken returns a Token object using the contractAddress
func NewToken(contractAddress []byte, client *rpc.KoinosRPCClient) *Token {
	return &Token{contractAddress: contractAddress, client: client}
}

// GetKoinToken returns the KOIN Token object
func GetKoinToken(client *rpc.KoinosRPCClient) *Token {
	koinKey, _ := integration.GetKey(integration.Koin)
	return &Token{key: koinKey, contractAddress: koinKey.AddressBytes(), client: client}
}

// Mint tokens to an address
func (t *Token) Mint(to []byte, value uint64) error {
	if t.key == nil {
		return fmt.Errorf("token must know key to mint tokens")
	}

	mintArgs := &token.MintArguments{
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
	balance, err := t.client.GetAccountBalance(address, t.contractAddress, balanceOfEntry)
	if err != nil {
		return 0, err
	}

	return balance, nil
}

// TotalSupply of the token
func (t *Token) TotalSupply() (uint64, error) {
	resp, err := t.client.ReadContract(make([]byte, 0), t.contractAddress, totalSupplyEntry)
	if err != nil {
		return 0, err
	}

	totalSupply := &token.TotalSupplyResult{}
	err = proto.Unmarshal(resp.GetResult(), totalSupply)
	if err != nil {
		return 0, err
	}

	return totalSupply.GetValue(), nil
}

// Transfer tokens from one address to another
func (t *Token) Transfer(from *util.KoinosKey, to []byte, value uint64) error {
	transferArgs := &token.TransferArguments{
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
