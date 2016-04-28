package snapclient

import (
	"asicd/asicdConstDefs"
	"asicdServices"
	"encoding/json"
	"fmt"
	//"net"
	nanomsg "github.com/op/go-nanomsg"
	//"utils/commonDefs"
)

type AsicdClient struct {
	VXLANClientBase
	ClientHdl *asicdServices.ASICDServicesClient
}

type portVlanValue struct {
	ifIndex string
	refCnt  int
}

var asicdclnt AsicdClient
var PortVlanDb map[uint16][]*portVlanValue

func ConvertVxlanConfigToVxlanAsicdConfig(c *VxlanConfig) *asicdInt.Vxlan {

	return &asicdInt.Vxlan{
		Vni:      int32(c.VNI),
		VlanId:   int16(c.VlanId),
		McDestIp: c.Group.String(),
		Mtu:      int32(c.MTU),
	}
}

func ConvertVtepToVxlanAsicdConfig(vtep *VtepDbEntry) *asicdInt.Vtep {

	ifindex := int32(0)
	for _, pc := range PortConfigMap {
		if pc.Name == vtep.SrcIfName {
			ifindex = pc.IfIndex
		}

	}

	return &asicdInt.Vtep{
		Vni:        int32(vtep.Vni),
		IfName:     vtep.VtepName,
		SrcIfIndex: ifindex,
		UDP:        int16(vtep.UDP),
		TTL:        int16(vtep.TTL),
		SrcIp:      vtep.SrcIp.String(),
		DstIp:      vtep.DstIp.String(),
		VlanId:     int16(vtep.VlanId),
		SrcMac:     vtep.SrcMac.String(),
	}
}

// ConstructPortConfigMap:
// Let caller know what ports are valid in system
func ConstructPortConfigMap() {
	currMarker := asicdServices.Int(hwconst.MIN_SYS_PORTS)
	if asicdclnt.ClientHdl != nil {
		count := asicdServices.Int(hwconst.MAX_SYS_PORTS)
		for {
			bulkInfo, err := asicdclnt.ClientHdl.GetBulkPortState(currMarker, count)
			if err != nil {
				return
			}

			bulkCfgInfo, err := asicdclnt.ClientHdl.GetBulkPort(currMarker, count)
			if err != nil {
				return
			}

			objCount := int(bulkInfo.Count)
			more := bool(bulkInfo.More)
			currMarker = asicdServices.Int(bulkInfo.EndIdx)
			for i := 0; i < objCount; i++ {
				ifindex := bulkInfo.PortStateList[i].IfIndex
				netMac, _ = net.ParseMAC(bulkCfgInfo.PortList[i].MacAddr)
				config := vxlan.PortConfig{
					PortNum:      bulkInfo.PortStateList[i].PortNum,
					IfIndex:      ifindex,
					Name:         bulkInfo.PortStateList[i].Name,
					HardwareAddr: netMac,
				}
				serverchannels.VxlanPortCreate <- config
			}
			if more == false {
				return
			}
		}
	}
}

// createASICdSubscriber
//
func (intf VXLANSnapClient) createASICdSubscriber() {
	for {
		logger.Info("Read on ASICd subscriber socket...")
		asicdrxBuf, err := intf.asicdSubSocket.Recv(0)
		if err != nil {
			logger.Err(fmt.Sprintln("Recv on ASICd subscriber socket failed with error:", err))
			intf.asicdSubSocketErrCh <- err
			continue
		}
		//server.logger.Info(fmt.Sprintln("ASIC subscriber recv returned:", asicdrxBuf))
		intf.asicdSubSocketCh <- asicdrxBuf
	}
}

func (intf VXLANSnapClient) listenForASICdUpdates(address string) error {
	var err error
	if intf.asicdSubSocket, err = nanomsg.NewSubSocket(); err != nil {
		logger.Err(fmt.Sprintln("Failed to create ASICd subscribe socket, error:", err))
		return err
	}

	if err = intf.asicdSubSocket.Subscribe(""); err != nil {
		logger.Err(fmt.Sprintln("Failed to subscribe to \"\" on ASICd subscribe socket, error:", err))
		return err
	}

	if _, err = intf.asicdSubSocket.Connect(address); err != nil {
		logger.Err(fmt.Sprintln("Failed to connect to ASICd publisher socket, address:", address, "error:", err))
		return err
	}

	logger.Info(fmt.Sprintln("Connected to ASICd publisher at address:", address))
	if err = intf.asicdSubSocket.SetRecvBuffer(1024 * 1024); err != nil {
		logger.Err(fmt.Sprintln("Failed to set the buffer size for ASICd publisher socket, error:", err))
		return err
	}
	return nil
}

func (intf VXLANSnapClient) processAsicdNotification(asicdrxBuf []byte) {
	var rxMsg asicdConstDefs.AsicdNotification
	err := json.Unmarshal(asicdrxBuf, &rxMsg)
	if err != nil {
		logger.Err(fmt.Sprintln("Unable to unmarshal asicdrxBuf:", asicdrxBuf))
		return
	}
	if rxMsg.MsgType == asicdConstDefs.NOTIFY_VLAN_UPDATE {
		//Vlan Create Msg
		logger.Info("Recvd VLAN notification")
		var vlanMsg asicdConstDefs.VlanNotifyMsg
		err = json.Unmarshal(rxMsg.Msg, &vlanMsg)
		if err != nil {
			logger.Err(fmt.Sprintln("Unable to unmashal vlanNotifyMsg:", rxMsg.Msg))
			return
		}
		intf.updateVlanAccessPorts(vlanMsg, rxMsg.MsgType)
	} else if rxMsg.MsgType == asicdConstDefs.NOTIFY_IPV4INTF_CREATE ||
		rxMsg.MsgType == asicdConstDefs.NOTIFY_IPV4INTF_DELETE {
		server.logger.Info("Recvd IPV4INTF notification")
		var v4Msg asicdConstDefs.IPv4IntfNotifyMsg
		err = json.Unmarshal(rxMsg.Msg, &v4Msg)
		if err != nil {
			server.logger.Err(fmt.Sprintln("Unable to unmashal ipv4IntfNotifyMsg:", rxMsg.Msg))
			return
		}
		server.updateIpv4Intf(v4Msg, rxMsg.MsgType)
	}
	/*
		} else if rxMsg.MsgType == asicdConstDefs.NOTIFY_L3INTF_STATE_CHANGE {
			//L3_INTF_STATE_CHANGE
			server.logger.Info("Recvd INTF_STATE_CHANGE notification")
			var l3IntfMsg asicdConstDefs.L3IntfStateNotifyMsg
			err = json.Unmarshal(rxMsg.Msg, &l3IntfMsg)
			if err != nil {
				server.logger.Err(fmt.Sprintln("Unable to unmashal l3IntfStateNotifyMsg:", rxMsg.Msg))
				return
			}
			server.processL3StateChange(l3IntfMsg)
		} else if rxMsg.MsgType == asicdConstDefs.NOTIFY_L2INTF_STATE_CHANGE {
			//L2_INTF_STATE_CHANGE
			server.logger.Info("Recvd INTF_STATE_CHANGE notification")
			var l2IntfMsg asicdConstDefs.L2IntfStateNotifyMsg
			err = json.Unmarshal(rxMsg.Msg, &l2IntfMsg)
			if err != nil {
				server.logger.Err(fmt.Sprintln("Unable to unmashal l2IntfStateNotifyMsg:", rxMsg.Msg))
				return
			}
			//server.processL2StateChange(l2IntfMsg)
		} else if rxMsg.MsgType == asicdConstDefs.NOTIFY_LAG_CREATE ||
			rxMsg.MsgType == asicdConstDefs.NOTIFY_LAG_UPDATE ||
			rxMsg.MsgType == asicdConstDefs.NOTIFY_LAG_DELETE {
			server.logger.Info("Recvd NOTIFY_LAG notification")
			var lagMsg asicdConstDefs.LagNotifyMsg
			err = json.Unmarshal(rxMsg.Msg, &lagMsg)
			if err != nil {
				server.logger.Err(fmt.Sprintln("Unable to unmashal lagNotifyMsg:", rxMsg.Msg))
				return
			}
			server.updateLagInfra(lagMsg, rxMsg.MsgType)
		} else if rxMsg.MsgType == asicdConstDefs.NOTIFY_IPV4NBR_MAC_MOVE {
			//IPv4 Neighbor mac move
			server.logger.Info("Recvd IPv4NBR_MAC_MOVE notification")
			var macMoveMsg asicdConstDefs.IPv4NbrMacMoveNotifyMsg
			err = json.Unmarshal(rxMsg.Msg, &macMoveMsg)
			if err != nil {
				server.logger.Err(fmt.Sprintln("Unable to unmashal macMoveNotifyMsg:", rxMsg.Msg))
				return
			}
			server.processIPv4NbrMacMove(macMoveMsg)
		}
	*/
}

// GetAccessPorts:
// Discovered the ports which have been provisioned with membership to the
// vxlan vlan.   This method will be called after Vxlan has been provisioned
func (intf VXLANSnapClient) GetAccessPortVlan(vlan uint16) {
	curMark := 0
	logger.Info("Calling Asicd for getting Vlan Property")
	count := 100
	for {
		bulkVlanInfo, _ := asicdclnt.asicdClient.ClientHdl.GetBulkVlan(asicdInt.Int(curMark), asicdInt.Int(count))
		if bulkVlanInfo == nil {
			break
		}
		/* Get bulk on vlan state can re-use curMark and count used by get bulk vlan, as there is a 1:1 mapping in terms of cfg/state objs */
		bulkVlanStateInfo, _ := asicdclnt.asicdClient.ClientHdl.GetBulkVlanState(asicdServices.Int(curMark), asicdServices.Int(count))
		if bulkVlanStateInfo == nil {
			break
		}
		objCnt := int(bulkVlanInfo.Count)
		more := bool(bulkVlanInfo.More)
		curMark = int(bulkVlanInfo.EndIdx)
		for i := 0; i < objCnt; i++ {
			ifindex := int(bulkVlanStateInfo.VlanStateList[i].IfIndex)
			config := vxlan.VxlanAccessPortVlan{
				Command:  vxlan.VxlanCommandCreate,
				Vlan:     uint16(asicdConstDefs.GetIfIndexFromIntfIdAndIntfType(ifindex, commonDefs.IfTypeVlan)),
				IntfList: bulkVlanInfo.VlanList[i].IfIndexList,
			}
			// lets send the config back to the server
			serverchannels.VxlanAccessPortVlanUpdate <- config
		}
		if more == false {
			break
		}
	}
	//server.logger.Info(fmt.Sprintln("Vlan Property Map:", server.vlanPropMap))
}

//
func (intf VXLANSnapClient) updateVlanAccessPorts(msg asicdConstDefs.VlanNotifyMsg, msgType uint8) {
	vlanId := int(msg.VlanId)
	ifIdx := int(asicdConstDefs.GetIfIndexFromIntfIdAndIntfType(vlanId, commonDefs.IfTypeVlan))
	portList := msg.UntagPorts
	if msgType == asicdConstDefs.NOTIFY_VLAN_UPDATE { //VLAN UPDATE
		logger.Info(fmt.Sprintln("Received Vlan Update Notification Vlan:", vlanId, "PortList:", portList))
		config := vxlan.VxlanAccessPortVlan{
			Command:  vxlan.VxlanCommandUpdate,
			Vlan:     vlanId,
			IntfList: portList,
		}
		// lets send the config back to the server
		serverchannels.VxlanAccessPortVlanUpdate <- config

	} else if msgType == asicdConstDefs.NOTIFY_VLAN_DELETE { // VLAN DELETE
		logger.Info(fmt.Sprintln("Received Vlan Delete Notification Vlan:", vlanId, "PortList:", portList))
		config := vxlan.VxlanAccessPortVlan{
			Command:  vxlan.VxlanCommandDelete,
			Vlan:     vlanId,
			IntfList: portList,
		}
		// lets send the config back to the server
		serverchannels.VxlanAccessPortVlanUpdate <- config
	}
	//server.logger.Info(fmt.Sprintln("Vlan Property Map:", server.vlanPropMap))
}

func (intf VXLANSnapClient) updateIpv4Intf(msg asicdConstDefs.IPv4IntfNotifyMsg, msgType uint8) {
	ipAddr := net.ParseIP(msg.IpAddr)
	IfIndex := msg.IfIndex

	nextindex := 0
	count := 1024
	var IfName string
	foundIntf := false
	// TODO when api is available should just call GetIntf...
	for {
		bulkIntf := asicdclnt.ClientHdl.GetBulkIntf(nextindex, count)

		for _, intf := range bulkIntf.IntfList {
			if intf.IfIndex == IfIndex {
				IfName = intf.IfName
				foundIntf = true
				break
			}
		}

		if !foundIntf {
			if bulkIntf.more == true {
				nextindex := count
			} else {
				break
			}
		} else {
			break
		}
	}
	logicalIntfState := asicdclnt.ClientHdl.GetLogicalIntfState(IfName)
	mac, _ := net.ParseMAC(logicalIntfState.SrcMac)
	config := vxlan.VxlanIntfInfo{
		Command:  vxlan.VxlanCommandCreate,
		Ip:       ipAddr,
		Mac:      mac,
		IfIndex:  IfIndex,
		IntfName: IfName,
	}

	if msgType == asicdConstDefs.NOTIFY_VLAN_CREATE {
		config.Command = vxlan.VxlanCommandCreate
		serverchannels.Vxlanintfinfo <- config

	} else if msgType == asicdConstDefs.NOTIFY_VLAN_DELETE {
		config.Command = vxlan.VxlanCommandDelete
		serverchannels.Vxlanintfinfo <- config
	}

}

func (intf VXLANSnapClient) CreateVxlan(vxlan *VxlanConfig) {
	// convert a vxland config to hw config
	if asicdclnt.ClientHdl != nil {
		asicdclnt.ClientHdl.CreateVxlan(ConvertVxlanConfigToVxlanAsicdConfig(vxlan))
	}
	/* Add as another interface
	else {

		// run standalone
		if softswitch == nil {
			softswitch = vxlan_linux.NewVxlanLinux(logger)
		}
		softswitch.CreateVxLAN(ConvertVxlanConfigToVxlanLinuxConfig(vxlan))
	}
	*/
}

func (intf VXLANSnapClient) DeleteVxlan(vxlan *VxlanConfig) {
	// convert a vxland config to hw config
	if asicdclnt.ClientHdl != nil {
		asicdclnt.ClientHdl.DeleteVxlan(ConvertVxlanConfigToVxlanAsicdConfig(vxlan))
	}
	/* Add as another interface
	else {
		// run standalone
		if softswitch != nil {
			softswitch.DeleteVxLAN(ConvertVxlanConfigToVxlanLinuxConfig(vxlan))
		}
	}
	*/
}

// CreateVtep:
// Creates a VTEP interface with the ASICD.  Should create an interface within
// the HW as well as within Linux stack.   AsicD also requires that vlan membership is
// provisioned separately from VTEP.  The vlan in question is the VLAN found
// within the VXLAN header.
func (intf VXLANSnapClient) CreateVtep(vtep *VtepDbEntry) {
	// convert a vxland config to hw config
	if asicdclnt.ClientHdl != nil {

		// need to create a vlan membership of the vtep vlan Id
		if _, ok := PortVlanDb[vtep.VlanId]; !ok {
			v := &portVlanValue{
				ifIndex: vtep.SrcIfName,
				refCnt:  1,
			}
			PortVlanDb[vtep.VlanId] = append(PortVlanDb[vtep.VlanId], v)
			pbmp := fmt.Sprintf("%d", vtep.SrcIfName)

			asicdVlan := &asicdServices.Vlan{
				VlanId:   int32(vtep.VlanId),
				IntfList: pbmp,
			}
			asicdclnt.ClientHdl.CreateVlan(asicdVlan)

		} else {
			portExists := -1
			for i, p := range PortVlanDb[vtep.VlanId] {
				if p.ifIndex == vtep.SrcIfName {
					portExists = i
					break
				}
			}
			if portExists == -1 {
				oldpbmp := ""
				for _, p := range PortVlanDb[vtep.VlanId] {
					oldpbmp += fmt.Sprintf("%s", p.ifIndex)
				}
				v := &portVlanValue{
					ifIndex: vtep.SrcIfName,
					refCnt:  1,
				}
				PortVlanDb[vtep.VlanId] = append(PortVlanDb[vtep.VlanId], v)
				newpbmp := ""
				for _, p := range PortVlanDb[vtep.VlanId] {
					newpbmp += fmt.Sprintf("%s", p.ifIndex)
				}

				oldAsicdVlan := &asicdServices.Vlan{
					VlanId:   int32(vtep.VlanId),
					IntfList: oldpbmp,
				}
				newAsicdVlan := &asicdServices.Vlan{
					VlanId:   int32(vtep.VlanId),
					IntfList: newpbmp,
				}
				// note if the thrift attribute id's change then
				// this attr may need to be updated
				attrset := []bool{false, true, false}
				asicdclnt.ClientHdl.UpdateVlan(oldAsicdVlan, newAsicdVlan, attrset)
			} else {
				v := PortVlanDb[vtep.VlanId][portExists]
				v.refCnt++
				PortVlanDb[vtep.VlanId][portExists] = v
			}
		}
		// create the vtep
		asicdclnt.ClientHdl.CreateVxlanVtep(ConvertVtepToVxlanAsicdConfig(vtep))
	}
	/* Add as another interface
	else {
		// run standalone
		if softswitch == nil {
			softswitch = vxlan_linux.NewVxlanLinux(logger)
		}
		softswitch.CreateVtep(ConvertVtepToVxlanLinuxConfig(vtep))
	}
	*/
}

// DeleteVtep:
// Delete a VTEP interface with the ASICD.  Should create an interface within
// the HW as well as within Linux stack. AsicD also requires that vlan membership is
// provisioned separately from VTEP.  The vlan in question is the VLAN found
// within the VXLAN header.
func (intf VXLANSnapClient) DeleteVtep(vtep *VtepDbEntry) {
	// convert a vxland config to hw config
	if asicdclnt.ClientHdl != nil {
		// delete the vtep
		asicdclnt.ClientHdl.DeleteVxlanVtep(ConvertVtepToVxlanAsicdConfig(vtep))

		// update the vlan the vtep was using
		if _, ok := PortVlanDb[vtep.VlanId]; ok {
			portExists := -1
			for i, p := range PortVlanDb[vtep.VlanId] {
				if p.ifIndex == vtep.SrcIfName {
					portExists = i
					break
				}
			}
			if portExists != -1 {
				v := PortVlanDb[vtep.VlanId][portExists]
				v.refCnt--
				PortVlanDb[vtep.VlanId][portExists] = v

				// lets remove this port from the vlan
				if v.refCnt == 0 {
					oldpbmp := ""
					for _, p := range PortVlanDb[vtep.VlanId] {
						oldpbmp += fmt.Sprintf("%s", p.ifIndex)
					}
					// remove from local list
					PortVlanDb[vtep.VlanId] = append(PortVlanDb[vtep.VlanId][:portExists], PortVlanDb[vtep.VlanId][portExists+1:]...)
					newpbmp := ""
					for _, p := range PortVlanDb[vtep.VlanId] {
						newpbmp += fmt.Sprintf("%s", p.ifIndex)
					}

					oldAsicdVlan := &asicdServices.Vlan{
						VlanId:   int32(vtep.VlanId),
						IntfList: oldpbmp,
					}
					newAsicdVlan := &asicdServices.Vlan{
						VlanId:   int32(vtep.VlanId),
						IntfList: newpbmp,
					}
					// note if the thrift attribute id's change then
					// this attr may need to be updated
					attrset := []bool{false, true, false}
					asicdclnt.ClientHdl.UpdateVlan(oldAsicdVlan, newAsicdVlan, attrset)
				}
				// lets remove the vlan
				if len(PortVlanDb[vtep.VlanId]) == 0 {

					asicdVlan := &asicdServices.Vlan{
						VlanId: int32(vtep.VlanId),
					}
					asicdclnt.ClientHdl.DeleteVlan(asicdVlan)
					delete(PortVlanDb, vtep.VlanId)

				}
			}
		}
	}
	/* Add as another interface
	else {
		// run standalone
		if softswitch != nil {
			softswitch.DeleteVtep(ConvertVtepToVxlanLinuxConfig(vtep))
		}
	}
	*/
}

func (intf VXLANSnapClient) GetIntfInfo(IfName string, intfchan <-chan VxlanIntfInfo) {
	// TODO
	nextindex := 0
	count := 1024
	var IfIndex int32
	var ipAddr net.IP
	foundIntf := false
	foundIp := false
	// TODO when api is available should just call GetIntf...
	for {
		bulkIntf := asicdclnt.ClientHdl.GetBulkIntf(nextindex, count)

		for _, intf := range bulkIntf.IntfList {
			if intf.IfName == IfName {
				IfIndex = intf.IfIndex
				foundIntf = true
				break
			}
		}

		if !foundIntf {
			if bulkIntf.more == true {
				nextindex := count
			} else {
				break
			}
		} else {
			break
		}
	}
	// if we found the interface all other objects at least the logical interface should exist
	if foundIntf {

		// get the object that holds the mac
		logicalIntfState := asicdclnt.ClientHdl.GetLogicalIntfState(IfName)
		mac, _ := net.ParseMAC(logicalIntfState.SrcMac)

		// lets get all the ip associated with this object
		nextindex = 0
		for {
			bulkIpV4 := asicdclnt.ClientHdl.GetBulkIPv4IntfState(nextindex, count)

			for _, ipv4 := range bulkIntf.IntfList {
				if ipv4.IfIndex == IfIndex {
					ipAddr = net.ParseIP(ipv4.IpAddr)
					foundIp = true
					break
				}
			}

			if !foundIp {
				if bulkIntf.more == true {
					nextindex := count
				} else {
					break
				}
			} else {
				break
			}
		}
	}

	if foundIp {
		config := vxlan.VxlanIntfInfo{
			Command:  vxlan.VxlanCommandCreate,
			Ip:       ipAddr,
			Mac:      mac,
			IfIndex:  IfIndex,
			IntfName: IfName,
		}
		serverchannels.Vxlanintfinfo <- config
	}
}
