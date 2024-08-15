package resource

import (
	"context"
	"fmt"
	"koinos-integration-tests/integration"
	"koinos-integration-tests/integration/token"
	"testing"
	"time"

	"github.com/koinos/koinos-proto-golang/v2/koinos/chain"
	"github.com/koinos/koinos-proto-golang/v2/koinos/contracts/resources"
	token_proto "github.com/koinos/koinos-proto-golang/v2/koinos/contracts/token"
	"github.com/koinos/koinos-proto-golang/v2/koinos/protocol"
	chainrpc "github.com/koinos/koinos-proto-golang/v2/koinos/rpc/chain"
	util "github.com/koinos/koinos-util-golang/v2"
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
	client := integration.NewKoinosMQClient("amqp://guest:guest@localhost:25673/")

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
	integration.InitGetContractMetadata(t, client)

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
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157212842, NetworkCost: 7335, ComputeSupply: 12098465551039, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157212280, NetworkCost: 7335, ComputeSupply: 12098465034584, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157211718, NetworkCost: 7335, ComputeSupply: 12098464518133, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157211156, NetworkCost: 7335, ComputeSupply: 12098464001686, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157210594, NetworkCost: 7335, ComputeSupply: 12098463485243, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157210032, NetworkCost: 7335, ComputeSupply: 12098462968804, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157209470, NetworkCost: 7335, ComputeSupply: 12098462452369, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157208908, NetworkCost: 7335, ComputeSupply: 12098461935938, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157208346, NetworkCost: 7335, ComputeSupply: 12098461419512, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157207784, NetworkCost: 7335, ComputeSupply: 12098460903090, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157207222, NetworkCost: 7335, ComputeSupply: 12098460386672, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157206660, NetworkCost: 7335, ComputeSupply: 12098459870258, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157206098, NetworkCost: 7335, ComputeSupply: 12098459353848, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157205536, NetworkCost: 7335, ComputeSupply: 12098458837442, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157204974, NetworkCost: 7335, ComputeSupply: 12098458321040, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157204412, NetworkCost: 7335, ComputeSupply: 12098457804643, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157203850, NetworkCost: 7335, ComputeSupply: 12098457288250, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157203288, NetworkCost: 7335, ComputeSupply: 12098456771861, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157202726, NetworkCost: 7335, ComputeSupply: 12098456255476, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157202164, NetworkCost: 7335, ComputeSupply: 12098455739095, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157201602, NetworkCost: 7335, ComputeSupply: 12098455222718, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157201040, NetworkCost: 7335, ComputeSupply: 12098454706345, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157200478, NetworkCost: 7335, ComputeSupply: 12098454189977, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157199916, NetworkCost: 7335, ComputeSupply: 12098453673613, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157199354, NetworkCost: 7335, ComputeSupply: 12098453157253, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157198792, NetworkCost: 7335, ComputeSupply: 12098452640897, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157198230, NetworkCost: 7335, ComputeSupply: 12098452124545, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157197668, NetworkCost: 7335, ComputeSupply: 12098451608197, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157197106, NetworkCost: 7335, ComputeSupply: 12098451091853, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157196544, NetworkCost: 7335, ComputeSupply: 12098450575514, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157195982, NetworkCost: 7335, ComputeSupply: 12098450059179, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157195420, NetworkCost: 7335, ComputeSupply: 12098449542848, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157194858, NetworkCost: 7335, ComputeSupply: 12098449026521, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157194296, NetworkCost: 7335, ComputeSupply: 12098448510198, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157193734, NetworkCost: 7335, ComputeSupply: 12098447993879, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157193172, NetworkCost: 7335, ComputeSupply: 12098447477564, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157192610, NetworkCost: 7335, ComputeSupply: 12098446961254, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157192048, NetworkCost: 7335, ComputeSupply: 12098446444948, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157191486, NetworkCost: 7335, ComputeSupply: 12098445928646, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157190924, NetworkCost: 7335, ComputeSupply: 12098445412348, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157190362, NetworkCost: 7335, ComputeSupply: 12098444896054, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157189800, NetworkCost: 7335, ComputeSupply: 12098444379764, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157189238, NetworkCost: 7335, ComputeSupply: 12098443863478, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157188676, NetworkCost: 7335, ComputeSupply: 12098443347197, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157188114, NetworkCost: 7335, ComputeSupply: 12098442830920, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157187552, NetworkCost: 7335, ComputeSupply: 12098442314647, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157186990, NetworkCost: 7335, ComputeSupply: 12098441798378, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157186428, NetworkCost: 7335, ComputeSupply: 12098441282113, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157185866, NetworkCost: 7335, ComputeSupply: 12098440765852, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157185304, NetworkCost: 7335, ComputeSupply: 12098440249595, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157184742, NetworkCost: 7335, ComputeSupply: 12098439733343, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157184180, NetworkCost: 7335, ComputeSupply: 12098439216563, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157183618, NetworkCost: 7335, ComputeSupply: 12098438699787, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157183056, NetworkCost: 7335, ComputeSupply: 12098438183015, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157182494, NetworkCost: 7335, ComputeSupply: 12098437666247, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157181932, NetworkCost: 7335, ComputeSupply: 12098437149483, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157181370, NetworkCost: 7335, ComputeSupply: 12098436632723, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157180808, NetworkCost: 7335, ComputeSupply: 12098436115968, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157180246, NetworkCost: 7335, ComputeSupply: 12098435599217, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157179684, NetworkCost: 7335, ComputeSupply: 12098435082470, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157179122, NetworkCost: 7335, ComputeSupply: 12098434565727, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157178560, NetworkCost: 7335, ComputeSupply: 12098434048988, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157177998, NetworkCost: 7335, ComputeSupply: 12098433532253, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157177436, NetworkCost: 7335, ComputeSupply: 12098433015522, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157176874, NetworkCost: 7335, ComputeSupply: 12098432498796, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157176312, NetworkCost: 7335, ComputeSupply: 12098431982074, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157175750, NetworkCost: 7335, ComputeSupply: 12098431465356, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157175188, NetworkCost: 7335, ComputeSupply: 12098430948642, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157174626, NetworkCost: 7335, ComputeSupply: 12098430431932, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157174064, NetworkCost: 7335, ComputeSupply: 12098429915226, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157173502, NetworkCost: 7335, ComputeSupply: 12098429398524, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157172940, NetworkCost: 7335, ComputeSupply: 12098428881827, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157172378, NetworkCost: 7335, ComputeSupply: 12098428365134, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157171816, NetworkCost: 7335, ComputeSupply: 12098427848445, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157171254, NetworkCost: 7335, ComputeSupply: 12098427331760, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157170692, NetworkCost: 7335, ComputeSupply: 12098426815079, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157170130, NetworkCost: 7335, ComputeSupply: 12098426298402, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157169568, NetworkCost: 7335, ComputeSupply: 12098425781729, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157169006, NetworkCost: 7335, ComputeSupply: 12098425265061, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157168444, NetworkCost: 7335, ComputeSupply: 12098424748397, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157167882, NetworkCost: 7335, ComputeSupply: 12098424231737, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157167320, NetworkCost: 7335, ComputeSupply: 12098423715081, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157166758, NetworkCost: 7335, ComputeSupply: 12098423198429, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157166196, NetworkCost: 7335, ComputeSupply: 12098422681781, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157165634, NetworkCost: 7335, ComputeSupply: 12098422165138, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157165072, NetworkCost: 7335, ComputeSupply: 12098421648499, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157164510, NetworkCost: 7335, ComputeSupply: 12098421131864, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157163948, NetworkCost: 7335, ComputeSupply: 12098420615233, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157163386, NetworkCost: 7335, ComputeSupply: 12098420098606, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157162824, NetworkCost: 7335, ComputeSupply: 12098419581983, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157162262, NetworkCost: 7335, ComputeSupply: 12098419065364, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157161700, NetworkCost: 7335, ComputeSupply: 12098418548750, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157161138, NetworkCost: 7335, ComputeSupply: 12098418032140, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157160576, NetworkCost: 7335, ComputeSupply: 12098417515534, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157160014, NetworkCost: 7335, ComputeSupply: 12098416998932, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157159452, NetworkCost: 7335, ComputeSupply: 12098416482334, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157158890, NetworkCost: 7335, ComputeSupply: 12098415965740, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157158328, NetworkCost: 7335, ComputeSupply: 12098415449150, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157157766, NetworkCost: 7335, ComputeSupply: 12098414932565, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48555, NetworkSupply: 55157157204, NetworkCost: 7335, ComputeSupply: 12098414415984, ComputeCost: 34},
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
