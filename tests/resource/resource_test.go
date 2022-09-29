package resource

import (
	"fmt"
	"koinos-integration-tests/integration"
	"koinos-integration-tests/integration/token"
	"testing"

	"github.com/koinos/koinos-proto-golang/koinos/chain"
	"github.com/koinos/koinos-proto-golang/koinos/contracts/resources"
	token_proto "github.com/koinos/koinos-proto-golang/koinos/contracts/token"
	"github.com/koinos/koinos-proto-golang/koinos/protocol"
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
	require.EqualValues(t, uint64(204800), markets.DiskStorage.BlockLimit)

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
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157212842, NetworkCost: 7335, ComputeSupply: 12098465547692, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157212280, NetworkCost: 7335, ComputeSupply: 12098465030374, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157211718, NetworkCost: 7335, ComputeSupply: 12098464513060, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157211156, NetworkCost: 7335, ComputeSupply: 12098463995750, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157210594, NetworkCost: 7335, ComputeSupply: 12098463478444, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157210032, NetworkCost: 7335, ComputeSupply: 12098462961142, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157209470, NetworkCost: 7335, ComputeSupply: 12098462443844, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157208908, NetworkCost: 7335, ComputeSupply: 12098461926551, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157208346, NetworkCost: 7335, ComputeSupply: 12098461409262, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157207784, NetworkCost: 7335, ComputeSupply: 12098460891977, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157207222, NetworkCost: 7335, ComputeSupply: 12098460374696, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157206660, NetworkCost: 7335, ComputeSupply: 12098459857419, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157206098, NetworkCost: 7335, ComputeSupply: 12098459340146, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157205536, NetworkCost: 7335, ComputeSupply: 12098458822877, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157204974, NetworkCost: 7335, ComputeSupply: 12098458305613, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157204412, NetworkCost: 7335, ComputeSupply: 12098457788353, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157203850, NetworkCost: 7335, ComputeSupply: 12098457271097, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157203288, NetworkCost: 7335, ComputeSupply: 12098456753845, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157202726, NetworkCost: 7335, ComputeSupply: 12098456236597, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157202164, NetworkCost: 7335, ComputeSupply: 12098455719353, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157201602, NetworkCost: 7335, ComputeSupply: 12098455202113, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157201040, NetworkCost: 7335, ComputeSupply: 12098454684878, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157200478, NetworkCost: 7335, ComputeSupply: 12098454167647, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157199916, NetworkCost: 7335, ComputeSupply: 12098453650420, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157199354, NetworkCost: 7335, ComputeSupply: 12098453133197, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157198792, NetworkCost: 7335, ComputeSupply: 12098452615978, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157198230, NetworkCost: 7335, ComputeSupply: 12098452098763, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157197668, NetworkCost: 7335, ComputeSupply: 12098451581553, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157197106, NetworkCost: 7335, ComputeSupply: 12098451064347, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157196544, NetworkCost: 7335, ComputeSupply: 12098450547145, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157195982, NetworkCost: 7335, ComputeSupply: 12098450029947, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157195420, NetworkCost: 7335, ComputeSupply: 12098449512753, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157194858, NetworkCost: 7335, ComputeSupply: 12098448995563, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157194296, NetworkCost: 7335, ComputeSupply: 12098448478377, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157193734, NetworkCost: 7335, ComputeSupply: 12098447961196, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157193172, NetworkCost: 7335, ComputeSupply: 12098447444019, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157192610, NetworkCost: 7335, ComputeSupply: 12098446926846, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157192048, NetworkCost: 7335, ComputeSupply: 12098446409677, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157191486, NetworkCost: 7335, ComputeSupply: 12098445892512, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157190924, NetworkCost: 7335, ComputeSupply: 12098445375351, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157190362, NetworkCost: 7335, ComputeSupply: 12098444858194, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157189800, NetworkCost: 7335, ComputeSupply: 12098444341042, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157189238, NetworkCost: 7335, ComputeSupply: 12098443823894, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157188676, NetworkCost: 7335, ComputeSupply: 12098443306750, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157188114, NetworkCost: 7335, ComputeSupply: 12098442789610, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157187552, NetworkCost: 7335, ComputeSupply: 12098442272474, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157186990, NetworkCost: 7335, ComputeSupply: 12098441755342, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157186428, NetworkCost: 7335, ComputeSupply: 12098441238214, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157185866, NetworkCost: 7335, ComputeSupply: 12098440721091, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157185304, NetworkCost: 7335, ComputeSupply: 12098440203972, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157184742, NetworkCost: 7335, ComputeSupply: 12098439686857, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157184180, NetworkCost: 7335, ComputeSupply: 12098439169746, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157183618, NetworkCost: 7335, ComputeSupply: 12098438652639, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157183056, NetworkCost: 7335, ComputeSupply: 12098438135536, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157182494, NetworkCost: 7335, ComputeSupply: 12098437618438, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157181932, NetworkCost: 7335, ComputeSupply: 12098437101344, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157181370, NetworkCost: 7335, ComputeSupply: 12098436583760, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157180808, NetworkCost: 7335, ComputeSupply: 12098436066180, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157180246, NetworkCost: 7335, ComputeSupply: 12098435548604, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157179684, NetworkCost: 7335, ComputeSupply: 12098435031032, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157179122, NetworkCost: 7335, ComputeSupply: 12098434513464, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157178560, NetworkCost: 7335, ComputeSupply: 12098433995901, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157177998, NetworkCost: 7335, ComputeSupply: 12098433478342, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157177436, NetworkCost: 7335, ComputeSupply: 12098432960787, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157176874, NetworkCost: 7335, ComputeSupply: 12098432443236, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157176312, NetworkCost: 7335, ComputeSupply: 12098431925689, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157175750, NetworkCost: 7335, ComputeSupply: 12098431408146, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157175188, NetworkCost: 7335, ComputeSupply: 12098430890607, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157174626, NetworkCost: 7335, ComputeSupply: 12098430373073, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157174064, NetworkCost: 7335, ComputeSupply: 12098429855543, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157173502, NetworkCost: 7335, ComputeSupply: 12098429338017, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157172940, NetworkCost: 7335, ComputeSupply: 12098428820495, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157172378, NetworkCost: 7335, ComputeSupply: 12098428302977, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157171816, NetworkCost: 7335, ComputeSupply: 12098427785463, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157171254, NetworkCost: 7335, ComputeSupply: 12098427267954, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157170692, NetworkCost: 7335, ComputeSupply: 12098426750449, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157170130, NetworkCost: 7335, ComputeSupply: 12098426232948, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157169568, NetworkCost: 7335, ComputeSupply: 12098425715451, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157169006, NetworkCost: 7335, ComputeSupply: 12098425197958, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157168444, NetworkCost: 7335, ComputeSupply: 12098424680469, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157167882, NetworkCost: 7335, ComputeSupply: 12098424162984, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157167320, NetworkCost: 7335, ComputeSupply: 12098423645504, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157166758, NetworkCost: 7335, ComputeSupply: 12098423128028, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157166196, NetworkCost: 7335, ComputeSupply: 12098422610556, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157165634, NetworkCost: 7335, ComputeSupply: 12098422093088, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157165072, NetworkCost: 7335, ComputeSupply: 12098421575624, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157164510, NetworkCost: 7335, ComputeSupply: 12098421058164, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157163948, NetworkCost: 7335, ComputeSupply: 12098420540709, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157163386, NetworkCost: 7335, ComputeSupply: 12098420023258, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157162824, NetworkCost: 7335, ComputeSupply: 12098419505811, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157162262, NetworkCost: 7335, ComputeSupply: 12098418988368, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157161700, NetworkCost: 7335, ComputeSupply: 12098418470929, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157161138, NetworkCost: 7335, ComputeSupply: 12098417953494, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157160576, NetworkCost: 7335, ComputeSupply: 12098417436063, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157160014, NetworkCost: 7335, ComputeSupply: 12098416918637, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157159452, NetworkCost: 7335, ComputeSupply: 12098416401215, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157158890, NetworkCost: 7335, ComputeSupply: 12098415883797, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157158328, NetworkCost: 7335, ComputeSupply: 12098415366383, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157157766, NetworkCost: 7335, ComputeSupply: 12098414848973, ComputeCost: 34},
		{DiskSupply: 8332061147, DiskCost: 48553, NetworkSupply: 55157157204, NetworkCost: 7335, ComputeSupply: 12098414331567, ComputeCost: 34},
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

		require.EqualValues(t, testValues[i].DiskSupply, markets.DiskStorage.ResourceSupply)
		require.EqualValues(t, testValues[i].NetworkSupply, markets.NetworkBandwidth.ResourceSupply)
		require.EqualValues(t, testValues[i].ComputeSupply, markets.ComputeBandwidth.ResourceSupply)
	}

	fmt.Print("]\n")
}
