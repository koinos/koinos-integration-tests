package bucketBrigade

import (
	"bytes"
	"testing"
	"time"

	types "github.com/koinos/koinos-types-golang"
	jsonrpc "github.com/ybbus/jsonrpc/v2"
)

func TestBucketBrigade(t *testing.T) {
	timer := time.NewTimer(80 * time.Second)
	go func() {
		<-timer.C
		t.Error("Timer expired")
	}()

	producerClient := jsonrpc.NewClient("http://localhost:8080/")

	headInfoRequest := types.GetHeadInfoRequest{}
	headInfoResponse := types.GetHeadInfoResponse{}

	for {
		response, err := producerClient.Call("chain.get_head_info", headInfoRequest)
		if err == nil && response.Error == nil {
			err := response.GetObject(&headInfoResponse)
			if err != nil {
				t.Error(err)
			}

			if headInfoResponse.HeadTopology.Height > 5 {
				break
			}
		}

		time.Sleep(time.Second)
	}

	endHeadInfoResponse := types.GetHeadInfoResponse{}

	endClient := jsonrpc.NewClient("http://localhost:8082/")
	for {
		response, err := endClient.Call("chain.get_head_info", headInfoRequest)
		if err == nil && response.Error == nil {
			err := response.GetObject(&endHeadInfoResponse)
			if err != nil {
				t.Error(err)
			}

			if endHeadInfoResponse.HeadTopology.Height >= headInfoResponse.HeadTopology.Height {
				break
			}
		}

		time.Sleep(time.Second)
	}

	if bytes.Compare(headInfoResponse.HeadTopology.ID.Digest, endHeadInfoResponse.HeadTopology.ID.Digest) != 0 || headInfoResponse.HeadTopology.ID.ID != endHeadInfoResponse.HeadTopology.ID.ID {
		t.Error("Head block IDs do not match")
	}
}
