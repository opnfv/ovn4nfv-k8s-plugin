package node

import (
	"ovn4nfv-k8s-plugin/internal/pkg/ovn"

	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("node")

//AddNodeLogicalPorts return nodeIntfMacAddr and nodeIntfIPAddr
func AddNodeLogicalPorts(node string) (nodeIntfMacAddr, nodeIntfIPAddr string, err error) {
	ovnCtl, err := ovn.GetOvnController()
	if err != nil {
		return "", "", err
	}

	log.Info("Calling CreateNodeLogicalPorts")
	nodeIntfMacAddr, nodeIntfIPAddr, err = ovnCtl.AddNodeLogicalPorts(node)
	if err != nil {
		return "", "", err
	}
	return nodeIntfMacAddr, nodeIntfIPAddr, nil
}

//DeleteNodeLogicalPorts return nil
func DeleteNodeLogicalPorts(name, namesapce string) error {
	// Run delete for all controllers;
	// Todo
	return nil
}
