package chainlink_oracle

import (
	"fmt"
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
	"github.com/kurtosistech/chainlink-testing/testsuite/services_impl/geth"
	"github.com/kurtosistech/chainlink-testing/testsuite/services_impl/postgres"
	"github.com/sirupsen/logrus"
	"os"
)

const (
	oracleEmail = "user@example.com"
	oraclePassword = "password"
	oracleWalletPassword = "walletPassword"

	passwordFileKey = "password-file"
	apiFileKey = "api-file"
	envFileKey = "env-file"

	port = 6688
)

type ChainlinkOracleInitializer struct {
	dockerImage         string
	linkContractAddress string
	gethClient	*geth.GethService
	postgresService	*postgres.PostgresService
}

func NewChainlinkOracleContainerInitializer(dockerImage string, linkContractAddress string,
	gethClient *geth.GethService, postgresService *postgres.PostgresService) *ChainlinkOracleInitializer {
	return &ChainlinkOracleInitializer{
		dockerImage:         dockerImage,
		linkContractAddress: linkContractAddress,
		gethClient: gethClient,
		postgresService: postgresService,
	}
}

func (initializer ChainlinkOracleInitializer) GetDockerImage() string {
	return initializer.dockerImage
}

func (initializer ChainlinkOracleInitializer) GetUsedPorts() map[string]bool {
	return map[string]bool{
		fmt.Sprintf("%v/tcp", port): true,
	}
}

func (initializer ChainlinkOracleInitializer) GetServiceWrappingFunc() func(ctx *services.ServiceContext) services.Service {
	return func(ctx *services.ServiceContext) services.Service {
		return NewChainlinkOracleService(ctx);
	};
}

func (initializer ChainlinkOracleInitializer) GetFilesToGenerate() map[string]bool {
	return map[string]bool{
		envFileKey: true,
		passwordFileKey: true,
		apiFileKey: true,
	}
}

func (initializer ChainlinkOracleInitializer) InitializeGeneratedFiles(mountedFiles map[string]*os.File) error {
	envFileString := getOracleEnvFile(
		string(geth.PrivateNetworkId), initializer.linkContractAddress,
		initializer.gethClient.GetIPAddress(), string(initializer.gethClient.GetWsPort()),
		initializer.postgresService.GetSuperUsername(), initializer.postgresService.GetSuperUserPassword(),
		initializer.postgresService.GetIPAddress(), string(initializer.postgresService.GetPort()),
		initializer.postgresService.GetDatabaseName())
	logrus.Infof("Env File: \n%v", envFileString)
	return nil
}

func (initializer ChainlinkOracleInitializer) GetFilesArtifactMountpoints() map[services.FilesArtifactID]string {
	return map[services.FilesArtifactID]string{}
}

func (initializer ChainlinkOracleInitializer) GetTestVolumeMountpoint() string {
	return geth.TestVolumeMountpoint
}

func (initializer ChainlinkOracleInitializer) GetStartCommandOverrides(mountedFileFilepaths map[string]string, ipPlaceholder string) (entrypointArgs []string, cmdArgs []string, resultErr error) {
	entrypointArgs = []string{
	}

	cmdArgs = []string{
		"local",
		"n",
		fmt.Sprintf("--env-file=%v", mountedFileFilepaths[envFileKey]),
		"-p",
		fmt.Sprintf("%v", mountedFileFilepaths[passwordFileKey]),
		"-a",
		fmt.Sprintf("%v", mountedFileFilepaths[apiFileKey]),
	}

	return nil, cmdArgs, nil
}




// ==========================================================================================
//								Helper methods
// ==========================================================================================

func getOracleEnvFile(chainId string, contractAddress string, gethClientIp string,
						gethClientWsPort string, postgresUsername string, postgresPassword string,
						postgresIpAddress string, postgresPort string, postgresDatabase string) string {
	return fmt.Sprintf(`ROOT=/chainlink
LOG_LEVEL=debug
ETH_CHAIN_ID=%v
MIN_OUTGOING_CONFIRMATIONS=2
LINK_CONTRACT_ADDRESS=%v
CHAINLINK_TLS_PORT=0
SECURE_COOKIES=false
GAS_UPDATER_ENABLED=true
ALLOW_ORIGINS=*
ETH_URL=ws://%v:%v
DATABASE_URL=postgresql://%v:%v@%v:%v/%v`, chainId, contractAddress,
	gethClientIp, gethClientWsPort,
	postgresUsername, postgresPassword, postgresIpAddress, postgresPort, postgresDatabase)
}

func getOracleApiFile(username string, password string) string {
	return fmt.Sprintf(`%v
%v`, username, password)
}

func getOraclePasswordFile(walletPassword string) string {
	return fmt.Sprintf("%v", walletPassword)
}