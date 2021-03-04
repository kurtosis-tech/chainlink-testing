package ethereum_funded_test

import (
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/networks"
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/testsuite"
	"github.com/kurtosistech/chainlink-testing/testsuite/services_impl/geth"
	"github.com/palantir/stacktrace"
	"time"
)

const (
	gethBootnodeServiceId services.ServiceID = "bootnode"

	waitForStartupTimeBetweenPolls = 1 * time.Second
	waitForStartupMaxPolls = 15
)

type EthereumFundedTest struct {
	gethServiceImage string
}

func NewEthereumFundedTest(gethServiceImage string) *EthereumFundedTest {
	return &EthereumFundedTest{gethServiceImage: gethServiceImage}
}

func (test EthereumFundedTest) Setup(networkCtx *networks.NetworkContext) (networks.Network, error) {
	bootnodeContainerInitializer := geth.NewGethContainerInitializer(test.gethServiceImage)
	_, availabilityChecker, err := networkCtx.AddService(gethBootnodeServiceId, bootnodeContainerInitializer)
	if err != nil {
		return nil, stacktrace.Propagate(err, "An error occurred adding the bootnode")
	}
	if err := availabilityChecker.WaitForStartup(waitForStartupTimeBetweenPolls, waitForStartupMaxPolls); err != nil {
		return nil, stacktrace.Propagate(err, "An error occurred waiting for the datastore service to become available")
	}
	return networkCtx, nil
}

func (test EthereumFundedTest) Run(network networks.Network, testCtx testsuite.TestContext) {
	// Necessary because Go doesn't have generics
	castedNetwork := network.(*networks.NetworkContext)

	uncastedService, err := castedNetwork.GetService(gethBootnodeServiceId)
	if err != nil {
		testCtx.Fatal(stacktrace.Propagate(err, "An error occurred getting the datastore service"))
	}

	// Necessary again due to no Go generics
	castedService := uncastedService.(*geth.GethService)

	isAvailable := castedService.IsAvailable()

	time.Sleep(90)

	testCtx.AssertTrue(isAvailable, stacktrace.NewError("Bootnode did not become available."))
}


func (test *EthereumFundedTest) GetTestConfiguration() testsuite.TestConfiguration {
	return testsuite.TestConfiguration{}
}

func (test *EthereumFundedTest) GetExecutionTimeout() time.Duration {
	return 60 * time.Second
}

func (test *EthereumFundedTest) GetSetupTimeout() time.Duration {
	return 60 * time.Second
}

