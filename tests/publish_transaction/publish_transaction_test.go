package publishTransaction

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	kjson "github.com/koinos/koinos-proto-golang/koinos/json"
	"github.com/koinos/koinos-proto-golang/koinos/protocol"
	"github.com/koinos/koinos-proto-golang/koinos/rpc/block_store"
	"github.com/koinos/koinos-proto-golang/koinos/rpc/chain"
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

	id, _ := hex.DecodeString("122076547abc9c854b5f215963bba62fac78e8c6ab9f2a0977fb3e7b60eb00c6ed51")
	activeData, _ := hex.DecodeString("08c0843d")
	sigData, _ := hex.DecodeString("1f0e51b5a9bdb8febe16a567b97526d1923fd6b37fabd4f875de2cadd5ff37d941362753bd02c7a9168666df674d4efe4e6de356dae0cd049513b2f5528700c6c1")

	submitTrxRequest := chain.SubmitTransactionRequest{}
	submitTrxRequest.Transaction = &protocol.Transaction{}
	submitTrxRequest.Transaction.Id = id
	submitTrxRequest.Transaction.Active = activeData
	submitTrxRequest.Transaction.SignatureData = sigData

	submitJSON, _ := kjson.Marshal(&submitTrxRequest)

	response, err := rpcClient.Call("chain.submit_transaction", json.RawMessage(submitJSON))
	if err != nil {
		t.Error(err)
	}

	if response.Error != nil {
		t.Error(response.Error)
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

	response, err = rpcClient.Call("block_store.get_blocks_by_height", json.RawMessage(blocksReq))
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

	if bytes.Compare(trx.SignatureData, sigData) != 0 {
		t.Errorf("Unexpected transaction signature")
	}

	if bytes.Compare(trx.Active, activeData) != 0 {
		t.Errorf("Unexpected transaction active data")
	}
}
