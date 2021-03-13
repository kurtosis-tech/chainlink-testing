package postgres

import (
	"fmt"
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
	"github.com/kurtosistech/chainlink-testing/testsuite/services_impl/geth"
	"os"
)

const (
	port = 5432

	entrypointScriptPath = "/docker-entrypoint.sh"
	postgresSuperUserPassword = "password"
	postgresSuperUserPasswordEnvVar = "POSTGRES_PASSWORD"
)

type PostgresContainerInitializer struct {
	dockerImage string
}

func NewPostgresContainerInitializer(dockerImage string) *PostgresContainerInitializer {
	return &PostgresContainerInitializer{
		dockerImage: dockerImage,
	}
}

func (initializer PostgresContainerInitializer) GetDockerImage() string {
	return initializer.dockerImage
}

func (initializer PostgresContainerInitializer) GetUsedPorts() map[string]bool {
	return map[string]bool{
		fmt.Sprintf("%v/tcp", port): true,
	}
}

func (initializer PostgresContainerInitializer) GetServiceWrappingFunc() func(ctx *services.ServiceContext) services.Service {
	return func(ctx *services.ServiceContext) services.Service {
		return NewPostgresService(ctx);
	};
}

func (initializer PostgresContainerInitializer) GetFilesToGenerate() map[string]bool {
	return map[string]bool{}
}

func (initializer PostgresContainerInitializer) InitializeGeneratedFiles(mountedFiles map[string]*os.File) error {
	return nil
}

func (initializer PostgresContainerInitializer) GetFilesArtifactMountpoints() map[services.FilesArtifactID]string {
	return map[services.FilesArtifactID]string{}
}

func (initializer PostgresContainerInitializer) GetTestVolumeMountpoint() string {
	return geth.TestVolumeMountpoint
}

func (initializer PostgresContainerInitializer) GetStartCommandOverrides(mountedFileFilepaths map[string]string, ipPlaceholder string) (entrypointArgs []string, cmdArgs []string, resultErr error) {
	entrypointArgs = []string{
		fmt.Sprintf("%v=%v %v",
			postgresSuperUserPasswordEnvVar,
			postgresSuperUserPassword,
			entrypointScriptPath,),
	}

	cmdArgs = []string{
		"-h",
		"*",
		"-p",
		fmt.Sprintf("%v", port),
	}
	return entrypointArgs, cmdArgs, nil
}

