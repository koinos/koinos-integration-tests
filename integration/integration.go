package integration

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcutil/base58"
	koinosmq "github.com/koinos/koinos-mq-golang"
	"github.com/koinos/koinos-proto-golang/v2/koinos/canonical"
	"github.com/koinos/koinos-proto-golang/v2/koinos/chain"
	name_service "github.com/koinos/koinos-proto-golang/v2/koinos/contracts/name-service"
	"github.com/koinos/koinos-proto-golang/v2/koinos/contracts/token"
	"github.com/koinos/koinos-proto-golang/v2/koinos/protocol"
	"github.com/koinos/koinos-proto-golang/v2/koinos/rpc/block_store"
	chainrpc "github.com/koinos/koinos-proto-golang/v2/koinos/rpc/chain"
	cmsrpc "github.com/koinos/koinos-proto-golang/v2/koinos/rpc/contract_meta_store"
	"github.com/koinos/koinos-proto-golang/v2/koinos/rpc/mempool"
	"github.com/koinos/koinos-proto-golang/v2/koinos/rpc/p2p"
	"github.com/koinos/koinos-proto-golang/v2/koinos/rpc/transaction_store"
	util "github.com/koinos/koinos-util-golang/v2"
	kjsonrpc "github.com/koinos/koinos-util-golang/v2/rpc"
	"github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

const (
	Genesis int = iota
	Governance
	Koin
	Pob
	PobProducer
	Pow
	Resources
	Vhp
	Claim
	ClaimDelegation
	NameService
)

var wifMap = map[int]string{
	Genesis:         "5KYPA63Gx4MxQUqDM3PMckvX9nVYDUaLigTKAsLPesTyGmKmbR2",
	Governance:      "5KdCtpQ4DiFxgPd8VhexLwDfucJ83Mzc81ZviqU1APSzba8vNZV",
	Koin:            "5JbxDqUqx581iL9Po1mLvHMLkxnmjvypDdnmdLQvK5TzSpCFSgH",
	Pob:             "5JChmh7kJTLLToW7Et45w6fULWAq7S6USLYDaRydyE6aa42U557",
	PobProducer:     "5JMYdb7hMY78q3U2Yro6ZSJiYE7uvt9m6HBbeF8rA9fGEPD14US",
	Pow:             "5KKuscNqrWadRaCCt7oCF7kz6XdL4QMJE9MAnAVShA3JGJEze3p",
	Resources:       "5J4f6NdoPEDow7oRuGvuD9ggjr1HvWzirjZP6sJKSvsNnKenyi3",
	Vhp:             "5JdSPo7YMCb5rozFjHhwZ1qcKKbgKdvDqVzSkvr8XbZ8VoUf5re",
	Claim:           "5HwGceo2dySLQencsReyEHfbRRyJWTowhUVw1jVj3QGdm4Aai4N",
	ClaimDelegation: "5JDEvoENsqk2zD7vZ25im3Gd2zXFMVtC3LhqiMS1Ktuu3aVN6Vm",
	NameService:     "5JkJUcpmegiTTjEGwgcfHCNzZ1JQw3xci2U3sTtdzuruggXjEQN",
}

const (
	ReadContractCall      = "chain.read_contract"
	GetAccountNonceCall   = "chain.get_account_nonce"
	GetAccountRcCall      = "chain.get_account_rc"
	SubmitTransactionCall = "chain.submit_transaction"
	GetChainIDCall        = "chain.get_chain_id"
	GetContractMetaCall   = "contract_meta_store.get_contract_meta"
	GetHeadInfoCall       = "chain.get_head_info"
	SubmitBlockCall       = "chain.submit_block"

	defaultTimeout = time.Second
)

// Client is an interface for different types of message clients
type Client interface {
	Call(ctx context.Context, method string, params proto.Message, returnType proto.Message) error
}

func InitNameService(t *testing.T, client Client) {
	nameServiceKey, err := GetKey(NameService)
	NoError(t, err)

	t.Logf("Uploading Name Service contract")
	_, err = UploadSystemContract(client, "../../contracts/name_service.wasm", nameServiceKey, "name_service")
	NoError(t, err)

	t.Logf("Overriding get_contract_name")
	err = SetSystemCallOverride(client, nameServiceKey, uint32(0xe5070a16), uint32(chain.SystemCallId_get_contract_name))
	NoError(t, err)

	t.Logf("Overriding get_contract_address")
	err = SetSystemCallOverride(client, nameServiceKey, uint32(0xa61ae5e8), uint32(chain.SystemCallId_get_contract_address))
	NoError(t, err)
}

// GetHeadInfo gets the head info of the chain
func GetHeadInfo(client Client) (*chainrpc.GetHeadInfoResponse, error) {
	params := chainrpc.GetHeadInfoRequest{}

	headInfo := &chainrpc.GetHeadInfoResponse{}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	err := client.Call(ctx, GetHeadInfoCall, &params, headInfo)
	if err != nil {
		return nil, err
	}

	return headInfo, nil
}

// GetAccountNonce gets the nonce of a given account
func GetAccountNonce(client Client, address []byte) (uint64, error) {
	// Build the contract request
	params := chainrpc.GetAccountNonceRequest{
		Account: address,
	}

	// Make the rpc call
	var cResp chainrpc.GetAccountNonceResponse

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	err := client.Call(ctx, GetAccountNonceCall, &params, &cResp)
	if err != nil {
		return 0, err
	}

	nonce, err := util.NonceBytesToUInt64(cResp.Nonce)
	if err != nil {
		return 0, err
	}

	return nonce, nil
}

// ReadContract reads from the given contract and returns the response
func ReadContract(client Client, args []byte, contractID []byte, entryPoint uint32) (*chainrpc.ReadContractResponse, error) {
	// Build the contract request
	params := chainrpc.ReadContractRequest{ContractId: contractID, EntryPoint: entryPoint, Args: args}

	// Make the rpc call
	var cResp chainrpc.ReadContractResponse

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	err := client.Call(ctx, ReadContractCall, &params, &cResp)
	if err != nil {
		return nil, err
	}

	return &cResp, nil
}

// GetAccountBalance gets the balance of a given account
func GetAccountBalance(client Client, address []byte, contractID []byte, balanceOfEntry uint32) (uint64, error) {
	// Make the rpc call
	balanceOfArgs := &token.BalanceOfArguments{
		Owner: address,
	}
	argBytes, err := proto.Marshal(balanceOfArgs)
	if err != nil {
		return 0, err
	}

	cResp, err := ReadContract(client, argBytes, contractID, balanceOfEntry)
	if err != nil {
		return 0, err
	}

	balanceOfReturn := &token.BalanceOfResult{}
	err = proto.Unmarshal(cResp.Result, balanceOfReturn)
	if err != nil {
		return 0, err
	}

	return balanceOfReturn.Value, nil
}

// GetAccountRc gets the rc of a given account
func GetAccountRc(client Client, address []byte) (uint64, error) {
	// Build the contract request
	params := chainrpc.GetAccountRcRequest{
		Account: address,
	}

	// Make the rpc call
	var cResp chainrpc.GetAccountRcResponse

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	err := client.Call(ctx, GetAccountRcCall, &params, &cResp)
	if err != nil {
		return 0, err
	}

	return cResp.Rc, nil
}

// GetChainID gets the chain id
func GetChainID(client Client) ([]byte, error) {
	// Build the contract request
	params := chainrpc.GetChainIdRequest{}

	// Make the rpc call
	var cResp chainrpc.GetChainIdResponse

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	err := client.Call(ctx, GetChainIDCall, &params, &cResp)
	if err != nil {
		return nil, err
	}

	return cResp.ChainId, nil
}

type MQClient struct {
	client *koinosmq.Client
}

func NewKoinosMQClient(url string) *MQClient {
	c := koinosmq.NewClient(url, koinosmq.NoRetry)
	c.Start(context.Background())
	return &MQClient{
		client: c,
	}
}

func translateRequest(service string, method string, param proto.Message) ([]byte, error) {
	var wrapped proto.Message

	switch service {
	case "chain":
		wrapped = &chainrpc.ChainRequest{}

	case "block_store":
		wrapped = &block_store.BlockStoreRequest{}

	case "p2p":
		wrapped = &p2p.P2PRequest{}

	case "contract_meta_store":
		wrapped = &cmsrpc.ContractMetaStoreRequest{}

	case "mempool":
		wrapped = &mempool.MempoolRequest{}

	case "transaction_store":
		wrapped = &transaction_store.TransactionStoreRequest{}

	default:
		return nil, fmt.Errorf("error during service param mapping")
	}

	desc := wrapped.ProtoReflect().Descriptor()
	fieldd := desc.Fields().ByName(protoreflect.Name(method))
	if fieldd == nil {
		return nil, errors.New("unable to find field")
	}
	req := dynamicpb.NewMessage(desc)
	req.Set(fieldd, protoreflect.ValueOf(param.ProtoReflect()))
	return proto.Marshal(req)
}

func translateResponse(service string, resBytes []byte, response proto.Message) error {
	var wrapped proto.Message

	switch service {
	case "chain":
		wrapped = &chainrpc.ChainResponse{}

	case "block_store":
		wrapped = &block_store.BlockStoreResponse{}

	case "p2p":
		wrapped = &p2p.P2PResponse{}

	case "contract_meta_store":
		wrapped = &cmsrpc.ContractMetaStoreResponse{}

	case "mempool":
		wrapped = &mempool.MempoolResponse{}

	case "transaction_store":
		wrapped = &transaction_store.TransactionStoreResponse{}

	default:
		return fmt.Errorf("unknown microservice")
	}

	err := proto.Unmarshal(resBytes, wrapped)
	if err != nil {
		return err
	}

	name := response.ProtoReflect().Descriptor().Name()
	fieldd := wrapped.ProtoReflect().Descriptor().Fields().ByName(name[0 : len(name)-9])
	proto.Merge(response, wrapped.ProtoReflect().Get(fieldd).Message().Interface())

	return nil
}

func (mq *MQClient) Call(ctx context.Context, method string, params proto.Message, returnType proto.Message) error {
	s := strings.Split(method, ".")
	if len(s) != 2 {
		return errors.New("unexpected method length")
	}

	reqBytes, err := translateRequest(s[0], s[1], params)
	if err != nil {
		return err
	}

	resBytes, err := mq.client.RPC(ctx, "application/octet-stream", s[0], reqBytes)
	if err != nil {
		return err
	}

	err = translateResponse(s[0], resBytes, returnType)
	if err != nil {
		return err
	}

	return err
}

func GetKey(keyType int) (*util.KoinosKey, error) {
	wif, exists := wifMap[keyType]
	if exists {
		return KeyFromWIF(wif)
	}
	return nil, errors.New("invalid key type")
}

type eventList []*protocol.EventData

func (e eventList) Len() int {
	return len(e)
}

func (e eventList) Less(i, j int) bool {
	return e[i].Sequence < e[j].Sequence
}

func (e eventList) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
}

// CreateBlock creates a block from a list of transactions
// Variadic arguments can be the following:
//
//	key *util.KoinosKey - Key to produce the block with
//	mod func(b *protocol.Block) error - Modification callback function
func CreateBlock(client Client, transactions []*protocol.Transaction, vars ...interface{}) (*protocol.BlockReceipt, error) {
	var key *util.KoinosKey
	var mod func(b *protocol.Block) error

	if len(vars) > 0 {
		for _, v := range vars {
			switch t := v.(type) {
			case *util.KoinosKey:
				key = t
			case func(b *protocol.Block) error:
				mod = t
			default:
				return nil, fmt.Errorf("unexpected argument")
			}
		}
	} else {
		genesisKey, err := GetKey(Genesis)
		if err != nil {
			return nil, err
		}

		key = genesisKey
		mod = nil
	}

	block := &protocol.Block{}
	block.Header = &protocol.BlockHeader{}

	headInfo, err := GetHeadInfo(client)

	block.Header.Previous = headInfo.HeadTopology.GetId()
	block.Header.Height = headInfo.HeadTopology.GetHeight() + 1
	block.Header.Timestamp = uint64(time.Now().UnixMilli())
	block.Header.PreviousStateMerkleRoot = headInfo.GetHeadStateMerkleRoot()
	block.Header.Signer = key.AddressBytes()

	block.Transactions = append(block.Transactions, transactions...)

	if mod != nil {
		if err = mod(block); err != nil {
			return nil, err
		}
	}

	// Get transaction multihashes
	transactionHashes := make([][]byte, len(transactions)*2)
	hasher := sha256.New()
	for i, tx := range transactions {
		transactionHashes[i*2] = tx.GetId()

		hasher.Reset()
		for _, sig := range tx.GetSignatures() {
			hasher.Write(sig)
		}
		sum, err := multihash.Encode(hasher.Sum(nil), multihash.SHA2_256)
		transactionHashes[i*2+1] = sum
		if err != nil {
			return nil, err
		}
	}

	// Find merkle root
	var merkleRoot []byte
	if len(transactionHashes) > 0 {
		merkleRoot, err = util.CalculateMerkleRoot(transactionHashes)
		if err != nil {
			return nil, err
		}
	} else {
		hasher := sha256.New()
		hasher.Reset()
		sum, err := multihash.Encode(hasher.Sum(nil), multihash.SHA2_256)
		if err != nil {
			return nil, err
		}
		merkleRoot = sum
	}

	block.Header.TransactionMerkleRoot = merkleRoot

	headerBytes, err := canonical.Marshal(block.Header)
	if err != nil {
		return nil, err
	}

	sha256Hasher := sha256.New()
	sha256Hasher.Write(headerBytes)
	id, err := multihash.Encode(sha256Hasher.Sum(nil), multihash.SHA2_256)
	if err != nil {
		return nil, err
	}

	block.Id = id

	privateKey, _ := btcec.PrivKeyFromBytes(btcec.S256(), key.PrivateBytes())

	// Decode the ID
	idBytes, err := multihash.Decode(block.Id)
	if err != nil {
		return nil, err
	}

	signatureBytes, err := btcec.SignCompact(btcec.S256(), privateKey, idBytes.Digest, true)
	if err != nil {
		return nil, err
	}

	block.Signature = signatureBytes

	var submitBlockResp chainrpc.SubmitBlockResponse

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	err = client.Call(ctx, SubmitBlockCall, &chainrpc.SubmitBlockRequest{Block: block}, &submitBlockResp)
	if err != nil {
		return nil, err
	}

	return submitBlockResp.Receipt, nil
}

// CreateBlocks creates 'n' empty blocks
// Variadic arguments can be the following:
//
//	key *util.KoinosKey - Key to produce the block with
//	mod func(b *protocol.Block) error - Modification callback function, called on each block
func CreateBlocks(client Client, n int, vars ...interface{}) ([]*protocol.BlockReceipt, error) {
	receipts := make([]*protocol.BlockReceipt, 0)

	for i := 0; i < n; i++ {
		receipt, err := CreateBlock(client, []*protocol.Transaction{}, vars...)
		if err != nil {
			return nil, err
		}

		receipts = append(receipts, receipt)
	}

	return receipts, nil
}

// CreateTransaction creates a transaction from a list of operations
// Variadic arguments can be the following:
//
//	mod func(b *protocol.Block) error - Modification callback function, called on each block
//	*util.KoinosKey - Key to sign the transaction with, the first key is used to retreive the nonce
func CreateTransaction(client Client, ops []*protocol.Operation, vars ...interface{}) (*protocol.Transaction, error) {
	var mod func(b *protocol.Transaction) error = nil
	keys := make([]*util.KoinosKey, 0)

	if len(vars) > 0 {
		for _, v := range vars {
			switch t := v.(type) {
			case *util.KoinosKey:
				keys = append(keys, t)
			case func(b *protocol.Transaction) error:
				mod = t
			default:
				return nil, fmt.Errorf("unexpected argument")
			}
		}
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("expected at least one key")
	}

	// Cache the public address
	address := keys[0].AddressBytes()

	// Fetch the account's nonce
	nonce, err := GetAccountNonce(client, address)
	if err != nil {
		return nil, err
	}

	// Convert none+1 to bytes
	nonceBytes, err := util.UInt64ToNonceBytes(nonce + 1)
	if err != nil {
		return nil, err
	}

	rcLimit, err := GetAccountRc(client, address)
	if err != nil {
		return nil, err
	}

	// Get operation multihashes
	opHashes := make([][]byte, len(ops))
	for i, op := range ops {
		opHashes[i], err = util.HashMessage(op)
		if err != nil {
			return nil, err
		}
	}

	// Find merkle root
	merkleRoot, err := util.CalculateMerkleRoot(opHashes)
	if err != nil {
		return nil, err
	}

	chainID, err := GetChainID(client)
	if err != nil {
		return nil, err
	}

	// Create the transaction
	header := protocol.TransactionHeader{ChainId: chainID, RcLimit: rcLimit, Nonce: nonceBytes, OperationMerkleRoot: merkleRoot, Payer: address}
	transaction := &protocol.Transaction{Header: &header, Operations: ops}

	if mod != nil {
		if err = mod(transaction); err != nil {
			return nil, err
		}
	}

	// Calculate the transaction ID
	headerBytes, err := canonical.Marshal(&header)
	if err != nil {
		return nil, err
	}

	sha256Hasher := sha256.New()
	sha256Hasher.Write(headerBytes)
	tid, err := multihash.Encode(sha256Hasher.Sum(nil), multihash.SHA2_256)
	if err != nil {
		return nil, err
	}

	transaction.Id = tid

	// Sign the transaction
	for _, key := range keys {
		if err := util.SignTransaction(key.PrivateBytes(), transaction); err != nil {
			return nil, err
		}
	}

	return transaction, nil
}

func SubmitTransaction(client Client, transaction *protocol.Transaction) (*protocol.TransactionReceipt, error) {

	request := &chainrpc.SubmitTransactionRequest{
		Transaction: transaction,
		Broadcast:   true,
	}

	response := chainrpc.SubmitTransactionResponse{}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	err := client.Call(ctx, SubmitTransactionCall, request, &response)
	if err != nil {
		return nil, err
	}

	return response.Receipt, nil
}

// AwaitChain blocks until the chain rpc is responding
func AwaitChain(t *testing.T, client Client) {
	var waitDuration int64 = 1
	const maxRetry int64 = 5

	for {
		_, err := GetHeadInfo(client)
		if err == nil {
			break
		}

		t.Logf("Waiting %s for chain to be ready...\n", time.Duration(waitDuration)*time.Second)
		time.Sleep(time.Duration(waitDuration) * time.Second)
		if waitDuration*2 > maxRetry {
			waitDuration = maxRetry
		} else {
			waitDuration = waitDuration * 2
		}
	}
}

// BytesFromFile reads a file and returns the byte contents
func BytesFromFile(file string, bufsize uint64) ([]byte, error) {
	fileDesc, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer fileDesc.Close()

	buf := make([]byte, bufsize)
	len, err := fileDesc.Read(buf)
	if err != nil {
		return nil, err
	}

	return buf[:len], nil
}

// KeyFromWIF Decodes a private key WIF returning a KoinosKey
func KeyFromWIF(wif string) (*util.KoinosKey, error) {
	bytes, err := util.DecodeWIF(wif)
	if err != nil {
		return nil, err
	}

	key, err := util.NewKoinosKeyFromBytes(bytes)
	if err != nil {
		return nil, err
	}

	return key, nil
}

// SetSystemCallOverride overrides a system call with a contract and entrypoint call target
func SetSystemCallOverride(client Client, key *util.KoinosKey, entryPoint uint32, systemCall uint32) error {
	op := &protocol.Operation{
		Op: &protocol.Operation_SetSystemCall{
			SetSystemCall: &protocol.SetSystemCallOperation{
				CallId: systemCall,
				Target: &protocol.SystemCallTarget{
					Target: &protocol.SystemCallTarget_SystemCallBundle{
						SystemCallBundle: &protocol.ContractCallBundle{
							ContractId: key.AddressBytes(),
							EntryPoint: entryPoint,
						},
					},
				},
			},
		},
	}

	genesisKey, err := GetKey(Genesis)
	if err != nil {
		return err
	}

	transaction, err := CreateTransaction(client, []*protocol.Operation{op}, genesisKey)
	if err != nil {
		return err
	}

	_, err = CreateBlock(client, []*protocol.Transaction{transaction}, genesisKey)
	return err
}

// UploadContractTransaction creates a transaction containing an upload contract operation
func UploadContractTransaction(client Client, file string, key *util.KoinosKey) (*protocol.Transaction, error) {
	wasm, err := BytesFromFile(file, 512000)
	if err != nil {
		return nil, err
	}

	uco := protocol.UploadContractOperation{}
	uco.ContractId = key.AddressBytes()
	uco.Bytecode = wasm

	uploadOperation := &protocol.Operation{
		Op: &protocol.Operation_UploadContract{
			UploadContract: &uco,
		},
	}

	transaction, err := CreateTransaction(client, []*protocol.Operation{uploadOperation}, key)
	if err != nil {
		return nil, err
	}

	return transaction, nil
}

// UploadContract uploads a contract
func UploadContract(client Client, file string, key *util.KoinosKey, mods ...func(b *protocol.UploadContractOperation) error) error {
	var mod func(b *protocol.UploadContractOperation) error = nil

	if len(mods) > 0 {
		mod = mods[0]
	}

	wasm, err := BytesFromFile(file, 512000)
	if err != nil {
		return err
	}

	uco := protocol.UploadContractOperation{}
	uco.ContractId = key.AddressBytes()
	uco.Bytecode = wasm

	uploadOperation := &protocol.Operation{
		Op: &protocol.Operation_UploadContract{
			UploadContract: &uco,
		},
	}

	if mod != nil {
		err = mod(uploadOperation.GetUploadContract())
		if err != nil {
			return err
		}
	}

	transaction1, err := CreateTransaction(client, []*protocol.Operation{uploadOperation}, key)
	if err != nil {
		return err
	}

	_, err = CreateBlock(client, []*protocol.Transaction{transaction1})
	return err
}

// UploadSystemContract uploads a contract and sets it as a system contract
func UploadSystemContract(client Client, file string, key *util.KoinosKey, name string, mods ...func(b *protocol.UploadContractOperation) error) (*protocol.BlockReceipt, error) {
	var mod func(b *protocol.UploadContractOperation) error = nil

	if len(mods) > 0 {
		mod = mods[0]
	}

	wasm, err := BytesFromFile(file, 512000)
	if err != nil {
		return nil, err
	}

	uco := protocol.UploadContractOperation{}
	uco.ContractId = key.AddressBytes()
	uco.Bytecode = wasm

	uploadOperation := &protocol.Operation{
		Op: &protocol.Operation_UploadContract{
			UploadContract: &uco,
		},
	}

	if mod != nil {
		err = mod(uploadOperation.GetUploadContract())
		if err != nil {
			return nil, err
		}
	}

	transaction1, err := CreateTransaction(client, []*protocol.Operation{uploadOperation}, key)
	if err != nil {
		return nil, err
	}

	genesisKey, err := GetKey(Genesis)
	if err != nil {
		return nil, err
	}

	ssc := protocol.SetSystemContractOperation{}
	ssc.ContractId = key.AddressBytes()
	ssc.SystemContract = true

	setSystemContractOperation := &protocol.Operation{
		Op: &protocol.Operation_SetSystemContract{
			SetSystemContract: &ssc,
		},
	}

	// Set name service mapping

	setRecordArgs := &name_service.SetRecordArguments{
		Name:    name,
		Address: key.AddressBytes(),
	}

	args, err := proto.Marshal(setRecordArgs)
	if err != nil {
		return nil, err
	}

	nsKey, err := GetKey(NameService)
	if err != nil {
		return nil, err
	}

	setNameOperation := &protocol.Operation{
		Op: &protocol.Operation_CallContract{
			CallContract: &protocol.CallContractOperation{
				ContractId: nsKey.AddressBytes(),
				EntryPoint: uint32(0xe248c73a),
				Args:       args,
			},
		},
	}

	transaction2, err := CreateTransaction(client, []*protocol.Operation{setSystemContractOperation, setNameOperation}, genesisKey)
	if err != nil {
		return nil, err
	}

	return CreateBlock(client, []*protocol.Transaction{transaction1, transaction2})
}

// EventsFromBlockReceipt parses a block receipt, returning all events contained within the receipt
func EventsFromBlockReceipt(blockReceipt *protocol.BlockReceipt) []*protocol.EventData {
	var events []*protocol.EventData

	events = append(events, blockReceipt.Events...)

	for _, transactionReceipt := range blockReceipt.TransactionReceipts {
		events = append(events, transactionReceipt.Events...)
	}

	sort.Sort(eventList(events))

	return events
}

// LogBlockReceipt logs log messages contained within a block receipt
func LogBlockReceipt(t *testing.T, blockReceipt *protocol.BlockReceipt) {
	blockID := base58.Encode(blockReceipt.Id)
	t.Logf("Block: " + blockID)
	t.Logf("Compute: " + strconv.FormatUint(blockReceipt.ComputeBandwidthUsed, 10))
	t.Logf("Disk: " + strconv.FormatUint(blockReceipt.DiskStorageUsed, 10))
	t.Logf("Network: " + strconv.FormatUint(blockReceipt.NetworkBandwidthUsed, 10))

	if len(blockReceipt.Logs) > 0 {
		t.Logf(" * Logs")
		for _, log := range blockReceipt.Logs {
			t.Logf("  - " + log)
		}
	}

	if len(blockReceipt.Events) > 0 {
		t.Logf(" * Events")
		for _, event := range blockReceipt.Events {
			bytes := base64.StdEncoding.EncodeToString(event.Data)
			t.Logf("  - " + event.Name + ": " + bytes)
		}
	}

	for _, txReceipt := range blockReceipt.TransactionReceipts {
		transactionID := base58.Encode(txReceipt.Id)
		if txReceipt.Reverted {
			t.Logf(" > Transaction: " + transactionID + " (reverted)")
		} else {
			t.Logf(" > Transaction: " + transactionID)
		}

		t.Logf("  * Compute: " + strconv.FormatUint(txReceipt.ComputeBandwidthUsed, 10))
		t.Logf("  * Disk: " + strconv.FormatUint(txReceipt.DiskStorageUsed, 10))
		t.Logf("  * Network: " + strconv.FormatUint(txReceipt.NetworkBandwidthUsed, 10))

		if len(txReceipt.Logs) > 0 {
			t.Logf("  * Logs")
			for _, log := range txReceipt.Logs {
				t.Logf("   - " + log)
			}
		}

		if len(txReceipt.Events) > 0 {
			t.Logf("  * Events")
			for _, event := range txReceipt.Events {
				bytes := base64.StdEncoding.EncodeToString(event.Data)
				t.Logf("   - " + event.Name + ": " + bytes)
			}
		}
	}
}

// LogTransactionReceipt logs log messages contained within a transaction receipt
func LogTransactionReceipt(t *testing.T, txReceipt *protocol.TransactionReceipt) {
	transactionID := base58.Encode(txReceipt.Id)
	if txReceipt.Reverted {
		t.Logf("Transaction: " + transactionID + " (reverted)")
	} else {
		t.Logf("Transaction: " + transactionID)
	}

	t.Logf("Compute: " + strconv.FormatUint(txReceipt.ComputeBandwidthUsed, 10))
	t.Logf("Disk: " + strconv.FormatUint(txReceipt.DiskStorageUsed, 10))
	t.Logf("Network: " + strconv.FormatUint(txReceipt.NetworkBandwidthUsed, 10))

	if len(txReceipt.Logs) > 0 {
		t.Logf(" * Logs")
		for _, log := range txReceipt.Logs {
			t.Logf("   - " + log)
		}
	}

	if len(txReceipt.Events) > 0 {
		t.Logf(" * Events")
		for _, event := range txReceipt.Events {
			bytes := base64.StdEncoding.EncodeToString(event.Data)
			t.Logf("   - " + event.Name + ": " + bytes)
		}
	}
}

// LogProto logs a protobuf message
func LogProto(t *testing.T, message protoreflect.ProtoMessage) {
	text, err := protojson.Marshal(message)
	NoError(t, err)
	t.Logf(string(text))
}

// NoError asserts err is nil, logging any logs in the process
func NoError(t *testing.T, err error) {
	var rpcErr kjsonrpc.KoinosRPCError

	if err != nil && errors.As(err, &rpcErr) {
		for _, l := range rpcErr.Logs {
			t.Logf(l)
		}
	}

	require.NoError(t, err)
}

func CalculateOperationMerkleRoot(ops []*protocol.Operation) ([]byte, error) {
	// Get operation multihashes
	opHashes := make([][]byte, len(ops))
	for i, op := range ops {
		hash, err := util.HashMessage(op)
		if err != nil {
			return nil, err
		}
		opHashes[i] = hash
	}

	// Find merkle root
	merkleRoot, err := util.CalculateMerkleRoot(opHashes)
	if err != nil {
		return nil, err
	}

	return merkleRoot, nil
}
