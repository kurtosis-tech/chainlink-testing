package networks_impl

import (
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/networks"
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
	"github.com/kurtosistech/chainlink-testing/testsuite/services_impl/geth"
	"github.com/palantir/stacktrace"
	"strconv"
	"time"
)

const (
	ethereumBootstrapperId services.ServiceID = "ethereum-bootstrapper"
	gethServiceIdPrefix                       = "ethereum-node-"

	waitForStartupTimeBetweenPolls = 1 * time.Second
	waitForStartupMaxNumPolls = 15
)

type ChainlinkNetwork struct {
	networkCtx            *networks.NetworkContext
	gethDataDirArtifactId  services.FilesArtifactID
	gethServiceImage      string
	gethBootsrapperService           *geth.GethService
	gethServices          map[services.ServiceID]*geth.GethService
	nextGethServiceId     int
}

func NewTestNetwork(networkCtx *networks.NetworkContext, gethDataDirArtifactId services.FilesArtifactID, gethServiceImage string) *ChainlinkNetwork {
	return &ChainlinkNetwork{
		networkCtx:            networkCtx,
		gethDataDirArtifactId:	gethDataDirArtifactId,
		gethServiceImage:	   gethServiceImage,
		gethBootsrapperService:      	   nil,
		gethServices:           map[services.ServiceID]*geth.GethService{},
		nextGethServiceId:     0,
	}
}

func (network *ChainlinkNetwork) AddBootstrapper() error {
	if (network.gethBootsrapperService != nil) {
		return stacktrace.NewError("Cannot add bootstrapper service to network; bootstrapper already exists!")
	}

	initializer := geth.NewGethContainerInitializer(network.gethServiceImage, network.gethDataDirArtifactId)
	uncastedDatastore, checker, err := network.networkCtx.AddService(ethereumBootstrapperId, initializer)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred adding the bootstrapper service")
	}
	if err := checker.WaitForStartup(waitForStartupTimeBetweenPolls, waitForStartupMaxNumPolls); err != nil {
		return stacktrace.Propagate(err, "An error occurred waiting for the bootstrapper service to start")
	}
	castedGethBootstrapperService := uncastedDatastore.(*geth.GethService)
	network.gethBootsrapperService = castedGethBootstrapperService
	return nil
}

func (network *ChainlinkNetwork) GetBootstrapper() *geth.GethService {
	return network.gethBootsrapperService
}

func (network *ChainlinkNetwork) AddEthereumNode() (services.ServiceID, error) {
	if (network.gethBootsrapperService == nil) {
		return "", stacktrace.NewError("Cannot add ethereum node to network; no bootstrap node exists")
	}

	serviceIdStr := gethServiceIdPrefix + strconv.Itoa(network.nextGethServiceId)
	network.nextGethServiceId = network.nextGethServiceId + 1
	serviceId := services.ServiceID(serviceIdStr)

	/*initializer := api.NewApiContainerInitializer(network.gethServiceImage, network.gethBootsrapperService)
	uncastedApiService, checker, err := network.networkCtx.AddService(serviceId, initializer)
	if err != nil {
		return "", stacktrace.Propagate(err, "An error occurred adding the ethereum node")
	}
	if err := checker.WaitForStartup(waitForStartupTimeBetweenPolls, waitForStartupMaxNumPolls); err != nil {
		return "", stacktrace.Propagate(err, "An error occurred waiting for the ethereum node to start")
	}
	castedApiService := uncastedApiService.(*api.ApiService)
	network.apiServices[serviceId] = castedApiService*/
	return serviceId, nil
}
/*
func (network *ChainlinkNetwork) GetApiService(serviceId services.ServiceID) (*api.ApiService, error) {
	service, found := network.apiServices[serviceId]
	if !found {
		return nil, stacktrace.NewError("No API service with ID '%v' has been added", serviceId)
	}
	return service, nil
}*/