package chainlink_oracle

import (
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
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

func (chainlinkOracleService ChainlinkOracleService) GetOperatorPort() int {
	return operatorUiPort
}

func (chainlinkOracleService ChainlinkOracleService) GetIPAddress() string {
	return chainlinkOracleService.serviceCtx.GetIPAddress()
}

// ===========================================================================================
//                              Service interface methods
// ===========================================================================================

func (chainlinkOracleService ChainlinkOracleService) IsAvailable() bool {
	conn, err := net.DialTimeout("tcp",
		net.JoinHostPort(chainlinkOracleService.GetIPAddress(), strconv.Itoa(operatorUiPort)), isAvailableDialTimeout)
	if err != nil {
		return false
	}
	if conn == nil {
		return false
	}
	defer conn.Close()
	return true
}

