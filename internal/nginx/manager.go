package nginx

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

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
	ServiceName string
	ListenPort  int
	Upstream    []UpstreamServer
}

// UpstreamServer 上游服务器
type UpstreamServer struct {
	IP   string
	Port nat.Port
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
			ServiceName: service.Name,
			ListenPort:  service.ListenPort,
			Upstream:    make([]UpstreamServer, 0),
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
	if len(streamConfig.Upstream) == 0 {
		// 如果没有上游服务器，删除配置文件
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
	var content strings.Builder

	// 构建upstream块
	content.WriteString(fmt.Sprintf("upstream %s {\n", httpConfig.ServiceName))
	for _, server := range httpConfig.Upstream {
		content.WriteString(fmt.Sprintf("    server %s:%s;\n", server.IP, server.Port.Port()))
	}
	content.WriteString("}\n\n")

	// 构建server块
	content.WriteString("server {\n")
	content.WriteString("    listen 80;\n")
	content.WriteString(fmt.Sprintf("    server_name %s;\n", httpConfig.Domain))
	content.WriteString("\n")

	// 构建location块
	content.WriteString(fmt.Sprintf("    location %s {\n", httpConfig.Path))

	// 使用服务配置的代理配置，如果没有则使用全局默认配置
	proxyConfig := httpConfig.ProxyConfig
	if proxyConfig == nil {
		proxyConfig = &m.config.Global.DefaultProxy
	}

	// 添加代理配置
	if proxyConfig.ClientMaxBodySize != "" {
		content.WriteString(fmt.Sprintf("        client_max_body_size %s;\n", proxyConfig.ClientMaxBodySize))
	}

	content.WriteString(fmt.Sprintf("        proxy_pass          http://%s/;\n", httpConfig.ServiceName))

	if proxyConfig.ProxyHTTPVersion != "" {
		content.WriteString(fmt.Sprintf("        proxy_http_version %s;\n", proxyConfig.ProxyHTTPVersion))
	}

	// 添加代理头
	for _, header := range proxyConfig.ProxyHeaders {
		content.WriteString(fmt.Sprintf("        proxy_set_header %s;\n", header))
	}

	if proxyConfig.ProxyRedirect != "" {
		content.WriteString(fmt.Sprintf("        proxy_redirect %s;\n", proxyConfig.ProxyRedirect))
	}

	content.WriteString("    }\n")
	content.WriteString("}\n")

	return content.String()
}

// buildStreamConfigContent 构建Stream配置内容
func (m *Manager) buildStreamConfigContent(streamConfig *StreamConfig) string {
	var content strings.Builder

	// 构建upstream块
	content.WriteString(fmt.Sprintf("upstream %s {\n", streamConfig.ServiceName))
	for _, server := range streamConfig.Upstream {
		content.WriteString(fmt.Sprintf("    server %s:%s;\n", server.IP, server.Port.Port()))
	}
	content.WriteString("}\n\n")

	// 构建server块
	content.WriteString("server {\n")
	content.WriteString(fmt.Sprintf("    listen %d;\n", streamConfig.ListenPort))
	content.WriteString(fmt.Sprintf("    proxy_pass %s;\n", streamConfig.ServiceName))
	content.WriteString("}\n")

	return content.String()
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
