package geth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
	"github.com/palantir/stacktrace"
	"github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

const (
	adminInfoRpcCall = `{"jsonrpc":"2.0","method": "admin_nodeInfo","params":[],"id":67}`
	adminPeerRpcCall = `{"jsonrpc":"2.0", "method": "admin_peers","params":[],"id":67}`
	enodePrefix = "enode://"
	ipcPath = "ipc:/genesis/geth.ipc"
)

type GethService struct {
	serviceCtx *services.ServiceContext
	rpcPort   int
}

type NodeInfoResponse struct {
	Result NodeInfo `json:"result"`
}

type NodeInfo struct {
	Enode string `json: "enode"`
}

type AddPeerResponse struct {
	Result bool `json:"result"`
}

type GetPeersResponse struct {
	Result []Peer `json:"result"`
}

type Peer struct {
	Enode string `json:"enode"`
	Id string `json:"id"`
	Network NetworkRecord `json:"network"`
}

type NetworkRecord struct {
	LocalAddress string `json:"localAddress"`
	RemoteAddress string `json:"remoteAddress"`
}

func NewGethService(serviceCtx *services.ServiceContext, port int) *GethService {
	return &GethService{serviceCtx: serviceCtx, rpcPort: port}
}

func (service GethService) GetIPAddress() string {
	ipAddress := service.serviceCtx.GetIPAddress()
	return ipAddress
}

func (service GethService) GetRpcPort() int {
	return rpcPort
}

func (service GethService) GetWsPort() int {
	return wsPort
}

func (service GethService) AddPeer(peerEnode string) (bool, error) {
	adminAddPeerRpcCall := fmt.Sprintf(`{"jsonrpc":"2.0", "method": "admin_addPeer", "params": ["%v"], "id":70}`, peerEnode)
	logrus.Tracef("Admin add peer rpc call: %v", adminAddPeerRpcCall)
	addPeerResponse := new(AddPeerResponse)
	err := service.sendRpcCall(adminAddPeerRpcCall, addPeerResponse)
	logrus.Tracef("AddPeer response: %+v", addPeerResponse)
	if err != nil {
		return false, stacktrace.Propagate(err, "Failed to send addPeer RPC call for enode %v", peerEnode)
	}
	return addPeerResponse.Result, nil
}

func (service GethService) GetPeers() ([]Peer, error) {
	getPeersResponse := new(GetPeersResponse)
	err := service.sendRpcCall(adminPeerRpcCall, getPeersResponse)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Failed to send getPeers RPC call for service %v", service.serviceCtx.GetServiceID())
	}
	return getPeersResponse.Result, nil
}

func (service GethService) GetEnodeAddress() (string, error) {
	nodeInfoResponse := new(NodeInfoResponse)
	err := service.sendRpcCall(adminInfoRpcCall, nodeInfoResponse)
	if err != nil {
		return "", stacktrace.Propagate(err, "Failed to send admin node info RPC request to geth node %v", service.serviceCtx.GetServiceID())
	}
	return nodeInfoResponse.Result.Enode, nil
}

func (service GethService) SendTransaction(from string, to string, amount string) (error) {
	cmdArgs := []string{
		"/bin/sh",
		"-c",
		fmt.Sprintf("geth attach %v --exec 'eth.sendTransaction({from: \"%v\",to: \"%v\", value: \"%v\"})'", ipcPath, from, to, amount),
	}
	_, logOutput, err := service.serviceCtx.ExecCommand(cmdArgs)
	if err != nil {
		return stacktrace.Propagate(err, "Failed to execute command to send eth.")
	}
	logrus.Debugf("Logoutput from sendTransaction: %+v", string(*logOutput))
	return nil
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
		// For debugging
		var teeBuf bytes.Buffer
		tee := io.TeeReader(resp.Body, &teeBuf)
		bodyBytes, err := ioutil.ReadAll(tee)
		if err != nil {
			return stacktrace.Propagate(err, "Error parsing geth node response into bytes.")
		}
		bodyString := string(bodyBytes)
		logrus.Tracef("Response for RPC call %v: %v", rpcJsonString, bodyString)

		err = json.NewDecoder(&teeBuf).Decode(targetStruct)
		if err != nil {
			return stacktrace.Propagate(err, "Error parsing geth node response into target struct.")
		}
		return nil
	} else {
		return stacktrace.NewError("Received non-200 status code rom admin RPC api: %v", resp.StatusCode)
	}
}