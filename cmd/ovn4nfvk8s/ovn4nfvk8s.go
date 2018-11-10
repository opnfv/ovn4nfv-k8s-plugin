package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	kexec "k8s.io/utils/exec"

	"ovn4nfv-k8s-plugin/internal/pkg/config"
	"ovn4nfv-k8s-plugin/internal/pkg/factory"
	"ovn4nfv-k8s-plugin/internal/pkg/ovn"
	"ovn4nfv-k8s-plugin/internal/pkg/util"
)

func main() {
	c := cli.NewApp()
	c.Name = "ovn4nfvk8s"
	c.Usage = "run ovn4nfvk8s to start pod watchers"
	c.Version = config.Version
	c.Flags = append([]cli.Flag{
		// Daemon file
		cli.StringFlag{
			Name:  "pidfile",
			Usage: "Name of file that will hold the ovn4nfvk8s pid (optional)",
		},
	}, config.Flags...)
	c.Action = func(c *cli.Context) error {
		return runOvnKube(c)
	}

	if err := c.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

func delPidfile(pidfile string) {
	if pidfile != "" {
		if _, err := os.Stat(pidfile); err == nil {
			if err := os.Remove(pidfile); err != nil {
				logrus.Errorf("%s delete failed: %v", pidfile, err)
			}
		}
	}
}

func runOvnKube(ctx *cli.Context) error {
	fmt.Println("ovn4nfvk8s started")
	exec := kexec.New()
	_, err := config.InitConfig(ctx, exec, nil)
	if err != nil {
		return err
	}
	pidfile := ctx.String("pidfile")

	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		delPidfile(pidfile)
		os.Exit(1)
	}()

	defer delPidfile(pidfile)

	if pidfile != "" {
		// need to test if already there
		_, err := os.Stat(pidfile)

		// Create if it doesn't exist, else exit with error
		if os.IsNotExist(err) {
			if err := ioutil.WriteFile(pidfile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644); err != nil {
				logrus.Errorf("failed to write pidfile %s (%v). Ignoring..", pidfile, err)
			}
		} else {
			// get the pid and see if it exists
			pid, err := ioutil.ReadFile(pidfile)
			if err != nil {
				logrus.Errorf("pidfile %s exists but can't be read", pidfile)
				return err
			}
			_, err1 := os.Stat("/proc/" + string(pid[:]) + "/cmdline")
			if os.IsNotExist(err1) {
				// Left over pid from dead process
				if err := ioutil.WriteFile(pidfile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644); err != nil {
					logrus.Errorf("failed to write pidfile %s (%v). Ignoring..", pidfile, err)
				}
			} else {
				logrus.Errorf("pidfile %s exists and ovn4nfvk8s is running", pidfile)
				os.Exit(1)
			}
		}
	}

	if err = util.SetExec(exec); err != nil {
		logrus.Errorf("Failed to initialize exec helper: %v", err)
		return err
	}

	clientset, err := config.NewClientset(&config.Kubernetes)
	if err != nil {
		panic(err.Error())
	}

	// Create distributed router and gateway for the deployment
	err = ovn.SetupMaster("ovn4nfv-master")
	if err != nil {
		logrus.Errorf(err.Error())
		panic(err.Error())
	}
	// create factory and start the ovn controller
	stopChan := make(chan struct{})
	factory, err := factory.NewWatchFactory(clientset, stopChan)
	if err != nil {
		panic(err.Error)
	}

	ovnController := ovn.NewOvnController(clientset, factory)
	if err := ovnController.Run(); err != nil {
		logrus.Errorf(err.Error())
		panic(err.Error())
	}
	// run forever
	select {}

	return nil
}
