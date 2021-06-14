package publishTransaction

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	types "github.com/koinos/koinos-types-golang"
	jsonrpc "github.com/ybbus/jsonrpc/v2"
)

func TestPublishTransaction(t *testing.T) {
	kill_timer := time.NewTimer(10 * time.Minute)
	go func() {
		<-kill_timer.C
		panic("Timer expired")
	}()

	rpcClient := jsonrpc.NewClient("http://localhost:8080/")

	headInfoRequest := types.GetHeadInfoRequest{}
	headInfoResponse := types.GetHeadInfoResponse{}

	for {
		response, err := rpcClient.Call("chain.get_head_info", headInfoRequest)
		if err == nil && response.Error == nil {
			err := response.GetObject(&headInfoResponse)
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
		response, err := rpcClient.Call("chain.get_head_info", headInfoRequest)
		if err == nil && response.Error == nil {
			err := response.GetObject(&headInfoResponse)
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

	submitTrxRequest := types.SubmitTransactionRequest{}
	submitTrxJSON := "{\"transaction\":{\"active_data\":{\"nonce\":0,\"operations\":[{\"type\":\"koinos::protocol::create_system_contract_operation\",\"value\":{\"bytecode\":\"z1V7mJPthM7N8fStfyib4zG6P8UZ3XxRu79LBUdyYUJFZMCD6LVbDSXVBYhqAF8gsGzW9EQTZ6P9ubXcLwzc76ouXrLY8jmWTfVe83a1xBVaKgLsKjtYMnxFvN9AdgEGumoERZoyZC7XFxo97pvdCKvfHGpESu9kSFMB5MVpPxJYfRbgCPyPNwLNU2KijTpiNyqksr1t7EbwyFZNxZNvECDi3qahqLb2R28tksichZZnoEJatWca4pJhVEtkSeR82TzgGWzunbTLikHVcFGofXBrdFXNaxQbMaMtWWYxVuroUTAEXTYMS1Akk3nsDHYeXp6EV365AhZS8Bgbbk2VBVC9o4xL7UE9nokewSKbWKZUkg6nRmQJUwpyWMwY6T8rvHNat7S94XBL5mGqKSEjWNDcztVmqrAqVigLsjqtV3dRt5Fg45CgbV97GnNTyPTBpD38L3dnkFEBi6C2Gc6CPG4U2xRtCnMr7HqT9kajrw1oUirfZio1ejoAtDNCCG3hef9kUqMjgKujU44gdSHACqJ9iKwGPj9b5EcfpuPZF17E1yat1xAkphxAfcRo2bH9aW6zQYfhLVD32jVx54NXRekFkzGBPZxtxMroP48tyxiKTRF9zWC2U6DjW4rvxGJZtocvFamjKwjCqrRMofdPuRcZ4h27zfREdac9hAZ7ik9brHEe4byQ6SuVkBpPqnsoigf6hu818QidHKuu3BcxLJSjk3BwvPDnDu5W35x7roYpQVGQH6nPSWWkDqGxXebY5L3w7orP3YU\",\"contract_id\":\"z3iBzP4BjB1QMXCM9yNWJGdPAP1Ar\",\"extensions\":{}}}],\"resource_limit\":1},\"id\":\"zQmWEeUp1FqmhVB8LUbDexcxduet7ZGAe7XKM4a7u39bgwE\",\"passive_data\":{},\"signature_data\":\"z3k9YmtYJr3BPFwHygjjmAHCcc8zGhu8c4jo1vweM8UYjiKxX9pumG2ftaspvaduYFtDwE3zS8zYaQyNqEGdud41Ro\"},\"verify_passive_data\":true,\"verify_transaction_signatures\":true}"

	err := json.Unmarshal([]byte(submitTrxJSON), &submitTrxRequest)
	if err != nil {
		t.Error(err)
	}

	response, err := rpcClient.Call("chain.submit_transaction", &submitTrxRequest)
	if err != nil {
		t.Error(err)
	}

	if response.Error != nil {
		t.Error(response.Error)
	}

	currentHeight := headInfoResponse.HeadTopology.Height

	for currentHeight <= startingBlock {
		response, err := rpcClient.Call("chain.get_head_info", &headInfoRequest)
		if err == nil {
			if response.Error != nil {
				t.Error(response.Error)
			}

			err := response.GetObject(&headInfoResponse)
			if err != nil {
				t.Error(err)
			}

			currentHeight = headInfoResponse.HeadTopology.Height
		}

		time.Sleep(time.Second)
	}

	getBlocksByHeightRequest := types.GetBlocksByHeightRequest{
		HeadBlockID:         headInfoResponse.HeadTopology.ID,
		AncestorStartHeight: startingBlock + 1,
		NumBlocks:           1,
		ReturnBlock:         true,
		ReturnReceipt:       false,
	}
	getBlocksByHeightResponse := types.GetBlocksByHeightResponse{}

	response, err = rpcClient.Call("block_store.get_blocks_by_height", &getBlocksByHeightRequest)
	if err != nil {
		t.Error(err)
	}

	if response.Error != nil {
		t.Error(response.Error)
	}

	err = response.GetObject(&getBlocksByHeightResponse)
	if err != nil {
		t.Error(err)
	}

	if len(getBlocksByHeightResponse.BlockItems) != 1 {
		t.Errorf("Expected 1 block item, was %v", len(getBlocksByHeightResponse.BlockItems))
	}

	blockItem := getBlocksByHeightResponse.BlockItems[0]

	if blockItem.BlockHeight != startingBlock+1 {
		t.Errorf("Expected block %v, was %v", startingBlock+1, blockItem.BlockHeight)
	}

	blockItem.Block.Unbox()
	if blockItem.Block.IsBoxed() {
		t.Error("Could not unbox returned block")
	}

	block, err := blockItem.Block.GetNative()
	if err != nil {
		t.Error(err)
	}

	if len(block.Transactions) != 1 {
		t.Errorf("Expected 1 transaction, was %v", len(block.Transactions))
	}

	trx := block.Transactions[0]

	b, _ := json.Marshal(trx.ID)
	if bytes.Compare(b, []byte("zQmWEeUp1FqmhVB8LUbDexcxduet7ZGAe7XKM4a7u39bgwE")) == 0 {
		t.Errorf("Unexpected transaction id, %v", string(b))
	}

	b, _ = json.Marshal(trx.SignatureData)
	if bytes.Compare(b, []byte("z3k9YmtYJr3BPFwHygjjmAHCcc8zGhu8c4jo1vweM8UYjiKxX9pumG2ftaspvaduYFtDwE3zS8zYaQyNqEGdud41Ro")) == 0 {
		t.Errorf("Unexpected transaction signature, %v", string(b))
	}

	trx.ActiveData.Unbox()
	if trx.ActiveData.IsBoxed() {
		t.Errorf("Could not unbox transaction active data")
	}

	// We could check contents of Active Data, but this should be sufficient for an integration test
}
