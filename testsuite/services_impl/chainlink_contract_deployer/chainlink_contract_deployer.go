package chainlink_contract_deployer

import (
	"fmt"
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
	"github.com/kurtosistech/chainlink-testing/testsuite/services_impl/geth"
	"github.com/palantir/stacktrace"
	"github.com/sirupsen/logrus"
	"os"
)

const (
	migrationConfigurationFileName = "truffle-config.js"
	execLogFilename = "dockerExecLogs.log"
	defaultTruffleConfigHost = "127.0.0.1"
	devNetworkId = "cldev"

	// TODO TODO TODO This is duplicated - refactor so that this is shared with geth service
	testVolumeMountpoint = "/test-volume"
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
		fmt.Sprintf("sed -ie \"s/host:\\ '%v'/host:\\ '%v'/g\" %v >> %v",
			defaultTruffleConfigHost,
			nodeIpAddress,
			migrationConfigurationFileName,
			testVolumeMountpoint + "/" + execLogFilename),
	}
	errorCode, err := deployer.serviceCtx.ExecCommand(overwriteMigrationIPAddressCommand)
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
		fmt.Sprintf("sed -ie 's/port: 8545/port: %v, from: \"%v\"/g' %v >> %v && cat %v >> %v",
			port,
			geth.FirstAccountPublicKey,
			migrationConfigurationFileName,
			testVolumeMountpoint + "/" + execLogFilename,
			migrationConfigurationFileName,
			testVolumeMountpoint + "/" + execLogFilename,),
	}
	logrus.Infof("migration command: %+v", overwriteMigrationPortCommand)
	errorCode, err := deployer.serviceCtx.ExecCommand(overwriteMigrationPortCommand)
	if err != nil {
		return stacktrace.Propagate(err, "Failed to execute command on contract deployer service.")
	} else if errorCode != 0 {
		return stacktrace.NewError("Got a non-zero exit code executing port override for contract migration: %v", errorCode)
	}
	return nil
}

func (deployer *ChainlinkContractDeployerService) DeployContract(gethServiceIpAddress string, gethServicePort string) error {
	err := deployer.overwriteMigrationIPAddress(gethServiceIpAddress)
	if err != nil {
		return stacktrace.Propagate(err, "Failed to deploy $LINK contract.")
	}
	err = deployer.overwriteMigrationPort(gethServicePort)
	if err != nil {
		return stacktrace.Propagate(err, "Failed to deploy $LINK contract.")
	}

	migrateCommand := []string{
		"/bin/sh",
		"-c",
		fmt.Sprintf("yarn migrate:dev >> %v",
			testVolumeMountpoint + string(os.PathSeparator) + execLogFilename),
	}
	errorCode, err := deployer.serviceCtx.ExecCommand(migrateCommand)
	if err != nil {
		return stacktrace.Propagate(err, "Failed to execute yarn migration command on contract deployer service.")
	} else if errorCode != 0 {
		return stacktrace.NewError("Got a non-zero exit code executing yarn migration for contract deployment: %v", errorCode)
	}
	deployer.isContractDeployed = true
	return nil
}

func (deployer ChainlinkContractDeployerService) FundLinkWalletContract() error {
	fundLinkWalletCommand := []string{
		"/bin/sh",
		"-c",
		fmt.Sprintf("npx truffle exec scripts/fund-contract.js --network %v >> %v",
			devNetworkId,
			testVolumeMountpoint + string(os.PathSeparator) + execLogFilename),
	}
	// We don't check the error code here because the fund-contract script from Chainlink
	// erroneously reports failures, see: https://github.com/smartcontractkit/box/issues/63
	_, err := deployer.serviceCtx.ExecCommand(fundLinkWalletCommand)
	if err != nil {
		return stacktrace.Propagate(err, "Failed to execute $LINK funding command on contract deployer service.")
	}
	return nil
}

// ===========================================================================================
//                              Service interface methods
// ===========================================================================================

func (deployer ChainlinkContractDeployerService) IsAvailable() bool {
	return true
}
