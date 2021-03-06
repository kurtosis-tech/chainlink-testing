package geth

import (
	"fmt"
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
)

type GethService struct {
	serviceCtx *services.ServiceContext
	rpcPort   int
}

func NewGethService(serviceCtx *services.ServiceContext, port int) *GethService {
	return &GethService{serviceCtx: serviceCtx, rpcPort: port}
}

func (service GethService) GetIPAddress() string {
	ipAddress := service.serviceCtx.GetIPAddress()
	return ipAddress
}

func (service GethService) GetEnodeAddress() string {
	// TODO TODO TODO Implement RPC call to service to get enode
	_ = fmt.Sprintf("http://%v:%v", service.serviceCtx.GetIPAddress(), rpcPort)
	return ""
}

// ===========================================================================================
//                              Service interface methods
// ===========================================================================================

func (service GethService) IsAvailable() bool {
	/*url := fmt.Sprintf("http://%v:%v/%v", service.GetIPAddress(), service.rpcPort)
	resp, err := http.Get(url)
	if err != nil {
		logrus.Debugf("An HTTP error occurred when polliong the health endpoint: %v", err)
		return false
	}
	if resp.StatusCode != http.StatusOK {
		logrus.Debugf("Received non-OK status code: %v", resp.StatusCode)
		return false
	}

	body := resp.Body
	defer body.Close()

	bodyBytes, err := ioutil.ReadAll(body)
	if err != nil {
		logrus.Debugf("An error occurred reading the response body: %v", err)
		return false
	}
	bodyStr := string(bodyBytes)

	return bodyStr == healthyValue*/
	return true
}
