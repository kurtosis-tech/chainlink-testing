package testsuite_impl

import (
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/testsuite"
	"github.com/kurtosistech/chainlink-testing/testsuite/testsuite_impl/ethereum_funded_test"
)

type ChainlinkTestsuite struct {
	gethServiceImage string
	chainlinkContractDeployerImage string
}

func NewChainlinkTestsuite(gethServiceImage string, chainlinkContractDeployerImage string) *ChainlinkTestsuite {
	return &ChainlinkTestsuite{
		gethServiceImage: gethServiceImage,
		chainlinkContractDeployerImage: chainlinkContractDeployerImage,
	}
}

func (suite ChainlinkTestsuite) GetTests() map[string]testsuite.Test {
	tests := map[string]testsuite.Test{
		"ethereumFundedTest": ethereum_funded_test.NewEthereumFundedTest(suite.gethServiceImage, suite.chainlinkContractDeployerImage),
	}
	return tests
}

func (suite ChainlinkTestsuite) GetNetworkWidthBits() uint32 {
	return 8
}



