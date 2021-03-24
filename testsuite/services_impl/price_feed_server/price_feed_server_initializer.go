package price_feed_server

import (
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
	"os"
)

const (
	testVolumeMountpoint = "/test-volume"
)

type PriceFeedServerInitializer struct {
	dockerImage string
}

func NewPriceFeedServerInitializer(dockerImage string) *PriceFeedServerInitializer {
	return &PriceFeedServerInitializer{
		dockerImage: dockerImage,
	}
}

func (initializer PriceFeedServerInitializer) GetDockerImage() string {
	return initializer.dockerImage
}

func (initializer PriceFeedServerInitializer) GetUsedPorts() map[string]bool {
	return map[string]bool{}
}

func (initializer PriceFeedServerInitializer) GetService(ctx *services.ServiceContext) services.Service {
	return NewPriceFeedServerService(ctx);
}

func (initializer PriceFeedServerInitializer) GetFilesToGenerate() map[string]bool {
	return map[string]bool{}
}

func (initializer PriceFeedServerInitializer) InitializeGeneratedFiles(mountedFiles map[string]*os.File) error {
	return nil
}

func (initializer PriceFeedServerInitializer) GetFilesArtifactMountpoints() map[services.FilesArtifactID]string {
	return map[services.FilesArtifactID]string{}
}

func (initializer PriceFeedServerInitializer) GetTestVolumeMountpoint() string {
	return testVolumeMountpoint
}

func (initializer PriceFeedServerInitializer) GetEnvironmentVariableOverrides() (map[string]string, error) {
	return map[string]string{}, nil
}

func (initializer PriceFeedServerInitializer) GetStartCommandOverrides(mountedFileFilepaths map[string]string, ipPlaceholder string) (entrypointArgs []string, cmdArgs []string, resultErr error) {
	return nil, nil, nil
}

