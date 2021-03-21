package chainlink_contract_deployer

import (
	"fmt"
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
	"os"
	"strconv"
)

const (
	sleepSeconds = 7200
	defaultLinkFunding = 1000000000000000000000
)

type ChainlinkContractDeployerInitializer struct {
	dockerImage string
}

func NewChainlinkContractDeployerInitializer(dockerImage string) *ChainlinkContractDeployerInitializer {
	return &ChainlinkContractDeployerInitializer{
		dockerImage: dockerImage,
	}
}

func (initializer ChainlinkContractDeployerInitializer) GetDockerImage() string {
	return initializer.dockerImage
}

func (initializer ChainlinkContractDeployerInitializer) GetUsedPorts() map[string]bool {
	return map[string]bool{}
}

func (initializer ChainlinkContractDeployerInitializer) GetService(ctx *services.ServiceContext) services.Service {
	return NewChainlinkContractDeployerService(ctx);
}

func (initializer ChainlinkContractDeployerInitializer) GetFilesToGenerate() map[string]bool {
	return map[string]bool{}
}

func (initializer ChainlinkContractDeployerInitializer) InitializeGeneratedFiles(mountedFiles map[string]*os.File) error {
	return nil
}

func (initializer ChainlinkContractDeployerInitializer) GetFilesArtifactMountpoints() map[services.FilesArtifactID]string {
	return map[services.FilesArtifactID]string{}
}

func (initializer ChainlinkContractDeployerInitializer) GetTestVolumeMountpoint() string {
	return testVolumeMountpoint
}

func (initializer ChainlinkContractDeployerInitializer) GetEnvironmentVariableOverrides() (map[string]string, error) {
	return map[string]string{
		"TRUFFLE_CL_BOX_PAYMENT": strconv.Itoa(defaultLinkFunding),
	}, nil
}

func (initializer ChainlinkContractDeployerInitializer) GetStartCommandOverrides(mountedFileFilepaths map[string]string, ipPlaceholder string) (entrypointArgs []string, cmdArgs []string, resultErr error) {
	entrypointArgs = []string{
		"/bin/sh",
		"-c",
		fmt.Sprintf("sleep %v", sleepSeconds),
	}
	return entrypointArgs, nil, nil
}

