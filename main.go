package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
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
)

func main() {
	prometheus.MustRegister(histogram)
	prometheus.MustRegister(gauge)

	conf := config{}
	conf.initConfig()
	startPing(conf.Targets)

	http.Handle("/metrics", promhttp.Handler())
	http.Handle("/", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("Ping Exporter"))
	}))
	log.Fatal(http.ListenAndServe(":9001", nil))
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

func startPing(targets []string) {
	for _, target := range targets {
		ticker := time.NewTicker(1 * time.Second)
		targetAddr := target
		go func() {
			for range ticker.C {
				Ping(targetAddr)
			}
		}()
	}

}

func Ping(target string) {
	ip, err := net.ResolveIPAddr("ip4", target)
	if err != nil {
		log.Println("解析 IP 失败:", err)
		return
	}
	conn, err := icmp.ListenPacket("udp4", "0.0.0.0")
	if err != nil {
		log.Println("监听地址失败:", err)
		return
	}
	defer conn.Close()

	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho, Code: 0,
		Body: &icmp.Echo{
			ID: os.Getpid() & 0xffff, Seq: 1,
			Data: []byte(""),
		},
	}
	msgBytes, err := msg.Marshal(nil)
	if err != nil {
		log.Printf("消息序列化失败 %v, %e", msgBytes, err)
		return
	}

	// 发送ICMP请求前记录时间
	beginTime := time.Now()

	// Write the message to the listening connection
	if _, err := conn.WriteTo(msgBytes, &net.UDPAddr{IP: net.ParseIP(ip.String())}); err != nil {
		log.Printf("写入请求失败 %v", err)
		return
	}

	// 接收ICMP响应
	err = conn.SetReadDeadline(time.Now().Add(time.Second * 1))
	if err != nil {
		log.Printf("请求超时 %v", err)
		return
	}
	reply := make([]byte, 1500)
	n, _, err := conn.ReadFrom(reply)
	if err != nil {
		log.Printf("读取响应失败 %v", err)
		return
	}
	// 接收到响应后记录时间
	endTime := time.Now()

	// 计算响应时间
	costTime := endTime.Sub(beginTime).Microseconds()
	log.Printf("从 %-25s 接收到响应，IP地址: %-16s, 耗时 %-10v us\n", target, ip.String(), costTime)

	parsedReply, err := icmp.ParseMessage(1, reply[:n])

	if err != nil {
		log.Printf("解析响应失败 %v", err)
		return
	}

	switch parsedReply.Code {
	case 0:
		// Got a reply so we can save this
		histogram.WithLabelValues(target, ip.String()).Observe(float64(costTime))
		gauge.WithLabelValues(target, ip.String()).Set(float64(costTime))
	case 11:
		// Time Exceeded so we can assume our network is slow
		log.Printf("地址 %s 响应慢\n", target)
	case 3:
	default:
		log.Printf("地址 %s 不可达\n", target)
		// Given that we don't expect google to be unreachable, we can assume that our network is down
		histogram.WithLabelValues(target, ip.String()).Observe(-1)
		gauge.WithLabelValues(target, ip.String()).Set(-1)
	}
}
