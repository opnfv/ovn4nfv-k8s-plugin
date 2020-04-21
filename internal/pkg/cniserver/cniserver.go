package cniserver

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
        "strings"
        "os"
        "net"
        "path/filepath"
        "syscall"
        "k8s.io/klog"

	"github.com/containernetworking/cni/pkg/types"
	"github.com/gorilla/mux"
	"k8s.io/client-go/kubernetes"
        "ovn4nfv-k8s-plugin/internal/pkg/config"
        utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
)

const CNIServerRunDir string = "/var/run/ovn4nfv-k8s-plugin/cniserver"
const CNIServerSocketName string = "ovn4nfv-k8s-plugin-cni-server.sock"
const CNIServerSocketPath string = CNIServerRunDir + "/" + CNIServerSocketName


type CNIcommand string

const CNIAdd CNIcommand = "ADD"
const CNIUpdate CNIcommand = "UPDATE"
const CNIDel CNIcommand = "DEL"

type CNIServerRequest struct {
	Command CNIcommand
	PodNamespace string
	PodName string
	SandboxID string
	Netns string
	IfName string
	CNIConf *types.NetConf
}

type cniServerRequestFunc func(request *CNIServerRequest, k8sclient kubernetes.Interface) ([]byte, error)

type CNIEndpointRequest struct {
	ArgEnv    map[string]string `json:"env,omitempty"`
	NetConfig []byte            `json:"config,omitempty"`
}
type CNIServer struct {
	http.Server
	requestFunc  cniServerRequestFunc
	serverrundir string
	k8sclient      kubernetes.Interface
}

func NewCNIServer(serverRunSir string, k8sclient kubernetes.Interface) *CNIServer {
	klog.Infof("Setting up CNI server in nfn-agent")
	if len(serverRunSir) == 0 {
		serverRunSir = CNIServerRunDir
	}

	router := mux.NewRouter()
	cs := &CNIServer{
		Server: http.Server{
			Handler: router,
		},
		serverrundir: serverRunSir,
                k8sclient: k8sclient,
	}
	router.NotFoundHandler = http.HandlerFunc(http.NotFound)
	router.HandleFunc("/", cs.handleCNIShimRequest).Methods("POST")
	return cs
}

func loadCNIShimArgs(env map[string]string) (map[string]string, error) {
	cnishimArgs, ok := env["CNI_ARGS"]
	if !ok {
		return nil, fmt.Errorf("cnishim req missing CNI_ARGS: '%s'", env)
	}

	mapArgs := make(map[string]string)
	for _, arg := range strings.Split(cnishimArgs, ";") {
		parts := strings.Split(arg, "=")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid CNI_ARG from cnishim '%s'", arg)
		}
		mapArgs[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return mapArgs, nil
}

func loadCNIRequestToCNIServer(r *CNIEndpointRequest) (*CNIServerRequest, error) {
	cmd, ok := r.ArgEnv["CNI_COMMAND"]
	if !ok {
		return nil, fmt.Errorf("cnishim req missing CNI_COMMAND")
	}

	cnishimreq := &CNIServerRequest{
		Command: CNIcommand(cmd),
	}

        cnishimreq.SandboxID, ok = r.ArgEnv["CNI_CONTAINERID"]
        if !ok {
                return nil, fmt.Errorf("cnishim req missing CNI_CONTAINERID")
        }

        cnishimreq.Netns, ok = r.ArgEnv["CNI_NETNS"]
        if !ok {
                return nil, fmt.Errorf("cnishim req missing CNI_NETNS")
        }

        cnishimreq.IfName, ok = r.ArgEnv["CNI_IFNAME"]
        if !ok {
                return nil, fmt.Errorf("cnishim req missing CNI_IFNAME")
        }

        cnishimArgs, err := loadCNIShimArgs(r.ArgEnv)
        if err != nil {
                return nil, err
        }

        cnishimreq.PodNamespace, ok = cnishimArgs["K8S_POD_NAMESPACE"]
        if !ok {
                return nil, fmt.Errorf("cnishim req missing K8S_POD_NAMESPACE")
        }

        cnishimreq.PodName, ok = cnishimArgs["K8S_POD_NAME"]
	if !ok {
		return nil, fmt.Errorf("cnishim req missing K8S_POD_NAME")
	}

        netconf, err := config.ConfigureNetConf(r.NetConfig)
        if err != nil {
                return nil, fmt.Errorf("cnishim req CNI arg configuration failed:%v",err)
        }

        cnishimreq.CNIConf = netconf
	return cnishimreq, nil
}

func (cs *CNIServer) handleCNIShimRequest(w http.ResponseWriter, r *http.Request) {
	var cr CNIEndpointRequest
	b, _ := ioutil.ReadAll(r.Body)
	if err := json.Unmarshal(b, &cr); err != nil {
		http.Error(w, fmt.Sprintf("%v", err), http.StatusBadRequest)
		return
	}

	req, err := loadCNIRequestToCNIServer(&cr)
	if err != nil {
		http.Error(w, fmt.Sprintf("%v", err), http.StatusBadRequest)
		return
	}

	klog.Infof("Waiting for %s result for CNI server pod %s/%s", req.Command, req.PodNamespace, req.PodName)
	result, err := cs.requestFunc(req, cs.k8sclient)
	if err != nil {
		http.Error(w, fmt.Sprintf("%v", err), http.StatusBadRequest)
	} else {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(result); err != nil {
                        klog.Warningf("Error writing %s HTTP response: %v", req.Command, err)
		}
        }
}

func HandleCNIcommandRequest(request *CNIServerRequest, k8sclient kubernetes.Interface) ([]byte, error) {
        var result []byte
	var err error
        klog.Infof("[PodNamespace:%s/PodName:%s] dispatching pod network request %v", request.PodNamespace, request.PodName, request)
        klog.Infof("k8sclient  %s", fmt.Sprintf("%v",k8sclient))
	switch request.Command {
	case CNIAdd:
		result, err = request.cmdAdd(k8sclient)
	case CNIDel:
		result, err = request.cmdDel()
	default:
	}
	klog.Infof("[PodNamespace:%s/PodName:%s] CNI request %v, result %q, err %v", request.PodNamespace, request.PodName, request, string(result), err)
	if err != nil {
		return nil, fmt.Errorf("[PodNamespace:%s/PodName:%s] CNI request %v %v", request.PodNamespace, request.PodName, request, err)
	}
	return result, nil
}

func (cs *CNIServer) Start(requestFunc cniServerRequestFunc) error {
	if requestFunc == nil {
		return fmt.Errorf("no CNI request handler")
	}
	cs.requestFunc = requestFunc
	socketPath := filepath.Join(cs.serverrundir, CNIServerSocketName)
	if err := os.RemoveAll(cs.serverrundir); err != nil && !os.IsNotExist(err) {
		info, err := os.Stat(cs.serverrundir)
		if err != nil {
			return fmt.Errorf("failed to stat old cni server info socket directory %s: %v", cs.serverrundir, err)
		}
		tmp := info.Sys()
		statt, ok := tmp.(*syscall.Stat_t)
		if !ok {
			return fmt.Errorf("failed to read CNI Server info socket directory stat info: %T", tmp)
		}
		if statt.Uid != 0 {
			return fmt.Errorf("insecure owner of CNI Server info socket directory %s: %v", cs.serverrundir, statt.Uid)
		}

		if info.Mode()&0777 != 0700 {
			return fmt.Errorf("insecure permissions on CNI Server info socket directory %s: %v", cs.serverrundir, info.Mode())
		}

		if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove old CNI Server info socket %s: %v", socketPath, err)
		}
	}
	if err := os.MkdirAll(cs.serverrundir, 0700); err != nil {
		return fmt.Errorf("failed to create CNI Server info socket directory %s: %v", cs.serverrundir, err)
	}

	unixListener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on CNI Server info socket: %v", err)
	}
	if err := os.Chmod(socketPath, 0600); err != nil {
		unixListener.Close()
		return fmt.Errorf("failed to set CNI Server info socket mode: %v", err)
	}

	cs.SetKeepAlivesEnabled(false)
	go utilwait.Forever(func() {
		if err := cs.Serve(unixListener); err != nil {
			utilruntime.HandleError(fmt.Errorf("CNI server Serve() failed: %v", err))
		}
	}, 0)
        return nil
}
