package bucketBrigade

import (
	"bytes"
	"encoding/json"
	"log"
	"testing"
	"time"

	kjson "github.com/koinos/koinos-proto-golang/koinos/json"
	"github.com/koinos/koinos-proto-golang/koinos/rpc/chain"
	jsonrpc "github.com/ybbus/jsonrpc/v2"
)

func TestBucketBrigade(t *testing.T) {
	kill_timer := time.NewTimer(10 * time.Minute)
	go func() {
		<-kill_timer.C
		panic("Timer expired")
	}()

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

			break

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

			break

			log.Printf("Bucket2 Height %d", endHeadInfoResponse.HeadTopology.Height)

			if endHeadInfoResponse.HeadTopology.Height >= headInfoResponse.HeadTopology.Height {
				break
			}
		}

		time.Sleep(time.Second)
	}

	if bytes.Compare(headInfoResponse.HeadTopology.Id, endHeadInfoResponse.HeadTopology.Id) != 0 {
		t.Error("Head block IDs do not match")
	}
}
