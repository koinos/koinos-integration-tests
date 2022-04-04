package integration

import (
	"crypto/sha256"
	"os"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/koinos/koinos-proto-golang/koinos/canonical"
	"github.com/koinos/koinos-proto-golang/koinos/contracts/token"
	"github.com/koinos/koinos-proto-golang/koinos/protocol"
	"github.com/koinos/koinos-proto-golang/koinos/rpc/chain"
	util "github.com/koinos/koinos-util-golang"
	kjsonrpc "github.com/koinos/koinos-util-golang/rpc"
	"github.com/multiformats/go-multihash"
	"google.golang.org/protobuf/proto"
)

const KoinWIF string = "5JbxDqUqx581iL9Po1mLvHMLkxnmjvypDdnmdLQvK5TzSpCFSgH"
const GenesisWIF string = "5KYPA63Gx4MxQUqDM3PMckvX9nVYDUaLigTKAsLPesTyGmKmbR2"
const GovernanceWIF string = "5KdCtpQ4DiFxgPd8VhexLwDfucJ83Mzc81ZviqU1APSzba8vNZV"
const ResourcesWIF string = "5J4f6NdoPEDow7oRuGvuD9ggjr1HvWzirjZP6sJKSvsNnKenyi3"
const PowWIF string = "5KKuscNqrWadRaCCt7oCF7kz6XdL4QMJE9MAnAVShA3JGJEze3p"

func SignBlock(block *protocol.Block, key *util.KoinosKey) error {
	privateKey, _ := btcec.PrivKeyFromBytes(btcec.S256(), key.PrivateBytes())

	// Decode the ID
	idBytes, err := multihash.Decode(block.Id)
	if err != nil {
		return err
	}

	signatureBytes, err := btcec.SignCompact(btcec.S256(), privateKey, idBytes.Digest, true)
	if err != nil {
		return err
	}

	block.Signature = signatureBytes

	return nil
}

// CreateBlock creates a block from a list of transactions
func CreateBlock(client *kjsonrpc.KoinosRPCClient, transactions []*protocol.Transaction, key *util.KoinosKey) (*protocol.Block, error) {
	block := protocol.Block{}
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

	return &block, nil
}

// CreateTransaction creates a transaction from a list of operations
func CreateTransaction(client *kjsonrpc.KoinosRPCClient, ops []*protocol.Operation, key *util.KoinosKey) (*protocol.Transaction, error) {
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

	// Calculate the transaction ID
	sha256Hasher := sha256.New()
	sha256Hasher.Write(headerBytes)
	tid, err := multihash.Encode(sha256Hasher.Sum(nil), multihash.SHA2_256)
	if err != nil {
		return nil, err
	}

	// Create the transaction
	transaction := protocol.Transaction{Header: &header, Operations: ops, Id: tid}

	// Sign the transaction
	err = util.SignTransaction(key.PrivateBytes(), &transaction)

	if err != nil {
		return nil, err
	}

	return &transaction, nil
}

func AwaitChain(t *testing.T, client *kjsonrpc.KoinosRPCClient) {
	headInfoResponse := chain.GetHeadInfoResponse{}

	var waitDuration int64 = 1
	const maxRetry int64 = 10

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

func UploadKoinContract(client *kjsonrpc.KoinosRPCClient) error {
	koinKey, err := KeyFromWIF(KoinWIF)
	if err != nil {
		return err
	}

	wasm, err := BytesFromFile("../../contracts/koin.wasm", 256000)
	if err != nil {
		return err
	}

	uco := protocol.UploadContractOperation{}
	uco.ContractId = koinKey.AddressBytes()
	uco.Bytecode = wasm

	uploadOperation := &protocol.Operation{
		Op: &protocol.Operation_UploadContract{
			UploadContract: &uco,
		},
	}

	transaction1, err := CreateTransaction(client, []*protocol.Operation{uploadOperation}, koinKey)
	if err != nil {
		return err
	}

	genesisKey, err := KeyFromWIF(GenesisWIF)
	if err != nil {
		return err
	}

	ssc := protocol.SetSystemContractOperation{}
	ssc.ContractId = koinKey.AddressBytes()
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

	block, err := CreateBlock(client, []*protocol.Transaction{transaction1, transaction2}, genesisKey)
	if err != nil {
		return err
	}

	err = SignBlock(block, genesisKey)
	if err != nil {
		return err
	}

	var submitBlockResp chain.SubmitBlockResponse
	err = client.Call("chain.submit_block", &chain.SubmitBlockRequest{Block: block}, &submitBlockResp)
	if err != nil {
		return err
	}

	return nil
}

func KoinMint(client *kjsonrpc.KoinosRPCClient, to []byte, value uint64) error {
	koinKey, err := KeyFromWIF(KoinWIF)
	if err != nil {
		return err
	}

	op, err := MakeKoinMintOperation(to, value)
	if err != nil {
		return err
	}

	transaction, err := CreateTransaction(client, []*protocol.Operation{op}, koinKey)
	if err != nil {
		return err
	}

	genesisKey, err := KeyFromWIF(GenesisWIF)
	if err != nil {
		return err
	}

	block, err := CreateBlock(client, []*protocol.Transaction{transaction}, genesisKey)
	if err != nil {
		return err
	}

	err = SignBlock(block, genesisKey)
	if err != nil {
		return err
	}

	var submitBlockResp chain.SubmitBlockResponse
	err = client.Call("chain.submit_block", &chain.SubmitBlockRequest{Block: block}, &submitBlockResp)
	if err != nil {
		return err
	}

	return nil
}

func MakeKoinMintOperation(to []byte, value uint64) (*protocol.Operation, error) {
	koinKey, err := KeyFromWIF(KoinWIF)
	if err != nil {
		return nil, err
	}

	mintArgs := &token.MintArguments{}
	mintArgs.To = to
	mintArgs.Value = value

	args, err := proto.Marshal(mintArgs)
	if err != nil {
		return nil, err
	}

	mop := &protocol.CallContractOperation{}
	mop.ContractId = koinKey.AddressBytes()
	mop.EntryPoint = 0xdc6f17bb
	mop.Args = args

	op := &protocol.Operation{
		Op: &protocol.Operation_CallContract{
			CallContract: mop,
		},
	}

	return op, nil
}

func KoinBalance(client *kjsonrpc.KoinosRPCClient, address []byte) (uint64, error) {
	koinKey, err := KeyFromWIF(KoinWIF)
	if err != nil {
		return 0, err
	}

	balance, err := client.GetAccountBalance(address, koinKey.AddressBytes(), 0x5c721497)
	if err != nil {
		return 0, err
	}

	return balance, nil
}

func KoinTotalSupply(client *kjsonrpc.KoinosRPCClient) (uint64, error) {
	koinKey, err := KeyFromWIF(KoinWIF)
	if err != nil {
		return 0, err
	}

	resp, err := client.ReadContract(make([]byte, 0), koinKey.AddressBytes(), 0xb0da3934)
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

func KoinTransfer(client *kjsonrpc.KoinosRPCClient, from *util.KoinosKey, to []byte, value uint64) error {
	op, err := MakeKoinTransferOperation(from.AddressBytes(), to, value)
	if err != nil {
		return err
	}

	transaction, err := CreateTransaction(client, []*protocol.Operation{op}, from)
	if err != nil {
		return err
	}

	genesisKey, err := KeyFromWIF(GenesisWIF)
	if err != nil {
		return err
	}

	block, err := CreateBlock(client, []*protocol.Transaction{transaction}, genesisKey)
	if err != nil {
		return err
	}

	err = SignBlock(block, genesisKey)
	if err != nil {
		return err
	}

	var submitBlockResp chain.SubmitBlockResponse
	err = client.Call("chain.submit_block", &chain.SubmitBlockRequest{Block: block}, &submitBlockResp)
	if err != nil {
		return err
	}

	return nil
}

func MakeKoinTransferOperation(from []byte, to []byte, value uint64) (*protocol.Operation, error) {
	koinKey, err := KeyFromWIF(KoinWIF)
	if err != nil {
		return nil, err
	}

	transferArgs := &token.TransferArguments{}
	transferArgs.From = from
	transferArgs.To = to
	transferArgs.Value = value

	args, err := proto.Marshal(transferArgs)
	if err != nil {
		return nil, err
	}

	xfer := &protocol.CallContractOperation{}
	xfer.ContractId = koinKey.AddressBytes()
	xfer.EntryPoint = 0x27f576ca
	xfer.Args = args

	transferOperation := &protocol.Operation{
		Op: &protocol.Operation_CallContract{
			CallContract: xfer,
		},
	}

	return transferOperation, nil
}
