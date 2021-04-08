package networks_impl

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/networks"
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
	"github.com/kurtosistech/chainlink-testing/testsuite/networks_impl/config"
	"github.com/kurtosistech/chainlink-testing/testsuite/services_impl/chainlink_contract_deployer"
	"github.com/kurtosistech/chainlink-testing/testsuite/services_impl/chainlink_oracle"
	"github.com/kurtosistech/chainlink-testing/testsuite/services_impl/geth"
	"github.com/kurtosistech/chainlink-testing/testsuite/services_impl/geth/genesis"
	"github.com/kurtosistech/chainlink-testing/testsuite/services_impl/postgres"
	"github.com/kurtosistech/chainlink-testing/testsuite/services_impl/price_feed_server"
	"github.com/palantir/stacktrace"
	"github.com/sirupsen/logrus"
	"github.com/smartcontractkit/libocr/gethwrappers/accesscontrolledoffchainaggregator"
	"github.com/smartcontractkit/libocr/gethwrappers/link_token_interface"
	"github.com/smartcontractkit/libocr/gethwrappers/offchainaggregator"
	"github.com/smartcontractkit/libocr/offchainreporting/types"
	"math/big"
	"strings"
	"time"
)

const (
	gethServiceIdPrefix                        = "ethereum-node-"
	linkContractDeployerId  services.ServiceID = "link-contract-deployer"
	postgresIdPrefix        = "postgres-"
	priceFeedServerId       services.ServiceID = "price-feed-server"
	chainlinkOracleIdPrefix = "chainlink-oracle-"

	// TODO Rename
	ocrContractLinkFundAmount = 1000000000000000000

	jobCompletedStatus      string             = "completed"

	// How long we'll wait for the OCR jobs on the oracles to complete
	timeToWaitForJobCompletion = 10 * time.Second

	timeBetweenJobCompletionChecks = 1 * time.Second

	// Num Geth nodes (including bootstrapper)
	numGethNodes = 3

	// Availability check constants
	gethTimeBetweenIsAvailablePolls = 1 * time.Second
	gethMaxIsAvailablePolls = 30
	contractDeployerTimeBetweenIsAvailablePolls = 1 * time.Second
	contractDeployerMaxIsAvailablePolls = 10
	postgresTimeBetweenIsAvailablePolls = 1 * time.Second
	postgresMaxIsAvailablePolls = 30
	oracleTimeBetweenIsAvailablePolls = 1 * time.Second
	oracleMaxIsAvailablePolls = 120
	priceFeedTimeBetweenIsAvailablePolls = 1 * time.Second
	priceFeedMaxIsAvailablePolls = 10

	maxNumGethValidatorConnectednessVerifications = 10
	timeBetweenGethValidatorConnectednessVerifications = 1 * time.Second

	waitForFundingFinalizationTime = 60 * time.Second

	oracleEthPreFundingAmount = "10000000000000000000000000000"

	// Number of oracle nodes (including bootstrapper)
	numOracleNodes = 5

	// Oracle nodes will have multiple ETH keys/addresses
	// This is the index of the transmitter address
	transmitterAddressIndex = 0

	// These prefixes need to be stripped off the OCR key bundle attributes
	p2pIdStrPrefix = "p2p_"
	onChainSigningAddrStrPrefix = "ocrsad_"
	offChainPublicKeyStrPrefix = "ocroff_"
	configPublicKeyStrPrefix = "ocrcfg_"

	maxNumCheckTransactionMinedRetries = 10
	timeBetweenCheckTransactionMinedRetries = 1 * time.Second
)

type oracleIdentityWithExtraInfo struct {
	inner config.OracleIdentity
	ocrKeyBundleId string
	sharedSecretEncryptionPublicKey types.SharedSecretEncryptionPublicKey
}

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
	priceFeedServerImage               string
	priceFeedServer                    *price_feed_server.PriceFeedServer
	priceFeedJobId                     string
	ocrContract 					   *offchainaggregator.OffchainAggregator
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
		priceFeedServerImage:        priceFeedServerImage,
		priceFeedServer:             nil,
		priceFeedJobId:              "",
		ocrContract:                 nil,
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
		big.NewInt(genesis.ChainId))
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
	/*
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
	 */

	// TODO REFACTOR INTO METHOD
	logrus.Info("Deploying LINK token contract...")
	linkContractAddress, linkContractTxn, linkContract, err := link_token_interface.DeployLinkToken(firstFundedAddrTransactor, gethBootstrapperClient)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred deploying the LINK token contract")
	}
	if err := waitUntilTransactionMined(gethBootstrapperClient, linkContractTxn.Hash()); err != nil {
		return stacktrace.Propagate(err, "An error occurred waiting for the LINK token contract to be mined")
	}
	logrus.Info("Deployed LINK token contract")

	logrus.Info("Deploying OCR oracle contract...")
	ocrContractAddr, ocrContract, err := deployOcrOracleContract(
		gethBootstrapperClient,
		firstFundedAddrTransactor,
		firstFundedAddr,
		linkContractAddress)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred deploying the OCR contract")
	}
	logrus.Info("Deployed OCR oracle contract")

	logrus.Info("Funding OCR contract with LINK...")
	ocrFundingTxn, err := linkContract.Transfer(firstFundedAddrTransactor, ocrContractAddr, big.NewInt(ocrContractLinkFundAmount))
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred funding the OCR contract with LINK")
	}
	if err := waitUntilTransactionMined(gethBootstrapperClient, ocrFundingTxn.Hash()); err != nil {
		return stacktrace.Propagate(err, "An error occurred waiting until the fund-OCR-contract-with-LINK transaction was mined")
	}
	logrus.Info("Funded OCR contract with LINK")

	logrus.Info("Adding Postgres nodes for oracles...")
	postgresServices, err := addPostgresServices(network.networkCtx, network.postgresImage, numOracleNodes)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred adding the postgres nodes")
	}
	logrus.Info("Added Postgres nodes for oracles")

	logrus.Info("Adding oracle nodes...")
	oracleServices, oracleBootstrapperServiceId, err := addOracleService(
		network.networkCtx,
		linkContractAddress,
		ocrContractAddr,
		gethBootstrapper,
		network.chainlinkOracleImage,
		postgresServices)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred adding the oracle services")
	}
	oracleBootstrapper := oracleServices[oracleBootstrapperServiceId]
	logrus.Info("Added oracle nodes")

	logrus.Info("Funding oracle ETH addresses...")
	if err := fundOracleEthAccounts(oracleServices, gethBootstrapper); err != nil {
		return stacktrace.Propagate(err, "An error occurred funding the oracle ETH accounts")
	}
	logrus.Info("Funded oracle ETH addresses")

	logrus.Info("Getting oracle identities...")
	oracleIdentities, err := getOracleIdentities(oracleServices)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred getting oracle identities")
	}
	logrus.Info("Got oracle identities")

	logrus.Info("Configuring OCR contract with oracles...")
	if err := configureOcrContract(firstFundedAddrTransactor, gethBootstrapperClient, ocrContract, oracleIdentities); err != nil {
		return stacktrace.Propagate(err, "An error occurred configuring the OCR contract")
	}
	logrus.Info("Configured OCR contract")

	logrus.Info("Deploying the price feed server...")
	priceFeedService, err := addPriceFeedServer(network.networkCtx, priceFeedServerId, network.priceFeedServerImage)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred adding the price feed server")
	}
	logrus.Info("Deployed price feed server")

	logrus.Info("Deploying OCR jobs on oracles...")
	datasourceUrl := fmt.Sprintf("http://%v:%v", priceFeedService.GetIPAddress(), priceFeedService.GetHTTPPort())
	if err := deployOcrJobsOnOracles(
			ocrContractAddr,
			ocrContract,
			oracleBootstrapperServiceId,
			oracleBootstrapper,
			oracleServices,
			oracleIdentities,
			datasourceUrl); err != nil {
		return stacktrace.Propagate(err, "An error occurred deploying the OCR jobs on the oracles")
	}
	logrus.Info("Deployed OCR jobs on oracles")

	/*
	logrus.Info("Funding data requester address...")
	linkFundingTxn, err := linkContract.Transfer(firstFundedAddrTransactor, firstFundedAddr, big.NewInt(gethBootstrapperLinkFundAmount))
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred funding the first funded address with LINK")
	}
	if err := waitUntilTransactionMined(gethBootstrapperClient, linkFundingTxn.Hash()); err != nil {
		return stacktrace.Propagate(err, "An error occurred waiting for the transaction that funds the first funded address with LINK to be finalized")
	}
	logrus.Info("Data requester address funded")
	 */


	// TODO DEBUGGINNG
	contractOpts := &bind.CallOpts{
		From:        firstFundedAddr,
	}
	for {
		ocrContractLinkBalance, err := linkContract.BalanceOf(nil, ocrContractAddr)
		if err != nil {
			logrus.Debugf("An error occurred getting the LINK balance of the OCR contract")
		} else {
			logrus.Debugf("OCR contract LINK balance: %v", ocrContractLinkBalance)
		}
		transactorLinkBalance, err := linkContract.BalanceOf(nil, firstFundedAddr)
		if err != nil {
			logrus.Debugf("An error occurred getting the LINK balance of address '%v'", firstFundedAddr.Hex())
		} else {
			logrus.Debugf("First funded address LINK balance: %v", transactorLinkBalance)
		}
		transactorEthBalance, err := gethBootstrapperClient.BalanceAt(context.Background(), firstFundedAddr, nil)
		if err != nil {
			logrus.Debugf("An error occurred getting the ETH balance of address '%v'", firstFundedAddr.Hex())
		} else {
			logrus.Debugf("First funded address ETH balance: %v", transactorEthBalance)
		}

		transmission, err := ocrContract.LatestTransmissionDetails(nil)
		if err != nil {
			logrus.Debugf("An error occurred getting the latest transmission info: %v", err)
		} else {
			logrus.Debugf("Latest transmission: %+v", transmission)
		}

		answer, err := ocrContract.LatestAnswer(contractOpts)
		if err != nil {
			logrus.Debugf("An error occurred getting the latest answer: %v", err)
		} else {
			logrus.Debugf("Latest answer: %v", answer.String())
		}

		for serviceId, identity := range oracleIdentities {
			ethBalance, err := gethBootstrapperClient.BalanceAt(context.Background(), identity.inner.TransmitAddress, nil)
			if err != nil {
				logrus.Debugf("An error occurred getting the ETH balance for node '%v': %v", serviceId, err)
			} else {
				logrus.Debugf("%v ETH balance: %v", serviceId, ethBalance)
			}
			linkBalance, err := linkContract.BalanceOf(nil, common.Address(identity.inner.TransmitAddress))
			if err != nil {
				logrus.Debugf("An error occurred getting the LINK balance for node '%v': %v", serviceId, err)
			} else {
				logrus.Debugf("%v LINK balance: %v", serviceId, linkBalance)
			}
		}
		/*
			for serviceId, oracleService := range oracleServices {
				jobId := jobIds[serviceId]
				runs, err := oracleService.GetRunsForJob(jobId)
				if err != nil {
					logrus.Debugf("An error occurred getting runs for the newly-deployed job '%v' on oracle '%v': %v", jobId, serviceId, err)
				}
				logrus.Debugf("Runs for job %v for oracle %v: %+v", jobId, serviceId, runs)
			}

		*/
		time.Sleep(5 * time.Second)
	}

	network.ocrContract = ocrContract

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

func (network ChainlinkNetwork) GetOCRContract() *offchainaggregator.OffchainAggregator {
	return network.ocrContract
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
	if err := checker.WaitForStartup(gethTimeBetweenIsAvailablePolls, gethMaxIsAvailablePolls); err != nil {
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
	if err := checker.WaitForStartup(contractDeployerTimeBetweenIsAvailablePolls, contractDeployerMaxIsAvailablePolls); err != nil {
		return nil, stacktrace.Propagate(err, "An error occurred waiting for the $LINK contract deployer service to start")
	}
	castedService, ok := uncastedService.(*chainlink_contract_deployer.ChainlinkContractDeployerService)
	if !ok {
		return nil, stacktrace.Propagate(err, "An error occurred downcasting a generic service to the contract deployer service")
	}
	return castedService,nil
}

func addPostgresServices(networkCtx *networks.NetworkContext, dockerImage string, numServices int) ([]*postgres.PostgresService, error) {
	postgresInitializer := postgres.NewPostgresContainerInitializer(dockerImage)
	postgresServices := []*postgres.PostgresService{}
	postgresCheckers := map[services.ServiceID]services.AvailabilityChecker{}
	for i := 0; i < numServices; i++ {
		serviceId := services.ServiceID(fmt.Sprintf("%v%v", postgresIdPrefix, i))
		uncastedService, checker, err := networkCtx.AddService(serviceId, postgresInitializer)
		if err != nil {
			return nil, stacktrace.Propagate(err, "An error occurred adding postgres service with ID '%v'", serviceId)
		}
		postgresCheckers[serviceId] = checker
		castedService, ok := uncastedService.(*postgres.PostgresService)
		if !ok {
			return nil, stacktrace.NewError("An error occurred downcasting postgres service with ID '%v' to the correct type", serviceId)
		}
		postgresServices = append(postgresServices, castedService)
	}

	for serviceId, checker := range postgresCheckers {
		if err := checker.WaitForStartup(postgresTimeBetweenIsAvailablePolls, postgresMaxIsAvailablePolls); err != nil {
			return nil, stacktrace.Propagate(err, "An error occurred waiting for postgres service with ID '%v' to start", serviceId)
		}
	}
	return postgresServices, nil
}

func addOracleService(
		networkCtx *networks.NetworkContext,
		linkContractAddr common.Address,
		oracleContractAddr common.Address,
		gethService *geth.GethService,
		dockerImage string,
		postgresServices []*postgres.PostgresService) (map[services.ServiceID]*chainlink_oracle.ChainlinkOracleService, services.ServiceID, error) {
	var oracleBootstrapperServiceId services.ServiceID
	oracleServices := map[services.ServiceID]*chainlink_oracle.ChainlinkOracleService{}
	for i := 0; i < len(postgresServices); i++ {
		serviceId := services.ServiceID(fmt.Sprintf("%v%v", chainlinkOracleIdPrefix, i))
		postgresService := postgresServices[i]
		initializer := chainlink_oracle.NewChainlinkOracleContainerInitializer(
			dockerImage,
			linkContractAddr,
			oracleContractAddr,
			gethService,
			postgresService)
		uncastedService, checker, err := networkCtx.AddService(serviceId, initializer)
		if err != nil {
			return nil, "", stacktrace.Propagate(err, "An error occurred adding oracle service with ID '%v'", serviceId)
		}

		// We wait for startup here, rather than starting everything in parallel then waiting for them all, because Chainlink nodes
		// use a lot of memory on startup (~1 GB) and so are liable to OOM the Docker VM when started all at once, which means
		// the Docker VM will start killing containers :(
		if err := checker.WaitForStartup(oracleTimeBetweenIsAvailablePolls, oracleMaxIsAvailablePolls); err != nil {
			return nil, "", stacktrace.Propagate(err, "An error occurred waiting for oracle service with ID '%v' to start up", serviceId)
		}

		castedService, ok := uncastedService.(*chainlink_oracle.ChainlinkOracleService)
		if !ok {
			return nil, "", stacktrace.NewError("Could not downcast oracle service to correct type for service with ID '%v'", serviceId)
		}
		oracleServices[serviceId] = castedService

		if oracleBootstrapperServiceId == "" {
			oracleBootstrapperServiceId = serviceId
		}
	}


	return oracleServices, oracleBootstrapperServiceId, nil
}

func fundOracleEthAccounts(oracleServices map[services.ServiceID]*chainlink_oracle.ChainlinkOracleService, gethService *geth.GethService) error {
	for serviceId, oracleService := range oracleServices {
		ethKeys, err := oracleService.GetEthKeys()
		if err != nil {
			return stacktrace.Propagate(err, "Couldn't get ETH keys for oracle '%v' in order to fund it", serviceId)
		}
		for _, ethKey := range ethKeys {
			toAddress := ethKey.Attributes.Address
			// TODO wait until this transaction is mined!
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
	accessControllerAddr, accessControllerTxn, _, err := accesscontrolledoffchainaggregator.DeploySimpleWriteAccessController(sendingTransactor, validatorClient)
	if err != nil {
		return common.Address{}, nil, stacktrace.Propagate(err, "An error occurred deploying the access controller contract")
	}
	if err := waitUntilTransactionMined(validatorClient, accessControllerTxn.Hash()); err != nil {
		return common.Address{}, nil, stacktrace.Propagate(err, "An error occurred waiting for the block with the access controller contract to be mined")
	}


	min, max := new(big.Int), new(big.Int)
	min.Exp(big.NewInt(-2), big.NewInt(191), nil)
	max.Exp(big.NewInt(2), big.NewInt(191), nil)
	max.Sub(max, big.NewInt(1))
	ocrContractAddress, ocrContractTxn, ocrContract, err := offchainaggregator.DeployOffchainAggregator(
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
	if err := waitUntilTransactionMined(validatorClient, ocrContractTxn.Hash()); err != nil {
		return common.Address{}, nil, stacktrace.Propagate(err, "An error occurred waiting for the block with the OCR contract to be mined")
	}
	return ocrContractAddress, ocrContract, nil
}

// If we try to use a contract immediately after submission without waiting for it to be mined, we'll get a "no contract code at address" error:
// https://github.com/ethereum/go-ethereum/issues/15930#issuecomment-532144875
func waitUntilTransactionMined(validatorClient *ethclient.Client, transactionHash common.Hash) error {
	for i := 0; i < maxNumCheckTransactionMinedRetries; i++ {
		receipt, err := validatorClient.TransactionReceipt(context.Background(), transactionHash)
		if err == nil && receipt != nil && receipt.BlockNumber != nil {
			return nil
		}
		if i < maxNumCheckTransactionMinedRetries - 1 {
			time.Sleep(timeBetweenCheckTransactionMinedRetries)
		}
	}
	return stacktrace.NewError(
		"Transaction with hash '%v' wasn't mined even after checking %v times with %v between checks",
		transactionHash.Hex(),
		maxNumCheckTransactionMinedRetries,
		timeBetweenCheckTransactionMinedRetries)
}

func getOracleIdentities(oracleServices map[services.ServiceID]*chainlink_oracle.ChainlinkOracleService) (map[services.ServiceID]oracleIdentityWithExtraInfo, error) {
	oracleIdentities := map[services.ServiceID]oracleIdentityWithExtraInfo{}
	for serviceId, oracleService := range oracleServices {
		// TODO Replace alllllll the handcrafting of OracleIdentityExtra inside here with a call to the Chainlink client
		//  The desired method is likely client.Client.ListOCRKeyBundles
		ethKeys, err := oracleService.GetEthKeys()
		if err != nil {
			return nil, stacktrace.Propagate(err, "An error occurred getting ETH addresses for oracle '%v'", serviceId)
		}
		if len(ethKeys) < transmitterAddressIndex + 1 {
			return nil, stacktrace.NewError(
				"Needed to get transmitter address at index %v but oracle '%v' only has %v keys/addresses",
				transmitterAddressIndex,
				serviceId,
				len(ethKeys))
		}
		transmitterKey := ethKeys[transmitterAddressIndex]

		allP2pKeys, err := oracleService.GetPeerToPeerKeys()
		if err != nil {
			return nil, stacktrace.Propagate(err, "An error occurred getting P2P keys for oracle '%v'", serviceId)
		}
		if len(allP2pKeys) != 1 {
			return nil, stacktrace.NewError("Expected exactly one P2P key but got %v", len(allP2pKeys))
		}
		p2pKey := allP2pKeys[0]

		allOcrKeyBundles, err := oracleService.GetOCRKeyBundles()
		if err != nil {
			return nil, stacktrace.Propagate(err, "An error occurred getting the OCR key bundle for oracle '%v'", serviceId)
		}
		if len(allOcrKeyBundles) != 1 {
			return nil, stacktrace.NewError("Expected exactly one OCR key bundle but got %v", len(allOcrKeyBundles))
		}
		ocrKeyBundle := allOcrKeyBundles[0]

		trimmedOnChainSigningAddrStr := strings.TrimPrefix(
			ocrKeyBundle.Attributes.OnChainSigningAddress,
			onChainSigningAddrStrPrefix,
		)
		onChainSigningAddr := types.OnChainSigningAddress(common.HexToAddress(trimmedOnChainSigningAddrStr))
		transmitterAddr := common.HexToAddress(transmitterKey.Attributes.Address)
		offChainPubKey, err := parseOcrPubKeyHexStr(ocrKeyBundle.Attributes.OffChainPublicKey, offChainPublicKeyStrPrefix)
		if err != nil {
			return nil, stacktrace.Propagate(err, "An error occurred parsing offchain pub key hex string")
		}
		configPubKey, err := parseOcrPubKeyHexStr(ocrKeyBundle.Attributes.ConfigPublicKey, configPublicKeyStrPrefix)
		if err != nil {
			return nil, stacktrace.Propagate(err, "An error occurred parsing config pub key hex string")
		}
		if len(configPubKey) != len(types.SharedSecretEncryptionPublicKey{}) {
			return nil, stacktrace.NewError(
				"Config pubkey must be of length %v but was length %v",
				len(types.SharedSecretEncryptionPublicKey{}),
				len(configPubKey))
		}
		var sharedSecretEncryptionPubKey types.SharedSecretEncryptionPublicKey
		copy(sharedSecretEncryptionPubKey[:], configPubKey)

		identity := oracleIdentityWithExtraInfo{
			inner: config.OracleIdentity{
				OnChainSigningAddress: onChainSigningAddr,
				TransmitAddress:       transmitterAddr,
				OffchainPublicKey:     types.OffchainPublicKey(offChainPubKey),
				PeerID:                strings.TrimPrefix(p2pKey.Attributes.PeerId, p2pIdStrPrefix),
			},
			sharedSecretEncryptionPublicKey: sharedSecretEncryptionPubKey,
			ocrKeyBundleId: ocrKeyBundle.Attributes.Id,
		}

		oracleIdentities[serviceId] = identity
	}

	for serviceId, identity := range oracleIdentities {
		logrus.Debugf("Oracle identity for node '%v': %+v", serviceId, identity)
	}

	return oracleIdentities, nil
}

func configureOcrContract(
		sendingTransactor *bind.TransactOpts,
		validatorClient *ethclient.Client,
		ocrContract *offchainaggregator.OffchainAggregator,
		oracleIdentities map[services.ServiceID]oracleIdentityWithExtraInfo) error {
	S := []int{}
	oracleIdentitiesList := []config.OracleIdentity{}
	sharedSecretEncryptionPublicKeys := []types.SharedSecretEncryptionPublicKey{}
	for _, identity := range oracleIdentities {
		S = append(S, 1) // No idea what this is; it's just copied from https://github.com/smartcontractkit/libocr/blob/master/offchainreporting/confighelper/confighelper.go#L115
		oracleIdentitiesList = append(oracleIdentitiesList, identity.inner)

		sharedSecretEncryptionPublicKeys = append(sharedSecretEncryptionPublicKeys, identity.sharedSecretEncryptionPublicKey)
	}

	// Most of these values are copied from:
	// https://github.com/smartcontractkit/libocr/blob/master/offchainreporting/confighelper/confighelper.go#L115
	sharedConfig := config.SharedConfig{
		config.PublicConfig{
			// These values roughly informed by the whitepaper
			DeltaProgress:    30 * time.Second,
			DeltaResend:      10 * time.Second,
			DeltaRound:       15 * time.Second,
			DeltaGrace:       2 * time.Second,
			DeltaC:           120 * time.Minute,   // How frequently new reports will be issued due to timeout
			AlphaPPB:         10000000,   // Deviation value from old observation for new report to be uploaded
			DeltaStage:       30 * time.Second,
			RMax:             4, // Max number of rounds in epoch
			S:                S,
			OracleIdentities: oracleIdentitiesList,
			F:                1,
			ConfigDigest:     types.ConfigDigest{},
		},
		&[config.SharedSecretSize]byte{1, 2, 3, 4, 5, 6, 7, 8, 1, 2, 3, 4, 5, 6, 7, 8},
	}
	signers, transmitters, threshold, encodedConfigVersion, encodedConfig, err := config.XXXContractSetConfigArgsFromSharedConfig(sharedConfig, sharedSecretEncryptionPublicKeys)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred creating the contract config args")
	}

	setPayeesTxn, err := ocrContract.SetPayees(sendingTransactor, transmitters, transmitters)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred setting the payees for the OCR contract")
	}
	if err := waitUntilTransactionMined(validatorClient, setPayeesTxn.Hash()); err != nil {
		return stacktrace.Propagate(err, "An error occurred waiting until the transaction setting payees was mined")
	}

	setOcrConfigTxn, err := ocrContract.SetConfig(
		sendingTransactor,
		signers,
		transmitters,
		threshold,
		encodedConfigVersion,
		encodedConfig)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred calling SetConfig on the OCR contract")
	}
	if err := waitUntilTransactionMined(validatorClient, setOcrConfigTxn.Hash()); err != nil {
		return stacktrace.Propagate(err, "An error occurred waiting until the transaction configuring the OCR contract was mined")
	}
	return nil
}

// NOTE: The OCR keys always come with a prefix, which needs to be removed before hex decoding
func parseOcrPubKeyHexStr(pubKeyHexStr string, prefix string) (ed25519.PublicKey, error) {
	trimmedHexStr := strings.TrimPrefix(pubKeyHexStr, prefix)
	pubKeyBytes, err := hex.DecodeString(trimmedHexStr)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Could not hex-decode pub key '%v'", trimmedHexStr)
	}
	return pubKeyBytes, nil
}

func deployOcrJobsOnOracles(
		ocrContractAddr common.Address,
		// TODO DEBUGGING
		ocrContract *offchainaggregator.OffchainAggregator,
		bootstrapperServiceId services.ServiceID,
		bootstrapperService *chainlink_oracle.ChainlinkOracleService,
		oracleServices map[services.ServiceID]*chainlink_oracle.ChainlinkOracleService,
		oracleIdentities map[services.ServiceID]oracleIdentityWithExtraInfo,
		datasourceUrl string) error {
	bootstrapperIdentity, found := oracleIdentities[bootstrapperServiceId]
	if !found {
		return stacktrace.NewError("No oracle identity found for bootstrapper ID '%v'", bootstrapperServiceId)
	}
	bootstrapperPeer2PeerId := bootstrapperIdentity.inner.PeerID

	logrus.Debugf("Deploying OCR jobs on oracles...")
	jobIds := map[services.ServiceID]string{}
	for serviceId, oracleService := range oracleServices {
		identity, found := oracleIdentities[serviceId]
		if !found {
			return stacktrace.NewError("Couldn't find oracle identity for oracle with service ID '%v'", serviceId)
		}
		isBootstrapper := serviceId == bootstrapperServiceId
		jobSpecTomlStr, err := generateOcrJobSpecTomlStr(
			ocrContractAddr,
			bootstrapperService.GetIPAddress(),
			bootstrapperService.GetPeerToPeerListenPort(),
			bootstrapperPeer2PeerId,
			identity.inner.PeerID,
			isBootstrapper,
			identity.ocrKeyBundleId,
			identity.inner.TransmitAddress,
			datasourceUrl)
		if err != nil {
			return stacktrace.Propagate(err, "An error occurred generating the OCR job spec TOML string")
		}
		logrus.Debugf("Job spec TOML string:\n%v", jobSpecTomlStr)

		jobId, err := oracleService.SetJobSpec(jobSpecTomlStr)
		if err != nil {
			return stacktrace.Propagate(err, "An error occurred deploying OCR job spec on oracle '%v'", serviceId)
		}
		jobIds[serviceId] = jobId

		logrus.Debugf("Successfully deployed OCR job spec on oracle '%v' referencing OCR contract address '%v', and got back job ID '%v'",
			serviceId,
			ocrContractAddr.Hex(),
			jobId)
	}


	/*
		// Now, wait for jobs to complete successfully
	logrus.Debugf("Waiting for oracle OCR jobs to complete...")
	for serviceId, oracleService := range oracleServices {
		jobId, found := jobIds[serviceId]
		if !found {
			return stacktrace.NewError("No OCR job ID found for oracle '%v' even though we just deployed it; this is a code bug", serviceId)
		}

		jobCompletedDeadline := time.Now().Add(timeToWaitForJobCompletion)
		jobCompleted := false
		for !jobCompleted && time.Now().Before(jobCompletedDeadline) {
			runs, err := oracleService.GetRunsForJob(jobId)
			if err != nil {
				return stacktrace.Propagate(err, "An error occurred getting runs for the newly-deployed job '%v' on oracle '%v'", jobId, serviceId)
			}
			if len(runs) != 1 {
				return stacktrace.NewError(
					"Expected exactly one run for newly-deployed job '%v' on oracle '%v', but got %v",
					jobId,
					serviceId,
					len(runs))
			}
			theRun := runs[0]
			jobCompleted = theRun.Attributes.Status == jobCompletedStatus
			if !jobCompleted {
				time.Sleep(timeBetweenJobCompletionChecks)
			}
		}
		if !jobCompleted {
			return stacktrace.NewError(
				"Newly-deployed OCR job '%v' on oracle '%v' didn't complete, even after waiting for %v",
				jobId,
				serviceId,
				timeToWaitForJobCompletion)
		}
		logrus.Debugf("OCR job '%v' on oracle '%v' completed successfully", jobId, serviceId)
	}

	 */

	return nil
}

func addPriceFeedServer(networkCtx *networks.NetworkContext, serviceId services.ServiceID, dockerImage string) (*price_feed_server.PriceFeedServer, error) {
	initializer := price_feed_server.NewPriceFeedServerInitializer(dockerImage)
	uncastedService, checker, err := networkCtx.AddService(serviceId, initializer)
	if err != nil {
		return nil, stacktrace.Propagate(err, "An error occurred adding price feed server with ID '%v'", serviceId)
	}
	if err := checker.WaitForStartup(priceFeedTimeBetweenIsAvailablePolls, priceFeedMaxIsAvailablePolls); err != nil {
		return nil, stacktrace.Propagate(err, "An error occurred waiting for price feed server with ID '%v' to start", serviceId)
	}
	castedService, ok := uncastedService.(*price_feed_server.PriceFeedServer)
	if !ok {
		return nil, stacktrace.NewError("Could not downcast generic service interface to price feed server interface for service with ID '%v'", serviceId)
	}
	return castedService, nil
}

// TODO Delete this
/*
func generateJobSpec(oracleContractAddress string) string {
	return fmt.Sprintf(`{
		  "initiators": [
			{
			  "type": "RunLog",
			  "params": { "address": "%v" }
			}
		  ],
		  "tasks": [
				{
				  "type": "HttpGetWithUnrestrictedNetworkAccess"
				},
				{
				  "type": "JsonParse"
				},
				{
				  "type": "Multiply"
				},
				{
				  "type": "EthInt256"
				},
				{
				  "type": "EthTx"
				}
		  ]
		}`, oracleContractAddress)
}
*/

func generateOcrJobSpecTomlStr(
		oracleContractAddress common.Address,
		bootstrapIpAddr string,
		bootstrapPeerToPeerListenPort int,
		bootstrapPeerToPeerId string,
		nodePeerToPeerId string,
		isBootstrapPeer bool,
		nodeOcrKeyBundleId string,
		nodeEthTransmitterAddress common.Address,
		datasourceUrlStr string) (string, error) {
	// TODO Add an EthInt256 step to this??
	// TODO Modify the tcp port for the p2pBootstrapPeers??
	// TODO Replace this string with an actual structured object from https://github.com/smartcontractkit/chainlink/blob/2f2dc24f3ef6a63a47d7a3a4d2c23239d89555c0/core/services/job/models.go#L101

	// Values not being used:
	// contractConfigTrackerSubscribeInterval = "2m"
	result := fmt.Sprintf(
		`type               = "offchainreporting"
schemaVersion      = 1
contractAddress    = "%v"
p2pBootstrapPeers  = [
	"/ip4/%v/tcp/%v/p2p/%v",
]
p2pPeerID          = "%v"
isBootstrapPeer    = %v
keyBundleID        = "%v"
transmitterAddress = "%v"
maxTaskDuration = "11s"
observationTimeout = "13s"   # This must be in range [1s,20s]
blockchainTimeout = "20s"
contractConfigTrackerPollInterval = "1m"
contractConfigTrackerSubscribeInterval = "2m"
contractConfigConfirmations = 3`,
		oracleContractAddress.Hex(),
		bootstrapIpAddr,
		bootstrapPeerToPeerListenPort,
		bootstrapPeerToPeerId,
		nodePeerToPeerId,
		isBootstrapPeer,
		nodeOcrKeyBundleId,
		nodeEthTransmitterAddress.Hex())

	// Only non-bootstrap peers have observation source according to:
	// https://github.com/smartcontractkit/chainlink/blob/c08a98c74ec591b471b7a960af9019b9fbb1b6b3/core/services/offchainreporting/validate.go#L80
	if !isBootstrapPeer {
		observationSourceStr := fmt.Sprintf(`observationSource = """
	// data source 1
	ds1          [type=http allowunrestrictednetworkaccess=true method=GET url="%v" requestData="{}"];
	ds1_parse    [type=jsonparse path="USD"];
	ds1_multiply [type=multiply times=10];

	ds1 -> ds1_parse -> ds1_multiply -> answer;
	answer [type=median];
"""`,
			datasourceUrlStr)
		result = fmt.Sprintf("%v\n%v", result, observationSourceStr)
	}
	return result, nil
}
