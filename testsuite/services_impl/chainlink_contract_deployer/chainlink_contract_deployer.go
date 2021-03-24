package chainlink_contract_deployer

import (
	"fmt"
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
	"github.com/kurtosistech/chainlink-testing/testsuite/services_impl/geth"
	"github.com/palantir/stacktrace"
	"github.com/sirupsen/logrus"
	"strings"
)

const (
	migrationConfigurationFileName = "truffle-config.js"
	defaultTruffleConfigHost = "127.0.0.1"
	devNetworkId = "cldev"
	priceFeedUrl = "https://min-api.cryptocompare.com/data/price?fsym=ETH&tsyms=USD"

	contractAddressSplitter   = "contract address:"
	addressContentSplitter    = "\n"
	linkTokenContractSplitter = "Deploying 'LinkToken'\n"
	oracleContractSplitter      = "Deploying 'Oracle'\n"
	myContractSplitter      = "Deploying 'MyContract'\n"
	setOracleFulfillmentPermissionsPath = "ethers_js_scripts/setOracleFulfillmentPermissions.js"

	// TODO TODO TODO This is duplicated - refactor so that this is shared with geth service
	testVolumeMountpoint = geth.TestVolumeMountpoint
)

type ChainlinkContractDeployerService struct {
	serviceCtx *services.ServiceContext
	isContractDeployed bool
}

func NewChainlinkContractDeployerService(serviceCtx *services.ServiceContext) *ChainlinkContractDeployerService {
	return &ChainlinkContractDeployerService{serviceCtx: serviceCtx}
}

func (deployer *ChainlinkContractDeployerService) overwriteMigrationIPAddress(nodeIpAddress string) error {
	overwriteMigrationIPAddressCommand := []string{
		"/bin/sh",
		"-c",
		fmt.Sprintf("sed -ie \"s/host:\\ '%v'/host:\\ '%v'/g\" %v",
			defaultTruffleConfigHost,
			nodeIpAddress,
			migrationConfigurationFileName),
	}
	errorCode, _, err := deployer.serviceCtx.ExecCommand(overwriteMigrationIPAddressCommand)
	if err != nil {
		return stacktrace.Propagate(err, "Failed to execute command on contract deployer service.")
	} else if errorCode != 0 {
		return stacktrace.NewError("Got a non-zero exit code executing IP address override for contract migration: %v", errorCode)
	}
	return nil
}

func (deployer *ChainlinkContractDeployerService) overwriteMigrationPort(port string) error {
	overwriteMigrationPortCommand := []string{
		"/bin/sh",
		"-c",
		fmt.Sprintf("sed -ie 's/port: 8545/port: %v, from: \"%v\"/g' %v",
			port,
			geth.FirstFundedAddress,
			migrationConfigurationFileName,),
	}
	errorCode, _, err := deployer.serviceCtx.ExecCommand(overwriteMigrationPortCommand)
	if err != nil {
		return stacktrace.Propagate(err, "Failed to execute command on contract deployer service.")
	} else if errorCode != 0 {
		return stacktrace.NewError("Got a non-zero exit code executing port override for contract migration: %v", errorCode)
	}
	return nil
}

func (deployer *ChainlinkContractDeployerService) DeployContract(gethServiceIpAddress string, gethServicePort string) (linkAddress string, oracleAddress string, err error) {
	err = deployer.overwriteMigrationIPAddress(gethServiceIpAddress)
	if err != nil {
		return "", "", stacktrace.Propagate(err, "Failed to deploy $LINK contract.")
	}
	err = deployer.overwriteMigrationPort(gethServicePort)
	if err != nil {
		return "", "", stacktrace.Propagate(err, "Failed to deploy $LINK contract.")
	}

	migrateCommand := []string{
		"/bin/sh",
		"-c",
		fmt.Sprintf("yarn migrate:dev",),
	}
	errorCode, logOutput, err := deployer.serviceCtx.ExecCommand(migrateCommand)
	if err != nil {
		return "", "", stacktrace.Propagate(err, "Failed to execute yarn migration command on contract deployer service.")
	} else if errorCode != 0 {
		return "", "", stacktrace.NewError("Got a non-zero exit code executing yarn migration for contract deployment: %v", errorCode)
	}
	logOutputStr := string(*logOutput)
	logrus.Debugf("Log output from contract deploy: %+v", logOutputStr)
	linkAddress, err = parseContractAddressFromTruffleMigrate(logOutputStr, linkTokenContractSplitter, oracleContractSplitter)
	if err != nil {
		return "", "", stacktrace.Propagate(err, "Failed to parse contract linkAddress.")
	}
	oracleAddress, err = parseContractAddressFromTruffleMigrate(logOutputStr, oracleContractSplitter, myContractSplitter)
	deployer.isContractDeployed = true
	return linkAddress, oracleAddress,nil
}

func (deployer ChainlinkContractDeployerService) FundLinkWalletContract() error {
	fundLinkWalletCommand := []string{
		"/bin/sh",
		"-c",
		fmt.Sprintf("npx truffle exec scripts/fund-contract.js --network %v",
			devNetworkId,),
	}
	// We don't check the error code here because the fund-contract script from Chainlink
	// erroneously reports failures, see: https://github.com/smartcontractkit/box/issues/63
	_, logOutput, err := deployer.serviceCtx.ExecCommand(fundLinkWalletCommand)
	if err != nil {
		return stacktrace.Propagate(err, "Failed to execute $LINK funding command on contract deployer service.")
	}
	logOutputStr := string(*logOutput)
	logrus.Debugf("Log output from funding wallet: %+v", logOutputStr)
	return nil
}

func (deployer ChainlinkContractDeployerService) SetFulfillmentPermissions(gethServiceIpAddress string, gethServicePort string,
		oracleContractAddress string, oracleEthereumAccount string) error {
	setPermissionCommand := []string {
		"/bin/sh",
		"-c",
		fmt.Sprintf("export ETH_RPC_URL=http://%v:%v && " +
			"export PRIVATE_KEY_JSON_PASSWORD=%v && " +
			"export ORACLE_CONTRACT_ADDRESS=%v && " +
			"export ORACLE_ETHEREUM_ADDRESS=%v && " +
			"node %v", gethServiceIpAddress, gethServicePort, geth.PrivateKeyPassword,
				oracleContractAddress, oracleEthereumAccount, setOracleFulfillmentPermissionsPath),
	}
	statusCode, logOutput, err := deployer.serviceCtx.ExecCommand(setPermissionCommand)
	if err != nil {
		return stacktrace.Propagate(err, "Failed to execute set permission script.")
	}
	if statusCode != 0 {
		return stacktrace.NewError("Got a non-zero exit code from set permission script with logOutput %v", logOutput)
	}
	return nil
}

func (deployer ChainlinkContractDeployerService) RunRequestDataScript(oracleContractAddress string, jobId string) error {
	requestDataCommand := []string{
		"/bin/sh",
		"-c",
		fmt.Sprintf("export TRUFFLE_CL_BOX_ORACLE_ADDRESS=%v && " +
			"export TRUFFLE_CL_BOX_JOB_ID=%v && " +
			"export TRUFFLE_CL_BOX_URL=%v && " +
			"npx truffle exec scripts/request-data.js --network %v",
			oracleContractAddress, jobId, priceFeedUrl, devNetworkId),
	}
	// We don't check the error code here because the fund-contract script from Chainlink
	// erroneously reports failures, see: https://github.com/smartcontractkit/box/issues/63
	_, logOutput, err := deployer.serviceCtx.ExecCommand(requestDataCommand)
	if err != nil {
		return stacktrace.Propagate(err, "Failed to execute request data command on contract deployer service.")
	}
	logOutputStr := string(*logOutput)
	logrus.Debugf("Log output from requesting data: %+v", logOutputStr)
	return nil
}

// ===========================================================================================
//                              Service interface methods
// ===========================================================================================

func (deployer ChainlinkContractDeployerService) IsAvailable() bool {
	return true
}

// ===========================================================================================
//                              Helper functions
// ===========================================================================================

func parseContractAddressFromTruffleMigrate(logOutputStr string, contractSplitter string, nextContractSplitter string) (string, error) {
	splitOnContract := strings.Split(logOutputStr, contractSplitter)
	splitCount := len(splitOnContract)
	if splitCount != 2 {
		return "", stacktrace.NewError("Expected truffle migrate command output to split into two on %+v, instead split into %v",
			contractSplitter,
			splitCount)
	}
	splitOnNextContract := strings.Split(splitOnContract[1], nextContractSplitter)
	splitCount = len(splitOnNextContract)
	if splitCount != 2 {
		return "", stacktrace.NewError("Expected link token contract suffix to split into two on %v, instead split into %v",
			nextContractSplitter,
			splitCount)
	}
	contractInfo := splitOnNextContract[0]
	splitOnContractAddress := strings.Split(contractInfo, contractAddressSplitter)
	splitCount = len(splitOnContractAddress)
	if splitCount != 2 {
		return "", stacktrace.NewError("Expected link token contract info to split into two on %v, instead split into %v",
			contractAddressSplitter,
			splitCount)
	}
	splitOnAddressContent := strings.Split(strings.TrimSpace(splitOnContractAddress[1]), addressContentSplitter)
	splitCount = len(splitOnAddressContent)
	if splitCount < 2 {
		return "", stacktrace.NewError("Expected address content to split into at least two on %v, instead split into %v",
			addressContentSplitter,
			splitCount)
	}
	address := splitOnAddressContent[0]
	return strings.TrimSpace(address), nil
}