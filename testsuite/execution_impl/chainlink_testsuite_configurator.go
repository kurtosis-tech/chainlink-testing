package execution_impl

import (
"encoding/json"
"github.com/kurtosis-tech/kurtosis-libs/golang/lib/testsuite"
"github.com/kurtosistech/chainlink-testing/testsuite/testsuite_impl"
"github.com/palantir/stacktrace"
"github.com/sirupsen/logrus"
"strings"
)

type ChainlinkTestsuiteConfigurator struct {}

func NewChainlinkTestsuiteConfigurator() *ChainlinkTestsuiteConfigurator {
	return &ChainlinkTestsuiteConfigurator{}
}

func (t ChainlinkTestsuiteConfigurator) SetLogLevel(logLevelStr string) error {
	level, err := logrus.ParseLevel(logLevelStr)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred parsing loglevel string '%v'", logLevelStr)
	}
	logrus.SetLevel(level)
	logrus.SetFormatter(&logrus.TextFormatter{
		ForceColors:   true,
		FullTimestamp: true,
	})
	return nil
}

func (t ChainlinkTestsuiteConfigurator) ParseParamsAndCreateSuite(paramsJsonStr string) (testsuite.TestSuite, error) {
	paramsJsonBytes := []byte(paramsJsonStr)
	var args ChainlinkTestsuiteArgs
	if err := json.Unmarshal(paramsJsonBytes, &args); err != nil {
		return nil, stacktrace.Propagate(err, "An error occurred deserializing the testsuite params JSON")
	}

	if err := validateChainlinkArgs(args); err != nil {
		return nil, stacktrace.Propagate(err, "An error occurred validating the deserialized testsuite params")
	}

	suite := testsuite_impl.NewChainlinkTestsuite(args.GethServiceImage)
	return suite, nil
}

func validateChainlinkArgs(args ChainlinkTestsuiteArgs) error {
	if strings.TrimSpace(args.GethServiceImage) == "" {
		return stacktrace.NewError("Geth service image is empty")
	}
	return nil
}

