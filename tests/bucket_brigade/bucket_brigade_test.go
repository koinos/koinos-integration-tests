package bucketBrigade

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"log"
	"testing"
	"time"

	kjson "github.com/koinos/koinos-proto-golang/encoding/json"
	"github.com/koinos/koinos-proto-golang/koinos/rpc/block_store"
	"github.com/koinos/koinos-proto-golang/koinos/rpc/chain"
	jsonrpc "github.com/ybbus/jsonrpc/v2"
)

func TestBucketBrigade(t *testing.T) {
	producerClient := jsonrpc.NewClient("http://localhost:8080/")
	endClient := jsonrpc.NewClient("http://localhost:8082/")

	headInfoResponse := chain.GetHeadInfoResponse{}

	for {
		response, err := endClient.Call("chain.get_head_info", json.RawMessage("{}"))
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

	log.Print("Starting test...")

	test_timer := time.NewTimer(120 * time.Second)
	go func() {
		<-test_timer.C
		panic("Timer expired")
	}()

	for {
		response, err := producerClient.Call("chain.get_head_info", json.RawMessage("{}"))
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

			log.Printf("Producer Height %d", headInfoResponse.HeadTopology.Height)

			if headInfoResponse.HeadTopology.Height > 5 {
				break
			}
		}

		time.Sleep(time.Second)
	}

	endHeadInfoResponse := chain.GetHeadInfoResponse{}

	for {
		response, err := endClient.Call("chain.get_head_info", json.RawMessage("{}"))
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

			log.Printf("Bucket2 Height %d", endHeadInfoResponse.HeadTopology.Height)

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

	response, err := endClient.Call("block_store.get_blocks_by_height", json.RawMessage(blocksReq))
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

	if bytes.Compare(headInfoResponse.HeadTopology.Id, blockItem.BlockId) != 0 {
		t.Errorf("Head block IDs do not match, (%s, %s)", hex.EncodeToString(headInfoResponse.HeadTopology.Id), hex.EncodeToString(blockItem.BlockId))
	}
}
