package geth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
	"github.com/palantir/stacktrace"
	"net/http"
	"strings"
)

const (
	adminInfoRpcCall = `{"jsonrpc":"2.0","method": "admin_nodeInfo","params":[],"id":67}`
	enodePrefix = "enode://"
)

type GethService struct {
	serviceCtx *services.ServiceContext
	rpcPort   int
}

type NodeInfoResponse struct {
	Result NodeInfo `json:"result""`
}

type NodeInfo struct {
	Enode string `json: "enode"`
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
	var jsonStr = []byte(adminInfoRpcCall)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonStr))
	if err != nil {
		return "", stacktrace.Propagate(err, "Failed to ping the admin RPC of the bootnode.")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		nodeInfoResponse := new(NodeInfoResponse)
		err = json.NewDecoder(resp.Body).Decode(nodeInfoResponse)
		if err != nil {
			return "", stacktrace.Propagate(err, "Error parsing node info response.")
		}
		return nodeInfoResponse.Result.Enode, nil
	} else {
		return "", stacktrace.NewError("Received non-200 status code rom admin RPC api: %v", resp.StatusCode)
	}
}

// ===========================================================================================
//                              Service interface methods
// ===========================================================================================

func (service GethService) IsAvailable() bool {
	enodeAddress, err := service.GetEnodeAddress()
	if err != nil {
		return false
	} else {
		return strings.HasPrefix(enodeAddress, enodePrefix)
	}
}
