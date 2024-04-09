package transaction_error

import (
	"context"
	"koinos-integration-tests/integration"
	"testing"
	"time"

	mq "github.com/koinos/koinos-mq-golang"
	"github.com/koinos/koinos-proto-golang/v2/koinos/chain"
	"github.com/koinos/koinos-proto-golang/v2/koinos/protocol"
	chainrpc "github.com/koinos/koinos-proto-golang/v2/koinos/rpc/chain"
	util "github.com/koinos/koinos-util-golang/v2"
	kjsonrpc "github.com/koinos/koinos-util-golang/v2/rpc"
	"google.golang.org/protobuf/proto"
)

func TestTransactionError(t *testing.T) {
	test_timer := time.NewTimer(30 * time.Second)
	go func() {
		<-test_timer.C
		panic("Timer expired")
	}()

	client := kjsonrpc.NewKoinosRPCClient("http://localhost:28080/")
	mqClient := mq.NewClient("amqp://guest:guest@localhost:25672/", mq.NoRetry)
	mqClient.Start(context.Background())

	t.Logf("Generating key for alice")
	aliceKey, err := util.GenerateKoinosKey()
	integration.NoError(t, err)
	t.Logf("Generating key for bob")
	bobKey, err := util.GenerateKoinosKey()
	integration.NoError(t, err)

	integration.AwaitChain(t, client)

	badNonceTransaction, err := integration.CreateTransaction(client, []*protocol.Operation{}, aliceKey, func(transaction *protocol.Transaction) error {
		nonce, err := util.NonceBytesToUInt64(transaction.Header.Nonce)
		if err != nil {
			return err
		}

		nonce += 1

		nonceBytes, err := util.UInt64ToNonceBytes(nonce)
		if err != nil {
			return err
		}

		transaction.Header.Nonce = nonceBytes

		return nil
	})
	integration.NoError(t, err)

	badRcLimitTransaction, err := integration.CreateTransaction(client, []*protocol.Operation{}, aliceKey, func(transaction *protocol.Transaction) error {
		transaction.Header.RcLimit = 1000000000000000 // 10,000,000 Mana
		return nil
	})
	integration.NoError(t, err)

	bobTransaction, err := integration.CreateTransaction(client, []*protocol.Operation{}, bobKey)
	integration.NoError(t, err)

	badSignatureTransaction, err := integration.CreateTransaction(client, []*protocol.Operation{}, aliceKey)
	badSignatureTransaction.Signatures = bobTransaction.Signatures
	integration.NoError(t, err)

	t.Logf("Submitting a transaction with a bad nonce")

	reqBytes, err := proto.Marshal(&chainrpc.ChainRequest{
		Request: &chainrpc.ChainRequest_SubmitTransaction{
			SubmitTransaction: &chainrpc.SubmitTransactionRequest{
				Transaction: badNonceTransaction,
				Broadcast:   true,
			},
		},
	})
	integration.NoError(t, err)

	respBytes, err := mqClient.RPC(context.Background(), mq.OctetStream, "chain", reqBytes)
	integration.NoError(t, err)

	response := &chainrpc.ChainResponse{}
	err = proto.Unmarshal(respBytes, response)
	integration.NoError(t, err)

	switch respType := response.Response.(type) {
	case *chainrpc.ChainResponse_Error:
		eDetails := chain.ErrorDetails{}
		success := false
		for _, detail := range respType.Error.Details {
			if err := detail.UnmarshalTo(&eDetails); err == nil {
				if eDetails.Code != int32(chain.ErrorCode_invalid_nonce) {
					t.Errorf("Submit transaction error code was %v, expected %v", eDetails.Code, chain.ErrorCode_invalid_nonce)
				} else {
					success = true
					break
				}
			}
		}
		if !success {
			t.Errorf("Expected chain ErrorDetails containing an invalid nonce error code. Was not found.")
		}
	default:
		t.Errorf("Expected error response from submit_transaction")
	}

	t.Logf("Submitting a transaction with a bad rc limit")

	// We need a block or else mempool won't reject the transaction for insufficient rc.
	genesisKey, err := integration.GetKey(integration.Genesis)
	integration.NoError(t, err)

	_, err = integration.CreateBlock(client, []*protocol.Transaction{}, genesisKey)
	integration.NoError(t, err)

	reqBytes, err = proto.Marshal(&chainrpc.ChainRequest{
		Request: &chainrpc.ChainRequest_SubmitTransaction{
			SubmitTransaction: &chainrpc.SubmitTransactionRequest{
				Transaction: badRcLimitTransaction,
				Broadcast:   true,
			},
		},
	})
	integration.NoError(t, err)

	respBytes, err = mqClient.RPC(context.Background(), mq.OctetStream, "chain", reqBytes)
	integration.NoError(t, err)

	response = &chainrpc.ChainResponse{}
	err = proto.Unmarshal(respBytes, response)
	integration.NoError(t, err)

	switch respType := response.Response.(type) {
	case *chainrpc.ChainResponse_Error:
		eDetails := chain.ErrorDetails{}
		success := false
		for _, detail := range respType.Error.Details {
			if err := detail.UnmarshalTo(&eDetails); err == nil {
				if eDetails.Code != int32(chain.ErrorCode_insufficient_rc) {
					t.Errorf("Submit transaction error code was %v, expected %v", eDetails.Code, chain.ErrorCode_insufficient_rc)
				} else {
					success = true
					break
				}
			}
		}
		if !success {
			t.Errorf("Expected chain ErrorDetails containing an insufficient rc error code. Was not found.")
		}
	default:
		t.Errorf("Expected error response from submit_transaction")
	}

	t.Logf("Submitting a transaction with an incorrect signature")

	reqBytes, err = proto.Marshal(&chainrpc.ChainRequest{
		Request: &chainrpc.ChainRequest_SubmitTransaction{
			SubmitTransaction: &chainrpc.SubmitTransactionRequest{
				Transaction: badSignatureTransaction,
				Broadcast:   true,
			},
		},
	})
	integration.NoError(t, err)

	respBytes, err = mqClient.RPC(context.Background(), mq.OctetStream, "chain", reqBytes)
	integration.NoError(t, err)

	response = &chainrpc.ChainResponse{}
	err = proto.Unmarshal(respBytes, response)
	integration.NoError(t, err)

	switch respType := response.Response.(type) {
	case *chainrpc.ChainResponse_Error:
		eDetails := chain.ErrorDetails{}
		success := false
		for _, detail := range respType.Error.Details {
			if err := detail.UnmarshalTo(&eDetails); err == nil {
				if eDetails.Code != int32(chain.ErrorCode_authorization_failure) {
					t.Errorf("Submit transaction error code was %v, expected %v", eDetails.Code, chain.ErrorCode_authorization_failure)
				} else {
					success = true
					break
				}
			}
		}
		if !success {
			t.Errorf("Expected chain ErrorDetails containing an authorization failure error code. Was not found.")
		}
	default:
		t.Errorf("Expected error response from submit_transaction")
	}
}
