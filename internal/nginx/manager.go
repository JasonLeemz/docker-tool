package nginx

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"text/template"

	"github.com/docker/go-connections/nat"

	"docker-tool/internal/config"
)

// Manager nginx配置管理器
type Manager struct {
	config        *config.Config
	httpConfigs   map[string]*HTTPConfig
	streamConfigs map[string]*StreamConfig
	mutex         sync.RWMutex
}

// HTTPConfig HTTP服务配置
type HTTPConfig struct {
	ServiceName string
	Domain      string
	Path        string
	Upstream    []UpstreamServer
	ProxyConfig *config.ProxyConfig
}

// StreamConfig Stream服务配置
type StreamConfig struct {
	ServiceName     string
	ListenPort      int
	Upstream        []UpstreamServer
	// SNI 路由相关字段
	EnableSNI       bool
	DomainRoutes    map[string]string     // 域名到upstream的映射
	StaticUpstreams map[string][]string   // 静态upstream配置
}

// UpstreamServer 上游服务器
type UpstreamServer struct {
	IP   string
	Port nat.Port
}

// HTTPTemplateData HTTP配置模板数据
type HTTPTemplateData struct {
	ServiceName          string
	Domain               string
	Path                 string
	Upstream             []UpstreamServer
	EnableWebSocket      bool
	ClientMaxBodySize    string
	ProxyHTTPVersion     string
	ProxyHeaders         []string
	ProxyRedirect        string
	// SSL 相关配置
	EnableSSL            bool
	SSLCertificate       string
	SSLCertificateKey    string
	ForceHTTPS           bool
}

// StreamTemplateData Stream配置模板数据
type StreamTemplateData struct {
	ServiceName   string
	ListenPort    int
	Upstream      []UpstreamServer
	// SNI 路由相关字段
	EnableSNI     bool
	DomainRoutes  map[string]string       // 域名到upstream的映射
	DefaultRoute  string                  // 默认路由
	StaticUpstreams map[string][]string   // 静态upstream配置
}

// loadTemplate 从文件加载模板内容
func (m *Manager) loadTemplate(templateFile string) (string, error) {
	if templateFile == "" {
		return "", fmt.Errorf("模板文件路径为空")
	}

	// 检查文件是否存在
	if _, err := os.Stat(templateFile); os.IsNotExist(err) {
		return "", fmt.Errorf("模板文件不存在: %s", templateFile)
	}

	// 读取文件内容
	file, err := os.Open(templateFile)
	if err != nil {
		return "", fmt.Errorf("打开模板文件失败: %w", err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("读取模板文件失败: %w", err)
	}

	return string(content), nil
}


// NewManager 创建nginx管理器
func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		config:        cfg,
		httpConfigs:   make(map[string]*HTTPConfig),
		streamConfigs: make(map[string]*StreamConfig),
	}
}

// UpdateService 更新服务配置
func (m *Manager) UpdateService(service *config.ServiceConfig, containerIP string, containerPort nat.Port) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	switch service.Type {
	case "http":
		return m.updateHTTPService(service, containerIP, containerPort)
	case "stream":
		return m.updateStreamService(service, containerIP, containerPort)
	default:
		return fmt.Errorf("不支持的服务类型: %s", service.Type)
	}
}

// updateHTTPService 更新HTTP服务配置
func (m *Manager) updateHTTPService(service *config.ServiceConfig, containerIP string, containerPort nat.Port) error {
	// 获取或创建HTTP配置
	httpConfig, exists := m.httpConfigs[service.Name]
	if !exists {
		httpConfig = &HTTPConfig{
			ServiceName: service.Name,
			Domain:      service.Domain,
			Path:        service.Path,
			Upstream:    make([]UpstreamServer, 0),
			ProxyConfig: service.ProxyConfig,
		}
		m.httpConfigs[service.Name] = httpConfig
	}

	// 更新上游服务器列表
	if containerIP != "" && containerPort != "" {
		// 添加或更新服务器
		server := UpstreamServer{
			IP:   containerIP,
			Port: containerPort,
		}
		m.updateUpstreamServer(&httpConfig.Upstream, server)
	} else {
		// 移除服务器
		m.removeUpstreamServer(&httpConfig.Upstream, containerIP)
	}

	// 生成配置文件
	return m.generateHTTPConfig(httpConfig)
}

// updateStreamService 更新Stream服务配置
func (m *Manager) updateStreamService(service *config.ServiceConfig, containerIP string, containerPort nat.Port) error {
	// 获取或创建Stream配置
	streamConfig, exists := m.streamConfigs[service.Name]
	if !exists {
		streamConfig = &StreamConfig{
			ServiceName:     service.Name,
			ListenPort:      service.ListenPort,
			Upstream:        make([]UpstreamServer, 0),
			EnableSNI:       service.EnableSNI,
			DomainRoutes:    service.DomainRoutes,
			StaticUpstreams: service.StaticUpstreams,
		}
		m.streamConfigs[service.Name] = streamConfig
	}

	// 更新上游服务器列表
	if containerIP != "" && containerPort != "" {
		// 添加或更新服务器
		server := UpstreamServer{
			IP:   containerIP,
			Port: containerPort,
		}
		m.updateUpstreamServer(&streamConfig.Upstream, server)
	} else {
		// 移除服务器
		m.removeUpstreamServer(&streamConfig.Upstream, containerIP)
	}

	// 生成配置文件
	return m.generateStreamConfig(streamConfig)
}

// updateUpstreamServer 更新上游服务器
func (m *Manager) updateUpstreamServer(upstream *[]UpstreamServer, server UpstreamServer) {
	// 查找是否已存在相同IP的服务器
	for i, existingServer := range *upstream {
		if existingServer.IP == server.IP {
			(*upstream)[i] = server
			return
		}
	}
	// 如果不存在，添加新服务器
	*upstream = append(*upstream, server)
}

// removeUpstreamServer 移除上游服务器
func (m *Manager) removeUpstreamServer(upstream *[]UpstreamServer, ip string) {
	for i, server := range *upstream {
		if server.IP == ip {
			*upstream = append((*upstream)[:i], (*upstream)[i+1:]...)
			return
		}
	}
}

// generateHTTPConfig 生成HTTP配置文件
func (m *Manager) generateHTTPConfig(httpConfig *HTTPConfig) error {
	if len(httpConfig.Upstream) == 0 {
		// 如果没有上游服务器，删除配置文件
		return m.deleteHTTPConfig(httpConfig.ServiceName)
	}

	// 生成配置内容
	configContent := m.buildHTTPConfigContent(httpConfig)

	// 写入配置文件
	filename := fmt.Sprintf("%s.conf", httpConfig.ServiceName)
	filepath := filepath.Join(m.config.Global.NginxConfigDir, filename)

	if err := os.WriteFile(filepath, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("写入HTTP配置文件失败 [%s]: %w", filename, err)
	}

	return nil
}

// generateStreamConfig 生成Stream配置文件
func (m *Manager) generateStreamConfig(streamConfig *StreamConfig) error {
	// 对于SNI配置，即使Upstream为空也要生成配置（使用StaticUpstreams）
	if len(streamConfig.Upstream) == 0 && !streamConfig.EnableSNI {
		// 如果没有上游服务器且不是SNI配置，删除配置文件
		return m.deleteStreamConfig(streamConfig.ServiceName)
	}

	// 生成配置内容
	configContent := m.buildStreamConfigContent(streamConfig)

	// 写入配置文件
	filename := fmt.Sprintf("%s.conf", streamConfig.ServiceName)
	filepath := filepath.Join(m.config.Global.StreamConfigDir, filename)

	if err := os.WriteFile(filepath, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("写入Stream配置文件失败 [%s]: %w", filename, err)
	}

	return nil
}

// buildHTTPConfigContent 构建HTTP配置内容
func (m *Manager) buildHTTPConfigContent(httpConfig *HTTPConfig) string {
	// 使用服务配置的代理配置，如果没有则使用全局默认配置
	proxyConfig := httpConfig.ProxyConfig
	if proxyConfig == nil {
		proxyConfig = &m.config.Global.DefaultProxy
	}

	// 准备模板数据
	templateData := HTTPTemplateData{
		ServiceName:          httpConfig.ServiceName,
		Domain:               httpConfig.Domain,
		Path:                 httpConfig.Path,
		Upstream:             httpConfig.Upstream,
		EnableWebSocket:      proxyConfig.EnableWebSocket,
		ClientMaxBodySize:    proxyConfig.ClientMaxBodySize,
		ProxyHTTPVersion:     proxyConfig.ProxyHTTPVersion,
		ProxyHeaders:         proxyConfig.ProxyHeaders,
		ProxyRedirect:        proxyConfig.ProxyRedirect,
		// SSL 配置
		EnableSSL:            m.config.Global.SSLCertPath != "" && m.config.Global.SSLKeyPath != "",
		SSLCertificate:       m.config.Global.SSLCertPath,
		SSLCertificateKey:    m.config.Global.SSLKeyPath,
		ForceHTTPS:           m.config.Global.ForceHTTPS,
	}

	// 加载模板内容
	templateContent, err := m.loadTemplate(m.config.Global.HTTPTemplateFile)
	if err != nil {
		log.Printf("加载HTTP配置模板失败: %v", err)
		return ""
	}

	// 解析模板
	tmpl, err := template.New("httpConfig").Parse(templateContent)
	if err != nil {
		log.Printf("解析HTTP配置模板失败: %v", err)
		return ""
	}

	// 渲染模板
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateData); err != nil {
		log.Printf("渲染HTTP配置模板失败: %v", err)
		return ""
	}

	return buf.String()
}

// buildStreamConfigContent 构建Stream配置内容
func (m *Manager) buildStreamConfigContent(streamConfig *StreamConfig) string {
	// 准备模板数据
	templateData := StreamTemplateData{
		ServiceName:     streamConfig.ServiceName,
		ListenPort:      streamConfig.ListenPort,
		Upstream:        streamConfig.Upstream,
		EnableSNI:       streamConfig.EnableSNI,
		DomainRoutes:    streamConfig.DomainRoutes,
		DefaultRoute:    streamConfig.ServiceName,
		StaticUpstreams: streamConfig.StaticUpstreams,
	}

	// 选择合适的模板文件
	templateFile := m.config.Global.StreamTemplateFile
	if streamConfig.EnableSNI {
		// 如果启用SNI，使用SNI模板
		templateFile = m.config.Global.StreamSNITemplateFile
		if templateFile == "" {
			templateFile = "conf/stream-sni.conf.tpl" // 默认SNI模板路径
		}
	}

	// 加载模板内容
	templateContent, err := m.loadTemplate(templateFile)
	if err != nil {
		log.Printf("加载Stream配置模板失败: %v", err)
		return ""
	}

	// 解析模板
	tmpl, err := template.New("streamConfig").Parse(templateContent)
	if err != nil {
		log.Printf("解析Stream配置模板失败: %v", err)
		return ""
	}

	// 渲染模板
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateData); err != nil {
		log.Printf("渲染Stream配置模板失败: %v", err)
		return ""
	}

	return buf.String()
}

// deleteHTTPConfig 删除HTTP配置文件
func (m *Manager) deleteHTTPConfig(serviceName string) error {
	filename := fmt.Sprintf("%s.conf", serviceName)
	filepath := filepath.Join(m.config.Global.NginxConfigDir, filename)
	
	if err := os.Remove(filepath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除HTTP配置文件失败: %w", err)
	}
	
	// 从内存中移除配置
	delete(m.httpConfigs, serviceName)
	return nil
}

// deleteStreamConfig 删除Stream配置文件
func (m *Manager) deleteStreamConfig(serviceName string) error {
	filename := fmt.Sprintf("%s.conf", serviceName)
	filepath := filepath.Join(m.config.Global.StreamConfigDir, filename)
	
	if err := os.Remove(filepath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除Stream配置文件失败: %w", err)
	}
	
	// 从内存中移除配置
	delete(m.streamConfigs, serviceName)
	return nil
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(cfg *config.Config) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.config = cfg
}

// Reload 重载nginx配置
func (m *Manager) Reload() error {
	log.Printf("执行nginx重载命令: %s", m.config.Global.NginxReloadCmd)
	
	// 解析命令
	parts := strings.Fields(m.config.Global.NginxReloadCmd)
	if len(parts) == 0 {
		return fmt.Errorf("nginx重载命令为空")
	}

	// 执行命令
	cmd := exec.Command(parts[0], parts[1:]...)
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		return fmt.Errorf("执行nginx重载命令失败: %w, 输出: %s", err, string(output))
	}

	log.Printf("nginx重载成功: %s", string(output))
	return nil
}
