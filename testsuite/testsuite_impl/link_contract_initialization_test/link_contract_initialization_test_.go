package link_contract_initialization_test

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

	gethDataDirArtifactId  services.FilesArtifactID = "geth-data-dir"
	gethDataDirArtifactUrl                          = "https://kurtosis-public-access.s3.amazonaws.com/client-artifacts/chainlink/geth-data-dir.tgz"
)

type LinkContractInitializationTest struct {
	gethServiceImage string
	chainlinkContractDeployerImage string
	chainlinkOracleImage string
	postgresImage string
	priceFeedServerImage string
	validatorIds []services.ServiceID
}

func NewLinkContractInitializationTest(gethServiceImage string, chainlinkContractDeployerImage string,
	chainlinkOracleImage string, postgresImage string, priceFeedServerImage string) *LinkContractInitializationTest {
	return &LinkContractInitializationTest{
		gethServiceImage: gethServiceImage,
		chainlinkContractDeployerImage: chainlinkContractDeployerImage,
		chainlinkOracleImage: chainlinkOracleImage,
		postgresImage: postgresImage,
		priceFeedServerImage: priceFeedServerImage,
		validatorIds: []services.ServiceID{},
	}
}

func (test *LinkContractInitializationTest) Setup(networkCtx *networks.NetworkContext) (networks.Network, error) {
	chainlinkNetwork := networks_impl.NewChainlinkNetwork(networkCtx,
		gethDataDirArtifactId,
		test.gethServiceImage,
		test.chainlinkContractDeployerImage,
		test.postgresImage,
		test.chainlinkOracleImage,
		test.priceFeedServerImage)

	if err := chainlinkNetwork.Setup(); err != nil {
		return nil, stacktrace.Propagate(err, "An error occurred setting up the Chainlink network")
	}

	return chainlinkNetwork, nil
}

func (test *LinkContractInitializationTest) Run(network networks.Network, testCtx testsuite.TestContext) {
	// Necessary because Go doesn't have generics
	 chainlinkNetwork := network.(*networks_impl.ChainlinkNetwork)

	/*
	oracleEthAccounts, err := network.chainlinkOracleServices[0].GetEthKeys()
	if err != nil {
		return stacktrace.Propagate(err, "Error occurred requesting ethereum key information.")
	}

	for _, ethAccount := range oracleEthAccounts {
		ethAddress := ethAccount.Attributes.Address
		logrus.Infof("Setting permissions for address %v to run code from oracle contract %v.",
			ethAddress,
			network.oracleContractAddress)
		err = network.linkContractDeployerService.SetFulfillmentPermissions(
			network.GetBootstrapper().GetIPAddress(),
			strconv.Itoa(network.GetBootstrapper().GetRpcPort()),
			network.oracleContractAddress,
			ethAddress,
		)
		if err != nil {
			return stacktrace.Propagate(err, "Error occurred setting fulfillent permissions.")
		}
	}

	 */

	// TODO DEBUGGING to show the call clearly in the logs
	time.Sleep(5 * time.Second)

	logrus.Infof("Calling OCR contract...")
	ocrContract := chainlinkNetwork.GetOCRContract()
	answer, err := ocrContract.LatestAnswer(nil)
	if err != nil {
		testCtx.Fatal(stacktrace.Propagate(err, "An error occurred getting the latest answer from the OCR contract"))
	}
	logrus.Info("Called OCR contract")

	logrus.Infof("Answer: %v", answer)
}


func (test *LinkContractInitializationTest) GetTestConfiguration() testsuite.TestConfiguration {
	return testsuite.TestConfiguration{
		FilesArtifactUrls: map[services.FilesArtifactID]string{
			gethDataDirArtifactId: gethDataDirArtifactUrl,
		},
	}
}

func (test *LinkContractInitializationTest) GetExecutionTimeout() time.Duration {
	// TODO DEBUGGING
	return 30000 * time.Second
}

func (test *LinkContractInitializationTest) GetSetupTimeout() time.Duration {
	// TODO DEBUGGING
	return 30000 * time.Second
}

