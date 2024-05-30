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
	Targets []string `yaml:"targets"`
}

var (
	pingLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ping_latency_seconds",
			Help:    "Ping latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"address", "ip"},
	)

	pingPackageSent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ping_packets_sent",
			Help: "Packets sent in ping",
		},
		[]string{"address", "ip"},
	)

	pingPackageLost = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ping_packets_lost",
			Help: "Packets lost in ping",
		},
		[]string{"address", "ip", "error"},
	)

	pingError = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ping_packets_error",
			Help: "Packets error in ping",
		},
		[]string{"address", "ip", "error"},
	)
)

func init() {
	prometheus.MustRegister(pingLatency)
	prometheus.MustRegister(pingPackageLost)
	prometheus.MustRegister(pingPackageSent)
	prometheus.MustRegister(pingError)
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

func pingTarget(address string) {
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

		if statistics.PacketLoss > 0 {
			pingPackageLost.WithLabelValues(address, statistics.Addr, "").Set(statistics.PacketLoss)
		}

		pingPackageSent.WithLabelValues(address, statistics.Addr).Add(float64(statistics.PacketsSent))
		pingLatency.WithLabelValues(address, statistics.IPAddr.String()).Observe(float64(statistics.AvgRtt.Microseconds()))

		log.Printf("Ping to %s: %v", address, statistics.AvgRtt.String())
		time.Sleep(time.Second)
	}
}

func main() {
	config, err := loadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	for _, target := range config.Targets {
		go pingTarget(target)
	}

	http.Handle("/metrics", promhttp.Handler())
	http.Handle("/", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("Ping Exporter"))
	}))
	log.Fatal(http.ListenAndServe(":9001", nil))
}
