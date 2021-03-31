package networks_impl

import (
	"fmt"
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/networks"
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
	"github.com/kurtosistech/chainlink-testing/testsuite/services_impl/chainlink_contract_deployer"
	"github.com/kurtosistech/chainlink-testing/testsuite/services_impl/chainlink_oracle"
	"github.com/kurtosistech/chainlink-testing/testsuite/services_impl/geth"
	"github.com/kurtosistech/chainlink-testing/testsuite/services_impl/postgres"
	"github.com/kurtosistech/chainlink-testing/testsuite/services_impl/price_feed_server"
	"github.com/palantir/stacktrace"
	"github.com/sirupsen/logrus"
	"strconv"
	"time"
)

const (
	gethServiceIdPrefix                        = "ethereum-node-"
	jobCompletedStatus      string             = "completed"
	linkContractDeployerId  services.ServiceID = "link-contract-deployer"
	postgresIdPrefix        = "postgres-"
	priceFeedServerId       services.ServiceID = "price-feed-server"
	chainlinkOracleIdPrefix = "chainlink-oracle-"

	// Number of nodes beyond the bootstrapper
	numExtraGethNodes = 2

	waitForStartupTimeBetweenPolls = 1 * time.Second
	waitForStartupMaxNumPolls = 30

	waitForTransactionFinalizationTimeBetweenPolls = 1 * time.Second
	waitForTransactionFinalizationPolls = 30

	waitForJobCompletionTimeBetweenPolls = 1 * time.Second
	waitForJobCompletionPolls = 30

	oracleEthPreFundingAmount = "10000000000000000000000000000"

	// Number of oracle nodes that will be deployed on the network (with the first being a bootstrapper)
	// TODO Up to 5
	numOracles = 1

	maxNumGethValidatorConnectednessVerifications = 3
	timeBetweenGethValidatorConnectednessVerifications = 1 * time.Second
)

type ChainlinkNetwork struct {
	networkCtx                         *networks.NetworkContext
	gethDataDirArtifactId              services.FilesArtifactID
	gethServiceImage                   string
	gethServices map[services.ServiceID]*geth.GethService
	nextGethServiceId                  int
	linkContractAddress                string
	oracleContractAddress              string
	linkContractDeployerImage          string
	linkContractDeployerService        *chainlink_contract_deployer.ChainlinkContractDeployerService
	postgresImage                      string
	chainlinkOracleImage               string
	chainlinkOracleBootstrapperService *chainlink_oracle.ChainlinkOracleService
	chainlinkOracleServices            []*chainlink_oracle.ChainlinkOracleService
	priceFeedServerImage               string
	priceFeedServer                    *price_feed_server.PriceFeedServer
	priceFeedJobId                     string
}

func NewChainlinkNetwork(networkCtx *networks.NetworkContext, gethDataDirArtifactId services.FilesArtifactID,
	gethServiceImage string, linkContractDeployerImage string, postgresImage string,
	chainlinkOracleImage string, priceFeedServerImage string) *ChainlinkNetwork {
	return &ChainlinkNetwork{
		networkCtx:                         networkCtx,
		gethDataDirArtifactId:              gethDataDirArtifactId,
		gethServiceImage:                   gethServiceImage,
		gethServices:                       map[services.ServiceID]*geth.GethService{},
		nextGethServiceId:                  0,
		linkContractAddress:                "",
		oracleContractAddress:              "",
		linkContractDeployerImage:          linkContractDeployerImage,
		linkContractDeployerService:        nil,
		postgresImage:                      postgresImage,
		chainlinkOracleImage:               chainlinkOracleImage,
		chainlinkOracleBootstrapperService: nil,
		chainlinkOracleServices:            []*chainlink_oracle.ChainlinkOracleService{},
		priceFeedServerImage:               priceFeedServerImage,
		priceFeedServer:                    nil,
		priceFeedJobId:                     "",
	}
}

func (network *ChainlinkNetwork) Setup() error {
	var gethBootstrapper *geth.GethService  // Nil indicates there is no bootstrapper
	for i := 0; i < numExtraGethNodes; i++ {
		serviceId := services.ServiceID(fmt.Sprintf("%v%v", gethServiceIdPrefix, i))
		service, err := network.addGethService(serviceId, nil)
		if err != nil {
			return stacktrace.Propagate(err, "An error occurred adding Geth service '%v'", serviceId)
		}
		if gethBootstrapper == nil {
			gethBootstrapper = service
		}
		network.gethServices[serviceId] = service
	}

	logrus.Infof("Manually connecting all Ethereum nodes together and verifying connectivity...")
	if err := manuallyConnectGethNodesAndVerifyConnectivity(network.gethServices); err != nil {
		return stacktrace.Propagate(err, "An error occurred manually connecting the Geth nodes and verifying connectivity")
	}
	logrus.Infof("Ethereum nodes connected and connectivity verified")

	// TODO rename this
	logrus.Info("Starting contract deployer service...")
	contractDeployerService, err := startContractDeployerService(network.networkCtx, network.linkContractDeployerImage)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred starting the contract deployer service")
	}
	logrus.Info("Contract deployer service started")

	logrus.Infof("Deploying LINK contracts on the testnet...")
	// We could pick any node here, but we go with the bootstrapper arbitrarily.
	linkContractAddress, oracleContractAddress, err := contractDeployerService.DeployContract(
		gethBootstrapper.GetIPAddress(),
		strconv.Itoa(gethBootstrapper.GetRpcPort()),
	)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred deploying the $LINK contract to the testnet.")
	}
	logrus.Infof("LINK contract deployed")

	logrus.Info("Funding LINK wallet...")
	if err := network.linkContractDeployerService.FundLinkWalletContract(); err != nil {
		return stacktrace.Propagate(err, "An error occurred funding an initial LINK wallet on the testnet")
	}
	logrus.Info("LINK wallet funded")

}

/*
	Runs scripts on the contract deployer container which request data from the oracle.
*/
func (network *ChainlinkNetwork) RequestData() error {
	if len(network.chainlinkOracleServices) == 0 {
		return stacktrace.NewError("Tried to request data before deploying any oracle services")
	}
	if network.oracleContractAddress == "" {
		return stacktrace.NewError("Tried to request data before deploying the oracle contract.")
	}
	if network.linkContractDeployerService == nil {
		return stacktrace.NewError("Tried to request data before deploying the link contract deployer service.")
	}
	if network.priceFeedServer == nil {
		return stacktrace.NewError("Tried to request data before deploying the in-network price feed server service.")
	}
	// TODO handle multiple
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

	logrus.Infof("Calling the oracle contract to run job %v.", network.priceFeedJobId)

	priceFeedUrl := fmt.Sprintf("http://%v:%v/", network.priceFeedServer.GetIPAddress(), network.priceFeedServer.GetHTTPPort())
	// Request data from the oracle smart contract, starting a job.
	err = network.linkContractDeployerService.RunRequestDataScript(network.oracleContractAddress, network.priceFeedJobId, priceFeedUrl)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred requesting data from the oracle contract on-chain.")
	}
	// Poll to see if the oracle job has completed.
	numPolls := 0
	jobCompleted := false
	for !jobCompleted && numPolls < waitForJobCompletionPolls {
		time.Sleep(waitForJobCompletionTimeBetweenPolls)
		// TODO Handle multiple
		runs, err := network.chainlinkOracleServices[0].GetRuns()
		if err != nil {
			return stacktrace.Propagate(err, "An error occurred getting data about job runs from the oracle service.")
		}
		for _, run := range(runs) {
			// If the oracle has a completed run with the same jobId as the priceFeed, job is complete.
			if run.Attributes.JobId == network.priceFeedJobId {
				jobCompleted = jobCompleted || run.Attributes.Status == jobCompletedStatus
			}
		}
		numPolls += 1
	}
	if !jobCompleted {
		return stacktrace.NewError("Oracle job %v failed.", network.priceFeedJobId)
	}
	return nil
}


func (network *ChainlinkNetwork) GetBootstrapper() *geth.GethService {
	return network.gethBootstrapperService
}

func (network *ChainlinkNetwork) GetLinkContractAddress() string {
	return network.linkContractAddress
}

func (network *ChainlinkNetwork) GetChainlinkOracles() []*chainlink_oracle.ChainlinkOracleService {
	return network.chainlinkOracleServices
}

func (network *ChainlinkNetwork) AddPriceFeedServer() error {
	initializer := price_feed_server.NewPriceFeedServerInitializer(network.priceFeedServerImage)
	uncastedPriceFeedServer, checker, err := network.networkCtx.AddService(priceFeedServerId, initializer)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred adding the price feed server.")
	}
	if err := checker.WaitForStartup(waitForStartupTimeBetweenPolls, waitForStartupMaxNumPolls); err != nil {
		return stacktrace.Propagate(err, "An error occurred waiting for the price feed server to start")
	}
	castedPriceFeedServer := uncastedPriceFeedServer.(*price_feed_server.PriceFeedServer)
	network.priceFeedServer = castedPriceFeedServer
	return nil
}


// ====================================================================================================
//                                          Private helper functions
// ====================================================================================================
// NOTE: Leave the bootstrapper nil to create a bootstrapper node
func (network ChainlinkNetwork) addGethService(serviceId services.ServiceID, bootstrapper *geth.GethService) (*geth.GethService, error) {
	initializer := geth.NewGethContainerInitializer(network.gethServiceImage, network.gethDataDirArtifactId, bootstrapper, true)
	uncastedService, checker, err := network.networkCtx.AddService(serviceId, initializer)
	if err != nil {
		return nil, stacktrace.Propagate(err, "An error occurred adding Geth service with ID '%v'", serviceId)
	}
	if err := checker.WaitForStartup(waitForStartupTimeBetweenPolls, waitForStartupMaxNumPolls); err != nil {
		return nil, stacktrace.Propagate(err, "An error occurred waiting for Geth service with ID '%v' to start", serviceId)
	}
	castedService, ok := uncastedService.(*geth.GethService)
	if !ok {
		return nil, stacktrace.NewError("An error occurred downcasting generic service interface to Geth service for service '%v'", serviceId)
	}
	return castedService, nil
}

func manuallyConnectGethNodesAndVerifyConnectivity(gethServices map[services.ServiceID]*geth.GethService) error {
	// Connect all nodes to each other
	for nodeId, nodeGethService := range gethServices {
		for peerId, peerGethService := range gethServices {
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

	// Now check that all nodes have all other nodes as peers
	expectedNumPeers := len(gethServices) - 1
	for nodeId, nodeGethService := range gethServices {
		seesAllPeers := false
		numVerificationsAttempted := 0
		for !seesAllPeers && numVerificationsAttempted < maxNumGethValidatorConnectednessVerifications {
			peers, err := nodeGethService.GetPeers()
			numVerificationsAttempted += 1
			seesAllPeers = err == nil && len(peers) == expectedNumPeers
			if !seesAllPeers {
				time.Sleep(timeBetweenGethValidatorConnectednessVerifications)
			}
		}
		if !seesAllPeers {
			return stacktrace.NewError(
				"Geth validator '%v' still didn't see all %v peers after %v tries with %v between tries",
				nodeId,
				expectedNumPeers,
				maxNumGethValidatorConnectednessVerifications,
				timeBetweenGethValidatorConnectednessVerifications)
		}
	}
	return nil
}

func startContractDeployerService(
		networkCtx *networks.NetworkContext,
		dockerImage string) (*chainlink_contract_deployer.ChainlinkContractDeployerService, error){
	initializer := chainlink_contract_deployer.NewChainlinkContractDeployerInitializer(dockerImage)
	uncastedService, checker, err := networkCtx.AddService(linkContractDeployerId, initializer)
	if err != nil {
		return nil, stacktrace.Propagate(err, "An error occurred adding the $LINK contract deployer to the network.")
	}
	if err := checker.WaitForStartup(waitForStartupTimeBetweenPolls, waitForStartupMaxNumPolls); err != nil {
		return nil, stacktrace.Propagate(err, "An error occurred waiting for the $LINK contract deployer service to start")
	}
	castedService, ok := uncastedService.(*chainlink_contract_deployer.ChainlinkContractDeployerService)
	if !ok {
		return nil, stacktrace.Propagate(err, "An error occurred downcasting a generic service to the contract deployer service")
	}
	return castedService,nil
}

func (network *ChainlinkNetwork) AddOracleBootstrapper() error {
	// TODO Rather than doing all this error-checking in each call, smash this all together into one big Setup function
	if network.linkContractAddress == "" {
		return stacktrace.NewError("Cannot add oracle bootstrapper service to network; LINK contract isn't deployed")
	}
	if network.oracleContractAddress == "" {
		return stacktrace.NewError("Cannot add oracle bootstrapper service to network; oracle contract isn't deployed")
	}
	if network.gethBootstrapperService == nil {
		return stacktrace.NewError("Cannot add oracle bootstrapper service to network; the Geth bootstrapper isn't deployed and we need a Geth client to interact with the network")
	}
	if network.chainlinkOracleBootstrapperService != nil {
		return stacktrace.NewError("Cannot add oracle bootstrapper service to network; oracle bootstrapper already exists!")
	}

	addOracleService(
		network.networkCtx,
		network.linkContractAddress,
		network.oracleContractAddress,

	)
}

func (network *ChainlinkNetwork) AddOracleServices() error {
	if network.linkContractAddress == "" {
		return stacktrace.NewError("Cannot add oracle services; the $LINK token contract has not yet been deployed.")
	}
	if network.oracleContractAddress == "" {
		return stacktrace.NewError("Cannot add oracle services; the oracle contract has not yet been deployed.")
	}
	if network.gethBootstrapperService == nil {
		return stacktrace.NewError("Cannot add oracle services to network; the Geth bootstrapper isn't deployed and we need a Geth client to interact with the network")
	}
	if network.chainlinkOracleBootstrapperService != nil {
		return stacktrace.NewError("Cannot ")
	}
	if len(network.chainlinkOracleServices) > 0 {
		return stacktrace.NewError("Cannot add oracle services to the network; oracle services already exist!")
	}

	postgresInitializer := postgres.NewPostgresContainerInitializer(network.postgresImage)
	for i := 0; i < numOracles; i++ {
		postgresServiceId := services.ServiceID(fmt.Sprintf("%v%v", postgresIdPrefix, i))
		uncastedPostgres, checker, err := network.networkCtx.AddService(postgresServiceId, postgresInitializer)
		if err != nil {
			return stacktrace.Propagate(err, "An error occurred adding postgres service with ID '%v'", postgresServiceId)
		}
		if err := checker.WaitForStartup(waitForStartupTimeBetweenPolls, waitForStartupMaxNumPolls); err != nil {
			return stacktrace.Propagate(err, "An error occurred waiting for postgres service with ID '%v' to start", postgresServiceId)
		}
		castedPostgres, ok := uncastedPostgres.(*postgres.PostgresService)
		if !ok {
			return stacktrace.NewError("An error occurred downcasting postgres service with ID '%v' to the correct type", postgresServiceId)
		}

		oracleInitializer := chainlink_oracle.NewChainlinkOracleContainerInitializer(network.chainlinkOracleImage,
			network.linkContractAddress, network.oracleContractAddress, network.gethBootstrapperService, castedPostgres)
		oracleServiceId := services.ServiceID(fmt.Sprintf("%v%v", chainlinkOracleIdPrefix, i))
		uncastedChainlinkOracle, checker, err := network.networkCtx.AddService(oracleServiceId, oracleInitializer)
		if err != nil {
			return stacktrace.Propagate(err, "An error occurred adding oracle service with ID '%v'", oracleServiceId)
		}
		if err := checker.WaitForStartup(waitForStartupTimeBetweenPolls, waitForStartupMaxNumPolls); err != nil {
			return stacktrace.Propagate(err, "An error occurred waiting for oracle service with ID '%v' to start up", oracleServiceId)
		}
		castedChainlinkOracle, ok := uncastedChainlinkOracle.(*chainlink_oracle.ChainlinkOracleService)
		if !ok {
			return stacktrace.NewError("Could not downcast oracle service to correct type")
		}
		network.chainlinkOracleServices = append(network.chainlinkOracleServices, castedChainlinkOracle)
	}
	return nil
}

func (network *ChainlinkNetwork) FundOracleEthAccounts() error {
	if len(network.chainlinkOracleServices) == 0 {
		return stacktrace.NewError("Tried to fund oracle ETH accounts before deploying any oracles")
	}
	// TODO handle multiple
	oracleEthAccounts, err := network.chainlinkOracleServices[0].GetEthKeys()
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred getting the oracle's ethereum accounts")
	}
	for _, ethAccount := range oracleEthAccounts {
		toAddress := ethAccount.Attributes.Address
		err = network.gethBootstrapperService.SendTransaction(geth.FirstFundedAddress, toAddress, oracleEthPreFundingAmount)
		if err != nil {
			return stacktrace.Propagate(err, "An error occurred sending eth between accounts.")
		}
	}

	/*
		Poll for transaction finalization so that we know that the oracle's ethereum accounts are funded.
		See: https://docs.chain.link/docs/running-a-chainlink-node#start-the-chainlink-node, "you will
		need to send some ETH to your node's address in order for it to fulfill requests".
	 */
	ethAccountsFunded := false
	numPolls := 0
	for !ethAccountsFunded && numPolls < waitForTransactionFinalizationPolls {
		time.Sleep(waitForTransactionFinalizationTimeBetweenPolls)
		// TODO Handle multiple
		oracleEthAccounts, err = network.chainlinkOracleServices[0].GetEthKeys()
		if err != nil {
			return stacktrace.Propagate(err, "An error occurred getting the oracle's ethereum accounts")
		}
		numPolls += 1

		// Eth Accounts are considered funded if every eth account the oracle owns is funded (has balance != 0)
		allAccountsFunded := true
		for _, account := range(oracleEthAccounts) {
			allAccountsFunded = allAccountsFunded && (account.Attributes.EthBalance != "0")
		}
		ethAccountsFunded = ethAccountsFunded || allAccountsFunded
	}
	return nil
}

func (network *ChainlinkNetwork) DeployOracleJobs() error {
	if network.oracleContractAddress == "" {
		return stacktrace.NewError("Can not deploy oracle jobs because oracle contract has not yet been deployed.")
	}
	if len(network.chainlinkOracleServices) == 0 {
		return stacktrace.NewError("Cannot deploy oracle jobs because oracle services are not yet deployed")
	}

	// TODO handle multiple
	jobId, err := network.chainlinkOracleServices[0].SetJobSpec(network.oracleContractAddress)
	if err != nil {
		return stacktrace.Propagate(err, "Failed to set job spec.")
	}
	network.priceFeedJobId = jobId
	logrus.Debugf("Information for running smart contract: oracle Address: %v, JobId: %v",
		network.oracleContractAddress,
		network.priceFeedJobId)
	return nil
}

func addOracleService(
		networkCtx *networks.NetworkContext,
		linkContractAddr string,
		oracleContractAddr string,
		gethBootstrapperService *geth.GethService,
		serviceId services.ServiceID,
		dockerImage string,
		postgresService *postgres.PostgresService) (*chainlink_oracle.ChainlinkOracleService, error) {
	initializer := chainlink_oracle.NewChainlinkOracleContainerInitializer(
		dockerImage,
		linkContractAddr,
		oracleContractAddr,
		gethBootstrapperService,
		postgresService)
	uncastedService, checker, err := networkCtx.AddService(serviceId, initializer)
	if err != nil {
		return nil, stacktrace.Propagate(err, "An error occurred adding oracle service with ID '%v'", serviceId)
	}
	if err := checker.WaitForStartup(waitForStartupTimeBetweenPolls, waitForStartupMaxNumPolls); err != nil {
		return nil, stacktrace.Propagate(err, "An error occurred waiting for oracle service with ID '%v' to start up", serviceId)
	}
	castedService, ok := uncastedService.(*chainlink_oracle.ChainlinkOracleService)
	if !ok {
		return nil, stacktrace.NewError("Could not downcast oracle service to correct type for service with ID '%v'", serviceId)
	}
	return castedService, nil
}
