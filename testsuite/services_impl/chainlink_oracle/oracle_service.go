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
	specsEndpoint = "v2/specs"

	keysEndpoint = "v2/keys"
	ethKeyEndpointSuffix = "eth"
	ocrKeyEndpointSuffix = "ocr"
	peerToPeerIdEndpointSuffix = "p2p"

	runsEndpoint = "v2/runs"

	jsonMimeType = "application/json"
)


type ChainlinkOracleService struct {
	serviceCtx *services.ServiceContext
	clientWithSession *http.Client
	sessionCookieJar *cookiejar.Jar
}

func NewChainlinkOracleService(serviceCtx *services.ServiceContext) *ChainlinkOracleService {
	return &ChainlinkOracleService{serviceCtx: serviceCtx}
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

func (service *ChainlinkOracleService) SetJobSpec(
		oracleContractAddress string) (jobId string, err error) {
	client, err := service.getOrCreateClientWithSession()
	if err != nil {
		return "", stacktrace.Propagate(err, "An error occurred getting the oracle session client")
	}

	// Get transmitter key
	ethAddrUrl := service.getApiRequestUrl(
		fmt.Sprintf("%v/%v", keysEndpoint, ethKeyEndpointSuffix),
	)
	ethAddrResp, err := client.Get(ethAddrUrl)
	if err != nil {
		return "", stacktrace.Propagate(err, "Failed to get Ethereum account info from oracle")
	}
	ethereumKeys := new(OracleEthereumKeysResponse)
	err = parseAndLogResponse(ethAddrResp, ethereumKeys)
	if err != nil {
		return "", stacktrace.Propagate(err, "Failed to parse Ethereum account info response")
	}

	// Get P2P ID
	peerToPeerIdUrl := service.getApiRequestUrl(
		fmt.Sprintf("%v/%v", keysEndpoint, peerToPeerIdEndpointSuffix),
	)
	peerToPeerIdResponse, err := client.Get(peerToPeerIdUrl)
	if err != nil {
		return "", stacktrace.Propagate(err, "Failed to get peer-to-peer ID from oracle")
	}
	// TODO DEBUGGING REMOVE
	// TODO STRIP OFF "p2p_" leader from the P2P key
	defer peerToPeerIdResponse.Body.Close()
	peerToPeerIdResponseBodyBytes, err := ioutil.ReadAll(peerToPeerIdResponse.Body)
	if err != nil {
		return "", stacktrace.Propagate(err, "An error occurred reading the peer-to-peer ID response body bytes")
	}
	logrus.Debugf("Peer-to-peer ID response body: %v", string(peerToPeerIdResponseBodyBytes))
	// TODO PARSE THE RESPONSE

	// Get OCR key bundle ID
	ocrKeyUrl := service.getApiRequestUrl(
		fmt.Sprintf("%v/%v", keysEndpoint, ocrKeyEndpointSuffix),
	)
	ocrKeyResponse, err := client.Get(ocrKeyUrl)
	if err != nil {
		return "", stacktrace.Propagate(err, "Failed to get OCR key bundle ID from oracle")
	}
	// TODO DEBUGGING REMOVE
	defer ocrKeyResponse.Body.Close()
	ocrKeyResponseBodyBytes, err := ioutil.ReadAll(ocrKeyResponse.Body)
	if err != nil {
		return "", stacktrace.Propagate(err, "An error occurred reading the OCR key bundle ID response body bytes")
	}
	logrus.Debugf("OCR key bundle ID response body: %v", string(ocrKeyResponseBodyBytes))
	// TODO PARSE THE RESPONSE

	jobSpecJsonStr := generateJobSpec(oracleContractAddress)
	jsonByteArray := []byte(jobSpecJsonStr)
	url := fmt.Sprintf(
		"http://%v:%v/%v",
		service.GetIPAddress(),
		service.GetOperatorPort(),
		specsEndpoint)

	resp, err := client.Post(url, jsonMimeType, bytes.NewBuffer(jsonByteArray))
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


/*
func generateJobSpec(
			oracleContractAddress string,
			bootstrapIpAddr string,
			bootstrapPeerToPeerId string,
			nodePeerToPeerId string,
			nodeOcrKeyBundleId string,
			nodeEthTransmitterAddress string) string {
		// TODO Add an EthInt256 step to this??
		return fmt.Sprintf(`
	type               = "offchainreporting"
	schemaVersion      = 1
	contractAddress    = "%v"
	p2pPeerID          = "%v"
	p2pBootstrapPeers  = [
		"/dns4/%v/tcp/1234/p2p/%v",
	]
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
		ds1          [type=http method=POST url="(http://external-adapter:6633)" requestData="{}"];
		ds1_parse    [type=jsonparse path="data,result"];
		ds1_multiply [type=multiply times=10];

		ds1 -> ds1_parse -> ds1_multiply -> answer;
		answer [type=median];
	"""`,
			oracleContractAddress,
			nodePeerToPeerId,
			bootstrapIpAddr,
			bootstrapPeerToPeerId,
			nodeOcrKeyBundleId,
			nodeEthTransmitterAddress)
}
*/

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


	err = json.NewDecoder(&teeBuf).Decode(targetStruct)
	if err != nil {
		return stacktrace.Propagate(err, "Error parsing Oracle response into a struct.")
	}
	logrus.Debugf("JSON-parsed response from oracle: %+v", targetStruct)
	return nil
}