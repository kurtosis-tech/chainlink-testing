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
	numberOfExtraNodes = 2

	gethDataDirArtifactId  services.FilesArtifactID = "geth-data-dir"
	gethDataDirArtifactUrl                          = "https://kurtosis-public-access.s3.us-east-1.amazonaws.com/client-artifacts/chainlink/geth-data-dir.tgz"
)

type EthereumFundedTest struct {
	gethServiceImage string
	validatorIds []services.ServiceID
}

func NewEthereumFundedTest(gethServiceImage string) *EthereumFundedTest {
	return &EthereumFundedTest{
		gethServiceImage: gethServiceImage,
		validatorIds: []services.ServiceID{},
	}
}

func (test *EthereumFundedTest) Setup(networkCtx *networks.NetworkContext) (networks.Network, error) {
	chainlinkNetwork := networks_impl.NewChainlinkNetwork(networkCtx, gethDataDirArtifactId, test.gethServiceImage)
	err := chainlinkNetwork.AddBootstrapper()
	if err != nil {
		return nil, stacktrace.Propagate(err, "Error adding bootstrapper to the network.")
	}
	for i := 0; i < numberOfExtraNodes; i++ {
		serviceId, err := chainlinkNetwork.AddGethService()
		if err != nil {
			return nil, stacktrace.Propagate(err, "Failed to add an ethereum node.")
		}
		logrus.Infof("Added a geth service with id: %v", serviceId)
		test.validatorIds = append(test.validatorIds, serviceId)
	}
	return chainlinkNetwork, nil
}

func (test *EthereumFundedTest) Run(network networks.Network, testCtx testsuite.TestContext) {
	// Necessary because Go doesn't have generics
	chainlinkNetwork := network.(*networks_impl.ChainlinkNetwork)

	bootstrapperService := chainlinkNetwork.GetBootstrapper()

	isAvailable := bootstrapperService.IsAvailable()

	enodeRecord, err := bootstrapperService.GetEnodeAddress()
	if err != nil {
		testCtx.Fatal(stacktrace.Propagate(err, "An error occurred getting the bootstrap enodeRecord."))
	}
	logrus.Infof("Bootnode enode record: %v", enodeRecord)

	for i := 0; i < numberOfExtraNodes; i++ {
		validatorService, err := chainlinkNetwork.GetGethService(test.validatorIds[i])
		if err != nil {
			testCtx.Fatal(stacktrace.Propagate(err, ""))
		}
		enodeRecord, err = validatorService.GetEnodeAddress()
		if err != nil {
			testCtx.Fatal(stacktrace.Propagate(err, "An error occurred getting the validator enodeRecord."))
		}
		logrus.Infof("Validator enode record: %v", enodeRecord)
	}
	time.Sleep(60 * time.Second)
	err = chainlinkNetwork.ManuallyConnectPeers()
	time.Sleep(60 * time.Second)


	if err != nil {
		testCtx.Fatal(stacktrace.Propagate(err, "Failed to manually connect peers in the network."))
	}

	testCtx.AssertTrue(isAvailable, stacktrace.NewError("Network did not become available."))
}


func (test *EthereumFundedTest) GetTestConfiguration() testsuite.TestConfiguration {
	return testsuite.TestConfiguration{
		FilesArtifactUrls: map[services.FilesArtifactID]string{
			gethDataDirArtifactId: gethDataDirArtifactUrl,
		},
	}
}

func (test *EthereumFundedTest) GetExecutionTimeout() time.Duration {
	return 960 * time.Second
}

func (test *EthereumFundedTest) GetSetupTimeout() time.Duration {
	return 960 * time.Second
}

