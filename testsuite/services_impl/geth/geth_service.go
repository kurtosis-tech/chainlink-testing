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
	adminPeerRpcCall = `{"jsonrpc":"2.0", "method": "admin_peers","params":[],"id":67}`
	adminAddPeerRpcCall = `{"jsonrpc":"2.0", "method": "admin_addPeer", "params": [url], "id":67}`
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
	nodeInfoResponse := new(NodeInfoResponse)
	err := service.sendRpcCall(adminInfoRpcCall, nodeInfoResponse)
	if err != nil {
		return "", stacktrace.Propagate(err, "Failed to send admin node info RPC request to geth node %v", service.serviceCtx.GetServiceID())
	}
	return nodeInfoResponse.Result.Enode, nil
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

// ==========================================================================================
//								RPC utility methods
// ==========================================================================================

func (service GethService) sendRpcCall(rpcJsonString string, targetStruct interface{}) error {
	url := fmt.Sprintf("http://%v:%v", service.serviceCtx.GetIPAddress(), rpcPort)
	var jsonByteArray = []byte(rpcJsonString)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonByteArray))
	if err != nil {
		return stacktrace.Propagate(err, "Failed to send RPC request to geth node %v", service.serviceCtx.GetServiceID())
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		err = json.NewDecoder(resp.Body).Decode(targetStruct)
		if err != nil {
			return stacktrace.Propagate(err, "Error parsing geth node response into target struct.")
		}
	} else {
		return stacktrace.NewError("Received non-200 status code rom admin RPC api: %v", resp.StatusCode)
	}
}