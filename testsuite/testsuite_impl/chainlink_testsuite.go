package testsuite_impl

import "github.com/kurtosis-tech/kurtosis-libs/golang/lib/testsuite"

type ChainlinkTestsuite struct {
	gethServiceImage string
}

func NewChainlinkTestsuite(gethServiceImage string) *ChainlinkTestsuite {
	return &ChainlinkTestsuite{gethServiceImage: gethServiceImage}
}

func (suite ChainlinkTestsuite) GetTests() map[string]testsuite.Test {
	tests := map[string]testsuite.Test{}
	return tests
}

func (suite ChainlinkTestsuite) GetNetworkWidthBits() uint32 {
	return 8
}



