package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-cmd/cmd"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/yaml.v2"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	histogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "metric",
		Name:      "ping_response",
		Help:      "Ping Response Time",
		Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 15, 20, 50, 100, 200, 500, 1000, 5_000, 10_000},
	}, []string{"target", "ip"})

	gauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "metric",
		Name:      "ping_response_time",
		Help:      "Ping Response Time Gauge",
	}, []string{"target", "ip"})

	addressMap = map[string]string{}
)

func main() {
	prometheus.MustRegister(histogram)
	prometheus.MustRegister(gauge)

	conf := config{}
	conf.initConfig()

	for _, target := range conf.Targets {
		addr, err := parseIpAddr(target)
		if err != nil {
			log.Println("解析地址: ", target, " 失败: ", err)
			continue
		}
		addressMap[addr] = target

		executePing(target)
	}

	http.Handle("/metrics", promhttp.Handler())
	http.Handle("/", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("Ping Exporter"))
	}))
	log.Fatal(http.ListenAndServe(":9001", nil))
}

func parseIpAddr(target string) (string, error) {
	addr, err := net.ResolveIPAddr("ip4:icmp", target)
	if err != nil {
		log.Println("无法解析地址", target)
		return "", err
	} else {
		log.Println("target: ", target, " 解析到的 IP 地址是: ", addr.String())
		return addr.String(), nil
	}
}

func executePing(address string) {
	findCmd := cmd.NewCmd("ping", address)
	findCmd.Start()

	ticker := time.NewTicker(1 * time.Second)

	go func() {
		for range ticker.C {
			status := findCmd.Status()
			n := len(status.Stdout)
			result := status.Stdout[n-1]
			log.Println("Execute result: ", result)

			if strings.Contains(result, "bytes from ") {
				ipContent := strings.Split(result, "bytes from ")
				ipStr := strings.Split(ipContent[1], ":")
				timeContent := strings.Split(result, "time=")
				timeStr := strings.Split(timeContent[1], " ms")
				timeValue, _ := strconv.ParseFloat(timeStr[0], 64)

				ipAddr := ipStr[0]
				domain := addressMap[ipAddr]

				histogram.WithLabelValues(domain, ipAddr).Observe(timeValue)
				gauge.WithLabelValues(domain, ipAddr).Set(timeValue)

			} else {
				histogram.WithLabelValues(address, "").Observe(-1)
				gauge.WithLabelValues(address, "").Set(-1)
			}
		}
	}()
}

type config struct {
	Targets []string `yaml:"targets"`
}

func (c *config) initConfig() *config {
	configFile := os.Getenv("CONFIG_FILE")
	if configFile == "" {
		configFile = "config.yaml"
	}
	yamlFile, err := os.ReadFile(configFile)
	if err != nil {
		log.Printf("yamlFile.Get err   #%v ", err)
	}
	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		log.Fatalf("Unmarshal: %v", err)
	}

	return c
}
