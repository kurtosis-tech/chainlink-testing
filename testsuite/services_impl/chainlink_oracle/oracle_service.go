package chainlink_oracle

import (
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
	"github.com/sirupsen/logrus"
	"net"
	"strconv"
	"time"
)

const (
	isAvailableDialTimeout = time.Second
)

type ChainlinkOracleService struct {
	serviceCtx *services.ServiceContext
}

func NewChainlinkOracleService(serviceCtx *services.ServiceContext) *ChainlinkOracleService {
	return &ChainlinkOracleService{serviceCtx: serviceCtx}
}

// ===========================================================================================
//                              Service interface methods
// ===========================================================================================

func (chainlinkOracleService ChainlinkOracleService) IsAvailable() bool {
	conn, err := net.DialTimeout("tcp",
		net.JoinHostPort(chainlinkOracleService.serviceCtx.GetIPAddress(), strconv.Itoa(operatorUiPort)), isAvailableDialTimeout)
	if err != nil {
		logrus.Infof("Encountered an error trying to connect to oracle: %v", err.Error())
		return false
	}
	if conn == nil {
		logrus.Infof("Connection was nil after connecting to oracle.")
		return false
	}
	defer conn.Close()
	return true
}

