package watcher

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"

	"docker-tool/internal/config"
	"docker-tool/internal/nginx"
)

// Watcher 容器监听器
type Watcher struct {
	client   *client.Client
	config   *config.Config
	nginxMgr *nginx.Manager
}

// New 创建新的容器监听器
func New(cfg *config.Config) (*Watcher, error) {
	// 创建Docker客户端
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("创建Docker客户端失败: %w", err)
	}

	// 创建nginx管理器
	nginxMgr := nginx.NewManager(cfg)

	return &Watcher{
		client:   dockerClient,
		config:   cfg,
		nginxMgr: nginxMgr,
	}, nil
}

// Start 启动监听器
func (w *Watcher) Start(ctx context.Context) error {
	log.Println("开始监听Docker容器事件...")

	// 启动事件监听
	go w.listenEvents(ctx)

	// 启动配置文件监听
	go w.watchConfigFile(ctx)

	// 启动时检查所有现有容器
	go w.checkExistingContainers(ctx)

	return nil
}

// Stop 停止监听器
func (w *Watcher) Stop() error {
	if w.client != nil {
		return w.client.Close()
	}
	return nil
}

// listenEvents 监听Docker事件
func (w *Watcher) listenEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Println("停止监听Docker事件")
			return
		default:
			w.startEventStream(ctx)
		}
	}
}

// startEventStream 启动事件流
func (w *Watcher) startEventStream(ctx context.Context) {
	// 设置事件过滤器
	eventFilters := filters.NewArgs()
	eventFilters.Add("type", "container")
	eventFilters.Add("event", "start")
	eventFilters.Add("event", "stop")
	eventFilters.Add("event", "die")
	eventFilters.Add("event", "rename")

	// 创建事件选项
	eventOptions := types.EventsOptions{
		Filters: eventFilters,
	}

	// 启动事件流
	eventStream, errStream := w.client.Events(ctx, eventOptions)

	for {
		select {
		case <-ctx.Done():
			return
		case event := <-eventStream:
			w.handleEvent(event)
		case err := <-errStream:
			log.Printf("Docker事件流错误: %v", err)
			// 等待一段时间后重连
			time.Sleep(5 * time.Second)
			return
		}
	}
}

// handleEvent 处理Docker事件
func (w *Watcher) handleEvent(event events.Message) {
	log.Printf("收到Docker事件: %s %s", event.Action, event.Actor.ID)

	switch event.Action {
	case "start":
		w.handleContainerStart(event.Actor.ID)
	case "stop", "die":
		w.handleContainerStop(event.Actor.ID)
	case "rename":
		w.handleContainerRename(event.Actor.ID)
	}
}

// handleContainerStart 处理容器启动事件
func (w *Watcher) handleContainerStart(containerID string) {
	container, err := w.getContainerInfo(containerID)
	if err != nil {
		log.Printf("警告: 获取容器信息失败 %s: %v", containerID, err)
		return
	}

	// 检查是否匹配配置中的服务
	service := w.config.GetServiceByContainerName(container.Name)
	if service == nil {
		// 降低日志级别，避免日志过多
		log.Printf("信息: 容器 %s 未匹配到任何服务配置", container.Name)
		return
	}

	// 验证服务配置
	if err := w.config.ValidateService(service); err != nil {
		log.Printf("警告: 服务 %s 配置无效，跳过处理: %v", service.Name, err)
		return
	}

	log.Printf("处理: 容器 %s 启动，更新nginx配置", container.Name)
	w.updateNginxConfig(service, container)
}

// handleContainerStop 处理容器停止事件
func (w *Watcher) handleContainerStop(containerID string) {
	container, err := w.getContainerInfo(containerID)
	if err != nil {
		log.Printf("警告: 获取容器信息失败 %s: %v", containerID, err)
		return
	}

	// 检查是否匹配配置中的服务
	service := w.config.GetServiceByContainerName(container.Name)
	if service == nil {
		return
	}

	// 验证服务配置
	if err := w.config.ValidateService(service); err != nil {
		log.Printf("警告: 服务 %s 配置无效，跳过处理: %v", service.Name, err)
		return
	}

	log.Printf("处理: 容器 %s 停止，更新nginx配置", container.Name)
	w.updateNginxConfig(service, nil)
}

// handleContainerRename 处理容器重命名事件
func (w *Watcher) handleContainerRename(containerID string) {
	container, err := w.getContainerInfo(containerID)
	if err != nil {
		log.Printf("警告: 获取容器信息失败 %s: %v", containerID, err)
		return
	}

	// 检查是否匹配配置中的服务
	service := w.config.GetServiceByContainerName(container.Name)
	if service == nil {
		return
	}

	// 验证服务配置
	if err := w.config.ValidateService(service); err != nil {
		log.Printf("警告: 服务 %s 配置无效，跳过处理: %v", service.Name, err)
		return
	}

	log.Printf("处理: 容器 %s 重命名，更新nginx配置", container.Name)
	w.updateNginxConfig(service, container)
}

// checkExistingContainers 检查现有容器
func (w *Watcher) checkExistingContainers(ctx context.Context) {
	time.Sleep(2 * time.Second) // 等待Docker daemon准备就绪

	log.Println("检查现有容器...")

	containers, err := w.client.ContainerList(ctx, types.ContainerListOptions{
		All: true,
	})
	if err != nil {
		log.Printf("警告: 获取容器列表失败: %v", err)
		return
	}

	processedCount := 0

	for _, container := range containers {
		if container.State == "running" {
			// 使用goroutine处理每个容器，避免一个容器出错影响其他容器
			go func(containerID string) {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("警告: 处理容器 %s 时发生panic: %v", containerID, r)
					}
				}()
				w.handleContainerStart(containerID)
			}(container.ID)
			processedCount++
		}
	}

	log.Printf("已处理 %d 个运行中的容器", processedCount)
}

// getContainerInfo 获取容器详细信息
func (w *Watcher) getContainerInfo(containerID string) (*types.ContainerJSON, error) {
	container, err := w.client.ContainerInspect(context.Background(), containerID)
	if err != nil {
		return nil, fmt.Errorf("获取容器详细信息失败: %w", err)
	}
	return &container, nil
}

// updateNginxConfig 更新nginx配置
func (w *Watcher) updateNginxConfig(service *config.ServiceConfig, container *types.ContainerJSON) {
	// 获取容器IP和端口
	var containerIP string
	var containerPort nat.Port

	if container != nil {
		containerIP = w.getContainerIP(container)
		containerPort = w.getContainerPort(container, service)

		// 检查IP和端口是否有效
		if containerIP == "" {
			log.Printf("警告: 服务 %s 无法获取容器IP，跳过配置更新", service.Name)
			return
		}
		if containerPort == "" {
			log.Printf("警告: 服务 %s 无法获取容器端口，跳过配置更新", service.Name)
			return
		}
	}

	// 更新nginx配置
	if err := w.nginxMgr.UpdateService(service, containerIP, containerPort); err != nil {
		log.Printf("警告: 更新nginx配置失败 [服务: %s]: %v", service.Name, err)
		return
	}

	// 重载nginx
	if err := w.nginxMgr.Reload(); err != nil {
		log.Printf("警告: 重载nginx失败 [服务: %s]: %v", service.Name, err)
		return
	}

	log.Printf("成功: 服务 %s 的nginx配置已更新并重载", service.Name)
}

// getContainerIP 获取容器IP地址
func (w *Watcher) getContainerIP(container *types.ContainerJSON) string {
	// 检查是否是host网络模式
	if _, exists := container.NetworkSettings.Networks["host"]; exists {
		// host网络模式，返回宿主机IP
		return w.config.Global.HostIP
	}

	// 优先获取macvlan网络的IP
	for networkName, network := range container.NetworkSettings.Networks {
		if networkName != "bridge" && network.IPAddress != "" {
			return network.IPAddress
		}
	}

	// 对于bridge网络，返回宿主机IP（使用宿主机端口映射）
	if _, exists := container.NetworkSettings.Networks["bridge"]; exists {
		return w.config.Global.HostIP
	}

	return ""
}

// getContainerPort 获取容器端口
func (w *Watcher) getContainerPort(container *types.ContainerJSON, service *config.ServiceConfig) nat.Port {
	var targetPort int

	if service.Type == "http" {
		targetPort = service.Port
	} else if service.Type == "stream" {
		targetPort = service.ContainerPort
	}

	// 检查是否是host网络模式
	if _, exists := container.NetworkSettings.Networks["host"]; exists {
		// host网络模式，直接返回配置的端口
		return nat.Port(fmt.Sprintf("%d/tcp", targetPort))
	}

	// 检查是否是bridge网络模式
	if _, exists := container.NetworkSettings.Networks["bridge"]; exists {
		// bridge网络模式，查找宿主机端口映射
		portStr := fmt.Sprintf("%d/tcp", targetPort)
		port := nat.Port(portStr)

		if portBindings, exists := container.NetworkSettings.Ports[port]; exists && len(portBindings) > 0 {
			// 返回宿主机端口
			hostPort := portBindings[0].HostPort
			return nat.Port(fmt.Sprintf("%s/tcp", hostPort))
		}

		// 如果没有找到TCP端口，尝试UDP
		portStr = fmt.Sprintf("%d/udp", targetPort)
		port = nat.Port(portStr)
		if portBindings, exists := container.NetworkSettings.Ports[port]; exists && len(portBindings) > 0 {
			hostPort := portBindings[0].HostPort
			return nat.Port(fmt.Sprintf("%s/udp", hostPort))
		}
	}

	// 对于其他网络模式（如macvlan），使用容器内部端口
	portStr := fmt.Sprintf("%d/tcp", targetPort)
	port := nat.Port(portStr)

	// 检查端口是否暴露
	if _, exists := container.NetworkSettings.Ports[port]; exists {
		return port
	}

	// 如果没有找到TCP端口，尝试UDP
	portStr = fmt.Sprintf("%d/udp", targetPort)
	port = nat.Port(portStr)
	if _, exists := container.NetworkSettings.Ports[port]; exists {
		return port
	}

	// 返回默认端口
	return nat.Port(fmt.Sprintf("%d/tcp", targetPort))
}

// watchConfigFile 监听配置文件变化
func (w *Watcher) watchConfigFile(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if w.config.HasChanged() {
				log.Println("检测到配置文件变化，重新加载配置...")

				// 重新加载配置
				if err := w.config.Reload(); err != nil {
					log.Printf("警告: 重新加载配置文件失败，继续使用当前配置: %v", err)
					continue
				}

				// 更新nginx管理器配置
				w.nginxMgr.UpdateConfig(w.config)

				log.Println("成功: 配置文件已重新加载，重新扫描所有容器...")

				// 重新扫描所有现有容器
				go w.checkExistingContainers(ctx)
			}
		}
	}
}
