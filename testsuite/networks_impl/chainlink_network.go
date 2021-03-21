package networks_impl

import (
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/networks"
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
	"github.com/kurtosistech/chainlink-testing/testsuite/services_impl/chainlink_contract_deployer"
	"github.com/kurtosistech/chainlink-testing/testsuite/services_impl/chainlink_oracle"
	"github.com/kurtosistech/chainlink-testing/testsuite/services_impl/geth"
	"github.com/kurtosistech/chainlink-testing/testsuite/services_impl/postgres"
	"github.com/palantir/stacktrace"
	"github.com/sirupsen/logrus"
	"strconv"
	"time"
)

const (
	ethereumBootstrapperId services.ServiceID = "ethereum-bootstrapper"
	gethServiceIdPrefix                       = "ethereum-node-"
	linkContractDeployerId services.ServiceID = "link-contract-deployer"
	postgresId services.ServiceID = "postgres"
	chainlinkOracleId services.ServiceID = "chainlink-oracle"

	waitForStartupTimeBetweenPolls = 1 * time.Second
	waitForStartupMaxNumPolls = 30

	oracleEthPreFundingAmount = "10000000000000000000"
)

type ChainlinkNetwork struct {
	networkCtx                  *networks.NetworkContext
	gethDataDirArtifactId       services.FilesArtifactID
	gethServiceImage            string
	gethBootsrapperService      *geth.GethService
	gethServices                map[services.ServiceID]*geth.GethService
	nextGethServiceId           int
	linkContractAddress         string
	oracleContractAddress		string
	linkContractDeployerImage   string
	linkContractDeployerService *chainlink_contract_deployer.ChainlinkContractDeployerService
	postgresImage               string
	postgresService             *postgres.PostgresService
	chainlinkOracleImage        string
	chainlinkOracleService      *chainlink_oracle.ChainlinkOracleService
	priceFeedJobId				string
}

func NewChainlinkNetwork(networkCtx *networks.NetworkContext, gethDataDirArtifactId services.FilesArtifactID,
	gethServiceImage string, linkContractDeployerImage string, postgresImage string, chainlinkOracleImage string) *ChainlinkNetwork {
	return &ChainlinkNetwork{
		networkCtx:                networkCtx,
		gethDataDirArtifactId:     gethDataDirArtifactId,
		gethServiceImage:          gethServiceImage,
		gethBootsrapperService:    nil,
		gethServices:              map[services.ServiceID]*geth.GethService{},
		nextGethServiceId:         0,
		linkContractAddress:       "",
		linkContractDeployerImage: linkContractDeployerImage,
		postgresImage:             postgresImage,
		chainlinkOracleImage:      chainlinkOracleImage,
	}
}

func (network *ChainlinkNetwork) DeployChainlinkContract() error {
	if len(network.gethServices) == 0 {
		return stacktrace.NewError("Can not deploy contract because the network does not have non-bootstrapper nodes yet.")
	}
	// TODO TODO TODO Be more principled about which service to deploy on
	deployService := network.gethBootsrapperService
	initializer := chainlink_contract_deployer.NewChainlinkContractDeployerInitializer(network.linkContractDeployerImage)
	uncastedContractDeployer, checker, err := network.networkCtx.AddService(linkContractDeployerId, initializer)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred adding the $LINK contract deployer to the network.")
	}
	if err := checker.WaitForStartup(waitForStartupTimeBetweenPolls, waitForStartupMaxNumPolls); err != nil {
		return stacktrace.Propagate(err, "An error occurred waiting for the $LINK contract deployer service to start")
	}
	castedContractDeployer := uncastedContractDeployer.(*chainlink_contract_deployer.ChainlinkContractDeployerService)
	network.linkContractDeployerService = castedContractDeployer

	linkContractAddress, oracleContractAddress, err := network.linkContractDeployerService.DeployContract(deployService.GetIPAddress(), strconv.Itoa(deployService.GetRpcPort()))
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred deploying the $LINK contract to the testnet.")
	}
	network.linkContractAddress = linkContractAddress
	network.oracleContractAddress = oracleContractAddress
	return nil
}

func (network *ChainlinkNetwork) DeployOracleJob() error {
	if network.oracleContractAddress == "" {
		return stacktrace.NewError("Can not deploy Oracle job because Oracle contract has not yet been deployed.")
	}
	oracleService := network.GetChainlinkOracle()
	jobId, err := oracleService.SetJobSpec(network.oracleContractAddress)
	if err != nil {
		return stacktrace.Propagate(err, "Failed to set job spec.")
	}
	network.priceFeedJobId = jobId
	logrus.Infof("Information for running smart contract: Oracle Address: %v, JobId: %v",
		network.oracleContractAddress,
		network.priceFeedJobId)
	return nil
}

func (network *ChainlinkNetwork) FundLinkWallet() error {
	if network.linkContractDeployerService == nil {
		return stacktrace.NewError("Tried to fund $LINK wallet before deploying $LINK contract.")
	}
	err := network.linkContractDeployerService.FundLinkWalletContract()
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred funding an initial $LINK wallet on the testnet.")
	}
	return nil
}

func (network *ChainlinkNetwork) FundOracleEthAccounts() error {
	if network.chainlinkOracleService == nil {
		return stacktrace.NewError("Tried to fund Oracle eth accounts before deploying Oracle.")
	}
	oracleEthAccounts, err := network.chainlinkOracleService.GetEthAccounts()
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred getting the Oracle's ethereum accounts")
	}
	for _, ethAccount := range(oracleEthAccounts) {
		toAddress := ethAccount.Attributes.Address
		err = network.gethBootsrapperService.SendTransaction(geth.FirstFundedAddress, toAddress, oracleEthPreFundingAmount)
		if err != nil {
			return stacktrace.Propagate(err, "An error occurred sending eth between accounts.")
		}
	}
	// TODO TODO TODO Replace this with polling network for transaction finalization
	time.Sleep(30 * time.Second)
	oracleEthAccounts, err = network.chainlinkOracleService.GetEthAccounts()
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred getting the Oracle's ethereum accounts")
	}
	logrus.Infof("Funded accounts: %+v", oracleEthAccounts)
	return nil
}

func (network *ChainlinkNetwork) AddBootstrapper() error {
	if network.gethBootsrapperService != nil {
		return stacktrace.NewError("Cannot add bootstrapper service to network; bootstrapper already exists!")
	}

	initializer := geth.NewGethContainerInitializer(network.gethServiceImage, network.gethDataDirArtifactId, nil, true)
	uncastedBootstrapper, checker, err := network.networkCtx.AddService(ethereumBootstrapperId, initializer)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred adding the bootstrapper service")
	}
	if err := checker.WaitForStartup(waitForStartupTimeBetweenPolls, waitForStartupMaxNumPolls); err != nil {
		return stacktrace.Propagate(err, "An error occurred waiting for the bootstrapper service to start")
	}
	castedGethBootstrapperService := uncastedBootstrapper.(*geth.GethService)
	network.gethBootsrapperService = castedGethBootstrapperService
	return nil
}

func (network *ChainlinkNetwork) AddPostgres() error {
	if network.postgresService != nil {
		return stacktrace.NewError("Cannot add postgres service to network; postgres service already exists!")
	}
	initializer := postgres.NewPostgresContainerInitializer(network.postgresImage)
	uncastedPostgres, checker, err := network.networkCtx.AddService(postgresId, initializer)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred adding the postgres service")
	}
	if err := checker.WaitForStartup(waitForStartupTimeBetweenPolls, waitForStartupMaxNumPolls); err != nil {
		return stacktrace.Propagate(err, "An error occurred waiting for the postgres service to start")
	}
	castedPostgres := uncastedPostgres.(*postgres.PostgresService)
	network.postgresService = castedPostgres
	return nil
}

func (network *ChainlinkNetwork) AddOracleService() error {
	if network.linkContractAddress == "" {
		return stacktrace.NewError("Tried to add an oracle service, but the $LINK token contract has not yet been deployed.")
	}
	if network.oracleContractAddress == "" {
		return stacktrace.NewError("Tried to add an oracle service, but the Oracle contract has not yet been deployed.")
	}
	if network.chainlinkOracleService != nil {
		return stacktrace.NewError("Tried to add an oracle service, but one has already been added!")
	}
	initializer := chainlink_oracle.NewChainlinkOracleContainerInitializer(network.chainlinkOracleImage,
		network.linkContractAddress, network.oracleContractAddress, network.gethBootsrapperService, network.postgresService)
	uncastedChainlinkOracle, checker, err := network.networkCtx.AddService(chainlinkOracleId, initializer)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred adding the Chainlink Oracle service.")
	}
	if err := checker.WaitForStartup(waitForStartupTimeBetweenPolls, waitForStartupMaxNumPolls); err != nil {
		return stacktrace.Propagate(err, "An error occurred waiting for an Oracle service to start up.")
	}
	castedChainlinkOracle := uncastedChainlinkOracle.(*chainlink_oracle.ChainlinkOracleService)
	network.chainlinkOracleService = castedChainlinkOracle
	return nil
}

func (network *ChainlinkNetwork) GetBootstrapper() *geth.GethService {
	return network.gethBootsrapperService
}

func (network *ChainlinkNetwork) GetLinkContractAddress() string {
	return network.linkContractAddress
}

func (network *ChainlinkNetwork) GetChainlinkOracle() *chainlink_oracle.ChainlinkOracleService {
	return network.chainlinkOracleService
}

func (network *ChainlinkNetwork) AddGethService() (services.ServiceID, error) {
	if (network.gethBootsrapperService == nil) {
		return "", stacktrace.NewError("Cannot add ethereum node to network; no bootstrap node exists")
	}

	serviceIdStr := gethServiceIdPrefix + strconv.Itoa(network.nextGethServiceId)
	network.nextGethServiceId = network.nextGethServiceId + 1
	serviceId := services.ServiceID(serviceIdStr)

	initializer := geth.NewGethContainerInitializer(network.gethServiceImage, network.gethDataDirArtifactId, network.gethBootsrapperService, false)
	uncastedGethService, checker, err := network.networkCtx.AddService(serviceId, initializer)
	if err != nil {
		return "", stacktrace.Propagate(err, "An error occurred adding the ethereum node")
	}
	if err := checker.WaitForStartup(waitForStartupTimeBetweenPolls, waitForStartupMaxNumPolls); err != nil {
		return "", stacktrace.Propagate(err, "An error occurred waiting for the ethereum node to start")
	}
	castedGethService := uncastedGethService.(*geth.GethService)

	network.gethServices[serviceId] = castedGethService
	return serviceId, nil
}

func (network *ChainlinkNetwork) ManuallyConnectPeers() error {
	allServices := map[services.ServiceID]*geth.GethService{
		ethereumBootstrapperId: network.gethBootsrapperService,
	}
	for id, service := range network.gethServices {
		allServices[id] = service
	}
	for nodeId, nodeGethService := range allServices {
		for peerId, peerGethService := range allServices {
			if nodeId == peerId {
				continue
			}
			peerGethServiceEnode, err := peerGethService.GetEnodeAddress()
			if err != nil {
				return stacktrace.Propagate(err, "Failed to get enode from peer %v", peerId)
			}
			ok, err := nodeGethService.AddPeer(peerGethServiceEnode)
			if err != nil {
				return stacktrace.Propagate(err, "Failed to call addPeer endpoint to add peer with enode %v", peerGethServiceEnode)
			}
			if !ok {
				return stacktrace.NewError("addPeer endpoint returned false on service %v, adding peer %v", nodeId, peerGethServiceEnode)
			}
		}
	}
	return nil
}

func (network *ChainlinkNetwork) GetGethService(serviceId services.ServiceID) (*geth.GethService, error) {
	service, found := network.gethServices[serviceId]
	if !found {
		return nil, stacktrace.NewError("No geth service with ID '%v' has been added", serviceId)
	}
	return service, nil
}