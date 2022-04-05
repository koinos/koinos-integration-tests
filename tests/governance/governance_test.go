package governance

import (
	"koinos-integration-tests/integration"
	"testing"

	kjsonrpc "github.com/koinos/koinos-util-golang/rpc"
	"github.com/stretchr/testify/assert"
)

func TestGovernance(t *testing.T) {
	client := kjsonrpc.NewKoinosRPCClient("http://localhost:8080/")

	integration.AwaitChain(t, client)

	governanceKey, err := integration.GetKey(integration.Governance)
	assert.NoError(t, err)

	t.Logf("Uploading governance contract")
	err = integration.UploadSystemContract(client, "../../contracts/governance.wasm", governanceKey)
	assert.NoError(t, err)
}
