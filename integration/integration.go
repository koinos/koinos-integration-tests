package integration

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcutil/base58"
	"github.com/koinos/koinos-proto-golang/koinos/canonical"
	"github.com/koinos/koinos-proto-golang/koinos/protocol"
	"github.com/koinos/koinos-proto-golang/koinos/rpc/chain"
	util "github.com/koinos/koinos-util-golang"
	kjsonrpc "github.com/koinos/koinos-util-golang/rpc"
	"github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const (
	Governance int = 0
	Koin       int = 1
	Genesis    int = 2
	Resources  int = 3
	Pow        int = 4
)

var wifMap = map[int]string{
	Governance: "5KdCtpQ4DiFxgPd8VhexLwDfucJ83Mzc81ZviqU1APSzba8vNZV",
	Koin:       "5JbxDqUqx581iL9Po1mLvHMLkxnmjvypDdnmdLQvK5TzSpCFSgH",
	Genesis:    "5KYPA63Gx4MxQUqDM3PMckvX9nVYDUaLigTKAsLPesTyGmKmbR2",
	Resources:  "5J4f6NdoPEDow7oRuGvuD9ggjr1HvWzirjZP6sJKSvsNnKenyi3",
	Pow:        "5KKuscNqrWadRaCCt7oCF7kz6XdL4QMJE9MAnAVShA3JGJEze3p",
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
//    key *util.KoinosKey - Key to produce the block with
//    mod func(b *protocol.Block) error - Modification callback function
func CreateBlock(client *kjsonrpc.KoinosRPCClient, transactions []*protocol.Transaction, vars ...interface{}) (*protocol.BlockReceipt, error) {
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

	headInfo := chain.GetHeadInfoResponse{}

	err := client.Call("chain.get_head_info", &chain.GetHeadInfoRequest{}, &headInfo)
	if err != nil {
		return nil, err
	}

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

	var submitBlockResp chain.SubmitBlockResponse
	err = client.Call("chain.submit_block", &chain.SubmitBlockRequest{Block: block}, &submitBlockResp)
	if err != nil {
		return nil, err
	}

	return submitBlockResp.Receipt, nil
}

// CreateBlocks creates 'n' empty blocks
// Variadic arguments can be the following:
//    key *util.KoinosKey - Key to produce the block with
//    mod func(b *protocol.Block) error - Modification callback function, called on each block
func CreateBlocks(client *kjsonrpc.KoinosRPCClient, n int, vars ...interface{}) ([]*protocol.BlockReceipt, error) {
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
//    mod func(b *protocol.Block) error - Modification callback function, called on each block
func CreateTransaction(client *kjsonrpc.KoinosRPCClient, ops []*protocol.Operation, key *util.KoinosKey, vars ...func(b *protocol.Transaction) error) (*protocol.Transaction, error) {
	var mod func(b *protocol.Transaction) error

	if len(vars) > 0 {
		mod = vars[0]
	} else {
		mod = nil
	}

	// Cache the public address
	address := key.AddressBytes()

	// Fetch the account's nonce
	nonce, err := client.GetAccountNonce(address)
	if err != nil {
		return nil, err
	}

	// Convert none+1 to bytes
	nonceBytes, err := util.UInt64ToNonceBytes(nonce + 1)
	if err != nil {
		return nil, err
	}

	rcLimit, err := client.GetAccountRc(address)
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

	chainID, err := client.GetChainID()
	if err != nil {
		return nil, err
	}

	// Create the header
	header := protocol.TransactionHeader{ChainId: chainID, RcLimit: rcLimit, Nonce: nonceBytes, OperationMerkleRoot: merkleRoot, Payer: address}
	headerBytes, err := canonical.Marshal(&header)
	if err != nil {
		return nil, err
	}

	// Create the transaction
	transaction := &protocol.Transaction{Header: &header, Operations: ops}

	if mod != nil {
		if err = mod(transaction); err != nil {
			return nil, err
		}
	}

	// Calculate the transaction ID
	sha256Hasher := sha256.New()
	sha256Hasher.Write(headerBytes)
	tid, err := multihash.Encode(sha256Hasher.Sum(nil), multihash.SHA2_256)
	if err != nil {
		return nil, err
	}

	transaction.Id = tid

	// Sign the transaction
	err = util.SignTransaction(key.PrivateBytes(), transaction)

	if err != nil {
		return nil, err
	}

	return transaction, nil
}

// AwaitChain blocks until the chain rpc is responding
func AwaitChain(t *testing.T, client *kjsonrpc.KoinosRPCClient) {
	headInfoResponse := chain.GetHeadInfoResponse{}

	var waitDuration int64 = 1
	const maxRetry int64 = 5

	for {
		err := client.Call("chain.get_head_info", &chain.GetHeadInfoRequest{}, &headInfoResponse)
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

	key, err := util.NewKoinosKeysFromBytes(bytes)
	if err != nil {
		return nil, err
	}

	return key, nil
}

// SetSystemCallOverride overrides a system call with a contract and entrypoint call target
func SetSystemCallOverride(client *kjsonrpc.KoinosRPCClient, key *util.KoinosKey, entryPoint uint32, systemCall uint32) error {
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
func UploadContractTransaction(client *kjsonrpc.KoinosRPCClient, file string, key *util.KoinosKey) (*protocol.Transaction, error) {
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

// UploadSystemContract uploads a contract and sets it as a system contract
func UploadSystemContract(client *kjsonrpc.KoinosRPCClient, file string, key *util.KoinosKey) error {
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

	transaction1, err := CreateTransaction(client, []*protocol.Operation{uploadOperation}, key)
	if err != nil {
		return err
	}

	genesisKey, err := GetKey(Genesis)
	if err != nil {
		return err
	}

	ssc := protocol.SetSystemContractOperation{}
	ssc.ContractId = key.AddressBytes()
	ssc.SystemContract = true

	setSystemContractOperation := &protocol.Operation{
		Op: &protocol.Operation_SetSystemContract{
			SetSystemContract: &ssc,
		},
	}

	transaction2, err := CreateTransaction(client, []*protocol.Operation{setSystemContractOperation}, genesisKey)
	if err != nil {
		return err
	}

	_, err = CreateBlock(client, []*protocol.Transaction{transaction1, transaction2})
	return err
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
		t.Logf(" > Transaction: " + transactionID)

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

	assert.NoError(t, err)
}
