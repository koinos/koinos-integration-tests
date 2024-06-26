package publishTransaction

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	kjson "github.com/koinos/koinos-proto-golang/v2/encoding/json"
	"github.com/koinos/koinos-proto-golang/v2/koinos/protocol"
	"github.com/koinos/koinos-proto-golang/v2/koinos/rpc/block_store"
	"github.com/koinos/koinos-proto-golang/v2/koinos/rpc/chain"
	util "github.com/koinos/koinos-util-golang/v2"
	kjsonrpc "github.com/koinos/koinos-util-golang/v2/rpc"
	jsonrpc "github.com/ybbus/jsonrpc/v3"
)

func TestPublishTransaction(t *testing.T) {
	rpcClient := jsonrpc.NewClient("http://localhost:28080/")

	headInfoResponse := chain.GetHeadInfoResponse{}

	for {
		response, err := rpcClient.Call(context.Background(), "chain.get_head_info", json.RawMessage("{}"))
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
		response, err := rpcClient.Call(context.Background(), "chain.get_head_info", json.RawMessage("{}"))
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

	krpc := kjsonrpc.NewKoinosRPCClient("http://localhost:28080/")

	ops := make([]*protocol.Operation, 1)
	ops[0] = &protocol.Operation{
		Op: &protocol.Operation_UploadContract{
			UploadContract: &protocol.UploadContractOperation{
				ContractId: key.AddressBytes(),
			},
		},
	}

	txReceipt, err := krpc.SubmitTransaction(context.Background(), ops, key, &kjsonrpc.SubmissionParams{Nonce: 0, RCLimit: 0}, true)

	if err != nil {
		t.Error(err)
	}

	currentHeight := headInfoResponse.HeadTopology.Height

	for currentHeight <= startingBlock {
		response, err := rpcClient.Call(context.Background(), "chain.get_head_info", json.RawMessage("{}"))
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
	if err != nil {
		t.Error(err)
	}

	response, err := rpcClient.Call(context.Background(), "block_store.get_blocks_by_height", json.RawMessage(blocksReq))
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

	if !bytes.Equal(trx.Id, txReceipt.Id) {
		t.Errorf("Unexpected transaction id")
	}
}
