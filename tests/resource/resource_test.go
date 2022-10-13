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

	t.Logf("Uploading KOIN contract")
	err = integration.UploadSystemContract(client, "../../contracts/koin.wasm", koinKey)
	integration.NoError(t, err)

	t.Logf("Uploading Resource contract")
	err = integration.UploadSystemContract(client, "../../contracts/resources.wasm", resourceKey)

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
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157212842, NetworkCost: 7335, ComputeSupply: 12098465545967, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157212280, NetworkCost: 7335, ComputeSupply: 12098465026924, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157211718, NetworkCost: 7335, ComputeSupply: 12098464507885, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157211156, NetworkCost: 7335, ComputeSupply: 12098463988850, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157210594, NetworkCost: 7335, ComputeSupply: 12098463469819, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157210032, NetworkCost: 7335, ComputeSupply: 12098462950792, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157209470, NetworkCost: 7335, ComputeSupply: 12098462431769, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157208908, NetworkCost: 7335, ComputeSupply: 12098461912751, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157208346, NetworkCost: 7335, ComputeSupply: 12098461393737, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157207784, NetworkCost: 7335, ComputeSupply: 12098460874727, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157207222, NetworkCost: 7335, ComputeSupply: 12098460355721, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157206660, NetworkCost: 7335, ComputeSupply: 12098459836719, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157206098, NetworkCost: 7335, ComputeSupply: 12098459317721, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157205536, NetworkCost: 7335, ComputeSupply: 12098458798728, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157204974, NetworkCost: 7335, ComputeSupply: 12098458279739, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157204412, NetworkCost: 7335, ComputeSupply: 12098457760754, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157203850, NetworkCost: 7335, ComputeSupply: 12098457241773, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157203288, NetworkCost: 7335, ComputeSupply: 12098456722796, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157202726, NetworkCost: 7335, ComputeSupply: 12098456203823, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157202164, NetworkCost: 7335, ComputeSupply: 12098455684855, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157201602, NetworkCost: 7335, ComputeSupply: 12098455165891, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157201040, NetworkCost: 7335, ComputeSupply: 12098454646931, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157200478, NetworkCost: 7335, ComputeSupply: 12098454127975, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157199916, NetworkCost: 7335, ComputeSupply: 12098453609023, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157199354, NetworkCost: 7335, ComputeSupply: 12098453090075, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157198792, NetworkCost: 7335, ComputeSupply: 12098452571132, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157198230, NetworkCost: 7335, ComputeSupply: 12098452052193, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157197668, NetworkCost: 7335, ComputeSupply: 12098451533258, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157197106, NetworkCost: 7335, ComputeSupply: 12098451014327, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157196544, NetworkCost: 7335, ComputeSupply: 12098450495400, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157195982, NetworkCost: 7335, ComputeSupply: 12098449976477, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157195420, NetworkCost: 7335, ComputeSupply: 12098449457559, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157194858, NetworkCost: 7335, ComputeSupply: 12098448938645, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157194296, NetworkCost: 7335, ComputeSupply: 12098448419735, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157193734, NetworkCost: 7335, ComputeSupply: 12098447900829, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157193172, NetworkCost: 7335, ComputeSupply: 12098447381927, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157192610, NetworkCost: 7335, ComputeSupply: 12098446863029, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157192048, NetworkCost: 7335, ComputeSupply: 12098446344136, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157191486, NetworkCost: 7335, ComputeSupply: 12098445825247, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157190924, NetworkCost: 7335, ComputeSupply: 12098445306362, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157190362, NetworkCost: 7335, ComputeSupply: 12098444787481, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157189800, NetworkCost: 7335, ComputeSupply: 12098444268604, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157189238, NetworkCost: 7335, ComputeSupply: 12098443749731, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157188676, NetworkCost: 7335, ComputeSupply: 12098443230862, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157188114, NetworkCost: 7335, ComputeSupply: 12098442711998, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157187552, NetworkCost: 7335, ComputeSupply: 12098442193138, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157186990, NetworkCost: 7335, ComputeSupply: 12098441674282, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157186428, NetworkCost: 7335, ComputeSupply: 12098441155430, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157185866, NetworkCost: 7335, ComputeSupply: 12098440636582, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157185304, NetworkCost: 7335, ComputeSupply: 12098440117738, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157184742, NetworkCost: 7335, ComputeSupply: 12098439598899, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157184180, NetworkCost: 7335, ComputeSupply: 12098439080064, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157183618, NetworkCost: 7335, ComputeSupply: 12098438561233, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157183056, NetworkCost: 7335, ComputeSupply: 12098438042406, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157182494, NetworkCost: 7335, ComputeSupply: 12098437523583, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157181932, NetworkCost: 7335, ComputeSupply: 12098437004764, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157181370, NetworkCost: 7335, ComputeSupply: 12098436485456, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157180808, NetworkCost: 7335, ComputeSupply: 12098435966152, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157180246, NetworkCost: 7335, ComputeSupply: 12098435446852, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157179684, NetworkCost: 7335, ComputeSupply: 12098434927556, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157179122, NetworkCost: 7335, ComputeSupply: 12098434408264, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157178560, NetworkCost: 7335, ComputeSupply: 12098433888976, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157177998, NetworkCost: 7335, ComputeSupply: 12098433369693, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157177436, NetworkCost: 7335, ComputeSupply: 12098432850414, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157176874, NetworkCost: 7335, ComputeSupply: 12098432331139, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157176312, NetworkCost: 7335, ComputeSupply: 12098431811868, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157175750, NetworkCost: 7335, ComputeSupply: 12098431292601, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157175188, NetworkCost: 7335, ComputeSupply: 12098430773338, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157174626, NetworkCost: 7335, ComputeSupply: 12098430254080, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157174064, NetworkCost: 7335, ComputeSupply: 12098429734826, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157173502, NetworkCost: 7335, ComputeSupply: 12098429215576, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157172940, NetworkCost: 7335, ComputeSupply: 12098428696330, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157172378, NetworkCost: 7335, ComputeSupply: 12098428177088, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157171816, NetworkCost: 7335, ComputeSupply: 12098427657850, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157171254, NetworkCost: 7335, ComputeSupply: 12098427138617, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157170692, NetworkCost: 7335, ComputeSupply: 12098426619388, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157170130, NetworkCost: 7335, ComputeSupply: 12098426100163, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157169568, NetworkCost: 7335, ComputeSupply: 12098425580942, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157169006, NetworkCost: 7335, ComputeSupply: 12098425061725, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157168444, NetworkCost: 7335, ComputeSupply: 12098424542512, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157167882, NetworkCost: 7335, ComputeSupply: 12098424023304, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157167320, NetworkCost: 7335, ComputeSupply: 12098423504100, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157166758, NetworkCost: 7335, ComputeSupply: 12098422984900, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157166196, NetworkCost: 7335, ComputeSupply: 12098422465704, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157165634, NetworkCost: 7335, ComputeSupply: 12098421946512, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157165072, NetworkCost: 7335, ComputeSupply: 12098421427324, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157164510, NetworkCost: 7335, ComputeSupply: 12098420908141, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157163948, NetworkCost: 7335, ComputeSupply: 12098420388962, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157163386, NetworkCost: 7335, ComputeSupply: 12098419869787, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157162824, NetworkCost: 7335, ComputeSupply: 12098419350616, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157162262, NetworkCost: 7335, ComputeSupply: 12098418831449, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157161700, NetworkCost: 7335, ComputeSupply: 12098418312286, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157161138, NetworkCost: 7335, ComputeSupply: 12098417793128, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157160576, NetworkCost: 7335, ComputeSupply: 12098417273974, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157160014, NetworkCost: 7335, ComputeSupply: 12098416754824, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157159452, NetworkCost: 7335, ComputeSupply: 12098416235678, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157158890, NetworkCost: 7335, ComputeSupply: 12098415716536, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157158328, NetworkCost: 7335, ComputeSupply: 12098415197398, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157157766, NetworkCost: 7335, ComputeSupply: 12098414678265, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157157204, NetworkCost: 7335, ComputeSupply: 12098414159136, ComputeCost: 34},
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
