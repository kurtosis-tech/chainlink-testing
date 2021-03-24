package price_feed_server

import (
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
	"net"
	"strconv"
	"time"
)

const (
	httpPort = 1323
	isAvailableDialTimeout = 5 * time.Second
)

type PriceFeedServer struct {
	serviceCtx *services.ServiceContext
}

func NewPriceFeedServerService(serviceCtx *services.ServiceContext) *PriceFeedServer {
	return &PriceFeedServer{serviceCtx: serviceCtx}
}

func (priceFeedServer PriceFeedServer) GetIPAddress() string {
	return priceFeedServer.serviceCtx.GetIPAddress()
}

func (priceFeedServer PriceFeedServer) GetHTTPPort() int {
	return httpPort
}

// ===========================================================================================
//                              Service interface methods
// ===========================================================================================

func (priceFeedServer PriceFeedServer) IsAvailable() bool {
	conn, err := net.DialTimeout("tcp",
		net.JoinHostPort(priceFeedServer.GetIPAddress(), strconv.Itoa(httpPort)), isAvailableDialTimeout)
	if err != nil {
		return false
	}
	if conn == nil {
		return false
	}
	defer conn.Close()
	return true
}