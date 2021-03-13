package chainlink_oracle

import (
	"fmt"
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
	"github.com/kurtosistech/chainlink-testing/testsuite/services_impl/geth"
	"github.com/kurtosistech/chainlink-testing/testsuite/services_impl/postgres"
	"github.com/palantir/stacktrace"
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
		geth.PrivateNetworkId, initializer.linkContractAddress,
		initializer.gethClient.GetIPAddress(), initializer.gethClient.GetWsPort(),
		initializer.postgresService.GetSuperUsername(), initializer.postgresService.GetSuperUserPassword(),
		initializer.postgresService.GetIPAddress(), initializer.postgresService.GetPort(),
		initializer.postgresService.GetDatabaseName())
	passwordFileString := getOraclePasswordFile(oracleWalletPassword)
	apiFileString := getOracleApiFile(oracleEmail, oraclePassword)
	logrus.Infof("Env File: \n%v", envFileString)
	logrus.Infof("Password File: \n%v", passwordFileString)
	logrus.Infof("API File: \n%v", apiFileString)

	envFileFp := mountedFiles[envFileKey]
	_, err := envFileFp.WriteString(envFileString)
	if err != nil {
		return stacktrace.Propagate(err, "Failed to generate environment file.")
	}
	passwordFileFp := mountedFiles[passwordFileKey]
	_, err = passwordFileFp.WriteString(passwordFileString)
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

func (initializer ChainlinkOracleInitializer) GetStartCommandOverrides(mountedFileFilepaths map[string]string, ipPlaceholder string) (entrypointArgs []string, cmdArgs []string, resultErr error) {
	entrypointArgs = []string{
		"/bin/bash",
		"-c",
		fmt.Sprintf("source %v && chainlink local n -p %v -a %v",
			fmt.Sprintf(mountedFileFilepaths[envFileKey]),
			fmt.Sprintf(mountedFileFilepaths[passwordFileKey]),
			fmt.Sprintf(mountedFileFilepaths[apiFileKey]),
		),
	}

	/*cmdArgs = []string{
		fmt.Sprintf("--env-file=%v", mountedFileFilepaths[envFileKey]),
		"local",
		"n",
		"-p",
		fmt.Sprintf("%v", mountedFileFilepaths[passwordFileKey]),
		"-a",
		fmt.Sprintf("%v", mountedFileFilepaths[apiFileKey]),
	}*/

	return entrypointArgs, nil, nil
}




// ==========================================================================================
//								Helper methods
// ==========================================================================================

func getOracleEnvFile(chainId int, contractAddress string, gethClientIp string,
						gethClientWsPort int, postgresUsername string, postgresPassword string,
						postgresIpAddress string, postgresPort int, postgresDatabase string) string {
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