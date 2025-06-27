package nacos

import (
	"fmt"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"log"
	"strings"
	"sync"
	"time"
)

type Registry struct {
	ServiceName string
	Port        uint64
	IP          string
	Group       string

	services map[string]struct{}
	mutex    sync.Mutex
	client   naming_client.INamingClient
}

// NewRegistry creates a new Registry instance
func NewRegistry(serviceName string, ip string, port uint64) *Registry {
	client := initNamingClient()

	return &Registry{
		ServiceName: serviceName,
		Port:        port,
		IP:          ip,
		Group:       "DEFAULT_GROUP",
		services:    make(map[string]struct{}),
		client:      client,
	}
}

// AddRemoteService adds a logical gRPC service name to metadata and re-registers
func (r *Registry) AddRemoteService(service string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.services[service]; exists {
		fmt.Println("Service already exists:", service)
		return
	}
	r.services[service] = struct{}{}

	err := r.reRegister()
	if err != nil {
		log.Println("Failed to re-register service:", err)
	}
}

// reRegister removes and re-registers the service with updated metadata
func (r *Registry) reRegister() error {
	// Step 1: Deregister old instance safely
	success, err := r.client.DeregisterInstance(vo.DeregisterInstanceParam{
		Ip:          r.IP,
		Port:        r.Port,
		ServiceName: r.ServiceName,
		GroupName:   r.Group,
		Cluster:     "DEFAULT",
		Ephemeral:   true,
	})
	if err != nil {
		log.Println("Warning: failed to deregister previous instance:", err)
	}
	if !success {
		log.Println("Warning: previous instance not found or already gone.")
	}

	// Step 2: Prepare metadata
	serviceList := r.serviceList()
	metadata := map[string]string{
		"protocol": "grpc",
		"services": serviceList,
	}

	// Step 3: Register new instance
	registered, err := r.client.RegisterInstance(vo.RegisterInstanceParam{
		Ip:          r.IP,
		Port:        r.Port,
		ServiceName: r.ServiceName,
		GroupName:   r.Group,
		ClusterName: "DEFAULT",
		Ephemeral:   true,
		Metadata:    metadata,
	})
	if err != nil {
		return fmt.Errorf("register failed: %w", err)
	}
	if !registered {
		return fmt.Errorf("register failed: returned false")
	}

	log.Println("✅ Re-registered service:", r.ServiceName, "with methods:", serviceList)
	return nil
}

// serviceList returns comma-separated service names
func (r *Registry) serviceList() string {
	var list []string
	for name := range r.services {
		list = append(list, name)
	}
	return strings.Join(list, ",")
}

// Watch prints instance list periodically (for debug/demo)
func (r *Registry) Watch() {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		for range ticker.C {
			instances, err := r.client.SelectAllInstances(vo.SelectAllInstancesParam{
				ServiceName: r.ServiceName,
				GroupName:   r.Group,
			})
			if err != nil {
				fmt.Println("Error getting instances:", err)
				continue
			}
			fmt.Println("Instances:")
			for _, inst := range instances {
				fmt.Printf("- %s:%d (%v)\n", inst.Ip, inst.Port, inst.Metadata)
			}
		}
	}()
}

// initNamingClient initializes the Nacos NamingClient singleton
func initNamingClient() naming_client.INamingClient {
	serverConfigs := []constant.ServerConfig{
		*constant.NewServerConfig("127.0.0.1", 8848),
	}
	clientConfig := *constant.NewClientConfig(
		constant.WithNamespaceId(""),
		constant.WithTimeoutMs(5000),
		constant.WithNotLoadCacheAtStart(true),
		constant.WithLogLevel("debug"),
	)
	client, err := clients.NewNamingClient(vo.NacosClientParam{
		ClientConfig:  &clientConfig,
		ServerConfigs: serverConfigs,
	})
	if err != nil {
		log.Fatalf("Failed to create naming client: %v", err)
	}
	return client
}
