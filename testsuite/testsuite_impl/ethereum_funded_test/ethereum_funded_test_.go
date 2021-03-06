package ethereum_funded_test

import (
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/networks"
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/testsuite"
	"github.com/kurtosistech/chainlink-testing/testsuite/networks_impl"
	"github.com/palantir/stacktrace"
	"github.com/sirupsen/logrus"
	"time"
)

const (
	gethBootnodeServiceId services.ServiceID = "bootnode"

	waitForStartupTimeBetweenPolls = 1 * time.Second
	waitForStartupMaxPolls = 15

	gethDataDirArtifactId  services.FilesArtifactID = "geth-data-dir"
	gethDataDirArtifactUrl                          = "https://kurtosis-public-access.s3.us-east-1.amazonaws.com/client-artifacts/chainlink/geth-data-dir.tgz"
)

type EthereumFundedTest struct {
	gethServiceImage string
}

func NewEthereumFundedTest(gethServiceImage string) *EthereumFundedTest {
	return &EthereumFundedTest{gethServiceImage: gethServiceImage}
}

func (test EthereumFundedTest) Setup(networkCtx *networks.NetworkContext) (networks.Network, error) {
	chainlinkNetwork := networks_impl.NewChainlinkNetwork(networkCtx, gethDataDirArtifactId, test.gethServiceImage)
	err := chainlinkNetwork.AddBootstrapper()
	if err != nil {
		return nil, stacktrace.Propagate(err, "Error adding bootstrapper to the network.")
	}
	return networkCtx, nil
}

func (test EthereumFundedTest) Run(network networks.Network, testCtx testsuite.TestContext) {
	// Necessary because Go doesn't have generics
	chainlinkNetwork := network.(*networks_impl.ChainlinkNetwork)

	bootstrapperService := chainlinkNetwork.GetBootstrapper()

	isAvailable := bootstrapperService.IsAvailable()

	enodeAddress, err := bootstrapperService.GetEnodeAddress()
	if err != nil {
		testCtx.Fatal(stacktrace.Propagate(err, "An error occurred getting the enodeAddress."))
	}
	logrus.Infof("Enode address response: %v", enodeAddress)

	testCtx.AssertTrue(isAvailable, stacktrace.NewError("Bootnode did not become available."))
}


func (test *EthereumFundedTest) GetTestConfiguration() testsuite.TestConfiguration {
	return testsuite.TestConfiguration{
		FilesArtifactUrls: map[services.FilesArtifactID]string{
			gethDataDirArtifactId: gethDataDirArtifactUrl,
		},
	}
}

func (test *EthereumFundedTest) GetExecutionTimeout() time.Duration {
	return 720 * time.Second
}

func (test *EthereumFundedTest) GetSetupTimeout() time.Duration {
	return 720 * time.Second
}

