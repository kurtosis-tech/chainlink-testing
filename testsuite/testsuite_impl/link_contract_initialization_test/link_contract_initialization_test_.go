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
	numberOfExtraNodes = 2

	gethDataDirArtifactId  services.FilesArtifactID = "geth-data-dir"
	gethDataDirArtifactUrl                          = "https://kurtosis-public-access.s3.us-east-1.amazonaws.com/client-geth_artifacts/chainlink/geth-data-dir.tgz"
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

	err := chainlinkNetwork.AddPostgres()
	if err != nil {
		return nil, stacktrace.Propagate(err, "Error adding postgres to the network.")
	}

	err = chainlinkNetwork.AddPriceFeedServer()
	if err != nil {
		return nil, stacktrace.Propagate(err, "Error adding the price feed server to the network.")
	}

	err = chainlinkNetwork.AddBootstrapper()
	if err != nil {
		return nil, stacktrace.Propagate(err, "Error adding bootstrapper to the network.")
	}
	logrus.Infof("Added a geth bootstrapper service.")
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

func (test *LinkContractInitializationTest) Run(network networks.Network, testCtx testsuite.TestContext) {
	// Necessary because Go doesn't have generics
	chainlinkNetwork := network.(*networks_impl.ChainlinkNetwork)

	logrus.Infof("Manually connecting all nodes of the Ethereum network.")
	err := chainlinkNetwork.ManuallyConnectPeers()
	if err != nil {
		testCtx.Fatal(stacktrace.Propagate(err, "Failed to manually connect peers in the network."))
	}

	bootstrapPeers, err := chainlinkNetwork.GetBootstrapper().GetPeers()
	if err != nil {
		testCtx.Fatal(stacktrace.Propagate(err, "Failed to get peers of the bootstrapper."))
	}
	testCtx.AssertTrue(len(bootstrapPeers) == numberOfExtraNodes, stacktrace.NewError("Bootstrapper is not connected to all of the network."))

	for _, validatorId := range test.validatorIds {
		gethService, err := chainlinkNetwork.GetGethService(validatorId)
		if err != nil {
			testCtx.Fatal(stacktrace.Propagate(err, "Failed to get validator %v", validatorId))
		}
		peers, err := gethService.GetPeers()
		if err != nil {
			testCtx.Fatal(stacktrace.Propagate(err, "Failed to get peers of validator %v", validatorId))
		}
		testCtx.AssertTrue(len(peers) == numberOfExtraNodes, stacktrace.NewError("Validator %v is not connected to all of the network.", validatorId))
	}

	logrus.Infof("Deploying $LINK contracts on the testnet.")
	err = chainlinkNetwork.DeployChainlinkContract()
	if err != nil {
		testCtx.Fatal(stacktrace.Propagate(err, "Failed to deploy the $LINK contract on the network."))
	}

	logrus.Infof("Funding a $LINK wallet contract on the testnet.")
	err = chainlinkNetwork.FundLinkWallet()
	if err != nil {
		testCtx.Fatal(stacktrace.Propagate(err, "Failed to fund a $LINK wallet on the network."))
	}

	logrus.Infof("Starting a Chainlink Oracle node, using $LINK contract deployed at %v", chainlinkNetwork.GetLinkContractAddress())
	err = chainlinkNetwork.AddOracleService()
	if err != nil {
		testCtx.Fatal(stacktrace.Propagate(err, "Error adding chainlink oracle to the network."))
	}
	logrus.Infof("Chainlink Oracle started and responsive on: %v:%v",
		chainlinkNetwork.GetChainlinkOracle().GetIPAddress(),
		chainlinkNetwork.GetChainlinkOracle().GetOperatorPort())

	logrus.Infof("Funding ethereum accounts owned by the Oracle so that it can fulfill requests.")
	err = chainlinkNetwork.FundOracleEthAccounts()
	if err != nil {
		testCtx.Fatal(stacktrace.Propagate(err, "Error funding Oracle accounts."))
	}

	logrus.Infof("Configuring and setting a JobSpec on the Oracle to access an example price feed.")
	err = chainlinkNetwork.DeployOracleJob()
	if err != nil {
		testCtx.Fatal(stacktrace.Propagate(err, "Error deploying Oracle job."))
	}

	logrus.Infof("Using on-chain smart contracts to trigger job from the Oracle smart contract.")
	err = chainlinkNetwork.RequestData()
	if err != nil {
		testCtx.Fatal(stacktrace.Propagate(err, "Error requesting data from Chainlink oracle."))
	}

	logrus.Infof("Oracle successfully ran job accessing a remote price feed URL.")
}


func (test *LinkContractInitializationTest) GetTestConfiguration() testsuite.TestConfiguration {
	return testsuite.TestConfiguration{
		FilesArtifactUrls: map[services.FilesArtifactID]string{
			gethDataDirArtifactId: gethDataDirArtifactUrl,
		},
	}
}

func (test *LinkContractInitializationTest) GetExecutionTimeout() time.Duration {
	return 30000 * time.Second
}

func (test *LinkContractInitializationTest) GetSetupTimeout() time.Duration {
	return 30000 * time.Second
}

