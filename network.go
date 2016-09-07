package main

import (
	"fmt"
	"net"
	"net/url"
	"sync"

	"golang.org/x/net/context"

	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/filters"
	skvs "github.com/experimental-platform/platform-skvs/client"
	"github.com/vishvananda/netlink"
)

type hostToProxyMap struct {
	actualMap map[string]*SwitchingProxy
	mutex     sync.RWMutex
}

func getAppMacvlanMap() []string {
	result := make([]string, 0)
	if _, err := skvs.Get("gitlab/enabled"); err == nil {
		result = append(result, "gitlab")
	}

	return result
}

func (hpm *hostToProxyMap) reload() (int, error) {
	newMap := make(map[string]*SwitchingProxy)
	boxName, err := skvs.Get("ptw/node_name")
	if err != nil {
		return 0, err
	}

	fmt.Println("new Host=>IP mapping:")
	for _, appName := range getAppMacvlanMap() {
		ifName := appIfName(appName)
		appIP, err := getAppIP(appName)
		if err != nil {
			return 0, err
		}

		url, err := url.Parse(fmt.Sprintf("http://%s:80/", appIP))
		if err != nil {
			return 0, err
		}
		appProxy := newSwitchingProxy(url)

		ptwAddr := fmt.Sprintf("%s.%s.protonet.info", appName, boxName)
		newMap[ptwAddr] = appProxy
		fmt.Printf("  %s => %s\n", ptwAddr, appIP)

		appInterface, err := net.InterfaceByName(ifName)
		if err != nil {
			if err = createAppInterface(appName); err != nil {
				return 0, err
			}

			if appInterface, err = net.InterfaceByName(ifName); err != nil {
				return 0, err
			}
		}

		extAppIP, err := getExtInterfaceIP(appInterface.Name)
		if err != nil {
			return 0, err
		}

		newMap[extAppIP] = appProxy
		fmt.Printf("  %s => %s\n", extAppIP, appIP)
	}

	hpm.mutex.Lock()
	hpm.actualMap = newMap
	defer hpm.mutex.Unlock()

	return len(newMap), nil
}

func (hpm *hostToProxyMap) matchHost(host string) *SwitchingProxy {
	hpm.mutex.RLock()
	defer hpm.mutex.RUnlock()

	if proxy, ok := hpm.actualMap[host]; ok {
		return proxy
	}

	return nil
}

func getRealDeviceIPs() ([]string, error) {
	var addresses []string
	list, err := netlink.LinkList()
	if err != nil {
		return nil, err
	}

	for _, link := range list {
		attrs := link.Attrs()
		if attrs.MasterIndex == 0 {
			addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
			if err != nil {
				return nil, err
			}

			for _, addr := range addrs {
				addresses = append(addresses, addr.IP.String())
			}
		}
	}

	return addresses, nil
}

func getAppIP(appName string) (string, error) {
	cli, err := client.NewEnvClient()
	if err != nil {
		return "", err
	}

	listOptions := types.ContainerListOptions{Filter: filters.NewArgs()}
	listOptions.Filter.Add("name", appName)

	containers, err := cli.ContainerList(context.Background(), listOptions)
	if err != nil {
		return "", err
	}
	if len(containers) == 0 {
		return "", fmt.Errorf("Found no container named '%s'", appName)
	}

	data, err := cli.ContainerInspect(context.Background(), containers[0].ID)
	if err != nil {
		return "", err
	}

	protonetNetworkData, ok := data.NetworkSettings.Networks["protonet"]
	if !ok {
		return "", fmt.Errorf("The '%s' container doesn't belong to the network 'protonet'.", appName)
	}

	return protonetNetworkData.IPAddress, nil
}

func getExtInterfaceIP(interfaceName string) (string, error) {
	list, err := netlink.LinkList()
	if err != nil {
		return "", err
	}

	for _, link := range list {
		attrs := link.Attrs()

		if attrs.Name == interfaceName {
			addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
			if err != nil {
				return "", err
			}

			if len(addrs) != 1 {
				return "", fmt.Errorf("Interface '%s' has %d IPv4 addresses: %+v", interfaceName, len(addrs), addrs)
			}

			return addrs[0].IP.String(), nil
		}
	}

	return "", fmt.Errorf("Interface '%s' not found", interfaceName)
}
