package ovn

import (
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"strings"
)

func (oc *Controller) getIPFromOvnAnnotation(ovnAnnotation string) string {
	if ovnAnnotation == "" {
		return ""
	}

	var ovnAnnotationMap map[string]string
	err := json.Unmarshal([]byte(ovnAnnotation), &ovnAnnotationMap)
	if err != nil {
		logrus.Errorf("Error in json unmarshaling ovn annotation "+
			"(%v)", err)
		return ""
	}

	ipAddressMask := strings.Split(ovnAnnotationMap["ip_address"], "/")
	if len(ipAddressMask) != 2 {
		logrus.Errorf("Error in splitting ip address")
		return ""
	}

	return ipAddressMask[0]
}

func (oc *Controller) getMacFromOvnAnnotation(ovnAnnotation string) string {
	if ovnAnnotation == "" {
		return ""
	}

	var ovnAnnotationMap map[string]string
	err := json.Unmarshal([]byte(ovnAnnotation), &ovnAnnotationMap)
	if err != nil {
		logrus.Errorf("Error in json unmarshaling ovn annotation "+
			"(%v)", err)
		return ""
	}

	return ovnAnnotationMap["mac_address"]
}

func stringSliceMembership(slice []string, key string) bool {
	for _, val := range slice {
		if val == key {
			return true
		}
	}
	return false
}

func (oc *Controller) getNetworkFromOvnAnnotation(ovnAnnotation string) string {
	if ovnAnnotation == "" {
		logrus.Errorf("getNetworkFromOvnAnnotation ovnAnnotation: %s", ovnAnnotation)
		return ""
	}
	logrus.Infof("getNetworkFromOvnAnnotation ovnAnnotation: %s", ovnAnnotation)

	var ovnAnnotationMap map[string]string
	err := json.Unmarshal([]byte(ovnAnnotation), &ovnAnnotationMap)
	if err != nil {
		logrus.Errorf("Error in json unmarshaling ovn annotation "+
			"(%v)", err)
		return ""
	}
	for key, value := range ovnAnnotationMap {
		logrus.Infof("getNetworkFromOvnAnnotation %s: %s", key, value)
	}
	return ovnAnnotationMap["name"]
}

func (oc *Controller) parseOvnNetworkObject(ovnnetwork string) ([]map[string]interface{}, error) {
	var ovnNet []map[string]interface{}

	if ovnnetwork == "" {
		return nil, fmt.Errorf("parseOvnNetworkObject:error")
	}

	if err := json.Unmarshal([]byte(ovnnetwork), &ovnNet); err != nil {
		return nil, fmt.Errorf("parseOvnNetworkObject: failed to load ovn network err: %v | ovn network: %v", err, ovnnetwork)
	}

	return ovnNet, nil
}
