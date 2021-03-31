package networks_impl

import (
	"encoding/hex"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/networks"
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
	"github.com/kurtosistech/chainlink-testing/testsuite/services_impl/chainlink_contract_deployer"
	"github.com/kurtosistech/chainlink-testing/testsuite/services_impl/chainlink_oracle"
	"github.com/kurtosistech/chainlink-testing/testsuite/services_impl/geth"
	"github.com/kurtosistech/chainlink-testing/testsuite/services_impl/postgres"
	"github.com/kurtosistech/chainlink-testing/testsuite/services_impl/price_feed_server"
	"github.com/palantir/stacktrace"
	"github.com/sirupsen/logrus"
	"github.com/smartcontractkit/libocr/gethwrappers/accesscontrolledoffchainaggregator"
	"github.com/smartcontractkit/libocr/gethwrappers/offchainaggregator"
	"github.com/smartcontractkit/libocr/offchainreporting/confighelper"
	"github.com/smartcontractkit/libocr/offchainreporting/types"
	"math/big"
	"strconv"
	"strings"
	"time"
)

const (
	gethServiceIdPrefix                        = "ethereum-node-"
	jobCompletedStatus      string             = "completed"
	linkContractDeployerId  services.ServiceID = "link-contract-deployer"
	postgresIdPrefix        = "postgres-"
	priceFeedServerId       services.ServiceID = "price-feed-server"
	chainlinkOracleIdPrefix = "chainlink-oracle-"

	// Num Geth nodes (including bootstrapper)
	numGethNodes = 3

	waitForStartupTimeBetweenPolls = 1 * time.Second
	waitForStartupMaxNumPolls = 30

	maxNumGethValidatorConnectednessVerifications = 3
	timeBetweenGethValidatorConnectednessVerifications = 1 * time.Second

	waitForFundingFinalizationTime = 60 * time.Second

	oracleEthPreFundingAmount = "10000000000000000000000000000"

	// Number of oracle nodes (including bootstrapper)
	numOracleNodes = 3

	waitForJobCompletionTimeBetweenPolls = 1 * time.Second
	waitForJobCompletionPolls = 30

	// Oracle nodes will have multiple ETH keys/addresses
	// This is the index of the transmitter address
	transmitterAddressIndex = 0

	// These prefixes need to be stripped off the OCR key bundle attributes
	onChainSigningAddrStrPrefix = "ocrsad_"
	offChainPublicKeyStrPrefix = "ocroff_"
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
	chainlinkOracleServices map[services.ServiceID]*chainlink_oracle.ChainlinkOracleService
	priceFeedServerImage               string
	priceFeedServer                    *price_feed_server.PriceFeedServer
	priceFeedJobId                     string
}

func NewChainlinkNetwork(networkCtx *networks.NetworkContext, gethDataDirArtifactId services.FilesArtifactID,
	gethServiceImage string, linkContractDeployerImage string, postgresImage string,
	chainlinkOracleImage string, priceFeedServerImage string) *ChainlinkNetwork {
	return &ChainlinkNetwork{
		networkCtx:                  networkCtx,
		gethDataDirArtifactId:       gethDataDirArtifactId,
		gethServiceImage:            gethServiceImage,
		gethServices:                map[services.ServiceID]*geth.GethService{},
		nextGethServiceId:           0,
		linkContractAddress:         "",
		oracleContractAddress:       "",
		linkContractDeployerImage:   linkContractDeployerImage,
		linkContractDeployerService: nil,
		postgresImage:               postgresImage,
		chainlinkOracleImage:        chainlinkOracleImage,
		chainlinkOracleServices:     map[services.ServiceID]*chainlink_oracle.ChainlinkOracleService{},
		priceFeedServerImage:        priceFeedServerImage,
		priceFeedServer:             nil,
		priceFeedJobId:              "",
	}
}

func (network *ChainlinkNetwork) Setup() error {
	var gethBootstrapper *geth.GethService  // Nil indicates there is no bootstrapper
	for i := 0; i < numGethNodes; i++ {
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
	gethBootstrapperClient, err := gethBootstrapper.GetClient()
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred getting the Geth bootstrapper ETH client")
	}

	// TODO THIS IS A GIGANTIC HACK - NEED A PROPER WAY TO GET THE GETH KEY!
	firstFundedAddrKey := "{\"address\":\"8ea1441a74ffbe9504a8cb3f7e4b7118d8ccfc56\",\"crypto\":{\"cipher\":\"aes-128-ctr\",\"ciphertext\":\"2dfb66792b39f458365f8604e959d000a57a44c5c9e935130da75edb21571666\",\"cipherparams\":{\"iv\":\"c75546ec881dcd668e7d9cb4f75d24f3\"},\"kdf\":\"scrypt\",\"kdfparams\":{\"dklen\":32,\"n\":262144,\"p\":1,\"r\":8,\"salt\":\"4cb212065dfaba68e7a2e99f42d2bf4e10edc5793390424bfeb4c73a381dbdfd\"},\"mac\":\"98c469923b668bd1655e8acdb40b7d9d5ceae53058b5fd706064595d10b67142\"},\"id\":\"f64bbf7e-e34f-442e-91b9-9bc0a1190edf\",\"version\":3}\n"
	password := "password"
	firstFundedAddrTransactor, err := bind.NewTransactorWithChainID(
		strings.NewReader(firstFundedAddrKey),
		password,
		big.NewInt(geth.PrivateNetworkId))
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred creating a transactor to sign the transaction")
	}
	firstFundedAddr := common.HexToAddress(geth.FirstFundedAddressHex)

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
	if err := contractDeployerService.FundLinkWalletContract(); err != nil {
		return stacktrace.Propagate(err, "An error occurred funding an initial LINK wallet on the testnet")
	}
	logrus.Info("LINK wallet funded")

	logrus.Info("Adding Postgres nodes for oracles...")
	postgresServices := []*postgres.PostgresService{}
	for i := 0; i < numOracleNodes; i++ {
		serviceId := services.ServiceID(fmt.Sprintf("%v%v", postgresIdPrefix, i))
		// TODO this will block until the node is available - we can speed this up by starting them all AND THEN
		//  waiting for them all
		service, err := addPostgresService(network.networkCtx, serviceId, network.postgresImage)
		if err != nil {
			return stacktrace.Propagate(err, "An error occurred adding postgres service '%v'", serviceId)
		}
		postgresServices = append(postgresServices, service)
	}
	logrus.Info("Added Postgres nodes for oracles")

	logrus.Info("Adding oracle nodes...")
	var oracleBootstrapper *chainlink_oracle.ChainlinkOracleService  // Nil indicates there is no bootstrapper
	for i := 0; i < numOracleNodes; i++ {
		serviceId := services.ServiceID(fmt.Sprintf("%v%v", chainlinkOracleIdPrefix, i))
		postgresService := postgresServices[i]
		// TODO This will wait for the oracle service to become available before starting the next one - we can speed this
		//  up by starting them all, THEN waiting for them all
		service, err := addOracleService(
			network.networkCtx,
			linkContractAddress,
			oracleContractAddress,
			gethBootstrapper,
			serviceId,
			network.chainlinkOracleImage,
			postgresService)
		if err != nil {
			return stacktrace.Propagate(err, "An error occurred adding oracle service '%v'", serviceId)
		}
		if oracleBootstrapper == nil {
			oracleBootstrapper = service
		}
		network.chainlinkOracleServices[serviceId] = service
	}
	logrus.Info("Added oracle nodes")

	logrus.Info("Funding oracle ETH addresses...")
	if err := fundOracleEthAccounts(network.chainlinkOracleServices, gethBootstrapper); err != nil {
		return stacktrace.Propagate(err, "An error occurred funding the oracle ETH accounts")
	}
	logrus.Info("Funded oracle ETH addresses")

	logrus.Info("Deploying OCR oracle contract...")
	ocrContractAddr, ocrContract, err := deployOcrOracleContract(
		gethBootstrapperClient,
		firstFundedAddrTransactor,
		firstFundedAddr,
		common.HexToAddress(linkContractAddress))
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred deploying the OCR contract")
	}

	logrus.Info("Configuring OCR contract with oracles...")
	configureOcrContract(firstFundedAddrTransactor, ocrContract, network.chainlinkOracleServices)
	logrus.Info("Configured OCR contract")

	// TODO Debugging
	logrus.Infof("OCR contract address: %v", ocrContractAddr.Hex())

	// TODO Set up oracle jobs corresponding to the OCR contract

	// TODO configure the Oracle contract

	// TODO Add price feed server

	return nil
}

/*
	Runs scripts on the contract deployer container which request data from the oracle.
*/
/*
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
 */

func (network *ChainlinkNetwork) GetLinkContractAddress() string {
	return network.linkContractAddress
}

func (network *ChainlinkNetwork) GetChainlinkOracles() map[services.ServiceID]*chainlink_oracle.ChainlinkOracleService {
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

func addPostgresService(networkCtx *networks.NetworkContext, serviceId services.ServiceID, dockerImage string) (*postgres.PostgresService, error) {
	postgresInitializer := postgres.NewPostgresContainerInitializer(dockerImage)
	uncastedService, checker, err := networkCtx.AddService(serviceId, postgresInitializer)
	if err != nil {
		return nil, stacktrace.Propagate(err, "An error occurred adding postgres service with ID '%v'", serviceId)
	}
	if err := checker.WaitForStartup(waitForStartupTimeBetweenPolls, waitForStartupMaxNumPolls); err != nil {
		return nil, stacktrace.Propagate(err, "An error occurred waiting for postgres service with ID '%v' to start", serviceId)
	}
	castedService, ok := uncastedService.(*postgres.PostgresService)
	if !ok {
		return nil, stacktrace.NewError("An error occurred downcasting postgres service with ID '%v' to the correct type", serviceId)
	}
	return castedService, nil
}

func addOracleService(
		networkCtx *networks.NetworkContext,
		linkContractAddr string,
		oracleContractAddr string,
		gethService *geth.GethService,
		serviceId services.ServiceID,
		dockerImage string,
		postgresService *postgres.PostgresService) (*chainlink_oracle.ChainlinkOracleService, error) {
	initializer := chainlink_oracle.NewChainlinkOracleContainerInitializer(
		dockerImage,
		linkContractAddr,
		oracleContractAddr,
		gethService,
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

func fundOracleEthAccounts(oracleServices map[services.ServiceID]*chainlink_oracle.ChainlinkOracleService, gethService *geth.GethService) error {
	for serviceId, oracleService := range oracleServices {
		ethKeys, err := oracleService.GetEthKeys()
		if err != nil {
			return stacktrace.Propagate(err, "Couldn't get ETH keys for oracle '%v' in order to fund it", serviceId)
		}
		for _, ethKey := range ethKeys {
			toAddress := ethKey.Attributes.Address
			if err := gethService.SendTransaction(geth.FirstFundedAddressHex, toAddress, oracleEthPreFundingAmount); err != nil {
				return stacktrace.Propagate(err, "An error occurred sending ETH to address '%v' owned by oracle '%v'", toAddress, serviceId)
			}
		}
	}

	/*
		Poll for transaction finalization so that we know that the oracle's ethereum accounts are funded.
		See: https://docs.chain.link/docs/running-a-chainlink-node#start-the-chainlink-node, "you will
		need to send some ETH to your node's address in order for it to fulfill requests".
	 */
	allTransactionsFinalizedDeadline := time.Now().Add(waitForFundingFinalizationTime)
	for serviceId, oracleService := range oracleServices {
		oracleFunded := false
		for !oracleFunded {
			if time.Now().After(allTransactionsFinalizedDeadline) {
				return stacktrace.NewError("Not all transactions to fund the oracles were finalized, even after %v", waitForFundingFinalizationTime)
			}

			ethKeys, err := oracleService.GetEthKeys()
			if err != nil {
				return stacktrace.Propagate(err, "An error occurred getting the ETH keys for oracle '%v' to check if they're funded", serviceId)
			}

			allAccountsFunded := true
			for _, ethKey := range ethKeys {
				allAccountsFunded = allAccountsFunded && (ethKey.Attributes.EthBalance != "0")
			}

			oracleFunded = allAccountsFunded
		}
	}
	return nil
}

// NOTE: Most of this method is copied from:
//	https://github.com/smartcontractkit/chainlink/blob/51944ed3b3d0ea390998a3fffe33abaf2e15a711/core/internal/features_test.go#L1303
func deployOcrOracleContract(validatorClient *ethclient.Client, sendingTransactor *bind.TransactOpts, sendingAddr common.Address, linkContractAddr common.Address) (ocrContractAddr common.Address, ocrContract *offchainaggregator.OffchainAggregator, resultErr error) {
	accessControllerAddr, _, _, err := accesscontrolledoffchainaggregator.DeploySimpleWriteAccessController(sendingTransactor, validatorClient)
	if err != nil {
		return common.Address{}, nil, stacktrace.Propagate(err, "An error occurred deploying the access controller contract")
	}

	min, max := new(big.Int), new(big.Int)
	min.Exp(big.NewInt(-2), big.NewInt(191), nil)
	max.Exp(big.NewInt(2), big.NewInt(191), nil)
	max.Sub(max, big.NewInt(1))
	ocrContractAddress, _, ocrContract, err := offchainaggregator.DeployOffchainAggregator(
		sendingTransactor,                     // auth *bind.TransactOpts
		validatorClient,                       // backend bind.ContractBackend
		1000,                                  // _maximumGasPrice uint32,
		200,                                   //_reasonableGasPrice uint32,
		3.6e7,                                 // 3.6e7 microLINK, or 36 LINK
		1e8,                                   // _linkGweiPerObservation uint32,
		4e8,                                   // _linkGweiPerTransmission uint32,
		linkContractAddr, //_link common.Address,
		sendingAddr,
		min,         // -2**191
		max,         // 2**191 - 1
		accessControllerAddr,
		accessControllerAddr,
		0,
		"Test OCR Contract")
	if err != nil {
		return common.Address{}, nil, stacktrace.Propagate(err, "An error occurred deploying the OCR contract")
	}
	return ocrContractAddress, ocrContract, nil
}

func configureOcrContract(
		sendingTransactor *bind.TransactOpts,
		ocrContract *offchainaggregator.OffchainAggregator,
		oracleServices map[services.ServiceID]*chainlink_oracle.ChainlinkOracleService) error {
	oracleIdentities := []confighelper.OracleIdentityExtra{}
	for serviceId, oracleService := range oracleServices {
		// TODO REPLACE ALL THESE CALLS WITH CALLS TO ACTUAL ORACLE CLIENT
		ethKeys, err := oracleService.GetEthKeys()
		if err != nil {
			return stacktrace.Propagate(err, "An error occurred getting ETH addresses for oracle '%v'", serviceId)
		}
		if len(ethKeys) < transmitterAddressIndex + 1 {
			return stacktrace.NewError(
				"Needed to get transmitter address at index %v but oracle '%v' only has %v keys/addresses",
				transmitterAddressIndex,
				serviceId,
				len(ethKeys))
		}
		transmitterKey := ethKeys[transmitterAddressIndex]

		allP2pKeys, err := oracleService.GetPeerToPeerKeys()
		if err != nil {
			return stacktrace.Propagate(err, "An error occurred getting P2P keys for oracle '%v'", serviceId)
		}
		if len(allP2pKeys) != 1 {
			return stacktrace.NewError("Expected exactly one P2P key but got %v", len(allP2pKeys))
		}
		p2pKey := allP2pKeys[0]

		allOcrKeyBundles, err := oracleService.GetOCRKeyBundles()
		if err != nil {
			return stacktrace.Propagate(err, "An error occurred getting the OCR key bundle for oracle '%v'", serviceId)
		}
		if len(allOcrKeyBundles) != 1 {
			return stacktrace.NewError("Expected exactly one OCR key bundle but got %v", len(allOcrKeyBundles))
		}
		ocrKeyBundle := allOcrKeyBundles[0]

		trimmedOnChainSigningAddrStr := strings.TrimPrefix(
			ocrKeyBundle.Attributes.OnChainSigningAddress,
			onChainSigningAddrStrPrefix,
		)
		onChainSigningAddr := types.OnChainSigningAddress(common.HexToAddress(trimmedOnChainSigningAddrStr))
		transmitterAddr := common.HexToAddress(transmitterKey.Attributes.Address)
		trimmedOffChainPubKeyStr := strings.TrimPrefix(ocrKeyBundle.Attributes.OffChainPublicKey, offChainPublicKeyStrPrefix)
		offChainPubKeyBytes, err := hex.DecodeString(trimmedOffChainPubKeyStr)
		if err != nil {
			return stacktrace.Propagate(err, "Could not hex-decode offchain pub key '%v'", trimmedOffChainPubKeyStr)
		}
		offChainPubKey := types.OffchainPublicKey(offChainPubKeyBytes)

		identity := confighelper.OracleIdentityExtra{
			OracleIdentity:                  confighelper.OracleIdentity{
				OnChainSigningAddress: onChainSigningAddr,
				TransmitAddress:       transmitterAddr,
				OffchainPublicKey:     offChainPubKey,
				PeerID:                p2pKey.Attributes.PeerId,
			},
			SharedSecretEncryptionPublicKey: types.SharedSecretEncryptionPublicKey{},
		}
		oracleIdentities = append(oracleIdentities, identity)
	}

	// Params for this method are copied from https://github.com/smartcontractkit/chainlink/blob/51944ed3b3d0ea390998a3fffe33abaf2e15a711/core/internal/features_test.go#L1416
	signers, transmitters, threshold, encodedConfigVersion, encodedConfig, err := confighelper.ContractSetConfigArgsForIntegrationTest(
		oracleIdentities,
		1, // F
		1000000000/100, // threshold PPB
	)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred generating the OCR contract SetConfig parameters")
	}

	if _, err := ocrContract.SetConfig(
		sendingTransactor,
		signers,
		transmitters,
		threshold,
		encodedConfigVersion,
		encodedConfig,
	); err != nil {
		return stacktrace.Propagate(err, "An error occurred calling SetConfig on the OCR contract")
	}
	return nil
}

/*
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

 */
