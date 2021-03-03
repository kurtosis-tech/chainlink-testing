package geth

import (
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
)

type GethService struct {
	serviceId services.ServiceID
	ipAddr    string
	rpcPort   int
}

func NewGethService(serviceId services.ServiceID, ipAddr string, port int) *GethService {
	return &GethService{serviceId: serviceId, ipAddr: ipAddr, rpcPort: port}
}

// ===========================================================================================
//                              Service interface methods
// ===========================================================================================
func (service GethService) GetServiceID() services.ServiceID {
	return service.serviceId
}

func (service GethService) GetIPAddress() string {
	return service.ipAddr
}

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
