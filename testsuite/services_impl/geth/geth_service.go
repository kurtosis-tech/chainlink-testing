package geth

import (
	"bytes"
	"fmt"
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
	"github.com/palantir/stacktrace"
	"io/ioutil"
	"net/http"
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

func (service GethService) GetEnodeAddress() (string, error) {
	// TODO TODO TODO Implement RPC call to service to get enode
	url := fmt.Sprintf("http://%v:%v", service.serviceCtx.GetIPAddress(), rpcPort)
	var jsonStr = []byte(`{"jsonrpc":"2.0","method": "admin_nodeInfo","params":[],"id":67}`)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonStr))
	if err != nil {
		return "", stacktrace.Propagate(err, "Failed to ping the admin RPC of the bootnode.")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", stacktrace.Propagate(err, "Errored in reading admin RPC api response.")
		}
		bodyString := string(bodyBytes)
		return fmt.Sprintf("Response: %v", bodyString), nil
	} else {
		return "", stacktrace.NewError("Received non-200 status code rom admin RPC api: %v", resp.StatusCode)
	}
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
