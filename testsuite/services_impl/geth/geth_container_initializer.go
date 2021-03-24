package geth

import (
	"fmt"
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
	"github.com/kurtosistech/chainlink-testing/testsuite/services_impl/geth/genesis"
	"github.com/palantir/stacktrace"
	"os"
)

const (
	rpcPort       = 8545
	wsPort 		  = 8546
	discoveryPort = 30303

	httpExposedApisString  = "admin,eth,net,web3,miner,personal,txpool,debug"
	wsExposedApisString    = "admin,eth,net,web3,miner,personal,txpool,debug"
	keystoreFilename       = "keystore"
	genesisJsonFilename    = "genesis.json"
	passwordFilename       = "password.txt"
	gasPrice               = 1
	gethDataMountedDirpath = "/geth-data-dir"
	gethTgzDataDir         = "geth-data-dir"
	privateKeyFilePassword = "password"
	targetGasLimit         = 10000000

	FirstFundedAddress  = "0x8eA1441a74ffbE9504a8Cb3F7e4b7118d8CcFc56"

	// The geth node opens a socket for IPC communication in the genesis directory.
	// This socket opening does not work on mounted filesystems, so runtime genesis directory needs to be off the mount.
	// See: https://github.com/ethereum/go-ethereum/issues/16342
	gethDataRuntimeDirpath = "/genesis"

	PrivateKeyPassword = "password"
	PrivateNetworkId     = 9
	TestVolumeMountpoint = "/test-volume"
)

type GethContainerInitializer struct {
	dockerImage string
	dataDirArtifactId services.FilesArtifactID
	gethBootstrapperService *GethService
	isMiner bool
}

func NewGethContainerInitializer(dockerImage string, dataDirArtifactId services.FilesArtifactID, gethBootstrapperService *GethService, isMiner bool) *GethContainerInitializer {
	return &GethContainerInitializer{
		dockerImage: dockerImage,
		dataDirArtifactId: dataDirArtifactId,
		gethBootstrapperService: gethBootstrapperService,
		isMiner: isMiner,
	}
}

func (initializer GethContainerInitializer) GetDockerImage() string {
	return initializer.dockerImage
}

func (initializer GethContainerInitializer) GetUsedPorts() map[string]bool {
	return map[string]bool{
		fmt.Sprintf("%v/tcp", rpcPort):       true,
		fmt.Sprintf("%v/tcp", wsPort):       true,
		fmt.Sprintf("%v/udp", discoveryPort): true,
		fmt.Sprintf("%v/tcp", discoveryPort): true,
	}
}

func (initializer GethContainerInitializer) GetService(ctx *services.ServiceContext) services.Service {
	return NewGethService(ctx, rpcPort);
}

func (initializer GethContainerInitializer) GetFilesToGenerate() map[string]bool {
	return map[string]bool{
		genesisJsonFilename: true,
		passwordFilename: true,
	}
}

func (initializer GethContainerInitializer) InitializeGeneratedFiles(mountedFiles map[string]*os.File) error {
	genesisJson := genesis.GenesisJson
	genesisFp := mountedFiles[genesisJsonFilename]
	_, err := genesisFp.WriteString(genesisJson)
	if err != nil {
		return stacktrace.Propagate(err, "Failed to write genesis config.")
	}
	_, err = mountedFiles[passwordFilename].WriteString(privateKeyFilePassword)
	if err != nil {
		return stacktrace.Propagate(err, "Failed to write password file.")
	}
	return nil
}

func (initializer GethContainerInitializer) GetFilesArtifactMountpoints() map[services.FilesArtifactID]string {
	return map[services.FilesArtifactID]string{
		initializer.dataDirArtifactId: gethDataMountedDirpath,
	}
}

func (initializer GethContainerInitializer) GetTestVolumeMountpoint() string {
	return TestVolumeMountpoint
}

func (initializer GethContainerInitializer) GetEnvironmentVariableOverrides() (map[string]string, error) {
	return map[string]string{}, nil
}

func (initializer GethContainerInitializer) GetStartCommandOverrides(mountedFileFilepaths map[string]string, ipPlaceholder string) (entrypointArgs []string, cmdArgs []string, resultErr error) {
	// This is a bootstrapper
	entrypointCommand := fmt.Sprintf("mkdir -p %v && cp -r %v/%v/* %v/ && ", gethDataRuntimeDirpath, gethDataMountedDirpath, gethTgzDataDir, gethDataRuntimeDirpath)
	entrypointCommand += fmt.Sprintf("geth init --datadir %v %v && ", gethDataRuntimeDirpath, mountedFileFilepaths[genesisJsonFilename])
	entrypointCommand += fmt.Sprintf("geth --nodiscover --verbosity 4 --keystore %v --datadir %v --networkid %v ",
		gethDataRuntimeDirpath + string(os.PathSeparator) + keystoreFilename,
		gethDataRuntimeDirpath,
		PrivateNetworkId)
	entrypointCommand += fmt.Sprintf("-http --http.api %v --http.addr %v --http.corsdomain '*' --nat extip:%v --gcmode archive --syncmode full ",
		httpExposedApisString,
		ipPlaceholder,
		ipPlaceholder)
	// Chainlink oracles require websocket communication
	entrypointCommand += fmt.Sprintf("--ws --ws.addr %v --ws.port %v --ws.api %v --ws.origins=\"*\" ", ipPlaceholder, wsPort, wsExposedApisString)
	if initializer.isMiner {
		entrypointCommand += fmt.Sprintf("--mine --miner.threads=1 --miner.etherbase=%v --miner.gasprice=%v --miner.gaslimit=%v ",
			FirstFundedAddress, gasPrice, targetGasLimit)
		// unlock the first account for use in spawning $LINK contract and distributing funds.
		entrypointCommand += fmt.Sprintf("--unlock %v --password %v  --allow-insecure-unlock ", FirstFundedAddress, mountedFileFilepaths[passwordFilename])
	}
	if initializer.gethBootstrapperService != nil {
		bootnodeEnodeRecord, err := initializer.gethBootstrapperService.GetEnodeAddress()
		if err != nil {
			return nil, nil, stacktrace.Propagate(err, "Failed to get bootnode enode record.")
		}
		entrypointCommand += fmt.Sprintf("--bootnodes %v", bootnodeEnodeRecord)
	}

	entrypointArgs = []string{
		"/bin/sh",
		"-c",
		entrypointCommand,
	}
	return entrypointArgs, nil, nil
}
