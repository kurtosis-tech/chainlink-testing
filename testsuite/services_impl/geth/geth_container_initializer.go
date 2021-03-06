package geth

import (
	"fmt"
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
	"github.com/kurtosistech/chainlink-testing/testsuite/services_impl/geth/data"
	"github.com/palantir/stacktrace"
	"os"
)

const (
	rpcPort       = 8545
	discoveryPort = 30303

	httpExposedApisString = "admin,eth,net,web3,miner,personal,txpool"
	keystoreFilename = "keystore"
	privateNetworkId = 9
	testVolumeMountpoint = "/test-volume"
	genesisJsonFilename = "genesis.json"
	gethDataMountedDirpath = "/geth-mounted-data"

	FirstAccountPublicKey = "0x8eA1441a74ffbE9504a8Cb3F7e4b7118d8CcFc56"

	// The geth node opens a socket for IPC communication in the data directory.
	// This socket opening does not work on mounted filesystems, so runtime data directory needs to be off the mount.
	// See: https://github.com/ethereum/go-ethereum/issues/16342
	gethDataRuntimeDirpath = "/data"
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
		fmt.Sprintf("%v/udp", discoveryPort): true,
		fmt.Sprintf("%v/tcp", discoveryPort): true,
	}
}

func (initializer GethContainerInitializer) GetServiceWrappingFunc() func(ctx *services.ServiceContext) services.Service {
	return func(ctx *services.ServiceContext) services.Service {
		return NewGethService(ctx, rpcPort);
	};
}

func (initializer GethContainerInitializer) GetFilesToGenerate() map[string]bool {
	return map[string]bool{
		genesisJsonFilename: true,
	}
}

func (initializer GethContainerInitializer) InitializeGeneratedFiles(mountedFiles map[string]*os.File) error {
	genesisJson := data.GenesisJson
	genesisFp := mountedFiles[genesisJsonFilename]
	_, err := genesisFp.WriteString(genesisJson)
	if err != nil {
		return stacktrace.Propagate(err, "Failed to write genesis config.")
	}
	return nil
}

func (initializer GethContainerInitializer) GetFilesArtifactMountpoints() map[services.FilesArtifactID]string {
	return map[services.FilesArtifactID]string{
		initializer.dataDirArtifactId: gethDataMountedDirpath,
	}
}

func (initializer GethContainerInitializer) GetTestVolumeMountpoint() string {
	return testVolumeMountpoint
}

func (initializer GethContainerInitializer) GetStartCommandOverrides(mountedFileFilepaths map[string]string, ipPlaceholder string) (entrypointArgs []string, cmdArgs []string, resultErr error) {
	// This is a bootstrapper
	entrypointCommand := fmt.Sprintf("cp -r %v %v && ", gethDataMountedDirpath, gethDataRuntimeDirpath)
	entrypointCommand += fmt.Sprintf("geth init --datadir %v %v && ", gethDataRuntimeDirpath, mountedFileFilepaths[genesisJsonFilename])
	entrypointCommand += fmt.Sprintf("geth --nodiscover --verbosity 4 --keystore %v --datadir %v --networkid %v ",
		gethDataRuntimeDirpath + string(os.PathSeparator) + keystoreFilename,
		gethDataRuntimeDirpath,
		privateNetworkId)
	entrypointCommand += fmt.Sprintf("-http --http.api %v --http.addr %v --http.corsdomain '*' --nat extip:%v ",
		httpExposedApisString,
		ipPlaceholder,
		ipPlaceholder)
	if initializer.isMiner {
		entrypointCommand += fmt.Sprintf("--mine --miner.threads=1 --miner.etherbase=%v ", FirstAccountPublicKey)
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
