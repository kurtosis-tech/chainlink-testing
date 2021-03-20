package chainlink_oracle

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
	"github.com/palantir/stacktrace"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/publicsuffix"
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
)

type OracleJobInitiatedResponse struct {
	Data OracleJobInitiatedData `json:"data"`
}

type OracleJobInitiatedData struct {
	Id string `json:"id"`
}

type ChainlinkOracleService struct {
	serviceCtx *services.ServiceContext
	sessionCookieJar *cookiejar.Jar
}

func NewChainlinkOracleService(serviceCtx *services.ServiceContext) *ChainlinkOracleService {
	return &ChainlinkOracleService{serviceCtx: serviceCtx}
}

func (chainlinkOracleService *ChainlinkOracleService) GetOperatorPort() int {
	return operatorUiPort
}

func (chainlinkOracleService *ChainlinkOracleService) GetIPAddress() string {
	return chainlinkOracleService.serviceCtx.GetIPAddress()
}

func (chainlinkOracleService *ChainlinkOracleService) SetJobSpec(oracleContractAddress string) (jobId string, err error) {
	if chainlinkOracleService.sessionCookieJar == nil {
		_, err := chainlinkOracleService.StartSession()
		if err != nil {
			return "", stacktrace.Propagate(err, "Failed to start session on Oracle.")
		}
	}
	jobSpecJsonStr := generateJobSpec(oracleContractAddress)
	jsonByteArray := []byte(jobSpecJsonStr)
	urlStr := fmt.Sprintf("http://%v:%v/%v",
		chainlinkOracleService.GetIPAddress(), chainlinkOracleService.GetOperatorPort(), specsEndpoint)

	// Create new http client with predefined options
	client := &http.Client{
		Jar:     chainlinkOracleService.sessionCookieJar,
		Timeout: time.Second * 60,
	}
	authResp, err := client.Post(urlStr, "application/json", bytes.NewBuffer(jsonByteArray))
	if err != nil {
		return "", stacktrace.Propagate(err, "Encountered an error trying to set job spec on the Oracle.")
	}
	response := new(OracleJobInitiatedResponse)
	defer authResp.Body.Close()
	err = json.NewDecoder(authResp.Body).Decode(response)
	if err != nil {
		return "", stacktrace.Propagate(err, "Error parsing Oracle response into bytes.")
	}
	logrus.Infof("Response from Chainlink Oracle: %+v", response)
	return response.Data.Id, nil
}

func (chainlinkOracleService *ChainlinkOracleService) StartSession() (string, error) {
	authJsonStr := fmt.Sprintf("{\"email\":\"%v\", \"password\":\"%v\"}",
		oracleEmail, oraclePassword)
	authByteArray := []byte(authJsonStr)
	urlStr := fmt.Sprintf("http://%v:%v/%v",
		chainlinkOracleService.GetIPAddress(), chainlinkOracleService.GetOperatorPort(), sessionsEndpoint)
	// Create new cookiejar for holding cookies
	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})

	// Create new http client with predefined options
	client := &http.Client{
		Jar:     jar,
		Timeout: time.Second * 60,
	}
	authResp, err := client.Post(urlStr, "application/json", bytes.NewBuffer(authByteArray))
	if err != nil {
		return "", stacktrace.Propagate(err, "Encountered an error trying to authenticate with the oracle service..")
	}
	logrus.Infof("After starting sessions, cookies look like: %+v", jar)
	chainlinkOracleService.sessionCookieJar = jar
	return authResp.Status, nil
}

// ===========================================================================================
//                              Service interface methods
// ===========================================================================================

func (chainlinkOracleService *ChainlinkOracleService) IsAvailable() bool {
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
				  "type": "httpget"
				},
				{
				  "type": "jsonparse"
				},
				{
				  "type": "multiply"
				},
				{
				  "type": "ethuint256"
				},
				{
				  "type": "ethtx"
				}
		  ],
		  "minPayment": "100"
		}`, oracleContractAddress)
}