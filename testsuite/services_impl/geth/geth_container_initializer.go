package geth

import (
	"fmt"
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
	"os"
)

const (
	rpcPort       = 8545
	discoveryPort = 30303

	testVolumeMountpoint = "/"
)

// Fields are public so we can marshal them as JSON
type config struct {
	DatastoreIp string	`json:"datastoreIp"`
	DatastorePort int	`json:"datastorePort"`
}

type GethContainerInitializer struct {
	dockerImage string
}

func NewGethContainerInitializer(dockerImage string) *GethContainerInitializer {
	return &GethContainerInitializer{dockerImage: dockerImage}
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

func (initializer GethContainerInitializer) GetServiceWrappingFunc() func(serviceId services.ServiceID, ipAddr string) services.Service {
	return func(serviceId services.ServiceID, ipAddr string) services.Service {
		return NewGethService(serviceId, ipAddr, rpcPort);
	};
}

func (initializer GethContainerInitializer) GetFilesToGenerate() map[string]bool {
	return map[string]bool{
	}
}

func (initializer GethContainerInitializer) InitializeGeneratedFiles(mountedFiles map[string]*os.File) error {
	return nil
}

func (initializer GethContainerInitializer) GetFilesArtifactMountpoints() map[services.FilesArtifactID]string {
	return map[services.FilesArtifactID]string{}
}

func (initializer GethContainerInitializer) GetTestVolumeMountpoint() string {
	return testVolumeMountpoint
}

func (initializer GethContainerInitializer) GetStartCommand(mountedFileFilepaths map[string]string, ipPlaceholder string) ([]string, error) {
	startCmd := []string{
		"./geth",
		"console",
	}
	return startCmd, nil
}
