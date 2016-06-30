//
//Copyright [2016] [SnapRoute Inc]
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
//	 Unless required by applicable law or agreed to in writing, software
//	 distributed under the License is distributed on an "AS IS" BASIS,
//	 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//	 See the License for the specific language governing permissions and
//	 limitations under the License.
//
// _______  __       __________   ___      _______.____    __    ____  __  .___________.  ______  __    __
// |   ____||  |     |   ____\  \ /  /     /       |\   \  /  \  /   / |  | |           | /      ||  |  |  |
// |  |__   |  |     |  |__   \  V  /     |   (----` \   \/    \/   /  |  | `---|  |----`|  ,----'|  |__|  |
// |   __|  |  |     |   __|   >   <       \   \      \            /   |  |     |  |     |  |     |   __   |
// |  |     |  `----.|  |____ /  .  \  .----)   |      \    /\    /    |  |     |  |     |  `----.|  |  |  |
// |__|     |_______||_______/__/ \__\ |_______/        \__/  \__/     |__|     |__|      \______||__|  |__|
//
package rx

import (
	"fmt"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	_ "github.com/google/gopacket/pcap"
	"infra/sysd/sysdCommonDefs"
	"l3/ndp/debug"
	"log/syslog"
	"net"
	"reflect"
	"testing"
	"utils/logging"
)

var testPkt = []byte{
	0x33, 0x33, 0xff, 0xf5, 0x00, 0x00, 0xc2, 0x00, 0x54, 0xf5, 0x00, 0x00, 0x86, 0xdd, 0x6e, 0x00,
	0x00, 0x00, 0x00, 0x18, 0x3a, 0xff, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x01, 0xff, 0xf5, 0x00, 0x00, 0x87, 0x00, 0x67, 0x3c, 0x00, 0x00, 0x00, 0x00, 0xfe, 0x80,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xc0, 0x00, 0x54, 0xff, 0xfe, 0xf5, 0x00, 0x00,
}

func NDPTestNewLogger(name string, tag string, listenToConfig bool) (*logging.Writer, error) {
	var err error
	srLogger := new(logging.Writer)
	srLogger.MyComponentName = name

	srLogger.SysLogger, err = syslog.New(syslog.LOG_INFO|syslog.LOG_DAEMON, tag)
	if err != nil {
		fmt.Println("Failed to initialize syslog - ", err)
		return srLogger, err
	}

	srLogger.GlobalLogging = true
	srLogger.MyLogLevel = sysdCommonDefs.INFO
	return srLogger, err
}

func DeepCheckIPv6Hdr(ipHdr, ipv6Want *layers.IPv6, t *testing.T) {
	if !reflect.DeepEqual(ipHdr.Version, ipv6Want.Version) {
		t.Error("Version mismatch", ipHdr.Version, ipv6Want.Version)
	}
	if !reflect.DeepEqual(ipHdr.TrafficClass, ipv6Want.TrafficClass) {
		t.Error("TrafficClass mismatch")
	}
	if !reflect.DeepEqual(ipHdr.HopLimit, ipv6Want.HopLimit) {
		t.Error("HopLimit mismatch")
	}
	if !reflect.DeepEqual(ipHdr.SrcIP, ipv6Want.SrcIP) {
		t.Error("SrcIP mismatch")
	}
	if !reflect.DeepEqual(ipHdr.DstIP, ipv6Want.DstIP) {
		t.Error("DstIP mismatch")
	}
	if !reflect.DeepEqual(ipHdr.NextHeader, ipv6Want.NextHeader) {
		t.Error("NextHeader mismatch")
	}
	if !reflect.DeepEqual(ipHdr.Length, ipv6Want.Length) {
		t.Error("lenght mismatch")
	}
}

func DeepCheckNDHdr(ndHeader, ndWant *layers.ICMPv6, t *testing.T) {
	if !reflect.DeepEqual(ndHeader.TypeCode, ndWant.TypeCode) {
		t.Error("TypeCode MisMatch")
	}
	if !reflect.DeepEqual(ndHeader.Checksum, ndWant.Checksum) {
		t.Error("Checksum MisMatch")
	}

}

// Test ND Solicitation message Decoder
func TestICMPv6NDSolicitation(t *testing.T) {
	var err error
	logger, err := NDPTestNewLogger("ndpd", "NDPTEST", true)
	if err != nil {
		t.Error("creating logger failed")
	}
	debug.NDPSetLogger(logger)
	p := gopacket.NewPacket(testPkt, layers.LinkTypeEthernet, gopacket.Default)
	if p.ErrorLayer() != nil {
		t.Error("Failed to decode packet:", p.ErrorLayer().Error())
	}
	ndHeader := &layers.ICMPv6{}
	ipHdr := &layers.IPv6{}
	err = GetIpAndNdSolicitationHeader(p, ipHdr, ndHeader)
	if err != nil {
		t.Error("Decoding ND Solicitation message failed", err)
	} else {
		t.Log("Decoding ND Solicitation message success")
	}
	t.Log(ipHdr)
	ipv6Want := &layers.IPv6{
		Version:      6,
		TrafficClass: 0xe0,
		FlowLabel:    0,
		Length:       24,
		NextHeader:   layers.IPProtocolICMPv6,
		HopLimit:     255,
		SrcIP:        net.IP{0x00, 0x00, 0x0, 0x0, 0x00, 0x0, 0x0, 0x0, 0x00, 0x00, 0x0, 0x00, 0x00, 0x00, 0x0, 0x00},
		DstIP:        net.IP{0xff, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0xff, 0xf5, 0x00, 0x00},
	}
	ndWant := &layers.ICMPv6{
		TypeCode: 0x8700,
		Checksum: 0x673c,
	}
	DeepCheckIPv6Hdr(ipHdr, ipv6Want, t)
	DeepCheckNDHdr(ndHeader, ndWant, t)
}
