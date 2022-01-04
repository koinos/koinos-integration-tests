package publishTransaction

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	kjson "github.com/koinos/koinos-proto-golang/encoding/json"
	"github.com/koinos/koinos-proto-golang/koinos/protocol"
	"github.com/koinos/koinos-proto-golang/koinos/rpc/block_store"
	"github.com/koinos/koinos-proto-golang/koinos/rpc/chain"
	util "github.com/koinos/koinos-util-golang"
	kjsonrpc "github.com/koinos/koinos-util-golang/rpc"
	jsonrpc "github.com/ybbus/jsonrpc/v2"
)

func TestPublishTransaction(t *testing.T) {
	kill_timer := time.NewTimer(10 * time.Minute)
	go func() {
		<-kill_timer.C
		panic("Timer expired")
	}()

	rpcClient := jsonrpc.NewClient("http://localhost:8080/")

	headInfoResponse := chain.GetHeadInfoResponse{}

	for {
		response, err := rpcClient.Call("chain.get_head_info", json.RawMessage("{}"))
		if err == nil && response.Error == nil {
			raw := json.RawMessage{}
			err := response.GetObject(&raw)
			if err != nil {
				t.Error(err)
			}

			err = kjson.Unmarshal([]byte(raw), &headInfoResponse)
			if err != nil {
				t.Error(err)
			}

			break
		}

		time.Sleep(time.Second)
	}

	test_timer := time.NewTimer(45 * time.Second)
	go func() {
		<-test_timer.C
		panic("Timer expired")
	}()

	for {
		response, err := rpcClient.Call("chain.get_head_info", json.RawMessage("{}"))
		if err == nil && response.Error == nil {
			raw := json.RawMessage{}
			err := response.GetObject(&raw)
			if err != nil {
				t.Error(err)
			}

			err = kjson.Unmarshal([]byte(raw), &headInfoResponse)
			if err != nil {
				t.Error(err)
			}

			if headInfoResponse.HeadTopology.Height > 0 {
				break
			}
		}

		time.Sleep(time.Second)
	}

	startingBlock := headInfoResponse.HeadTopology.Height

	key, err := util.GenerateKoinosKey()
	if err != nil {
		t.Error(err)
	}

	krpc := kjsonrpc.NewKoinosRPCClient("http://localhost:8080/")

	ops := make([]*protocol.Operation, 1)
	ops[0] = &protocol.Operation{
		Op: &protocol.Operation_UploadContract{
			UploadContract: &protocol.UploadContractOperation{
				ContractId: key.AddressBytes(),
			},
		},
	}

	id, err := krpc.SubmitTransaction(ops, key)

	if err != nil {
		t.Error(err)
	}

	currentHeight := headInfoResponse.HeadTopology.Height

	for currentHeight <= startingBlock {
		response, err := rpcClient.Call("chain.get_head_info", json.RawMessage("{}"))
		if err == nil {
			raw := json.RawMessage{}
			err := response.GetObject(&raw)
			if err != nil {
				t.Error(err)
			}

			err = kjson.Unmarshal([]byte(raw), &headInfoResponse)
			if err != nil {
				t.Error(err)
			}

			currentHeight = headInfoResponse.HeadTopology.Height
		}

		time.Sleep(time.Second)
	}

	getBlocksByHeightRequest := block_store.GetBlocksByHeightRequest{
		HeadBlockId:         headInfoResponse.HeadTopology.Id,
		AncestorStartHeight: startingBlock + 1,
		NumBlocks:           1,
		ReturnBlock:         true,
		ReturnReceipt:       false,
	}
	blocksReq, err := kjson.Marshal(&getBlocksByHeightRequest)

	response, err := rpcClient.Call("block_store.get_blocks_by_height", json.RawMessage(blocksReq))
	if err != nil {
		t.Error(err)
	}

	if response.Error != nil {
		t.Error(response.Error)
	}

	getBlocksByHeightResponse := &block_store.GetBlocksByHeightResponse{}
	raw := json.RawMessage{}
	err = response.GetObject(&raw)
	if err != nil {
		t.Error(err)
	}

	err = kjson.Unmarshal([]byte(raw), getBlocksByHeightResponse)
	if err != nil {
		t.Error(err)
	}

	if getBlocksByHeightResponse.BlockItems == nil || len(getBlocksByHeightResponse.BlockItems) != 1 {
		t.Errorf("Expected 1 block item, was %v", len(getBlocksByHeightResponse.BlockItems))
	}

	blockItem := getBlocksByHeightResponse.BlockItems[0]

	if blockItem.BlockHeight != startingBlock+1 {
		t.Errorf("Expected block %v, was %v", startingBlock+1, blockItem.BlockHeight)
	}

	if blockItem.Block == nil {
		t.Errorf("Block was unexpectedly null")
	}

	if len(blockItem.Block.Transactions) != 1 {
		t.Errorf("Expected 1 transaction, was %v", len(blockItem.Block.Transactions))
	}

	trx := blockItem.Block.Transactions[0]

	if bytes.Compare(trx.Id, id) != 0 {
		t.Errorf("Unexpected transaction id")
	}
}
