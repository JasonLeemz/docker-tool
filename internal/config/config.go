package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 主配置结构
type Config struct {
	Global   GlobalConfig    `yaml:"global"`
	Services []ServiceConfig `yaml:"services"`
	filePath string
	lastMod  time.Time
}

// GlobalConfig 全局配置
type GlobalConfig struct {
	NginxConfigDir     string      `yaml:"nginx_config_dir"`
	StreamConfigDir    string      `yaml:"stream_config_dir"`
	NginxReloadCmd     string      `yaml:"nginx_reload_cmd"`
	HTTPTemplateFile   string      `yaml:"http_template_file,omitempty"`
	StreamTemplateFile string      `yaml:"stream_template_file,omitempty"`
	DefaultProxy       ProxyConfig `yaml:"default_proxy"`
	// 宿主机IP
	HostIP string `yaml:"host_ip"`
	// ssl公钥路径
	SSLCertPath string `yaml:"ssl_certificate,omitempty"`
	// ssl私钥路径
	SSLKeyPath string `yaml:"ssl_certificate_key,omitempty"`
	// 强制走https
	ForceHTTPS bool `yaml:"force_https,omitempty"`
}

// ServiceConfig 服务配置
type ServiceConfig struct {
	Name          string       `yaml:"name"`
	Type          string       `yaml:"type"` // http 或 stream
	ContainerName string       `yaml:"container_name"`
	Domain        string       `yaml:"domain,omitempty"`
	Path          string       `yaml:"path,omitempty"`
	Port          int          `yaml:"port,omitempty"`
	ListenPort    int          `yaml:"listen_port,omitempty"`
	ContainerPort int          `yaml:"container_port,omitempty"`
	UpstreamName  string       `yaml:"upstream_name"`
	ProxyConfig   *ProxyConfig `yaml:"proxy_config,omitempty"`
}

// ProxyConfig 代理配置
type ProxyConfig struct {
	EnableWebSocket   bool     `yaml:"enable_websocket,omitempty"`
	ClientMaxBodySize string   `yaml:"client_max_body_size,omitempty"`
	ProxyHTTPVersion  string   `yaml:"proxy_http_version,omitempty"`
	ProxyHeaders      []string `yaml:"proxy_headers,omitempty"`
	ProxyRedirect     string   `yaml:"proxy_redirect,omitempty"`
}

// Load 加载配置文件
func Load(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 记录文件路径和修改时间
	config.filePath = filename
	if stat, err := os.Stat(filename); err == nil {
		config.lastMod = stat.ModTime()
	}

	// 验证配置
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	return &config, nil
}

// Reload 重新加载配置文件
func (c *Config) Reload() error {
	newConfig, err := Load(c.filePath)
	if err != nil {
		return err
	}

	// 更新配置
	*c = *newConfig
	return nil
}

// HasChanged 检查配置文件是否已修改
func (c *Config) HasChanged() bool {
	stat, err := os.Stat(c.filePath)
	if err != nil {
		return false
	}
	return stat.ModTime().After(c.lastMod)
}

// Validate 验证配置
func (c *Config) Validate() error {
	// 只验证全局配置，服务配置在运行时验证
	if c.Global.NginxConfigDir == "" {
		return fmt.Errorf("nginx_config_dir 不能为空")
	}
	if c.Global.StreamConfigDir == "" {
		return fmt.Errorf("stream_config_dir 不能为空")
	}
	if c.Global.NginxReloadCmd == "" {
		return fmt.Errorf("nginx_reload_cmd 不能为空")
	}

	return nil
}

// ValidateService 验证单个服务配置
func (c *Config) ValidateService(service *ServiceConfig) error {
	if service.Name == "" {
		return fmt.Errorf("服务 name 不能为空")
	}
	if service.Type != "http" && service.Type != "stream" {
		return fmt.Errorf("服务 %s 的 type 必须是 http 或 stream", service.Name)
	}
	if service.ContainerName == "" {
		return fmt.Errorf("服务 %s 的 container_name 不能为空", service.Name)
	}
	if service.UpstreamName == "" {
		return fmt.Errorf("服务 %s 的 upstream_name 不能为空", service.Name)
	}

	if service.Type == "http" {
		if service.Domain == "" {
			return fmt.Errorf("HTTP服务 %s 的 domain 不能为空", service.Name)
		}
		if service.Port == 0 {
			return fmt.Errorf("HTTP服务 %s 的 port 不能为空", service.Name)
		}
	}

	if service.Type == "stream" {
		if service.ListenPort == 0 {
			return fmt.Errorf("Stream服务 %s 的 listen_port 不能为空", service.Name)
		}
		if service.ContainerPort == 0 {
			return fmt.Errorf("Stream服务 %s 的 container_port 不能为空", service.Name)
		}
	}

	return nil
}

// GetServiceByContainerName 根据容器名称获取服务配置
func (c *Config) GetServiceByContainerName(containerName string) *ServiceConfig {
	// 去掉容器名称前的 / 符号
	normalizedName := strings.TrimPrefix(containerName, "/")

	for _, service := range c.Services {
		// 也去掉配置中的容器名称前的 / 符号进行比较
		configName := strings.TrimPrefix(service.ContainerName, "/")
		if configName == normalizedName {
			return &service
		}
	}
	return nil
}
