package governance

import (
	"fmt"
	"koinos-integration-tests/integration"
	"testing"
	"time"

	"github.com/koinos/koinos-proto-golang/koinos/protocol"
	"github.com/koinos/koinos-proto-golang/koinos/rpc/chain"
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

	genesisKey, err := integration.KeyFromWIF("5KYPA63Gx4MxQUqDM3PMckvX9nVYDUaLigTKAsLPesTyGmKmbR2")
	assert.NoError(t, err)

	// koinKey, err := integration.KeyFromWIF("5JbxDqUqx581iL9Po1mLvHMLkxnmjvypDdnmdLQvK5TzSpCFSgH")
	// assert.NoError(t, err)

	// governanceKey, err := integration.KeyFromWIF("5KdCtpQ4DiFxgPd8VhexLwDfucJ83Mzc81ZviqU1APSzba8vNZV")
	// assert.NoError(t, err)

	block, err := integration.CreateBlock(client, []*protocol.Transaction{}, genesisKey)
	assert.NoError(t, err)

	err = integration.SignBlock(block, genesisKey)
	assert.NoError(t, err)

	var submitBlockResp chain.SubmitBlockResponse
	err = client.Call("chain.submit_block", &chain.SubmitBlockRequest{Block: block}, &submitBlockResp)
	assert.NoError(t, err)

	b, err := protojson.Marshal(submitBlockResp.GetReceipt())
	assert.NoError(t, err)
	fmt.Println(string(b))

	var headInfResp chain.GetHeadInfoResponse
	err = client.Call("chain.get_head_info", &chain.GetHeadInfoRequest{}, &headInfResp)
	assert.NoError(t, err)

	str, err := protojson.Marshal(headInfResp.GetHeadTopology())
	assert.NoError(t, err)

	fmt.Println(string(str))
}
