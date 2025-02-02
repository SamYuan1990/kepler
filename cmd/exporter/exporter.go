/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/sustainable-computing-io/kepler/pkg/bpf"
	"github.com/sustainable-computing-io/kepler/pkg/build"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/manager"
	"github.com/sustainable-computing-io/kepler/pkg/metrics"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/components"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/platform"
	"gopkg.in/yaml.v3"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"k8s.io/klog/v2"
)

const (

	// to change these msg, you also need to update the e2e test
	finishingMsg = "Exiting..."
	startedMsg   = "Started Kepler in %s"
)

type TLSConfig struct {
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

type TLSServerConfig struct {
	TLSConfig TLSConfig `yaml:"tls_server_config"`
}

// AppConfig holds the configuration info for the application.
type AppConfig struct {
	BaseDir                      string
	Address                      string
	MetricsPath                  string
	EnableGPU                    bool
	EnableEBPFCgroupID           bool
	ExposeHardwareCounterMetrics bool
	EnableMSR                    bool
	Kubeconfig                   string
	ApiserverEnabled             bool
	RedfishCredFilePath          string
	ExposeEstimatedIdlePower     bool
	MachineSpecFilePath          string
	DisablePowerMeter            bool
	TLSFilePath                  string
}
// Comments below is assisted by Gen AI
// // newAppConfig initializes and returns a new instance of AppConfig with default values.
// It sets up command-line flags for configuring various aspects of the application.
// The function returns a pointer to the initialized AppConfig struct.
//
// The following flags are defined:
//   - config-dir: Path to the base directory for configuration files (default: config.BaseDir).
//   - address: The bind address for the server (default: "0.0.0.0:8888").
//   - metrics-path: The path where Prometheus metrics are exposed (default: "/metrics").
//   - enable-gpu: Whether to enable GPU support (requires libnvidia-ml to be installed) (default: false).
//   - enable-cgroup-id: Whether to enable eBPF to collect cgroup IDs (default: true).
//   - expose-hardware-counter-metrics: Whether to expose hardware counters as Prometheus metrics (default: true).
//   - enable-msr: Whether to allow MSR (Model-Specific Registers) to obtain energy data (default: false).
//   - kubeconfig: Absolute path to the kubeconfig file; if empty, in-cluster configuration is used (default: "").
//   - apiserver: Whether to enable the API server; if disabled, pod information is collected from kubelet (default: true).
//   - redfish-cred-file-path: Path to the Redfish credential file (default: "").
//   - expose-estimated-idle-power: Whether to expose the estimated idle power as a metric (default: false).
//   - machine-spec: Path to the machine specification file in JSON format (default: "").
//   - disable-power-meter: Whether to disable power meter readings and use the estimator for node powers (default: false).
//   - web.config.file: Path to the TLS web configuration file (default: "").
//
// Returns:
//   - *AppConfig: A pointer to the initialized AppConfig struct with default values.
func newAppConfig() *AppConfig {
	// Initialize flags
	cfg := &AppConfig{}
	flag.StringVar(&cfg.BaseDir, "config-dir", config.BaseDir, "path to config base directory")
	flag.StringVar(&cfg.Address, "address", "0.0.0.0:8888", "bind address")
	flag.StringVar(&cfg.MetricsPath, "metrics-path", "/metrics", "metrics path")
	flag.BoolVar(&cfg.EnableGPU, "enable-gpu", false, "whether enable gpu (need to have libnvidia-ml installed)")
	flag.BoolVar(&cfg.EnableEBPFCgroupID, "enable-cgroup-id", true, "whether enable eBPF to collect cgroup id")
	flag.BoolVar(&cfg.ExposeHardwareCounterMetrics, "expose-hardware-counter-metrics", true, "whether expose hardware counter as prometheus metrics")
	flag.BoolVar(&cfg.EnableMSR, "enable-msr", false, "whether MSR is allowed to obtain energy data")
	flag.StringVar(&cfg.Kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file, if empty we use the in-cluster configuration")
	flag.BoolVar(&cfg.ApiserverEnabled, "apiserver", true, "if apiserver is disabled, we collect pod information from kubelet")
	flag.StringVar(&cfg.RedfishCredFilePath, "redfish-cred-file-path", "", "path to the redfish credential file")
	flag.BoolVar(&cfg.ExposeEstimatedIdlePower, "expose-estimated-idle-power", false, "Whether to expose the estimated idle power as a metric")
	flag.StringVar(&cfg.MachineSpecFilePath, "machine-spec", "", "path to the machine spec file in json format")
	flag.BoolVar(&cfg.DisablePowerMeter, "disable-power-meter", false, "whether manually disable power meter read and forcefully apply the estimator for node powers")
	flag.StringVar(&cfg.TLSFilePath, "web.config.file", "", "path to TLS web config file")

	return cfg
}
// Comments below is assisted by Gen AI
// // healthProbe is an HTTP handler function that responds to health check requests.
// It writes an HTTP 200 OK status code and a simple "ok" message to the response.
// If the response cannot be written, the function logs a fatal error and terminates the program.
//
// Example usage:
//   http.HandleFunc("/health", healthProbe)
//
// Parameters:
//   - w: The http.ResponseWriter used to write the HTTP response.
//   - req: The *http.Request representing the incoming HTTP request.
//
// Notes:
//   - This function is typically used as a health check endpoint in microservices or applications.
//   - If the response writing fails, the function logs the error and exits the program using klog.Fatalf.
func healthProbe(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(`ok`))
	if err != nil {
		klog.Fatalf("%s", fmt.Sprintf("failed to write response: %v", err))
	}
}
// Comments below is assisted by Gen AI
// // main is the entry point of the application. It initializes the application configuration,
// parses command-line flags, and sets up the necessary configurations for the application to run.
//
// The function performs the following steps:
//  1. Initializes logging flags using klog.
//  2. Creates a new application configuration using the newAppConfig function.
//  3. Parses command-line flags provided by the user.
//  4. Initializes the application configuration by calling config.Initialize with the base directory
//     specified in the appConfig. If initialization fails, the function logs a fatal error and exits.
//
// Example usage:
//   go run main.go --base-dir=/path/to/config
//
// Note: Ensure that all required flags are defined in the newAppConfig function before calling flag.Parse.
func main() {
	start := time.Now()
	klog.InitFlags(nil)
	appConfig := newAppConfig() // Initialize appConfig and define flags
	flag.Parse()                // Parse command-line flags

	if _, err := config.Initialize(appConfig.BaseDir); err != nil {
		klog.Fatalf("Failed to initialize config: %v", err)
	}

	klog.Infof("Kepler running on version: %s", build.Version)

	registry := metrics.GetRegistry()
	registry.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "kepler_exporter_build_info",
			Help: "A metric with a constant '1' value labeled by version, revision, branch, os and arch from which kepler_exporter was built.",
			ConstLabels: prometheus.Labels{
				"branch":   build.Branch,
				"revision": build.Revision,
				"version":  build.Version,
				"os":       build.OS,
				"arch":     build.Arch,
			},
		},
		func() float64 { return 1 },
	))

	platform.SetIsSystemCollectionSupported(!appConfig.DisablePowerMeter)
	components.SetIsSystemCollectionSupported(!appConfig.DisablePowerMeter)

	config.SetEnabledEBPFCgroupID(appConfig.EnableEBPFCgroupID)
	config.SetEnabledHardwareCounterMetrics(appConfig.ExposeHardwareCounterMetrics)
	config.SetEnabledGPU(appConfig.EnableGPU)
	config.SetEnabledMSR(appConfig.EnableMSR)
	config.SetEnabledIdlePower(appConfig.ExposeEstimatedIdlePower)

	config.SetKubeConfig(appConfig.Kubeconfig)
	config.SetEnableAPIServer(appConfig.ApiserverEnabled)

	// set redfish credential file path
	if appConfig.RedfishCredFilePath != "" {
		config.SetRedfishCredFilePath(appConfig.RedfishCredFilePath)
	}

	if appConfig.MachineSpecFilePath != "" {
		config.SetMachineSpecFilePath(appConfig.MachineSpecFilePath)
	}

	config.LogConfigs()

	components.InitPowerImpl()
	defer components.StopPower()
	platform.InitPowerImpl()
	defer platform.StopPower()

	if config.IsGPUEnabled() {
		r := accelerator.GetRegistry()
		if a, err := accelerator.New(config.GPU, true); err == nil {
			r.MustRegister(a) // Register the accelerator with the registry
		} else {
			klog.Errorf("failed to init GPU accelerators: %v", err)
		}
		defer accelerator.Shutdown()
	}

	bpfExporter, err := bpf.NewExporter()
	if err != nil {
		klog.Fatalf("failed to create eBPF exporter: %v", err)
	}
	defer bpfExporter.Detach()

	m := manager.New(bpfExporter)
	if m == nil {
		klog.Fatal("could not create a collector manager")
	}
	defer m.Stop()

	// starting a CollectorManager instance to collect data and report metrics
	if startErr := m.Start(); startErr != nil {
		klog.Infof("%s", fmt.Sprintf("failed to start : %v", startErr))
	}
	metricPathConfig := config.GetMetricPath(appConfig.MetricsPath)
	bindAddressConfig := config.GetBindAddress(appConfig.Address)

	var certFile, keyFile string
	tlsConfigured := false

	// Retrieve the TLS config
	if appConfig.TLSFilePath != "" {
		configPath := appConfig.TLSFilePath

		configFile, err := os.Open(configPath)
		if err != nil {
			klog.Errorf("Error opening config file: %v\n", err)
		}
		defer configFile.Close()

		var tlsServerConfig TLSServerConfig
		decoder := yaml.NewDecoder(configFile)
		if err := decoder.Decode(&tlsServerConfig); err != nil {
			klog.Errorf("Error parsing config file: %v\n", err)
		}

		if tlsServerConfig.TLSConfig.CertFile != "" && tlsServerConfig.TLSConfig.KeyFile != "" {
			certFile = tlsServerConfig.TLSConfig.CertFile
			keyFile = tlsServerConfig.TLSConfig.KeyFile
			tlsConfigured = true
		}
	}

	handler := http.ServeMux{}
	reg := m.PrometheusCollector.RegisterMetrics()
	handler.Handle(metricPathConfig, promhttp.HandlerFor(
		reg,
		promhttp.HandlerOpts{
			Registry: reg,
		},
	))
	handler.HandleFunc("/healthz", healthProbe)
	handler.HandleFunc("/", rootHandler(metricPathConfig))
	handler.HandleFunc("/debug/pprof/", http.DefaultServeMux.ServeHTTP)
	srv := &http.Server{
		Addr:    bindAddressConfig,
		Handler: &handler,
	}

	klog.Infof("starting to listen on %s", bindAddressConfig)
	errChan := make(chan error)

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		if tlsConfigured {
			// Run server in TLS mode
			klog.Infof("Starting server with TLS")
			err = srv.ListenAndServeTLS(certFile, keyFile)
		} else {
			// Fall back to non-TLS mode
			klog.Infof("Starting server without TLS")
			err = srv.ListenAndServe()
		}

		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
	}()
	klog.Infof(startedMsg, time.Since(start))
	klog.Flush() // force flush to parse the start msg in the e2e test

	// Wait for an exit signal

	ctx := context.Background()
	select {
	case err := <-errChan:
		klog.Fatalf("%s", fmt.Sprintf("failed to listen and serve: %v", err))
	case <-signalChan:
		klog.Infof("Received shutdown signal")
		ctx, cancel := context.WithDeadline(ctx, time.Now().Add(5*time.Second))
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			klog.Fatalf("%s", fmt.Sprintf("failed to shutdown gracefully: %v", err))
		}
	}
	wg.Wait()
	klog.Infoln(finishingMsg)
	klog.FlushAndExit(klog.ExitFlushTimeout, 0)
}

func rootHandler(metricPathConfig string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte(`<html>
					<head><title>Energy Stats Exporter</title></head>
					<body>
					<h1>Energy Stats Exporter</h1>
					<p><a href="` + metricPathConfig + `">Metrics</a></p>
					</body>
					</html>`)); err != nil {
			klog.Errorf("%s", fmt.Sprintf("failed to write http response: %v", err))
		}
	}
}
