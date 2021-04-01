package chainlink_oracle

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
	"github.com/palantir/stacktrace"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/publicsuffix"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"time"
)

const (
	isAvailableDialTimeout = time.Second

	sessionsEndpoint = "sessions"
	jobSpecsEndpoint = "v2/jobs"

	keysEndpoint = "v2/keys"
	ethKeyEndpointSuffix = "eth"
	ocrKeyEndpointSuffix = "ocr"
	peerToPeerIdEndpointSuffix = "p2p"

	runsEndpoint = "v2/runs"

	jsonMimeType = "application/json"


	// Nodes will come with two ETH keys; the first one is the transmitter key and the second one is an emergency funding key
	// See: https://chainlink-growth.slack.com/archives/C01NSF9GH6Y/p1617142577011600?thread_ts=1617139632.008300&cid=C01NSF9GH6Y
	transmitterEthKeyIndex = 0
)

// TODO Make this return an instance of client.Client from the chainlink repo, so users get full access!!
//  This prevents us from hand-crafting RPC requests to the node
type ChainlinkOracleService struct {
	serviceCtx *services.ServiceContext
	clientWithSession *http.Client
	sessionCookieJar *cookiejar.Jar
}

func NewChainlinkOracleService(serviceCtx *services.ServiceContext) *ChainlinkOracleService {
	return &ChainlinkOracleService{
		serviceCtx:        serviceCtx,
		clientWithSession: nil,
		sessionCookieJar:  nil,
	}
}

func (service *ChainlinkOracleService) GetOperatorPort() int {
	return operatorUiPort
}

func (service *ChainlinkOracleService) GetIPAddress() string {
	return service.serviceCtx.GetIPAddress()
}

func (service *ChainlinkOracleService) GetRuns() ([]Run, error) {
	client, err := service.getOrCreateClientWithSession()
	if err != nil {
		return nil, stacktrace.Propagate(err, "An error occurred getting the oracle session client")
	}
	urlStr := fmt.Sprintf("http://%v:%v/%v",
		service.GetIPAddress(), service.GetOperatorPort(), runsEndpoint)
	response, err := client.Get(urlStr)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Failed to get runs information from Oracle.")
	}
	runsResponse := new(RunsResponse)
	err = parseAndLogResponse(response, runsResponse)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Failed to parse Oracle response into a struct.")
	}
	return runsResponse.Data, nil
}

func (service *ChainlinkOracleService) GetEthKeys() ([]OracleEthereumKey, error) {
	endpoint := fmt.Sprintf("%v/%v", keysEndpoint, ethKeyEndpointSuffix)
	responseObj := new(OracleEthereumKeysResponse)
	if err := service.makeAndParseApiGetRequest(endpoint, responseObj); err != nil {
		return nil, stacktrace.Propagate(err, "An error occurred getting the ETH keys from the oracle API")
	}
	return responseObj.Data, nil
}

func (service *ChainlinkOracleService) GetPeerToPeerKeys() ([]OraclePeerToPeerKey, error) {
	endpoint := fmt.Sprintf("%v/%v", keysEndpoint, peerToPeerIdEndpointSuffix)
	responseObj := new(OraclePeerToPeerKeysResponse)
	if err := service.makeAndParseApiGetRequest(endpoint, responseObj); err != nil {
		return nil, stacktrace.Propagate(err, "An error occurred getting the P2P keys from the oracle API")
	}
	return responseObj.Data, nil
}

func (service *ChainlinkOracleService) GetOCRKeyBundles() ([]OracleOcrKeyBundle, error) {
	endpoint := fmt.Sprintf("%v/%v", keysEndpoint, ocrKeyEndpointSuffix)
	responseObj := new(OracleOcrKeyBundlesResponse)
	if err := service.makeAndParseApiGetRequest(endpoint, responseObj); err != nil {
		return nil, stacktrace.Propagate(err, "An error occurred getting the OCR key bundles from the oracle API")
	}
	return responseObj.Data, nil
}

// TODO Replace the hand-crafted POST wtih a call to the Chainlink client.Client
func (service *ChainlinkOracleService) SetJobSpec(
		oracleContractAddress string,
		bootstrapperIpAddr string,
		bootstrapperPeerId string,
		datasourceUrl string) (jobId string, err error) {
	client, err := service.getOrCreateClientWithSession()
	if err != nil {
		return "", stacktrace.Propagate(err, "An error occurred getting the oracle session client")
	}

	// Transmitter ETH address
	ethKeys, err := service.GetEthKeys()
	if err != nil {
		return "", stacktrace.Propagate(err, "An error occurred getting the ETH keys from the oracle")
	}
	transmitterKey := ethKeys[transmitterEthKeyIndex]
	transmitterAddress := transmitterKey.Attributes.Address

	// P2P ID
	allPeer2PeerKeys, err := service.GetPeerToPeerKeys()
	if err != nil {
		return "", stacktrace.Propagate(err, "An error occurred getting the P2P keys from the oracle")
	}
	if len(allPeer2PeerKeys) != 1 {
		return "", stacktrace.NewError("Expected exactly 1 P2P key but found %v", len(allPeer2PeerKeys))
	}
	peer2PeerKey := allPeer2PeerKeys[0]
	peer2PeerId := peer2PeerKey.Attributes.PeerId

	// OCR key bundle ID
	allOcrKeyBundles, err := service.GetOCRKeyBundles()
	if err != nil {
		return "", stacktrace.Propagate(err, "An error occurred getting the OCR key bundles from the oracle")
	}
	if len(allOcrKeyBundles) != 1 {
		return "", stacktrace.NewError("Expected exactly 1 OCR key bundle but found %v", len(allOcrKeyBundles))
	}
	ocrKeyBundle := allOcrKeyBundles[0]
	ocrKeyBundleId := ocrKeyBundle.Attributes.Id

	jobSpecTomlStr := generateJobSpec(
		oracleContractAddress,
		bootstrapperIpAddr,
		bootstrapperPeerId,
		peer2PeerId,
		ocrKeyBundleId,
		transmitterAddress,
		datasourceUrl)
	jobSpecReq := CreateTomlJobRequest{TOML: jobSpecTomlStr}
	serializedJobSpecReq, err := json.Marshal(jobSpecReq)
	if err != nil {
		return "", stacktrace.Propagate(err, "An error occurred serializing the following job spec request object to JSON: %+v", jobSpecReq)
	}
	url := fmt.Sprintf(
		"http://%v:%v/%v",
		service.GetIPAddress(),
		service.GetOperatorPort(),
		jobSpecsEndpoint)

	resp, err := client.Post(url, jsonMimeType, bytes.NewBuffer(serializedJobSpecReq))
	if err != nil {
		return "", stacktrace.Propagate(err, "Encountered an error trying to set job spec on the Oracle.")
	}
	jobInitiatedResponse := new(OracleJobInitiatedResponse)
	defer resp.Body.Close()

	err = parseAndLogResponse(resp, jobInitiatedResponse)
	if err != nil {
		return "", stacktrace.Propagate(err, "Failed to parse Oracle response into a struct.")
	}
	return jobInitiatedResponse.Data.Id, nil
}

// ===========================================================================================
//                              Service interface methods
// ===========================================================================================

func (service *ChainlinkOracleService) IsAvailable() bool {
	conn, err := net.DialTimeout("tcp",
		net.JoinHostPort(service.GetIPAddress(), strconv.Itoa(operatorUiPort)), isAvailableDialTimeout)
	if err != nil {
		return false
	}
	if conn == nil {
		return false
	}
	defer conn.Close()
	return true
}


// ===========================================================================================
//                              Private helper functions
// ===========================================================================================
// TODO Push this into a supplier struct, so that users don't accidentally use the uninitialized clientWithSession
//  property on the struct
func (service *ChainlinkOracleService) getOrCreateClientWithSession() (*http.Client, error) {
	if service.clientWithSession != nil {
		return service.clientWithSession, nil
	}

	authJsonStr := fmt.Sprintf(
		"{\"email\":\"%v\", \"password\":\"%v\"}",
		oracleEmail,
		oraclePassword)
	authByteArray := []byte(authJsonStr)
	urlStr := fmt.Sprintf("http://%v:%v/%v",
		service.GetIPAddress(), service.GetOperatorPort(), sessionsEndpoint)
	// Create new cookiejar for holding cookies
	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})

	// Create new http client with predefined options
	client := &http.Client{
		Jar:     jar,
		Timeout: time.Second * 60,
	}
	_, err := client.Post(urlStr, jsonMimeType, bytes.NewBuffer(authByteArray))
	if err != nil {
		return nil, stacktrace.Propagate(err, "Encountered an error trying to authenticate with the oracle service..")
	}
	logrus.Debugf("After starting sessions, cookies look like: %+v", jar)
	service.clientWithSession = client
	return client, nil
}

func (service *ChainlinkOracleService) getApiRequestUrl(endpoint string) string {
	return fmt.Sprintf(
		"http://%v:%v/%v",
		service.GetIPAddress(),
		service.GetOperatorPort(),
		endpoint)
}

// TODO Delete this
/*
func generateJobSpec(oracleContractAddress string) string {
	return fmt.Sprintf(`{
		  "initiators": [
			{
			  "type": "RunLog",
			  "params": { "address": "%v" }
			}
		  ],
		  "tasks": [
				{
				  "type": "HttpGetWithUnrestrictedNetworkAccess"
				},
				{
				  "type": "JsonParse"
				},
				{
				  "type": "Multiply"
				},
				{
				  "type": "EthInt256"
				},
				{
				  "type": "EthTx"
				}
		  ]
		}`, oracleContractAddress)
}
*/

func generateJobSpec(
			oracleContractAddress string,
			bootstrapIpAddr string,
			bootstrapPeerToPeerId string,
			nodePeerToPeerId string,
			nodeOcrKeyBundleId string,
			nodeEthTransmitterAddress string,
			datasourceUrl string) string {
	// TODO Add an EthInt256 step to this??
	// TODO Modify the tcp port for the p2pBootstrapPeers??
	// TODO Replace this string with an actual structured object from https://github.com/smartcontractkit/chainlink/blob/2f2dc24f3ef6a63a47d7a3a4d2c23239d89555c0/core/services/job/models.go#L101
	return fmt.Sprintf(
			`
type               = "offchainreporting"
schemaVersion      = 1
contractAddress    = "%v"
p2pBootstrapPeers  = [
	"/dns4/%v/tcp/1234/p2p/%v",
]
p2pPeerID          = "%v"
isBootstrapPeer    = false
keyBundleID        = "%v"
monitoringEndpoint = "chain.link:4321"
transmitterAddress = "%v"
observationTimeout = "10s"
blockchainTimeout  = "20s"
contractConfigTrackerSubscribeInterval = "2m"
contractConfigTrackerPollInterval = "1m"
contractConfigConfirmations = 3
observationSource = """
	// data source 1
	ds1          [type=http method=POST url="(%v)" requestData="{}"];
	ds1_parse    [type=jsonparse path="data,result"];
	ds1_multiply [type=multiply times=10];

	ds1 -> ds1_parse -> ds1_multiply -> answer;
	answer [type=median];
"""
		`,
		oracleContractAddress,
		bootstrapIpAddr,
		bootstrapPeerToPeerId,
		nodePeerToPeerId,
		nodeOcrKeyBundleId,
		nodeEthTransmitterAddress,
		datasourceUrl)
}

func (service *ChainlinkOracleService) makeAndParseApiGetRequest(apiEndpoint string, targetStruct interface{}) error {
	client, err := service.getOrCreateClientWithSession()
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred getting the oracle session client")
	}
	urlStr := service.getApiRequestUrl(apiEndpoint)
	response, err := client.Get(urlStr)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred making the get request")
	}

	err = parseAndLogResponse(response, targetStruct)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred JSON-parsing the response from the oracle API")
	}
	return nil
}

/*
	Parses an HTTP response into the target struct, while also logging it as a string to help develop and debug.
 */
func parseAndLogResponse(resp *http.Response, targetStruct interface{}) error{
	var teeBuf bytes.Buffer
	tee := io.TeeReader(resp.Body, &teeBuf)
	bodyBytes, err := ioutil.ReadAll(tee)
	if err != nil {
		return stacktrace.Propagate(err, "Error parsing Oracle response into bytes.")
	}
	bodyString := string(bodyBytes)
	logrus.Debugf("Raw response from oracle: %v", bodyString)

	var errResponse ErrorsResponse
	if err := json.Unmarshal(bodyBytes, &errResponse); err != nil {
		return stacktrace.Propagate(err, "An error occurred trying to unmarshal the response into the error response type")
	}
	if errResponse.Errors != nil {
		return stacktrace.NewError("The oracle server returned errors: %+v", errResponse.Errors)
	}

	if err := json.Unmarshal(bodyBytes, targetStruct); err != nil {
		return stacktrace.Propagate(err, "An error occurred parsing the raw oracle response into a structured object")
	}
	logrus.Debugf("JSON-parsed response from oracle: %+v", targetStruct)
	return nil
}