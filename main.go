package main

import (
	"log"
	"net/http"
	"os"
	"time"

	ping "github.com/prometheus-community/pro-bing"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Targets map[string][]string `yaml:"targets"`
}

var (
	pingLatency = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "ping_latency",
			Help: "Ping latency in millisecond",
		},
		[]string{"group", "address", "ip"},
	)

	pingPackageSent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ping_packets_sent",
			Help: "Packets sent in ping",
		},
		[]string{"group", "address", "ip"},
	)

	pingPackageLost = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ping_packets_lost",
			Help: "Packets lost in ping",
		},
		[]string{"group", "address", "ip"},
	)
)

func init() {
	prometheus.MustRegister(pingLatency)
	prometheus.MustRegister(pingPackageLost)
	prometheus.MustRegister(pingPackageSent)
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func pingTarget(group, address string) {
	pinger, err := ping.NewPinger(address)
	if err != nil {
		log.Printf("Failed to create pinger for %s: %v", address, err)
		return
	}
	pinger.Count = 1
	pinger.Timeout = time.Second

	for {
		err = pinger.Run()
		if err != nil {
			log.Printf("Run Ping to %s failed: %v", address, err)
			time.Sleep(time.Second) // Sleep before retrying
			continue
		}

		statistics := pinger.Statistics()

		pingPackageLost.WithLabelValues(group, address, statistics.Addr).Set(statistics.PacketLoss)
		pingPackageSent.WithLabelValues(group, address, statistics.Addr).Add(float64(statistics.PacketsSent))
		pingLatency.WithLabelValues(group, address, statistics.IPAddr.String()).Observe(float64(statistics.AvgRtt.Milliseconds()))

		log.Printf("Ping to %s: %v", address, statistics.AvgRtt.String())
		time.Sleep(time.Second)
	}
}

func main() {
	configFile := os.Getenv("CONFIG_FILE")
	if configFile == "" {
		configFile = "config.yaml"
	}
	config, err := loadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	for group, targets := range config.Targets {
		for _, address := range targets {
			go pingTarget(group, address)
		}
	}

	http.Handle("/metrics", promhttp.Handler())
	http.Handle("/", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("Ping Exporter"))
	}))
	log.Fatal(http.ListenAndServe(":9001", nil))
}
