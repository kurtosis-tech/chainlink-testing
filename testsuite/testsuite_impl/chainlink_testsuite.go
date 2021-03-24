package testsuite_impl

import (
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/testsuite"
	"github.com/kurtosistech/chainlink-testing/testsuite/testsuite_impl/link_contract_initialization_test"
)

type ChainlinkTestsuite struct {
	gethServiceImage string
	chainlinkContractDeployerImage string
	chainlinkOracleImage string
	postgresImage string
	priceFeedServerImage string
}

func NewChainlinkTestsuite(gethServiceImage string, chainlinkContractDeployerImage string,
	chainlinkOracleImage string, postgresImage string, priceFeedServerImage string) *ChainlinkTestsuite {
	return &ChainlinkTestsuite{
		gethServiceImage: gethServiceImage,
		chainlinkContractDeployerImage: chainlinkContractDeployerImage,
		chainlinkOracleImage: chainlinkOracleImage,
		postgresImage: postgresImage,
		priceFeedServerImage: priceFeedServerImage,
	}
}

func (suite ChainlinkTestsuite) GetTests() map[string]testsuite.Test {
	tests := map[string]testsuite.Test{
		"linkContractInitializationTest": link_contract_initialization_test.NewLinkContractInitializationTest(
			suite.gethServiceImage,
			suite.chainlinkContractDeployerImage,
			suite.chainlinkOracleImage,
			suite.postgresImage,
			suite.priceFeedServerImage,),
	}
	return tests
}

func (suite ChainlinkTestsuite) GetNetworkWidthBits() uint32 {
	return 8
}



