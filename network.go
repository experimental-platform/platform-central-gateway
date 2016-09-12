package main

import (
	"fmt"
	"net"
	"net/url"
	"sync"
	"time"

	"golang.org/x/net/context"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/filters"
	skvs "github.com/experimental-platform/platform-skvs/client"
	"github.com/vishvananda/netlink"
)

type hostToProxyMap struct {
	actualMap      map[string]*SwitchingProxy
	mutex          sync.RWMutex
	watcherStopper chan struct{}
	watcherWG      sync.WaitGroup
}

func (hpm *hostToProxyMap) monitorAppExternalIP(appName string, interval time.Duration, stop chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	defer log.Infof("Stopped IP change monitor for app '%s'\n", appName)
	lastKnownIP, err := getAppExternalIP(appName)
	if err != nil {
		log.Errorf("Failed to get external IP of app '%s': %s\ntherefore pp '%s' will not be monitored for IP changes", appName, err.Error(), appName)
		return
	}

	for {
		select {
		case _ = <-stop:
			return
		default:
		}

		currentIP, err := getAppExternalIP(appName)
		if err != nil {
			log.Errorf("Failed to get external IP of app '%s': %s\ntherefore pp '%s' will not be monitored for IP changes", appName, err.Error(), appName)
			return
		}

		if currentIP != lastKnownIP {
			log.Infof("IP of app '%s' changed '%s'->'%s'. Reloading gateway config.", appName, lastKnownIP, currentIP)
			go hpm.reload()
			return
		}

		time.Sleep(interval)
	}
}

func (hpm *hostToProxyMap) stopAppExternalIPMonitoring() {
	hpm.mutex.Lock()
	defer hpm.mutex.Unlock()
	if hpm.watcherStopper == nil {
		return
	}

	close(hpm.watcherStopper)
	hpm.watcherStopper = nil
	hpm.watcherWG.Wait()
	log.Infof("All IP change monitors stopped.\n")
}

func (hpm *hostToProxyMap) startAppExternalIPMonitoring(apps []string) {
	hpm.mutex.Lock()
	defer hpm.mutex.Unlock()

	if hpm.watcherStopper != nil {
		log.Errorln("hostToProxyMap.startAppExternalIPMonitoring(): Trying to start app IP monitoring before stopping old monitoring!")
		return
	}

	hpm.watcherStopper = make(chan struct{})

	for _, appName := range apps {
		hpm.watcherWG.Add(1)
		go hpm.monitorAppExternalIP(appName, 1000*time.Millisecond, hpm.watcherStopper, &hpm.watcherWG)
		log.Infof("Added IP change monitor for app '%s'\n", appName)
	}
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
	apps := getAppMacvlanMap()
	for _, appName := range apps {
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
			log.Warningf("hostToProxyMap.reload(): interface '%s' doesn't exist - creating\n", ifName)
			if err = createAppInterface(appName); err != nil {
				log.Warningf("hostToProxyMap.reload(): failed to create interface '%s': %s\n", ifName, err.Error())
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

	hpm.stopAppExternalIPMonitoring()

	hpm.mutex.Lock()
	hpm.actualMap = newMap
	hpm.mutex.Unlock()

	hpm.startAppExternalIPMonitoring(apps)

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
