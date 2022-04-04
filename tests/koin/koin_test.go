package koin

import (
	"koinos-integration-tests/integration"
	"testing"
	"time"

	kjsonrpc "github.com/koinos/koinos-util-golang/rpc"
	"github.com/stretchr/testify/assert"
)

func TestKoin(t *testing.T) {
	killTimer := time.NewTimer(10 * time.Minute)
	go func() {
		<-killTimer.C
		panic("Timer expired")
	}()

	client := kjsonrpc.NewKoinosRPCClient("http://localhost:8080/")

	integration.AwaitChain(client)

	// genesisKey, err := integration.KeyFromWIF("5KYPA63Gx4MxQUqDM3PMckvX9nVYDUaLigTKAsLPesTyGmKmbR2")
	// assert.NoError(t, err)

	err := integration.UploadKoinContract(client)
	assert.NoError(t, err)
}
