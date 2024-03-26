package name_service

import (
	"koinos-integration-tests/integration"
	"testing"

	name_service "github.com/koinos/koinos-proto-golang/v2/koinos/contracts/name-service"
	"github.com/koinos/koinos-proto-golang/v2/koinos/protocol"
	util "github.com/koinos/koinos-util-golang/v2"
	"google.golang.org/protobuf/proto"
)

const (
	SetRecordEntry  uint32 = 0xe248c73a
	GetNameEntry    uint32 = 0xe5070a16
	GetAddressEntry uint32 = 0xa61ae5e8
)

// A wrapper around the NameService contract
type NameService struct {
	key    *util.KoinosKey
	client integration.Client
}

// GetGoverance returns the goverance contract object
func GetNameService(client integration.Client) *NameService {
	nameServiceKey, _ := integration.GetKey(integration.NameService)

	return &NameService{key: nameServiceKey, client: client}
}

// SetRecord sets a record in the name service
func (n *NameService) SetRecord(t *testing.T, payer *util.KoinosKey, name string, address []byte) (*protocol.BlockReceipt, error) {
	setRecordArgs := &name_service.SetRecordArguments{
		Name:    name,
		Address: address,
	}

	args, err := proto.Marshal(setRecordArgs)
	if err != nil {
		return nil, err
	}

	op := &protocol.Operation{
		Op: &protocol.Operation_CallContract{
			CallContract: &protocol.CallContractOperation{
				ContractId: n.key.AddressBytes(),
				EntryPoint: SetRecordEntry,
				Args:       args,
			},
		},
	}

	transaction, err := integration.CreateTransaction(n.client, []*protocol.Operation{op}, payer)
	if err != nil {
		return nil, err
	}

	return integration.CreateBlock(n.client, []*protocol.Transaction{transaction})
}

// GetName returns the name of the contract at a given address
func (n *NameService) GetName(t *testing.T, address []byte) (*name_service.NameRecord, error) {
	getContractNameArgs := &name_service.GetNameArguments{
		Address: address,
	}

	args, err := proto.Marshal(getContractNameArgs)
	if err != nil {
		return nil, err
	}

	resp, err := integration.ReadContract(n.client, args, n.key.AddressBytes(), GetNameEntry)
	if err != nil {
		return nil, err
	}

	result := &name_service.GetNameResult{}
	err = proto.Unmarshal(resp.GetResult(), result)
	if err != nil {
		return nil, err
	}

	return result.GetValue(), nil
}

// GetAddress returns the name of the contract at a given address
func (n *NameService) GetAddress(t *testing.T, name string) (*name_service.AddressRecord, error) {
	getContractAddressArgs := &name_service.GetAddressArguments{
		Name: name,
	}

	args, err := proto.Marshal(getContractAddressArgs)
	if err != nil {
		return nil, err
	}

	resp, err := integration.ReadContract(n.client, args, n.key.AddressBytes(), GetAddressEntry)
	if err != nil {
		return nil, err
	}

	result := &name_service.GetAddressResult{}
	err = proto.Unmarshal(resp.GetResult(), result)
	if err != nil {
		return nil, err
	}

	return result.GetValue(), nil
}
