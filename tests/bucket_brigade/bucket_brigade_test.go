package bucketBrigade

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	kjson "github.com/koinos/koinos-proto-golang/v2/encoding/json"
	"github.com/koinos/koinos-proto-golang/v2/koinos/rpc/block_store"
	"github.com/koinos/koinos-proto-golang/v2/koinos/rpc/chain"
	jsonrpc "github.com/ybbus/jsonrpc/v3"
)

func TestBucketBrigade(t *testing.T) {
	producerClient := jsonrpc.NewClient("http://localhost:28080/")
	endClient := jsonrpc.NewClient("http://localhost:28082/")

	headInfoResponse := chain.GetHeadInfoResponse{}

	for {
		response, err := endClient.Call(context.Background(), "chain.get_head_info", json.RawMessage("{}"))
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

	t.Logf("Starting test...")

	test_timer := time.NewTimer(120 * time.Second)
	go func() {
		<-test_timer.C
		panic("Timer expired")
	}()

	for {
		response, err := producerClient.Call(context.Background(), "chain.get_head_info", json.RawMessage("{}"))
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

			t.Logf("Producer Height %d", headInfoResponse.HeadTopology.Height)

			if headInfoResponse.HeadTopology.Height > 5 {
				break
			}
		}

		time.Sleep(time.Second)
	}

	endHeadInfoResponse := chain.GetHeadInfoResponse{}

	for {
		response, err := endClient.Call(context.Background(), "chain.get_head_info", json.RawMessage("{}"))
		if err == nil && response.Error == nil {
			raw := json.RawMessage{}
			err := response.GetObject(&raw)
			if err != nil {
				t.Error(err)
			}

			err = kjson.Unmarshal([]byte(raw), &endHeadInfoResponse)
			if err != nil {
				t.Error(err)
			}

			t.Logf("Bucket2 Height %d", endHeadInfoResponse.HeadTopology.Height)

			if endHeadInfoResponse.HeadTopology.Height >= headInfoResponse.HeadTopology.Height {
				break
			}
		}

		time.Sleep(time.Second)
	}

	getBlocksByHeightRequest := block_store.GetBlocksByHeightRequest{
		HeadBlockId:         headInfoResponse.HeadTopology.Id,
		AncestorStartHeight: headInfoResponse.HeadTopology.Height,
		NumBlocks:           1,
		ReturnBlock:         false,
		ReturnReceipt:       false,
	}

	blocksReq, err := kjson.Marshal(&getBlocksByHeightRequest)
	if err != nil {
		t.Error(err)
	}

	response, err := endClient.Call(context.Background(), "block_store.get_blocks_by_height", json.RawMessage(blocksReq))
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

	if !bytes.Equal(headInfoResponse.HeadTopology.Id, blockItem.BlockId) {
		t.Errorf("Head block IDs do not match, (%s, %s)", hex.EncodeToString(headInfoResponse.HeadTopology.Id), hex.EncodeToString(blockItem.BlockId))
	}
}
