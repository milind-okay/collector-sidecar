package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/Sirupsen/logrus"
	"github.com/kardianos/service"
	"github.com/rakyll/globalconf"

	"github.com/Graylog2/sidecar/backends"
	"github.com/Graylog2/sidecar/context"
	"github.com/Graylog2/sidecar/services"
	"github.com/Graylog2/sidecar/util"

	// importing backend packages to ensure init() is called
	_ "github.com/Graylog2/sidecar/backends/nxlog"
)

func main() {
	sidecarPath, err := util.GetSidecarPath()
	if err != nil {
		logrus.Fatal("Can not find path to Sidecar installation.")
	}

	sidecarConfigurationFile := filepath.Join(sidecarPath, "sidecar.ini")
	if _, err := os.Stat(sidecarConfigurationFile); os.IsNotExist(err) {
		logrus.Fatal("Can not open configuration file " + sidecarConfigurationFile)
	}

	// parse .ini file or use command line switches
	conf, _ := globalconf.NewWithOptions(&globalconf.Options{
		Filename:  sidecarConfigurationFile,
		EnvPrefix: "SIDECAR_",
	})

	var (
		svcFlag       = flag.String("service", "", "Control the system service")
		collectorPath = flag.String("collector-path", "", "Path to collector installation")
		serverUrl     = flag.String("server-url", "", "Graylog server URL")
		nodeId        = flag.String("node-id", "graylog-collector", "Collector identification string")
		collectorId   = flag.String("collector-id", "", "UUID used for collector registration")
		tags          = flag.String("tags", "", "Comma separated tag list")
	)
	conf.ParseAll()

	// initialize application context
	context := context.NewContext(*serverUrl, *collectorPath, *nodeId, *collectorId)
	context.Tags = util.SplitCommaList(*tags)
	if (len(context.Tags) != 0) {
		logrus.Info("Fetching configuration tagged by: ", context.Tags)
	}


	nxlog, err := backends.GetBackend("nxlog")
	if err != nil {
		logrus.Fatal("Exiting.")
	}
	context.Backend = nxlog(*collectorPath)

	// set backend related context values
	context.Config.Exec = context.Backend.ExecPath()
	context.Config.Args = context.Backend.ExecArgs(sidecarPath)

	// setup system service
	serviceConfig := &service.Config{
		Name:        context.Config.Name,
		DisplayName: context.Config.DisplayName,
		Description: context.Config.Description,
	}

	s, err := service.New(context.Program, serviceConfig)
	if err != nil {
		log.Fatal(err)
	}
	context.Program.BindToService(s)
	context.Service = s

	if len(*svcFlag) != 0 {
		err := service.Control(s, *svcFlag)
		if err != nil {
			logrus.Info("Valid service actions: %q\n", service.ControlAction)
			logrus.Fatal(err)
		}
		return
	}

	// start main loop
	services.StartPeriodicals(context)
	err = s.Run()
	if err != nil {
		logrus.Fatal(err)
	}
}
