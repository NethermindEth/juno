// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package contracts

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
)

// CpuOodsMetaData contains all meta data concerning the CpuOods contract.
var CpuOodsMetaData = &bind.MetaData{
	ABI: "[{\"stateMutability\":\"nonpayable\",\"type\":\"fallback\"}]",
	Bin: "0x608060405234801561001057600080fd5b506060602060016000350102604051915080820160405280600083375060008160098151811061003c57fe5b6020026020010151905060606051600201826002020267ffffffffffffffff8111801561006857600080fd5b50604051908082528060200260200182016040528015610092578160200160208202803683370190505b50905061009f838261198b565b60007e4000000000000110000000000001210000000000000000000000000000000090507f080000000000001100000000000000000000000000000000000000000000000184610dc08101606086028101617ae08301620110e08401602088015b8385101561197c5760008789855109886142208901518a0382018a6161808b01518651090982019150886142408901518a0382018a6161a08b01516020870151090982019150886142608901518a0382018a6161c08b01516080870151090982019150886142808901518a0382018a6161e08b0151610100870151090982019150886142a08901518a0382018a6162008b0151610180870151090982019150886142c08901518a0382018a6162208b01516102e0870151090982019150886142e08901518a0382018a6162408b0151610380870151090982019150886143008901518a0382018a6162608b01516103c0870151090982019150886143208901518a0382018a6162808b0151610440870151090982019150886143408901518a0382018a6162a08b0151610480870151090982019150886143608901518a0382018a6162c08b01516104e0870151090982019150886143808901518a0382018a6162e08b0151610500870151090982019150508789602086015109886143a08901518a0382018a6163008b01518651090982019150886143c08901518a0382018a6163208b01516020870151090982019150886143e08901518a0382018a6163408b01516040870151090982019150886144008901518a0382018a6163608b01516060870151090982019150886144208901518a0382018a6163808b01516080870151090982019150886144408901518a0382018a6163a08b015160a0870151090982019150886144608901518a0382018a6163c08b015160c0870151090982019150886144808901518a0382018a6163e08b015160e0870151090982019150886144a08901518a0382018a6164008b0151610100870151090982019150886144c08901518a0382018a6164208b0151610120870151090982019150886144e08901518a0382018a6164408b0151610140870151090982019150886145008901518a0382018a6164608b0151610160870151090982019150886145208901518a0382018a6164808b0151610180870151090982019150886145408901518a0382018a6164a08b01516101a0870151090982019150886145608901518a0382018a6164c08b01516101c0870151090982019150886145808901518a0382018a6164e08b01516101e0870151090982019150508789604086015109886145a08901518a0382018a6165008b01518651090982019150886145c08901518a0382018a6165208b0151602087015109098201915050878960608601510988896145e08a01518b0383018b6165408c01518751090983089150886146008901518a0382018a6165608b01516020870151090982019150886146208901518a0382018a6165808b01516106c0870151090982019150886146408901518a0382018a6165a08b01516106e0870151090982019150886146608901518a0382018a6165c08b01516107c0870151090982019150508789608086015109886146808901518a0382018a6165e08b01518651090982019150886146a08901518a0382018a6166008b01516020870151090982019150886146c08901518a0382018a6166208b01516106c0870151090982019150886146e08901518a0382018a6166408b01516106e087015109098201915050878960a086015109886147008901518a0382018a6166608b01518651090982019150886147208901518a0382018a6166808b01516106c087015109098201915050878960c086015109886147408901518a0382018a6166a08b01518651090982019150886147608901518a0382018a6166c08b01516020870151090982019150886147808901518a0382018a6166e08b01516105a0870151090982019150886147a08901518a0382018a6167008b01516105c0870151090982019150886147c08901518a0382018a6167208b01516105e0870151090982019150886147e08901518a0382018a6167408b0151610600870151090982019150886148008901518a0382018a6167608b0151610680870151090982019150886148208901518a0382018a6167808b01516106a0870151090982019150886148408901518a0382018a6167a08b01516106e087015109098201915050878960e086015109886148608901518a0382018a6167c08b01518651090982019150886148808901518a0382018a6167e08b01516020870151090982019150886148a08901518a0382018a6168008b01516106c0870151090982019150886148c08901518a0382018a6168208b01516106e0870151090982019150886148e08901518a0382018a6168408b01516107c087015109098201915050878961010086015109886149008901518a0382018a6168608b01518651090982019150886149208901518a0382018a6168808b01516020870151090982019150886149408901518a0382018a6168a08b01516106c0870151090982019150886149608901518a0382018a6168c08b01516106e087015109098201915050878961012086015109886149808901518a0382018a6168e08b01518651090982019150886149a08901518a0382018a6169008b01516106c08701510909820191505087896101408601510988896149c08a01518b0383018b6169208c01518751090983089150886149e08901518a0382018a6169408b0151602087015109098201915088614a008901518a0382018a6169608b01516105a087015109098201915088614a208901518a0382018a6169808b01516105c087015109098201915088614a408901518a0382018a6169a08b01516105e087015109098201915088614a608901518a0382018a6169c08b015161060087015109098201915088614a808901518a0382018a6169e08b015161068087015109098201915088614aa08901518a0382018a616a008b01516106a087015109098201915088614ac08901518a0382018a616a208b01516106e08701510909820191505087896101608601510988614ae08901518a0382018a616a408b0151865109098201915088614b008901518a0382018a616a608b0151602087015109098201915088614b208901518a0382018a616a808b01516106c087015109098201915088614b408901518a0382018a616aa08b01516106e087015109098201915088614b608901518a0382018a616ac08b01516107c08701510909820191505087896101808601510988614b808901518a0382018a616ae08b0151865109098201915088614ba08901518a0382018a616b008b0151602087015109098201915088614bc08901518a0382018a616b208b01516106c087015109098201915088614be08901518a0382018a616b408b01516106e08701510909820191505087896101a08601510988614c008901518a0382018a616b608b0151865109098201915088614c208901518a0382018a616b808b01516106c08701510909820191505087896101c08601510988614c408901518a0382018a616ba08b0151865109098201915088614c608901518a0382018a616bc08b0151602087015109098201915088614c808901518a0382018a616be08b01516105a087015109098201915088614ca08901518a0382018a616c008b01516105c087015109098201915088614cc08901518a0382018a616c208b01516105e087015109098201915088614ce08901518a0382018a616c408b015161060087015109098201915088614d008901518a0382018a616c608b015161068087015109098201915088614d208901518a0382018a616c808b01516106a087015109098201915088614d408901518a0382018a616ca08b01516106e08701510909820191505087896101e08601510988614d608901518a0382018a616cc08b0151865109098201915088614d808901518a0382018a616ce08b015160208701510909820191508889614da08a01518b0383018b616d008c01516106c088015109098308915088614dc08901518a0382018a616d208b01516106e087015109098201915088614de08901518a0382018a616d408b01516107c08701510909820191505087896102008601510988614e008901518a0382018a616d608b0151865109098201915088614e208901518a0382018a616d808b0151602087015109098201915088614e408901518a0382018a616da08b01516106c087015109098201915088614e608901518a0382018a616dc08b01516106e08701510909820191505087896102208601510988614e808901518a0382018a616de08b0151865109098201915088614ea08901518a0382018a616e008b01516106c08701510909820191505087896102408601510988614ec08901518a0382018a616e208b0151865109098201915088614ee08901518a0382018a616e408b0151602087015109098201915088614f008901518a0382018a616e608b01516105a087015109098201915088614f208901518a0382018a616e808b01516105c087015109098201915088614f408901518a0382018a616ea08b01516105e087015109098201915088614f608901518a0382018a616ec08b015161060087015109098201915088614f808901518a0382018a616ee08b015161068087015109098201915088614fa08901518a0382018a616f008b01516106a087015109098201915088614fc08901518a0382018a616f208b01516106e08701510909820191505087896102608601510988614fe08901518a0382018a616f408b01518651090982019150886150008901518a0382018a616f608b01516020870151090982019150886150208901518a0382018a616f808b01516040870151090982019150886150408901518a0382018a616fa08b01516060870151090982019150886150608901518a0382018a616fc08b01516080870151090982019150886150808901518a0382018a616fe08b015160a0870151090982019150886150a08901518a0382018a6170008b015160c0870151090982019150886150c08901518a0382018a6170208b015160e0870151090982019150886150e08901518a0382018a6170408b0151610100870151090982019150886151008901518a0382018a6170608b0151610120870151090982019150886151208901518a0382018a6170808b0151610180870151090982019150886151408901518a0382018a6170a08b01516101a0870151090982019150886151608901518a0382018a6170c08b015161020087015109098201915088896151808a01518b0383018b6170e08c0151610260880151090983089150886151a08901518a0382018a6171008b0151610280870151090982019150886151c08901518a0382018a6171208b0151610340870151090982019150886151e08901518a0382018a6171408b0151610360870151090982019150886152008901518a0382018a6171608b0151610400870151090982019150886152208901518a0382018a6171808b0151610420870151090982019150886152408901518a0382018a6171a08b01516104a0870151090982019150886152608901518a0382018a6171c08b01516104c0870151090982019150886152808901518a0382018a6171e08b0151610520870151090982019150886152a08901518a0382018a6172008b0151610540870151090982019150886152c08901518a0382018a6172208b0151610580870151090982019150886152e08901518a0382018a6172408b0151610620870151090982019150886153008901518a0382018a6172608b0151610660870151090982019150886153208901518a0382018a6172808b0151610700870151090982019150886153408901518a0382018a6172a08b0151610720870151090982019150886153608901518a0382018a6172c08b0151610740870151090982019150886153808901518a0382018a6172e08b0151610760870151090982019150886153a08901518a0382018a6173008b0151610780870151090982019150886153c08901518a0382018a6173208b01516107a0870151090982019150886153e08901518a0382018a6173408b01516108c0870151090982019150886154008901518a0382018a6173608b01516108e0870151090982019150886154208901518a0382018a6173808b0151610a0087015109098201915050878961028086015109886154408901518a0382018a6173a08b01518651090982019150886154608901518a0382018a6173c08b01516020870151090982019150886154808901518a0382018a6173e08b01516040870151090982019150886154a08901518a0382018a6174008b015160608701510909820191505087896102a086015109886154c08901518a0382018a6174208b01518651090982019150886154e08901518a0382018a6174408b01516020870151090982019150886155008901518a0382018a6174608b01516040870151090982019150886155208901518a0382018a6174808b01516060870151090982019150886155408901518a0382018a6174a08b0151608087015109098201915088896155608a01518b0383018b6174c08c015160a0880151090983089150886155808901518a0382018a6174e08b015160c0870151090982019150886155a08901518a0382018a6175008b015160e0870151090982019150886155c08901518a0382018a6175208b0151610100870151090982019150886155e08901518a0382018a6175408b0151610120870151090982019150886156008901518a0382018a6175608b0151610140870151090982019150886156208901518a0382018a6175808b0151610160870151090982019150886156408901518a0382018a6175a08b0151610180870151090982019150886156608901518a0382018a6175c08b01516101a0870151090982019150886156808901518a0382018a6175e08b01516101c0870151090982019150886156a08901518a0382018a6176008b01516101e0870151090982019150886156c08901518a0382018a6176208b0151610200870151090982019150886156e08901518a0382018a6176408b0151610220870151090982019150886157008901518a0382018a6176608b0151610240870151090982019150886157208901518a0382018a6176808b0151610260870151090982019150886157408901518a0382018a6176a08b0151610280870151090982019150886157608901518a0382018a6176c08b01516102a0870151090982019150886157808901518a0382018a6176e08b01516102c0870151090982019150886157a08901518a0382018a6177008b0151610300870151090982019150886157c08901518a0382018a6177208b0151610320870151090982019150886157e08901518a0382018a6177408b0151610360870151090982019150886158008901518a0382018a6177608b01516103a0870151090982019150886158208901518a0382018a6177808b01516103e0870151090982019150886158408901518a0382018a6177a08b01516107e0870151090982019150886158608901518a0382018a6177c08b0151610800870151090982019150886158808901518a0382018a6177e08b0151610820870151090982019150886158a08901518a0382018a6178008b0151610840870151090982019150886158c08901518a0382018a6178208b0151610860870151090982019150886158e08901518a0382018a6178408b0151610880870151090982019150886159008901518a0382018a6178608b01516108a0870151090982019150886159208901518a0382018a6178808b015161092087015109098201915088896159408a01518b0383018b6178a08c0151610940880151090983089150886159608901518a0382018a6178c08b0151610960870151090982019150886159808901518a0382018a6178e08b0151610980870151090982019150886159a08901518a0382018a6179008b01516109a0870151090982019150886159c08901518a0382018a6179208b01516109c0870151090982019150886159e08901518a0382018a6179408b01516109e08701510909820191505087896102c08601510988615a008901518a0382018a6179608b0151865109098201915088615a208901518a0382018a6179808b015161020087015109098201915088615a408901518a0382018a6179a08b015161046087015109098201915088615a608901518a0382018a6179c08b015161056087015109098201915088615a808901518a0382018a6179e08b015161064087015109098201915088615aa08901518a0382018a617a008b01516109008701510909820191505087896102e08601510988615ac08901518a0382018a617a208b0151865109098201915088615ae08901518a0382018a617a408b015160208701510909820191505087896103008601510988615b008901518a0382018a617a608b0151865109098201915088615b208901518a0382018a617a808b015160408701510909820191505061032084019350878984510988615b408901518a0382018a617aa08b0151610a2087015109098201915050878960208501510988615b608901518a0382018a617ac08b0151610a208701510909909101889006602087015250610a408101516040868101919091526060909501949190910190610a6001610100565b5050505050611200610dc08201f35b6003611995612b19565b6119e0565b600060405160208152602080820152602060408201528260608201528360808201528460a082015260208160c0836005600019fa6119d757600080fd5b51949350505050565b612b208401517f080000000000001100000000000000000000000000000000000000000000000180828309835280828451096020840152808260208501510960408401528082604085015109606084015280826060850151096080840152808260808501510960a0840152808260a08501510960c0840152808260c08501510960e0840152808260e08501510961010084015280826101008501510961012084015280826101208501510961014084018190526040840151829109610160840181905260608401518291096101808401528082610180850151096101a084015280826101a0850151096101c0840181905283518291096101e0840181905260a08401518291096102008401528081836101c08601510961020085015109610220840181905260c0840151829109610240840152611b2081610df28461199a565b610260840152611b3381610fc98461199a565b610280840152612b40860151808203806102a0860152828482099050806102c0860152828482099050806102e08601528284820990508061030086015282848209905080610320860152828482099050806103408601528284820990508061036086015282848209905080610380860152828482099050806103a0860152828482099050806103c0860152828482099050806103e08601528284820990508061040086015282848209905080610420860152828482099050806104408601528284820990508061046086015282848209905080610480860152828482099050806104a086015282602086015182099050806104c086015282855182099050806104e086015282848209905080610500860152828482099050806105208601528284820990508061054086015282848209905080610560860152826020860151820990508061058086015282855182099050806105a0860152828482099050806105c08601528260a086015182099050806105e0860152828482099050806106008601528260608601518209905080610620860152826101208601518209905080610640860152826060860151820990508061066086015282602086015182099050806106808601528260a086015182099050806106a0860152828482099050806106c086015282606086015182099050806106e086015282604086015182099050806107008601528261014086015182099050806107208601528261010086015182099050806107408601528284820990508061076086015282606086015182099050806107808601528261016086015182099050806107a08601528261010086015182099050806107c0860152828482099050806107e08601528260e08601518209905080610800860152826101c08601518209905080610820860152826101e08601518209905080610840860152828482099050806108608601528260208601518209905080610880860152828482099050806108a086015282855182099050806108c08601528260e086015182099050806108e0860152826101a08601518209905080610900860152826101808601518209905080610920860152828482099050806109408601528260208601518209905080610960860152828482099050806109808601528260a086015182099050806109a08601528261020086015182099050806109c08601528261020086015182099050806109e0860152826102408601518209905080610a00860152826102008601518209905080610a20860152826102008601518209905080610a40860152826102208601518209905080610a60860152826102608601518209905080610a808601528260408601518209905080610aa08601528260408601518209905080610ac08601528285518209905080610ae08601528285518209905080610b008601528260e08601518209905080610b208601528260c08601518209905080610b408601528260c08601518209905080610b6086015282848209905080610b80860152826102808601518209905080610ba08601528260a08601518209905080610bc08601528260c08601518209905080610be08601528260808601518209905080610c008601528285518209905080610c208601528285518209905080610c408601528285518209905080610c608601528285518209905080610c80860152826101e08601518209905080610ca086015250615b80870192506020610140880151028301602087016010885102808201600186868709870395505b848810156129ae578751878b82096102a08b01518101838752808552898185099350506102c08b01518101836020880152806020860152898185099350506102e08b01518101836040880152806040860152898185099350506103008b01518101836060880152806060860152898185099350506103208b01518101836080880152806080860152898185099350506103408b015181018360a08801528060a0860152898185099350506103608b015181018360c08801528060c0860152898185099350506103808b015181018360e08801528060e0860152898185099350506103a08b015181018361010088015280610100860152898185099350506103c08b015181018361012088015280610120860152898185099350506103e08b015181018361014088015280610140860152898185099350506104008b015181018361016088015280610160860152898185099350506104208b015181018361018088015280610180860152898185099350506104408b01518101836101a0880152806101a0860152898185099350506104608b01518101836101c0880152806101c0860152898185099350506104808b01518101836101e0880152806101e0860152898185099350506104a08b015181018361020088015280610200860152898185099350506104c08b015181018361022088015280610220860152898185099350506104e08b015181018361024088015280610240860152898185099350506105008b015181018361026088015280610260860152898185099350506105208b015181018361028088015280610280860152898185099350506105408b01518101836102a0880152806102a0860152898185099350506105608b01518101836102c0880152806102c0860152898185099350506105808b01518101836102e0880152806102e0860152898185099350506105a08b015181018361030088015280610300860152898185099350506105c08b015181018361032088015280610320860152898185099350506105e08b015181018361034088015280610340860152898185099350506106008b015181018361036088015280610360860152898185099350506106208b015181018361038088015280610380860152898185099350506106408b01518101836103a0880152806103a0860152898185099350506106608b01518101836103c0880152806103c0860152898185099350506106808b01518101836103e0880152806103e0860152898185099350506106a08b015181018361040088015280610400860152898185099350506106c08b015181018361042088015280610420860152898185099350506106e08b015181018361044088015280610440860152898185099350506107008b015181018361046088015280610460860152898185099350506107208b015181018361048088015280610480860152898185099350506107408b01518101836104a0880152806104a0860152898185099350506107608b01518101836104c0880152806104c0860152898185099350506107808b01518101836104e0880152806104e0860152898185099350506107a08b015181018361050088015280610500860152898185099350506107c08b015181018361052088015280610520860152898185099350506107e08b015181018361054088015280610540860152898185099350506108008b015181018361056088015280610560860152898185099350506108208b015181018361058088015280610580860152898185099350506108408b01518101836105a0880152806105a0860152898185099350506108608b01518101836105c0880152806105c0860152898185099350506108808b01518101836105e0880152806105e0860152898185099350506108a08b015181018361060088015280610600860152898185099350506108c08b015181018361062088015280610620860152898185099350506108e08b015181018361064088015280610640860152898185099350506109008b015181018361066088015280610660860152898185099350506109208b015181018361068088015280610680860152898185099350506109408b01518101836106a0880152806106a0860152898185099350506109608b01518101836106c0880152806106c0860152898185099350506109808b01518101836106e0880152806106e0860152898185099350506109a08b015181018361070088015280610700860152898185099350506109c08b015181018361072088015280610720860152898185099350506109e08b01518101836107408801528061074086015289818509935050610a008b01518101836107608801528061076086015289818509935050610a208b01518101836107808801528061078086015289818509935050610a408b01518101836107a0880152806107a086015289818509935050610a608b01518101836107c0880152806107c086015289818509935050610a808b01518101836107e0880152806107e086015289818509935050610aa08b01518101836108008801528061080086015289818509935050610ac08b01518101836108208801528061082086015289818509935050610ae08b01518101836108408801528061084086015289818509935050610b008b01518101836108608801528061086086015289818509935050610b208b01518101836108808801528061088086015289818509935050610b408b01518101836108a0880152806108a086015289818509935050610b608b01518101836108c0880152806108c086015289818509935050610b808b01518101836108e0880152806108e086015289818509935050610ba08b01518101836109008801528061090086015289818509935050610bc08b01518101836109208801528061092086015289818509935050610be08b01518101836109408801528061094086015289818509935050610c008b01518101836109608801528061096086015289818509935050610c208b01518101836109808801528061098086015289818509935050610c408b01518101836109a0880152806109a086015289818509935050610c608b01518101836109c0880152806109c086015289818509935050610c808b01518101836109e0880152806109e086015289818509935050610ca08b0151810183610a0088015280610a008601528981850993505087810183610a2088015280610a20860152898185099350505081610a4086015280610a4084015287818309915050610a6084019350610a6082019150602088019750612042565b60208b0197506129c287600289038361199a565b9550505083612a23577f08c379a0000000000000000000000000000000000000000000000000000000006000526020600452601e6024527f426174636820696e76657273652070726f64756374206973207a65726f2e000060445260626000fd5b90915060e08501905b81831115612ae9576020830392508484845109835284818401518509935060208303925084848451098352848184015185099350602083039250848484510983528481840151850993506020830392508484845109835284818401518509935060208303925084848451098352848184015185099350602083039250848484510983528481840151850993506020830392508484845109835284818401518509935060208303925084848451098352848184015185099350612a2c565b5b85831115612b0d5760208303925084848451098352848184015185099350612aea565b50505050505050505050565b60405180610cc00160405280606690602082028036833750919291505056fea2646970667358221220dcf9a1247c17d6d05c43314bd855b17811bf0ec0b0d29512b5201c3f57add2eb64736f6c634300060c0033",
}

// CpuOodsABI is the input ABI used to generate the binding from.
// Deprecated: Use CpuOodsMetaData.ABI instead.
var CpuOodsABI = CpuOodsMetaData.ABI

// CpuOodsBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use CpuOodsMetaData.Bin instead.
var CpuOodsBin = CpuOodsMetaData.Bin

// DeployCpuOods deploys a new Ethereum contract, binding an instance of CpuOods to it.
func DeployCpuOods(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *CpuOods, error) {
	parsed, err := CpuOodsMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(CpuOodsBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &CpuOods{CpuOodsCaller: CpuOodsCaller{contract: contract}, CpuOodsTransactor: CpuOodsTransactor{contract: contract}, CpuOodsFilterer: CpuOodsFilterer{contract: contract}}, nil
}

// CpuOods is an auto generated Go binding around an Ethereum contract.
type CpuOods struct {
	CpuOodsCaller     // Read-only binding to the contract
	CpuOodsTransactor // Write-only binding to the contract
	CpuOodsFilterer   // Log filterer for contract events
}

// CpuOodsCaller is an auto generated read-only Go binding around an Ethereum contract.
type CpuOodsCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// CpuOodsTransactor is an auto generated write-only Go binding around an Ethereum contract.
type CpuOodsTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// CpuOodsFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type CpuOodsFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// CpuOodsSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type CpuOodsSession struct {
	Contract     *CpuOods          // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// CpuOodsCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type CpuOodsCallerSession struct {
	Contract *CpuOodsCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts  // Call options to use throughout this session
}

// CpuOodsTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type CpuOodsTransactorSession struct {
	Contract     *CpuOodsTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts  // Transaction auth options to use throughout this session
}

// CpuOodsRaw is an auto generated low-level Go binding around an Ethereum contract.
type CpuOodsRaw struct {
	Contract *CpuOods // Generic contract binding to access the raw methods on
}

// CpuOodsCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type CpuOodsCallerRaw struct {
	Contract *CpuOodsCaller // Generic read-only contract binding to access the raw methods on
}

// CpuOodsTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type CpuOodsTransactorRaw struct {
	Contract *CpuOodsTransactor // Generic write-only contract binding to access the raw methods on
}

// NewCpuOods creates a new instance of CpuOods, bound to a specific deployed contract.
func NewCpuOods(address common.Address, backend bind.ContractBackend) (*CpuOods, error) {
	contract, err := bindCpuOods(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &CpuOods{CpuOodsCaller: CpuOodsCaller{contract: contract}, CpuOodsTransactor: CpuOodsTransactor{contract: contract}, CpuOodsFilterer: CpuOodsFilterer{contract: contract}}, nil
}

// NewCpuOodsCaller creates a new read-only instance of CpuOods, bound to a specific deployed contract.
func NewCpuOodsCaller(address common.Address, caller bind.ContractCaller) (*CpuOodsCaller, error) {
	contract, err := bindCpuOods(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &CpuOodsCaller{contract: contract}, nil
}

// NewCpuOodsTransactor creates a new write-only instance of CpuOods, bound to a specific deployed contract.
func NewCpuOodsTransactor(address common.Address, transactor bind.ContractTransactor) (*CpuOodsTransactor, error) {
	contract, err := bindCpuOods(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &CpuOodsTransactor{contract: contract}, nil
}

// NewCpuOodsFilterer creates a new log filterer instance of CpuOods, bound to a specific deployed contract.
func NewCpuOodsFilterer(address common.Address, filterer bind.ContractFilterer) (*CpuOodsFilterer, error) {
	contract, err := bindCpuOods(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &CpuOodsFilterer{contract: contract}, nil
}

// bindCpuOods binds a generic wrapper to an already deployed contract.
func bindCpuOods(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(CpuOodsABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_CpuOods *CpuOodsRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _CpuOods.Contract.CpuOodsCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_CpuOods *CpuOodsRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _CpuOods.Contract.CpuOodsTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_CpuOods *CpuOodsRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _CpuOods.Contract.CpuOodsTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_CpuOods *CpuOodsCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _CpuOods.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_CpuOods *CpuOodsTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _CpuOods.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_CpuOods *CpuOodsTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _CpuOods.Contract.contract.Transact(opts, method, params...)
}

// Fallback is a paid mutator transaction binding the contract fallback function.
//
// Solidity: fallback() returns()
func (_CpuOods *CpuOodsTransactor) Fallback(opts *bind.TransactOpts, calldata []byte) (*types.Transaction, error) {
	return _CpuOods.contract.RawTransact(opts, calldata)
}

// Fallback is a paid mutator transaction binding the contract fallback function.
//
// Solidity: fallback() returns()
func (_CpuOods *CpuOodsSession) Fallback(calldata []byte) (*types.Transaction, error) {
	return _CpuOods.Contract.Fallback(&_CpuOods.TransactOpts, calldata)
}

// Fallback is a paid mutator transaction binding the contract fallback function.
//
// Solidity: fallback() returns()
func (_CpuOods *CpuOodsTransactorSession) Fallback(calldata []byte) (*types.Transaction, error) {
	return _CpuOods.Contract.Fallback(&_CpuOods.TransactOpts, calldata)
}
