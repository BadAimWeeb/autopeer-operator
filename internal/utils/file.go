/*
Copyright 2025 BadAimWeeb.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"fmt"

	autopeerv1 "github.com/BadAimWeeb/autopeer-operator/api/v1"
)

type FileConfig struct {
	BIRD      string
	WireGuard *string
}

func GenerateConfig(peering *autopeerv1.PeeringSpec, node *autopeerv1.NodeSpec) *FileConfig {
	wireguardConfig := ""
	if node != nil && peering != nil && peering.TunnelType == "wireguard" && peering.WireGuard != nil {
		wireguardConfig += "[Interface]\n"
		wireguardConfig += "PrivateKey = " + *peering.WireGuard.PrivateKey + "\n"
		if peering.LocalIPv6 != nil && len(*peering.LocalIPv6) >= 6 && (*peering.LocalIPv6)[:6] == "fe80::" {
			wireguardConfig += "Address = " + *peering.LocalIPv6 + "/64\n"
		}
		if peering.WireGuard.LocalPort != 0 {
			wireguardConfig += "ListenPort = " + fmt.Sprintf("%d", peering.WireGuard.LocalPort) + "\n"
		}
		wireguardConfig += "Table = off\n"
		if peering.PeerIPv4 != nil && node.IPv4Address != nil {
			wireguardConfig += "PostUp = /usr/sbin/ip addr add dev %i " + *node.IPv4Address + "/32 peer " + *peering.PeerIPv4 + "/32\n"
			wireguardConfig += "PostUp = /usr/sbin/ip route add " + *peering.PeerIPv4 + "/32 dev %i scope link src " + *node.IPv4Address + " table 42\n"
			wireguardConfig += "PreDown = /usr/sbin/ip route del " + *peering.PeerIPv4 + "/32 dev %i table 42 || true\n"
		}
		if peering.PeerIPv6 != nil && peering.LocalIPv6 != nil && (len(*peering.PeerIPv6) < 6 || (*peering.PeerIPv6)[:6] != "fe80::") {
			wireguardConfig += "PostUp = /usr/sbin/ip -6 addr add dev %i " + *peering.LocalIPv6 + "/128 peer " + *peering.PeerIPv6 + "/128\n"
			wireguardConfig += "PostUp = /usr/sbin/ip -6 route add " + *peering.PeerIPv6 + "/128 dev %i scope link src " + *peering.LocalIPv6 + " table 42\n"
			wireguardConfig += "PreDown = /usr/sbin/ip -6 route del " + *peering.PeerIPv6 + "/128 dev %i table 42 || true\n"
		}
		wireguardConfig += "\n[Peer]\n"
		wireguardConfig += "PublicKey = " + peering.WireGuard.PeerPublicKey + "\n"
		if peering.WireGuard.PresharedKey != nil {
			wireguardConfig += "PresharedKey = " + *peering.WireGuard.PresharedKey + "\n"
		}
		if *peering.WireGuard.Endpoint != "" {
			wireguardConfig += "Endpoint = " + *peering.WireGuard.Endpoint + "\n"
		} else {
			wireguardConfig += "PersistentKeepalive = 25\n"
		}
		wireguardConfig += "AllowedIPs = 172.16.0.0/12, 10.0.0.0/8, fd00::/8, fe80::/10\n"
	}

	birdConfig := ""
	if peering != nil {
		birdConfig += "# AS" + fmt.Sprintf("%d", peering.PeerASN) + "\n"
		if peering.MPBGP {
			birdConfig += fmt.Sprintf("protocol bgp as%d from dnpeers {\n", peering.PeerASN)
			birdConfig += fmt.Sprintf("    neighbor %s%%wgbgp%d as %d;\n", *peering.PeerIPv6, peering.PeerASN, peering.PeerASN)
			birdConfig += "    ipv4 {\n"
			if peering.ENH {
				birdConfig += "        extended next hop yes;\n"
			}
			if peering.Cost > 0 {
				birdConfig += fmt.Sprintf("        cost %d;\n", peering.Cost)
			}
			birdConfig += "    };\n"
			birdConfig += "    ipv6 {\n"
			if peering.Cost > 0 {
				birdConfig += fmt.Sprintf("        cost %d;\n", peering.Cost)
			}
			birdConfig += "    };\n"
			birdConfig += "}\n\n"
		} else {
			if peering.PeerIPv4 != nil {
				birdConfig += fmt.Sprintf("protocol bgp as%d_v4 from dnpeersv4 {\n", peering.PeerASN)
				birdConfig += fmt.Sprintf("    neighbor %s as %d;\n", *peering.PeerIPv4, peering.PeerASN)
				if peering.Cost > 0 {
					birdConfig += "    ipv4 {\n"
					birdConfig += fmt.Sprintf("        cost %d;\n", peering.Cost)
					birdConfig += "    };\n"
				}
				birdConfig += "}\n\n"
			}
			if peering.PeerIPv6 != nil {
				birdConfig += fmt.Sprintf("protocol bgp as%d_v6 from dnpeersv6 {\n", peering.PeerASN)
				birdConfig += fmt.Sprintf("    neighbor %s%%wgbgp%d as %d;\n", *peering.PeerIPv6, peering.PeerASN, peering.PeerASN)
				if peering.Cost > 0 {
					birdConfig += "    ipv6 {\n"
					birdConfig += fmt.Sprintf("        cost %d;\n", peering.Cost)
					birdConfig += "    };\n"
				}
				birdConfig += "}\n\n"
			}
		}
	}

	var wgConfigPtr *string
	if wireguardConfig != "" {
		wgConfigPtr = &wireguardConfig
	}

	return &FileConfig{
		BIRD:      birdConfig,
		WireGuard: wgConfigPtr,
	}
}
