package chainlink_contract_deployer

import (
	"fmt"
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
	"github.com/palantir/stacktrace"
)

const (
	migrationConfigurationFileName = "truffle-config.js"

	// TODO TODO TODO This is duplicated - refactor so that this is shared with geth service
	testVolumeMountpoint = "/test-volume"
)

type ChainlinkContractDeployerService struct {
	serviceCtx *services.ServiceContext
}

func NewChainlinkContractDeployerService(serviceCtx *services.ServiceContext) *ChainlinkContractDeployerService {
	return &ChainlinkContractDeployerService{serviceCtx: serviceCtx}
}

func (deployer ChainlinkContractDeployerService) overwriteMigrationIPAddress(nodeIpAddress string) error {
	overwriteMigrationIPAddressCommand := []string{
		"sed",
		"-ie",
		fmt.Sprintf("s/host:\\ 'localhost'/host:\\ '%v'/g %v", nodeIpAddress, migrationConfigurationFileName),
	}
	errorCode, err := deployer.serviceCtx.ExecCommand(overwriteMigrationIPAddressCommand)
	if err != nil {
		return stacktrace.Propagate(err, "Failed to execute command on contract deployer service.")
	} else if errorCode != 0 {
		return stacktrace.NewError("Got a non-zero exit code executing IP address override for contract migration: %v", errorCode)
	}
	return nil
}

func (deployer ChainlinkContractDeployerService) overwriteMigrationPort(port string) error {
	overwriteMigrationPortCommand := []string{
		"sed",
		"-ie",
		fmt.Sprintf("s/port:\\ 8545/port:\\ %v/g %v", port, migrationConfigurationFileName),
	}
	errorCode, err := deployer.serviceCtx.ExecCommand(overwriteMigrationPortCommand)
	if err != nil {
		return stacktrace.Propagate(err, "Failed to execute command on contract deployer service.")
	} else if errorCode != 0 {
		return stacktrace.NewError("Got a non-zero exit code executing port override for contract migration: %v", errorCode)
	}
	return nil
}

func (deployer ChainlinkContractDeployerService) DeployContract(gethServiceIpAddress string, gethServicePort string) error {
	err := deployer.overwriteMigrationIPAddress(gethServiceIpAddress)
	if err != nil {
		return stacktrace.Propagate(err, "Failed to deploy $LINK contract.")
	}
	err = deployer.overwriteMigrationPort(gethServicePort)
	if err != nil {
		return stacktrace.Propagate(err, "Failed to deploy $LINK contract.")
	}
	migrateCommand := []string{
		"yarn",
		"migrate:v0.4",
	}
	errorCode, err := deployer.serviceCtx.ExecCommand(migrateCommand)
	if err != nil {
		return stacktrace.Propagate(err, "Failed to execute yarn migration command on contract deployer service.")
	} else if errorCode != 0 {
		return stacktrace.NewError("Got a non-zero exit code executing yarn migration for contract deployment: %v", errorCode)
	}
	return nil
}

// ===========================================================================================
//                              Service interface methods
// ===========================================================================================

func (deployer ChainlinkContractDeployerService) IsAvailable() bool {
	return true
}
