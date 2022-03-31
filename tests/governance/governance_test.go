package governance

import (
	"fmt"
	"koinos-integration-tests/integration"
	"testing"
	"time"

	"github.com/koinos/koinos-proto-golang/koinos/protocol"
	"github.com/koinos/koinos-proto-golang/koinos/rpc/chain"
	util "github.com/koinos/koinos-util-golang"
	kjsonrpc "github.com/koinos/koinos-util-golang/rpc"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestGovernance(t *testing.T) {
	killTimer := time.NewTimer(10 * time.Minute)
	go func() {
		<-killTimer.C
		panic("Timer expired")
	}()

	client := kjsonrpc.NewKoinosRPCClient("http://localhost:8080/")

	integration.AwaitChain(client)

	keyBytes, err := util.DecodeWIF("5KYPA63Gx4MxQUqDM3PMckvX9nVYDUaLigTKAsLPesTyGmKmbR2")
	assert.NoError(t, err)

	key, err := util.NewKoinosKeysFromBytes(keyBytes)
	assert.NoError(t, err)

	block, err := integration.CreateBlock(client, []*protocol.Transaction{}, key)
	assert.NoError(t, err)

	err = integration.SignBlock(block, key)
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

	t.Fail()
}
