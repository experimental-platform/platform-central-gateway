package main

import (
	"fmt"
	"math/rand"
	"net"

	log "github.com/Sirupsen/logrus"
	skvs "github.com/experimental-platform/platform-skvs/client"
	"github.com/experimental-platform/platform-utils/netutil"
	"github.com/milosgajdos83/tenus"
)

func generateMac() string {
	r := make([]byte, 3)
	_, err := rand.Read(r)
	if err != nil {
		panic(fmt.Sprintf("createMac(): Failed to generate random MAC bytes: %s", err.Error()))
	}

	return fmt.Sprintf("00:11:22:%x:%x:%x", r[0:1], r[1:2], r[2:3])
}

func getAppMac(appName string) string {
	var mac string
	var err error

	macSKVSPath := fmt.Sprintf("apps/%s/mac", appName)

	mac, err = skvs.Get(macSKVSPath)
	if err != nil {
		mac = generateMac()
		if err = skvs.Set(macSKVSPath, mac); err != nil {
			log.Errorf("Failed to persist MAC address for app '%s' in SKVS: %s", appName, err.Error())
		}
	}

	return mac
}

func appIfName(appName string) string {
	return fmt.Sprintf("app_%s0", appName)
}

func createAppInterface(appName string) error {
	ifName := appIfName(appName)
	_, err := net.InterfaceByName(ifName)
	if err == nil {
		log.Infof("Interface '%s' already exists.\n", ifName)
		return nil
	}

	mac := getAppMac(appName)

	defaultInterface, err := netutil.GetDefaultInterface()
	if err != nil {
		return err
	}

	link, err := tenus.NewMacVlanLinkWithOptions(defaultInterface, tenus.MacVlanOptions{Dev: ifName, MacAddr: mac})
	if err != nil {
		return err
	}

	err = link.SetLinkUp()
	if err != nil {
		return err
	}

	return nil
}

func getInterfaceIP(ifName string) (string, error) {
	interf, err := net.InterfaceByName(ifName)
	if err != nil {
		return "", err
	}

	addrs, err := interf.Addrs()
	if err != nil {
		return "", err
	}

	// TODO: what about len(addr) > 1 ?
	if len(addrs) == 0 {
		return "", fmt.Errorf("the device %s has no network addresses", ifName)
	}

	cidr := addrs[0].String()
	ip, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", fmt.Errorf("error parsing CIDR '%s'", cidr)
	}

	return ip.String(), nil
}

func getAppExternalIP(appName string) (string, error) {
	ifName := appIfName(appName)
	return getInterfaceIP(ifName)
}

func deleteAppInterface(appName string) error {
	ifName := appIfName(appName)
	return tenus.DeleteLink(ifName)
}
