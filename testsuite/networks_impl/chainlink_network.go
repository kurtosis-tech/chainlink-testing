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
	ethereumBootstrapperId services.ServiceID = "ethereum-bootstrapper"
	gethServiceIdPrefix                       = "ethereum-node-"
	jobCompletedStatus string				  = "completed"
	linkContractDeployerId services.ServiceID = "link-contract-deployer"
	postgresId services.ServiceID = "postgres"
	priceFeedServerId services.ServiceID = "price-feed-server"
	chainlinkOracleId services.ServiceID = "chainlink-oracle"

	waitForStartupTimeBetweenPolls = 1 * time.Second
	waitForStartupMaxNumPolls = 30

	waitForTransactionFinalizationTimeBetweenPolls = 1 * time.Second
	waitForTransactionFinalizationPolls = 30

	waitForJobCompletionTimeBetweenPolls = 1 * time.Second
	waitForJobCompletionPolls = 30

	oracleEthPreFundingAmount = "10000000000000000000000000000"

	maxNumGethValidatorConnectednessVerifications = 3
	timeBetweenGethValidatorConnectednessVerifications = 1 * time.Second
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
	priceFeedServerImage		string
	priceFeedServer				*price_feed_server.PriceFeedServer
	priceFeedJobId				string
}

func NewChainlinkNetwork(networkCtx *networks.NetworkContext, gethDataDirArtifactId services.FilesArtifactID,
	gethServiceImage string, linkContractDeployerImage string, postgresImage string,
	chainlinkOracleImage string, priceFeedServerImage string) *ChainlinkNetwork {
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
		priceFeedServerImage:	   priceFeedServerImage,
	}
}

func (network *ChainlinkNetwork) DeployChainlinkContract() error {
	if len(network.gethServices) == 0 {
		return stacktrace.NewError("Can not deploy contract because the network does not have non-bootstrapper nodes yet.")
	}

	// We could pick any node here, but we go with the bootstrapper arbitrarily.
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
	logrus.Debugf("Information for running smart contract: Oracle Address: %v, JobId: %v",
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
	for _, ethAccount := range oracleEthAccounts {
		toAddress := ethAccount.Attributes.Address
		err = network.gethBootsrapperService.SendTransaction(geth.FirstFundedAddress, toAddress, oracleEthPreFundingAmount)
		if err != nil {
			return stacktrace.Propagate(err, "An error occurred sending eth between accounts.")
		}
	}

	/*
		Poll for transaction finalization so that we know that the Oracle's ethereum accounts are funded.
		See: https://docs.chain.link/docs/running-a-chainlink-node#start-the-chainlink-node, "you will
		need to send some ETH to your node's address in order for it to fulfill requests".
	 */
	ethAccountsFunded := false
	numPolls := 0
	for !ethAccountsFunded && numPolls < waitForTransactionFinalizationPolls {
		time.Sleep(waitForTransactionFinalizationTimeBetweenPolls)
		oracleEthAccounts, err = network.chainlinkOracleService.GetEthAccounts()
		if err != nil {
			return stacktrace.Propagate(err, "An error occurred getting the Oracle's ethereum accounts")
		}
		numPolls += 1

		// Eth Accounts are considered funded if every eth account the Oracle owns is funded (has balance != 0)
		allAccountsFunded := true
		for _, account := range(oracleEthAccounts) {
			allAccountsFunded = allAccountsFunded && (account.Attributes.EthBalance != "0")
		}
		ethAccountsFunded = ethAccountsFunded || allAccountsFunded
	}
	return nil
}

/*
	Runs scripts on the contract deployer container which request data from the Oracle.
 */
func (network *ChainlinkNetwork) RequestData() error {
	if network.chainlinkOracleService == nil {
		return stacktrace.NewError("Tried to request data before deploying the oracle service.")
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
	oracleEthAccounts, err := network.chainlinkOracleService.GetEthAccounts()
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

	logrus.Infof("Calling the Oracle contract to run job %v.", network.priceFeedJobId)

	priceFeedUrl := fmt.Sprintf("http://%v:%v/", network.priceFeedServer.GetIPAddress(), network.priceFeedServer.GetHTTPPort())
	// Request data from the Oracle smart contract, starting a job.
	err = network.linkContractDeployerService.RunRequestDataScript(network.oracleContractAddress, network.priceFeedJobId, priceFeedUrl)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred requesting data from the Oracle contract on-chain.")
	}
	// Poll to see if the Oracle job has completed.
	numPolls := 0
	jobCompleted := false
	for !jobCompleted && numPolls < waitForJobCompletionPolls {
		time.Sleep(waitForJobCompletionTimeBetweenPolls)
		runs, err := network.chainlinkOracleService.GetRuns()
		if err != nil {
			return stacktrace.Propagate(err, "An error occurred getting data about job runs from the Oracle service.")
		}
		for _, run := range(runs) {
			// If the Oracle has a completed run with the same jobId as the priceFeed, job is complete.
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

	// Connect all nodes to each other
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

	// Now check that all nodes have all other nodes as peers
	expectedNumPeers := len(allServices) - 1
	for nodeId, nodeGethService := range allServices {
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

func (network *ChainlinkNetwork) GetGethService(serviceId services.ServiceID) (*geth.GethService, error) {
	service, found := network.gethServices[serviceId]
	if !found {
		return nil, stacktrace.NewError("No geth service with ID '%v' has been added", serviceId)
	}
	return service, nil
}