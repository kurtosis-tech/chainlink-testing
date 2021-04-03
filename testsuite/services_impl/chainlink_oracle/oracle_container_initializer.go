package chainlink_oracle

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
	"github.com/kurtosistech/chainlink-testing/testsuite/services_impl/geth"
	"github.com/kurtosistech/chainlink-testing/testsuite/services_impl/geth/genesis"
	"github.com/kurtosistech/chainlink-testing/testsuite/services_impl/postgres"
	"github.com/palantir/stacktrace"
	"os"
	"strconv"
)

const (
	oracleEmail = "user@example.com"
	oraclePassword = "qWeRtY123!@#qWeRtY123!@#"
	oracleWalletPassword = "qWeRtY123!@#qWeRtY123!@#"

	passwordFileKey = "password-file"
	apiFileKey = "api-file"
	envFileKey = "env-file"

	gasUpdaterDelay = 1
	gasPriceBumpThreshold = 2
	ethGasBumpWei = "100000000000000000000"
	gasPriceDefault = 100
	minOutgoingConfirmations = 12
	minIncomingConfirmations = 0

	// TODO Parameterize this
	isOffchainReportingEnabled = true

	// Only set if offchain reporting is enabled
	peer2PeerListenPort = 1234

	operatorUiPort = 6688
)

type ChainlinkOracleInitializer struct {
	dockerImage         string
	linkContractAddress common.Address
	oracleContractAddress common.Address
	gethClient	*geth.GethService
	postgresService	*postgres.PostgresService
}

func NewChainlinkOracleContainerInitializer(
		dockerImage string,
		linkContractAddress common.Address,
		oracleContractAddress common.Address,
		gethClient *geth.GethService,
		postgresService *postgres.PostgresService) *ChainlinkOracleInitializer {
	return &ChainlinkOracleInitializer{
		dockerImage:         dockerImage,
		linkContractAddress: linkContractAddress,
		oracleContractAddress: oracleContractAddress,
		gethClient: gethClient,
		postgresService: postgresService,
	}
}

func (initializer ChainlinkOracleInitializer) GetDockerImage() string {
	return initializer.dockerImage
}

func (initializer ChainlinkOracleInitializer) GetUsedPorts() map[string]bool {
	return map[string]bool{
		fmt.Sprintf("%v/tcp", operatorUiPort): true,
		fmt.Sprintf("%v/tcp", peer2PeerListenPort): true,
	}
}

func (initializer ChainlinkOracleInitializer) GetService(ctx *services.ServiceContext) services.Service {
	return NewChainlinkOracleService(ctx);
}

func (initializer ChainlinkOracleInitializer) GetFilesToGenerate() map[string]bool {
	return map[string]bool{
		envFileKey: true,
		passwordFileKey: true,
		apiFileKey: true,
	}
}

func (initializer ChainlinkOracleInitializer) GetEnvironmentVariableOverrides() (map[string]string, error) {
	result := map[string]string {
		"ROOT": "/chainlink",
		"LOG_LEVEL": "debug",
		"ETH_CHAIN_ID": fmt.Sprintf("%v", genesis.ChainId),
		"MIN_OUTGOING_CONFIRMATIONS": strconv.Itoa(minOutgoingConfirmations),
		"MIN_INCOMING_CONFIRMATIONS": strconv.Itoa(minIncomingConfirmations),
		"ETH_GAS_PRICE_DEFAULT": strconv.Itoa(gasPriceDefault),
		"ETH_GAS_BUMP_THRESHOLD": strconv.Itoa(gasPriceBumpThreshold),
		"ETH_GAS_BUMP_WEI": ethGasBumpWei,
		"LINK_CONTRACT_ADDRESS": initializer.linkContractAddress.Hex(),
		"OPERATOR_CONTRACT_ADDRESS": initializer.oracleContractAddress.Hex(),
		"CHAINLINK_TLS_PORT": "0",
		"SECURE_COOKIES": "false",
		"GAS_UPDATER_ENABLED": "true",
		"GAS_UPDATER_BLOCK_DELAY": strconv.Itoa(gasUpdaterDelay),
		"ALLOW_ORIGINS":"*",
		"FEATURE_OFFCHAIN_REPORTING": strconv.FormatBool(isOffchainReportingEnabled),
		"P2P_LISTEN_PORT": strconv.Itoa(peer2PeerListenPort), // Required when offchain reporting == true
		"ETH_URL": fmt.Sprintf("ws://%v:%v", initializer.gethClient.GetIPAddress(), initializer.gethClient.GetWsPort()),
		"DATABASE_URL": fmt.Sprintf("postgresql://%v:%v@%v:%v/%v?sslmode=disable",
			initializer.postgresService.GetSuperUsername(), initializer.postgresService.GetSuperUserPassword(),
			initializer.postgresService.GetIPAddress(), initializer.postgresService.GetPort(), initializer.postgresService.GetDatabaseName()),
	}
	return result, nil
}

func (initializer ChainlinkOracleInitializer) InitializeGeneratedFiles(mountedFiles map[string]*os.File) error {
	passwordFileString := getOraclePasswordFile(oracleWalletPassword)
	apiFileString := getOracleApiFile(oracleEmail, oraclePassword)

	passwordFileFp := mountedFiles[passwordFileKey]
	_, err := passwordFileFp.WriteString(passwordFileString)
	if err != nil {
		return stacktrace.Propagate(err, "Failed to generate password file.")
	}
	apiFileFp := mountedFiles[apiFileKey]
	_, err = apiFileFp.WriteString(apiFileString)
	if err != nil {
		return stacktrace.Propagate(err, "Failed to generate API file.")
	}
	return nil
}

func (initializer ChainlinkOracleInitializer) GetFilesArtifactMountpoints() map[services.FilesArtifactID]string {
	return map[services.FilesArtifactID]string{}
}

func (initializer ChainlinkOracleInitializer) GetTestVolumeMountpoint() string {
	return geth.TestVolumeMountpoint
}

func (initializer ChainlinkOracleInitializer) GetStartCommandOverrides(mountedFileFilepaths map[string]string, ipAddr string) (entrypointArgs []string, cmdArgs []string, resultErr error) {
	entrypointArgs = []string{
		"/bin/bash",
		"-c",
		fmt.Sprintf("chainlink local n -p %v -a %v",
			fmt.Sprintf(mountedFileFilepaths[passwordFileKey]),
			fmt.Sprintf(mountedFileFilepaths[apiFileKey]),
		),
	}

	return entrypointArgs, nil, nil
}




// ==========================================================================================
//								Helper methods
// ==========================================================================================

func getOracleApiFile(username string, password string) string {
	return fmt.Sprintf(`%v
%v`, username, password)
}

func getOraclePasswordFile(walletPassword string) string {
	return fmt.Sprintf("%v", walletPassword)
}