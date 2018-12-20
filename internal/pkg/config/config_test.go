package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/urfave/cli"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Config Test Suite")
}

var _ = AfterSuite(func() {
})

var _ = Describe("Test Config", func() {
	var app *cli.App
	var cfgFile *os.File
	var logFile *os.File
	var kubecfgFile *os.File

	BeforeEach(func() {
		app = cli.NewApp()
		app.Name = "test"
		app.Flags = Flags

		var err error
		cfgFile, err = ioutil.TempFile("", "ovn4nfvconf-")
		Expect(err).NotTo(HaveOccurred())
		logFile, err = ioutil.TempFile("", "ovn4nfvlog-")
		Expect(err).NotTo(HaveOccurred())
		kubecfgFile, err = ioutil.TempFile("", "ovn4nfvkubecfg-")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.Remove(cfgFile.Name())
		os.Remove(logFile.Name())
		os.Remove(kubecfgFile.Name())
	})

	It("uses expected defaults", func() {
		app.Action = func(ctx *cli.Context) error {
			cfgPath, err := InitConfig(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(cfgPath).To(Equal(cfgFile.Name()))

			Expect(Default.MTU).To(Equal(1400))
			Expect(Logging.File).To(Equal(""))
			Expect(Logging.Level).To(Equal(4))
			Expect(CNI.ConfDir).To(Equal("/etc/cni/net.d"))
			Expect(CNI.Plugin).To(Equal("ovn4nfvk8s-cni"))
			return nil
		}
		err := app.Run([]string{app.Name, "-config-file=" + cfgFile.Name(), "-k8s-kubeconfig=" + kubecfgFile.Name()})
		Expect(err).NotTo(HaveOccurred())
	})

	It("missing kubeconfig", func() {
		app.Action = func(ctx *cli.Context) error {
			_, err := InitConfig(ctx)
			Expect(err).To(HaveOccurred())

			return nil
		}
		err := app.Run([]string{app.Name, "-config-file=" + cfgFile.Name()})
		Expect(err).NotTo(HaveOccurred())
	})

	It("tests Config missing file", func() {
		app.Action = func(ctx *cli.Context) error {
			_, err := InitConfig(ctx)
			Expect(err).To(MatchError("failed to open config file NoExistant: open NoExistant: no such file or directory"))
			return nil
		}

		err := app.Run([]string{app.Name, "-config-file=NoExistant"})
		Expect(err).NotTo(HaveOccurred())
	})

	It("tests default config file", func() {
		app.Action = func(ctx *cli.Context) error {
			cfgPath, err := InitConfig(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(cfgPath).To(Equal(""))
			return nil
		}
		err := app.Run([]string{app.Name, "-k8s-kubeconfig=" + kubecfgFile.Name(), "-logfile=" + logFile.Name()})
		Expect(err).NotTo(HaveOccurred())
	})

	It("overrides defaults with config file options", func() {
		cfgData := fmt.Sprintf(`[default]
mtu=1500

[kubernetes]
kubeconfig=%s

[logging]
loglevel=5
logfile=%s

[cni]
conf-dir=/etc/cni/net.blah
plugin=ovn-nfv-k8s-blah`, kubecfgFile.Name(), logFile.Name())
		err := ioutil.WriteFile(cfgFile.Name(), []byte(cfgData), 0644)
		Expect(err).NotTo(HaveOccurred())

		app.Action = func(ctx *cli.Context) error {
			var cfgPath string
			cfgPath, err = InitConfig(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(cfgPath).To(Equal(cfgFile.Name()))

			Expect(Default.MTU).To(Equal(1500))
			Expect(Logging.File).To(Equal(logFile.Name()))
			Expect(Logging.Level).To(Equal(5))
			Expect(CNI.ConfDir).To(Equal("/etc/cni/net.blah"))
			Expect(CNI.Plugin).To(Equal("ovn-nfv-k8s-blah"))
			Expect(Kubernetes.Kubeconfig).To(Equal(kubecfgFile.Name()))

			return nil
		}
		err = app.Run([]string{app.Name, "-config-file=" + cfgFile.Name()})
		Expect(err).NotTo(HaveOccurred())
	})

	It("overrides defaults with command line options", func() {
		app.Action = func(ctx *cli.Context) error {
			var cfgPath string
			cfgPath, err := InitConfig(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(cfgPath).To(Equal(cfgFile.Name()))

			Expect(Default.MTU).To(Equal(1500))
			Expect(Logging.File).To(Equal(logFile.Name()))
			Expect(Logging.Level).To(Equal(5))
			Expect(CNI.ConfDir).To(Equal("/etc/cni/net.blah"))
			Expect(CNI.Plugin).To(Equal("ovn-nfv-k8s-blah"))
			Expect(Kubernetes.Kubeconfig).To(Equal(kubecfgFile.Name()))

			return nil
		}
		args := []string{
			app.Name,
			"-config-file=" + cfgFile.Name(),
			"-k8s-kubeconfig=" + kubecfgFile.Name(),
			"-mtu=1500",
			"-loglevel=5", "-logfile=" + logFile.Name(),
			"-cni-conf-dir=/etc/cni/net.blah",
			"-cni-plugin=ovn-nfv-k8s-blah"}

		err := app.Run(args)
		Expect(err).NotTo(HaveOccurred())
	})
})
