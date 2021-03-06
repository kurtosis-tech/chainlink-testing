package networks_impl

import (
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/networks"
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
	"github.com/kurtosistech/chainlink-testing/testsuite/services_impl/geth"
	"github.com/palantir/stacktrace"
	"github.com/sirupsen/logrus"
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

func NewChainlinkNetwork(networkCtx *networks.NetworkContext, gethDataDirArtifactId services.FilesArtifactID, gethServiceImage string) *ChainlinkNetwork {
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
	if network.gethBootsrapperService != nil {
		return stacktrace.NewError("Cannot add bootstrapper service to network; bootstrapper already exists!")
	}

	initializer := geth.NewGethContainerInitializer(network.gethServiceImage, network.gethDataDirArtifactId, nil)
	uncastedBootstrapper, checker, err := network.networkCtx.AddService(ethereumBootstrapperId, initializer)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred adding the bootstrapper service")
	}
	if err := checker.WaitForStartup(waitForStartupTimeBetweenPolls, waitForStartupMaxNumPolls); err != nil {
		return stacktrace.Propagate(err, "An error occurred waiting for the bootstrapper service to start")
	}
	castedGethBootstrapperService := uncastedBootstrapper.(*geth.GethService)
	network.gethBootsrapperService = castedGethBootstrapperService
	return nil
}

func (network *ChainlinkNetwork) GetBootstrapper() *geth.GethService {
	return network.gethBootsrapperService
}

func (network *ChainlinkNetwork) AddGethService() (services.ServiceID, error) {
	if (network.gethBootsrapperService == nil) {
		return "", stacktrace.NewError("Cannot add ethereum node to network; no bootstrap node exists")
	}

	serviceIdStr := gethServiceIdPrefix + strconv.Itoa(network.nextGethServiceId)
	network.nextGethServiceId = network.nextGethServiceId + 1
	serviceId := services.ServiceID(serviceIdStr)

	initializer := geth.NewGethContainerInitializer(network.gethServiceImage, network.gethDataDirArtifactId, network.gethBootsrapperService)
	uncastedGethService, checker, err := network.networkCtx.AddService(serviceId, initializer)
	if err != nil {
		return "", stacktrace.Propagate(err, "An error occurred adding the ethereum node")
	}
	if err := checker.WaitForStartup(waitForStartupTimeBetweenPolls, waitForStartupMaxNumPolls); err != nil {
		return "", stacktrace.Propagate(err, "An error occurred waiting for the ethereum node to start")
	}
	castedGethService := uncastedGethService.(*geth.GethService)

	network.gethServices[serviceId] = castedGethService
	return serviceId, nil
}

func (network *ChainlinkNetwork) ManuallyConnectPeers() error {
	for nodeId, nodeGethService := range network.gethServices {
		for peerId, peerGethService := range network.gethServices {
			if nodeId == peerId {
				continue
			}
			peerGethServiceEnode, err := peerGethService.GetEnodeAddress()
			if err != nil {
				return stacktrace.Propagate(err, "Failed to get enode from peer %v", peerId)
			}
			ok, err := nodeGethService.AddPeer(peerGethServiceEnode)
			if err != nil {
				return stacktrace.Propagate(err, "Failed to call addPeer endpoint to add peer with enode %v", peerGethServiceEnode)
			}
			if !ok {
				//return stacktrace.NewError("addPeer endpoint returned false on service %v, adding peer %v", nodeId, peerGethServiceEnode)
				logrus.Infof("AddPeer endpoint for nodeId %v connecting to peerId %v returned false.", nodeId, peerId)
			} else {
				logrus.Infof("AddPeer endpoint for nodeId %v connecting to peerId %v returned true.", nodeId, peerId)
			}
		}
	}
	return nil
}

func (network *ChainlinkNetwork) GetGethService(serviceId services.ServiceID) (*geth.GethService, error) {
	service, found := network.gethServices[serviceId]
	if !found {
		return nil, stacktrace.NewError("No geth service with ID '%v' has been added", serviceId)
	}
	return service, nil
}