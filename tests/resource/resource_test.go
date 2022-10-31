package resource

import (
	"context"
	"fmt"
	"koinos-integration-tests/integration"
	"koinos-integration-tests/integration/token"
	"testing"
	"time"

	"github.com/koinos/koinos-proto-golang/koinos/chain"
	"github.com/koinos/koinos-proto-golang/koinos/contracts/resources"
	token_proto "github.com/koinos/koinos-proto-golang/koinos/contracts/token"
	"github.com/koinos/koinos-proto-golang/koinos/protocol"
	chainrpc "github.com/koinos/koinos-proto-golang/koinos/rpc/chain"
	util "github.com/koinos/koinos-util-golang"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

const (
	getResourceLimitsEntry     uint32 = 0x427a0394
	consumeBlockResourcesEntry        = 0x9850b1fd
	getResourceMarketsEntry           = 0xebe9b9e7
)

func getMarkets(client integration.Client, resourceAddress []byte) (*resources.ResourceMarkets, error) {
	// Make the rpc call
	marketsArgs := &resources.GetResourceMarketsArguments{}

	argBytes, err := proto.Marshal(marketsArgs)
	if err != nil {
		return nil, err
	}

	cResp, err := integration.ReadContract(client, argBytes, resourceAddress, getResourceMarketsEntry)
	if err != nil {
		return nil, err
	}

	marketsReturn := &resources.GetResourceMarketsResult{}
	err = proto.Unmarshal(cResp.Result, marketsReturn)
	if err != nil {
		return nil, err
	}

	return marketsReturn.Value, nil
}

func TestResource(t *testing.T) {
	client := integration.NewKoinosMQClient("amqp://guest:guest@localhost:5672/")

	genesisKey, err := integration.GetKey(integration.Genesis)
	integration.NoError(t, err)

	t.Logf("Generating key for alice")
	aliceKey, err := util.GenerateKoinosKey()
	integration.NoError(t, err)

	t.Logf("Generating key for bob")
	bobKey, err := util.GenerateKoinosKey()
	integration.NoError(t, err)

	t.Logf("Generating key for the resource contract")
	resourceKey, err := util.GenerateKoinosKey()
	integration.NoError(t, err)

	require.NotEqualValues(t, aliceKey, bobKey)
	require.NotEqualValues(t, aliceKey, resourceKey)
	require.NotEqualValues(t, bobKey, resourceKey)

	koinKey, err := integration.GetKey(integration.Koin)
	integration.NoError(t, err)

	integration.AwaitChain(t, client)

	integration.InitNameService(t, client)

	t.Logf("Uploading KOIN contract")
	_, err = integration.UploadSystemContract(client, "../../contracts/koin.wasm", koinKey, "koin")
	integration.NoError(t, err)

	t.Logf("Uploading Resource contract")
	_, err = integration.UploadSystemContract(client, "../../contracts/resources.wasm", resourceKey, "resources")

	t.Logf("Minting 50M tKOIN to alice")
	koin := token.GetKoinToken(client)
	err = koin.Mint(aliceKey.AddressBytes(), uint64(5000000000000000))
	integration.NoError(t, err)

	supply, err := koin.TotalSupply()
	integration.NoError(t, err)

	require.EqualValues(t, uint64(5000000000000000), supply)

	enableResource := []*protocol.Operation{
		{
			Op: &protocol.Operation_SetSystemCall{
				SetSystemCall: &protocol.SetSystemCallOperation{
					CallId: uint32(chain.SystemCallId_get_resource_limits),
					Target: &protocol.SystemCallTarget{
						Target: &protocol.SystemCallTarget_SystemCallBundle{
							SystemCallBundle: &protocol.ContractCallBundle{
								ContractId: resourceKey.AddressBytes(),
								EntryPoint: getResourceLimitsEntry,
							},
						},
					},
				},
			},
		},
		{
			Op: &protocol.Operation_SetSystemCall{
				SetSystemCall: &protocol.SetSystemCallOperation{
					CallId: uint32(chain.SystemCallId_consume_block_resources),
					Target: &protocol.SystemCallTarget{
						Target: &protocol.SystemCallTarget_SystemCallBundle{
							SystemCallBundle: &protocol.ContractCallBundle{
								ContractId: resourceKey.AddressBytes(),
								EntryPoint: consumeBlockResourcesEntry,
							},
						},
					},
				},
			},
		},
	}

	tx, err := integration.CreateTransaction(client, enableResource, genesisKey)
	integration.NoError(t, err)

	_, err = integration.CreateBlock(client, []*protocol.Transaction{tx}, genesisKey)
	integration.NoError(t, err)

	markets, err := getMarkets(client, resourceKey.AddressBytes())
	integration.NoError(t, err)

	fmt.Printf("%v\n", markets)

	require.EqualValues(t, uint64(8332061253), markets.DiskStorage.ResourceSupply)
	require.EqualValues(t, uint64(39600), markets.DiskStorage.BlockBudget)
	require.EqualValues(t, uint64(524288), markets.DiskStorage.BlockLimit)

	require.EqualValues(t, uint64(55157213404), markets.NetworkBandwidth.ResourceSupply)
	require.EqualValues(t, uint64(262144), markets.NetworkBandwidth.BlockBudget)
	require.EqualValues(t, uint64(1048576), markets.NetworkBandwidth.BlockLimit)

	require.EqualValues(t, uint64(12098466059839), markets.ComputeBandwidth.ResourceSupply)
	require.EqualValues(t, uint64(57500000), markets.ComputeBandwidth.BlockBudget)
	require.EqualValues(t, uint64(287500000), markets.ComputeBandwidth.BlockLimit)

	type marketTestValue struct {
		DiskSupply    uint64
		DiskCost      uint64
		NetworkSupply uint64
		NetworkCost   uint64
		ComputeSupply uint64
		ComputeCost   uint64
	}

	transferArgs := &token_proto.TransferArguments{
		From:  aliceKey.AddressBytes(),
		To:    bobKey.AddressBytes(),
		Value: 1,
	}

	args, err := proto.Marshal(transferArgs)
	integration.NoError(t, err)

	transferOp := &protocol.Operation{
		Op: &protocol.Operation_CallContract{
			CallContract: &protocol.CallContractOperation{
				ContractId: koinKey.AddressBytes(),
				EntryPoint: 0x27f576ca,
				Args:       args,
			},
		},
	}

	testValues := []marketTestValue{
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157212842, NetworkCost: 7335, ComputeSupply: 12098465541337, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157212280, NetworkCost: 7335, ComputeSupply: 12098465017664, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157211718, NetworkCost: 7335, ComputeSupply: 12098464493995, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157211156, NetworkCost: 7335, ComputeSupply: 12098463970330, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157210594, NetworkCost: 7335, ComputeSupply: 12098463446669, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157210032, NetworkCost: 7335, ComputeSupply: 12098462923012, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157209470, NetworkCost: 7335, ComputeSupply: 12098462399360, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157208908, NetworkCost: 7335, ComputeSupply: 12098461875712, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157208346, NetworkCost: 7335, ComputeSupply: 12098461352068, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157207784, NetworkCost: 7335, ComputeSupply: 12098460828428, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157207222, NetworkCost: 7335, ComputeSupply: 12098460304792, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157206660, NetworkCost: 7335, ComputeSupply: 12098459781161, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157206098, NetworkCost: 7335, ComputeSupply: 12098459257534, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157205536, NetworkCost: 7335, ComputeSupply: 12098458733911, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157204974, NetworkCost: 7335, ComputeSupply: 12098458210292, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157204412, NetworkCost: 7335, ComputeSupply: 12098457686677, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157203850, NetworkCost: 7335, ComputeSupply: 12098457163067, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157203288, NetworkCost: 7335, ComputeSupply: 12098456639461, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157202726, NetworkCost: 7335, ComputeSupply: 12098456115859, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157202164, NetworkCost: 7335, ComputeSupply: 12098455592261, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157201602, NetworkCost: 7335, ComputeSupply: 12098455068667, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157201040, NetworkCost: 7335, ComputeSupply: 12098454545078, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157200478, NetworkCost: 7335, ComputeSupply: 12098454021493, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157199916, NetworkCost: 7335, ComputeSupply: 12098453497912, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157199354, NetworkCost: 7335, ComputeSupply: 12098452974335, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157198792, NetworkCost: 7335, ComputeSupply: 12098452450762, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157198230, NetworkCost: 7335, ComputeSupply: 12098451927194, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157197668, NetworkCost: 7335, ComputeSupply: 12098451403630, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157197106, NetworkCost: 7335, ComputeSupply: 12098450880070, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157196544, NetworkCost: 7335, ComputeSupply: 12098450356514, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157195982, NetworkCost: 7335, ComputeSupply: 12098449832962, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157195420, NetworkCost: 7335, ComputeSupply: 12098449309415, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157194858, NetworkCost: 7335, ComputeSupply: 12098448785872, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157194296, NetworkCost: 7335, ComputeSupply: 12098448262333, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157193734, NetworkCost: 7335, ComputeSupply: 12098447738798, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157193172, NetworkCost: 7335, ComputeSupply: 12098447215267, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157192610, NetworkCost: 7335, ComputeSupply: 12098446691741, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157192048, NetworkCost: 7335, ComputeSupply: 12098446168219, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157191486, NetworkCost: 7335, ComputeSupply: 12098445644701, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157190924, NetworkCost: 7335, ComputeSupply: 12098445121187, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157190362, NetworkCost: 7335, ComputeSupply: 12098444597677, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157189800, NetworkCost: 7335, ComputeSupply: 12098444074172, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157189238, NetworkCost: 7335, ComputeSupply: 12098443550671, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157188676, NetworkCost: 7335, ComputeSupply: 12098443027174, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157188114, NetworkCost: 7335, ComputeSupply: 12098442503681, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157187552, NetworkCost: 7335, ComputeSupply: 12098441980192, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157186990, NetworkCost: 7335, ComputeSupply: 12098441456708, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157186428, NetworkCost: 7335, ComputeSupply: 12098440933228, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157185866, NetworkCost: 7335, ComputeSupply: 12098440409752, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157185304, NetworkCost: 7335, ComputeSupply: 12098439886280, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157184742, NetworkCost: 7335, ComputeSupply: 12098439362812, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157184180, NetworkCost: 7335, ComputeSupply: 12098438839349, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157183618, NetworkCost: 7335, ComputeSupply: 12098438315890, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157183056, NetworkCost: 7335, ComputeSupply: 12098437791941, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157182494, NetworkCost: 7335, ComputeSupply: 12098437267996, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157181932, NetworkCost: 7335, ComputeSupply: 12098436744055, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157181370, NetworkCost: 7335, ComputeSupply: 12098436220119, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157180808, NetworkCost: 7335, ComputeSupply: 12098435696187, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157180246, NetworkCost: 7335, ComputeSupply: 12098435172259, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157179684, NetworkCost: 7335, ComputeSupply: 12098434648335, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157179122, NetworkCost: 7335, ComputeSupply: 12098434124415, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157178560, NetworkCost: 7335, ComputeSupply: 12098433600500, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157177998, NetworkCost: 7335, ComputeSupply: 12098433076589, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157177436, NetworkCost: 7335, ComputeSupply: 12098432552682, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157176874, NetworkCost: 7335, ComputeSupply: 12098432028779, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157176312, NetworkCost: 7335, ComputeSupply: 12098431504881, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157175750, NetworkCost: 7335, ComputeSupply: 12098430980987, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157175188, NetworkCost: 7335, ComputeSupply: 12098430457097, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157174626, NetworkCost: 7335, ComputeSupply: 12098429933211, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157174064, NetworkCost: 7335, ComputeSupply: 12098429409329, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157173502, NetworkCost: 7335, ComputeSupply: 12098428885452, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157172940, NetworkCost: 7335, ComputeSupply: 12098428361579, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157172378, NetworkCost: 7335, ComputeSupply: 12098427837710, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157171816, NetworkCost: 7335, ComputeSupply: 12098427313845, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157171254, NetworkCost: 7335, ComputeSupply: 12098426789984, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157170692, NetworkCost: 7335, ComputeSupply: 12098426266128, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157170130, NetworkCost: 7335, ComputeSupply: 12098425742276, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157169568, NetworkCost: 7335, ComputeSupply: 12098425218428, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157169006, NetworkCost: 7335, ComputeSupply: 12098424694584, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157168444, NetworkCost: 7335, ComputeSupply: 12098424170744, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157167882, NetworkCost: 7335, ComputeSupply: 12098423646909, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157167320, NetworkCost: 7335, ComputeSupply: 12098423123078, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157166758, NetworkCost: 7335, ComputeSupply: 12098422599251, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157166196, NetworkCost: 7335, ComputeSupply: 12098422075428, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157165634, NetworkCost: 7335, ComputeSupply: 12098421551609, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157165072, NetworkCost: 7335, ComputeSupply: 12098421027795, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157164510, NetworkCost: 7335, ComputeSupply: 12098420503985, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157163948, NetworkCost: 7335, ComputeSupply: 12098419980179, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157163386, NetworkCost: 7335, ComputeSupply: 12098419456377, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157162824, NetworkCost: 7335, ComputeSupply: 12098418932579, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157162262, NetworkCost: 7335, ComputeSupply: 12098418408786, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157161700, NetworkCost: 7335, ComputeSupply: 12098417884997, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157161138, NetworkCost: 7335, ComputeSupply: 12098417361212, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157160576, NetworkCost: 7335, ComputeSupply: 12098416837431, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157160014, NetworkCost: 7335, ComputeSupply: 12098416313654, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157159452, NetworkCost: 7335, ComputeSupply: 12098415789882, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157158890, NetworkCost: 7335, ComputeSupply: 12098415266114, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157158328, NetworkCost: 7335, ComputeSupply: 12098414742350, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157157766, NetworkCost: 7335, ComputeSupply: 12098414218590, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157157204, NetworkCost: 7335, ComputeSupply: 12098413694834, ComputeCost: 34},
	}

	fmt.Print("testConsumption = [\n")

	for i := 0; i < 100; i++ {
		trx, err := integration.CreateTransaction(client, []*protocol.Operation{transferOp}, aliceKey)
		integration.NoError(t, err)

		receipt, err := integration.CreateBlock(client, []*protocol.Transaction{trx}, genesisKey)
		integration.NoError(t, err)

		fmt.Printf("   [%v, %v, %v],\n", receipt.DiskStorageCharged, receipt.NetworkBandwidthCharged, receipt.ComputeBandwidthCharged)
		markets, err = getMarkets(client, resourceKey.AddressBytes())
		integration.NoError(t, err)

		limitsReq := &chainrpc.GetResourceLimitsRequest{}
		limitsResp := &chainrpc.GetResourceLimitsResponse{}

		limitsCtx, limitsTimeout := context.WithTimeout(context.Background(), time.Second)
		defer limitsTimeout()
		client.Call(limitsCtx, "chain.get_resource_limits", limitsReq, limitsResp)

		require.EqualValues(t, testValues[i].DiskSupply, markets.DiskStorage.ResourceSupply)
		require.EqualValues(t, testValues[i].DiskCost, limitsResp.ResourceLimitData.DiskStorageCost)
		require.EqualValues(t, testValues[i].NetworkSupply, markets.NetworkBandwidth.ResourceSupply)
		require.EqualValues(t, testValues[i].NetworkCost, limitsResp.ResourceLimitData.NetworkBandwidthCost)
		require.EqualValues(t, testValues[i].ComputeSupply, markets.ComputeBandwidth.ResourceSupply)
		require.EqualValues(t, testValues[i].ComputeCost, limitsResp.ResourceLimitData.ComputeBandwidthCost)
	}

	fmt.Print("]\n")
}
