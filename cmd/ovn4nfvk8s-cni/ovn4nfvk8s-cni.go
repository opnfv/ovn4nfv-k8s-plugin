package main

import (
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	cni "ovn4nfv-k8s-plugin/internal/pkg/cnishim"
	"ovn4nfv-k8s-plugin/internal/pkg/config"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/containernetworking/plugins/pkg/utils/buildversion"
)

func main() {
	logrus.Infof("ovn4nfvk8s-cni shim cni")
	c := cli.NewApp()
	c.Name = "ovn4nfvk8s-cni"
	c.Usage = "a CNI plugin to set up or tear down a additional interfaces with OVN"
	c.Version = "0.1.0"
	c.Flags = config.Flags

	ep := cni.CNIEndpoint("")
	c.Action = func(ctx *cli.Context) error {
		if _, err := config.InitConfig(ctx); err != nil {
			return err
		}
		skel.PluginMain(
			ep.CmdAdd,
			ep.CmdCheck,
			ep.CmdDel,
			version.All,
			buildversion.BuildString("ovn4nfv-k8s shim cni"))

		return nil
	}

	if err := c.Run(os.Args); err != nil {
		// Print the error to stdout in conformance with the CNI spec
		e, ok := err.(*types.Error)
		if !ok {
			e = &types.Error{Code: 100, Msg: err.Error()}
		}
		e.Print()
	}
}
