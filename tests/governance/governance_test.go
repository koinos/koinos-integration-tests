package governance

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/koinos/koinos-proto-golang/koinos/canonical"
	"github.com/koinos/koinos-proto-golang/koinos/protocol"
	"github.com/koinos/koinos-proto-golang/koinos/rpc/chain"
	util "github.com/koinos/koinos-util-golang"
	kjsonrpc "github.com/koinos/koinos-util-golang/rpc"
	"github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/encoding/protojson"
)

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
	transactionHashes := make([][]byte, len(transactions))
	for i, tx := range transactions {
		transactionHashes[i], err = util.HashMessage(tx)
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

func awaitChain(client *kjsonrpc.KoinosRPCClient) {
	headInfoResponse := chain.GetHeadInfoResponse{}

	var waitDuration int64 = 1
	const maxRetry int64 = 10

	for {
		err := client.Call("chain.get_head_info", &chain.GetHeadInfoRequest{}, &headInfoResponse)
		if err == nil {
			break
		}

		fmt.Printf("Waiting %s for chain to be ready...\n", time.Duration(waitDuration)*time.Second)
		time.Sleep(time.Duration(waitDuration) * time.Second)
		if waitDuration*2 > maxRetry {
			waitDuration = maxRetry
		} else {
			waitDuration = waitDuration * 2
		}
	}
}

func outputDocker(t *testing.T) {
	cmd := exec.Command("docker-compose", "logs")
	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()
	assert.NoError(t, err)

	fmt.Print(out.String())
}

func TestGovernance(t *testing.T) {
	defer outputDocker(t)
	kill_timer := time.NewTimer(10 * time.Minute)
	go func() {
		<-kill_timer.C
		panic("Timer expired")
	}()

	client := kjsonrpc.NewKoinosRPCClient("http://localhost:8080/")

	awaitChain(client)

	keyBytes, err := util.DecodeWIF("5KYPA63Gx4MxQUqDM3PMckvX9nVYDUaLigTKAsLPesTyGmKmbR2")
	assert.NoError(t, err)

	key, err := util.NewKoinosKeysFromBytes(keyBytes)
	assert.NoError(t, err)

	block, err := CreateBlock(client, []*protocol.Transaction{}, key)
	assert.NoError(t, err)

	err = SignBlock(block, key)
	assert.NoError(t, err)

	var cResp chain.SubmitBlockResponse
	err = client.Call("chain.submit_block", &chain.SubmitBlockRequest{Block: block}, &cResp)
	assert.NoError(t, err)

	b, err := protojson.Marshal(cResp.GetReceipt())
	assert.NoError(t, err)
	fmt.Println(string(b))

	var headInfResp chain.GetHeadInfoResponse
	err = client.Call("chain.get_head_info", &chain.GetHeadInfoRequest{}, &headInfResp)
	assert.NoError(t, err)

	str, err := protojson.Marshal(headInfResp.GetHeadTopology())
	assert.NoError(t, err)

	fmt.Println(string(str))
}
