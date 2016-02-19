// server.go
package rpc

import (
	"bgpd"
	"database/sql"
	"fmt"
	"l3/bgp/config"
	"l3/bgp/server"
	"log/syslog"
	"net"

	_ "github.com/mattn/go-sqlite3"
)

const DBName string = "UsrConfDb.db"

type PeerConfigCommands struct {
	IP      net.IP
	Command int
}

type BGPHandler struct {
	PeerCommandCh chan PeerConfigCommands
	server        *server.BGPServer
	logger        *syslog.Writer
}

func NewBGPHandler(server *server.BGPServer, logger *syslog.Writer, filePath string) *BGPHandler {
	h := new(BGPHandler)
	h.PeerCommandCh = make(chan PeerConfigCommands)
	h.server = server
	h.logger = logger
	h.readConfigFromDB(filePath)
	return h
}

func (h *BGPHandler) handleGlobalConfig(dbHdl *sql.DB) error {
	dbCmd := "select * from BGPGlobalConfig"
	rows, err := dbHdl.Query(dbCmd)
	if err != nil {
		h.logger.Err(fmt.Sprintf("DB method Query failed for %s with error %s", dbCmd, err))
		return err
	}

	defer rows.Close()

	var gConf config.GlobalConfig
	var routerIP string
	for rows.Next() {
		if err = rows.Scan(&gConf.AS, &routerIP, &gConf.UseMultiplePaths, &gConf.EBGPMaxPaths,
			&gConf.EBGPAllowMultipleAS, &gConf.IBGPMaxPaths); err != nil {
			h.logger.Err(fmt.Sprintf("DB method Scan failed when iterating over BGPGlobalConfig rows with error %s", err))
			return err
		}

		gConf.RouterId = h.convertStrIPToNetIP(routerIP)
		if gConf.RouterId == nil {
			h.logger.Err(fmt.Sprintln("handleGlobalConfig - IP is not valid:", routerIP))
			return config.IPError{routerIP}
		}

		h.server.GlobalConfigCh <- gConf
	}

	return nil
}

func (h *BGPHandler) handlePeerGroup(dbHdl *sql.DB) error {
	dbCmd := "select * from BGPPeerGroup"
	rows, err := dbHdl.Query(dbCmd)
	if err != nil {
		h.logger.Err(fmt.Sprintf("DB method Query failed for '%s' with error %s", dbCmd, err))
		return err
	}

	defer rows.Close()

	var group config.PeerGroupConfig
	for rows.Next() {
		if err = rows.Scan(&group.PeerAS, &group.LocalAS, &group.AuthPassword, &group.Description, &group.Name,
			&group.RouteReflectorClusterId, &group.RouteReflectorClient, &group.MultiHopEnable, &group.MultiHopTTL,
			&group.ConnectRetryTime, &group.HoldTime, &group.KeepaliveTime); err != nil {
			h.logger.Err(fmt.Sprintf("DB method Scan failed when iterating over BGPPeerGroup rows with error %s", err))
			return err
		}

		h.server.AddPeerGroupCh <- server.PeerGroupUpdate{config.PeerGroupConfig{}, group, make([]bool, 0)}
	}

	return nil
}

func (h *BGPHandler) handleNeighborConfig(dbHdl *sql.DB) error {
	dbCmd := "select * from BGPNeighborConfig"
	rows, err := dbHdl.Query(dbCmd)
	if err != nil {
		h.logger.Err(fmt.Sprintf("DB method Query failed for '%s' with error %s", dbCmd, err))
		return err
	}

	defer rows.Close()

	var nConf config.NeighborConfig
	var neighborIP string
	for rows.Next() {
		if err = rows.Scan(&nConf.PeerAS, &nConf.LocalAS, &nConf.AuthPassword, &nConf.Description, &neighborIP,
			&nConf.RouteReflectorClusterId, &nConf.RouteReflectorClient, &nConf.MultiHopEnable, &nConf.MultiHopTTL,
			&nConf.ConnectRetryTime, &nConf.HoldTime, &nConf.KeepaliveTime, &nConf.PeerGroup); err != nil {
			h.logger.Err(fmt.Sprintf("DB method Scan failed when iterating over BGPNeighborConfig rows with error %s", err))
			return err
		}

		nConf.NeighborAddress = net.ParseIP(neighborIP)
		if nConf.NeighborAddress == nil {
			h.logger.Info(fmt.Sprintf("Can't create BGP neighbor - IP[%s] not valid", neighborIP))
			return config.IPError{neighborIP}
		}

		h.server.AddPeerCh <- server.PeerUpdate{config.NeighborConfig{}, nConf, make([]bool, 0)}
	}

	return nil
}

func (h *BGPHandler) handleBGPAggregate(dbHdl *sql.DB) error {
	dbCmd := "select * from BGPAggregate"
	rows, err := dbHdl.Query(dbCmd)
	if err != nil {
		h.logger.Err(fmt.Sprintf("handleBGPAggregate: DB method Query failed for %s with error %s", dbCmd, err))
		return err
	}

	defer rows.Close()

	var agg config.BGPAggregate
	var ipPrefix string
	for rows.Next() {
		if err = rows.Scan(&ipPrefix, &agg.GenerateASSet, &agg.SendSummaryOnly); err != nil {
			h.logger.Err(fmt.Sprintf("handleBGPAggregate: DB method Next() failed on BGPAggregate with error %s", err))
			return err
		}

		_, ipNet, err := net.ParseCIDR(ipPrefix)
		if err != nil {
			h.logger.Info(fmt.Sprintln("SendBGPAggregate: ParseCIDR for IPPrefix", ipPrefix, "failed with err", err))
			return err
		}

		ones, _ := ipNet.Mask.Size()
		ipPrefix := config.IPPrefix{
			Prefix: ipNet.IP,
			Length: uint8(ones),
		}
		agg.IPPrefix = ipPrefix

		h.server.AddAggCh <- server.AggUpdate{config.BGPAggregate{}, agg, make([]bool, 0)}
	}

	return nil
}

func (h *BGPHandler) readConfigFromDB(filePath string) error {
	var dbPath string = filePath + DBName

	dbHdl, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		h.logger.Err(fmt.Sprintf("Failed to open the DB at %s with error %s", dbPath, err))
		return err
	}

	defer dbHdl.Close()

	if err = h.handleGlobalConfig(dbHdl); err != nil {
		return err
	}

	if err = h.handlePeerGroup(dbHdl); err != nil {
		return err
	}

	if err = h.handleNeighborConfig(dbHdl); err != nil {
		return err
	}

	return nil
}

func (h *BGPHandler) convertStrIPToNetIP(ip string) net.IP {
	if ip == "localhost" {
		ip = "127.0.0.1"
	}

	netIP := net.ParseIP(ip)
	return netIP
}

func (h *BGPHandler) SendBGPGlobal(bgpGlobal *bgpd.BGPGlobalConfig) bool {
	ip := h.convertStrIPToNetIP(bgpGlobal.RouterId)
	if ip == nil {
		h.logger.Info(fmt.Sprintln("SendBGPGlobal: IP", bgpGlobal.RouterId, "is not valid"))
		return false
	}

	gConf := config.GlobalConfig{
		AS:                  uint32(bgpGlobal.ASNum),
		RouterId:            ip,
		UseMultiplePaths:    bgpGlobal.UseMultiplePaths,
		EBGPMaxPaths:        uint32(bgpGlobal.EBGPMaxPaths),
		EBGPAllowMultipleAS: bgpGlobal.EBGPAllowMultipleAS,
		IBGPMaxPaths:        uint32(bgpGlobal.IBGPMaxPaths),
	}
	h.server.GlobalConfigCh <- gConf
	return true
}

func (h *BGPHandler) CreateBGPGlobal(bgpGlobal *bgpd.BGPGlobalConfig) (bool, error) {
	h.logger.Info(fmt.Sprintln("Create global config attrs:", bgpGlobal))
	return h.SendBGPGlobal(bgpGlobal), nil
}

func (h *BGPHandler) GetBGPGlobal() (*bgpd.BGPGlobalState, error) {
	bgpGlobal := h.server.GetBGPGlobalState()
	bgpGlobalResponse := bgpd.NewBGPGlobalState()
	bgpGlobalResponse.AS = int32(bgpGlobal.AS)
	bgpGlobalResponse.RouterId = bgpGlobal.RouterId.String()
	bgpGlobalResponse.UseMultiplePaths = bgpGlobal.UseMultiplePaths
	bgpGlobalResponse.EBGPMaxPaths = int32(bgpGlobal.EBGPMaxPaths)
	bgpGlobalResponse.EBGPAllowMultipleAS = bgpGlobal.EBGPAllowMultipleAS
	bgpGlobalResponse.IBGPMaxPaths = int32(bgpGlobal.IBGPMaxPaths)
	bgpGlobalResponse.TotalPaths = int32(bgpGlobal.TotalPaths)
	bgpGlobalResponse.TotalPrefixes = int32(bgpGlobal.TotalPrefixes)
	return bgpGlobalResponse, nil
}

func (h *BGPHandler) UpdateBGPGlobal(origG *bgpd.BGPGlobalConfig, updatedG *bgpd.BGPGlobalConfig, attrSet []bool) (bool, error) {
	h.logger.Info(fmt.Sprintln("Update global config attrs:", updatedG, "old config:", origG))
	return h.SendBGPGlobal(updatedG), nil
}

func (h *BGPHandler) DeleteBGPGlobal(bgpGlobal *bgpd.BGPGlobalConfig) (bool, error) {
	h.logger.Info(fmt.Sprintln("Delete global config attrs:", bgpGlobal))
	return true, nil
}

func (h *BGPHandler) ValidateBGPNeighbor(bgpNeighbor *bgpd.BGPNeighborConfig) (config.NeighborConfig, bool) {
	if bgpNeighbor == nil {
		return config.NeighborConfig{}, true
	}

	ip := net.ParseIP(bgpNeighbor.NeighborAddress)
	if ip == nil {
		h.logger.Info(fmt.Sprintf("ValidateBGPNeighbor: Address %s is not valid", bgpNeighbor.NeighborAddress))
		return config.NeighborConfig{}, false
	}

	pConf := config.NeighborConfig{
		BaseConfig: config.BaseConfig{
			PeerAS:                  uint32(bgpNeighbor.PeerAS),
			LocalAS:                 uint32(bgpNeighbor.LocalAS),
			AuthPassword:            bgpNeighbor.AuthPassword,
			Description:             bgpNeighbor.Description,
			RouteReflectorClusterId: uint32(bgpNeighbor.RouteReflectorClusterId),
			RouteReflectorClient:    bgpNeighbor.RouteReflectorClient,
			MultiHopEnable:          bgpNeighbor.MultiHopEnable,
			MultiHopTTL:             uint8(bgpNeighbor.MultiHopTTL),
			ConnectRetryTime:        uint32(bgpNeighbor.ConnectRetryTime),
			HoldTime:                uint32(bgpNeighbor.HoldTime),
			KeepaliveTime:           uint32(bgpNeighbor.KeepaliveTime),
		},
		NeighborAddress: ip,
		PeerGroup:       bgpNeighbor.PeerGroup,
	}
	return pConf, true
}

func (h *BGPHandler) SendBGPNeighbor(oldNeighbor *bgpd.BGPNeighborConfig, newNeighbor *bgpd.BGPNeighborConfig, attrSet []bool) bool {
	oldNeighConf, err := h.ValidateBGPNeighbor(oldNeighbor)
	if !err {
		return false
	}

	newNeighConf, err := h.ValidateBGPNeighbor(newNeighbor)
	if !err {
		return false
	}

	h.server.AddPeerCh <- server.PeerUpdate{oldNeighConf, newNeighConf, attrSet}
	return true
}

func (h *BGPHandler) CreateBGPNeighbor(bgpNeighbor *bgpd.BGPNeighborConfig) (bool, error) {
	h.logger.Info(fmt.Sprintln("Create BGP neighbor attrs:", bgpNeighbor))
	return h.SendBGPNeighbor(nil, bgpNeighbor, make([]bool, 0)), nil
}

func (h *BGPHandler) convertToThriftNeighbor(neighborState *config.NeighborState) *bgpd.BGPNeighborState {
	bgpNeighborResponse := bgpd.NewBGPNeighborState()
	bgpNeighborResponse.PeerAS = int32(neighborState.PeerAS)
	bgpNeighborResponse.LocalAS = int32(neighborState.LocalAS)
	bgpNeighborResponse.AuthPassword = neighborState.AuthPassword
	bgpNeighborResponse.PeerType = bgpd.PeerType(neighborState.PeerType)
	bgpNeighborResponse.Description = neighborState.Description
	bgpNeighborResponse.NeighborAddress = neighborState.NeighborAddress.String()
	bgpNeighborResponse.SessionState = int32(neighborState.SessionState)
	bgpNeighborResponse.RouteReflectorClusterId = int32(neighborState.RouteReflectorClusterId)
	bgpNeighborResponse.RouteReflectorClient = neighborState.RouteReflectorClient
	bgpNeighborResponse.MultiHopEnable = neighborState.MultiHopEnable
	bgpNeighborResponse.MultiHopTTL = int8(neighborState.MultiHopTTL)
	bgpNeighborResponse.ConnectRetryTime = int32(neighborState.ConnectRetryTime)
	bgpNeighborResponse.HoldTime = int32(neighborState.HoldTime)
	bgpNeighborResponse.KeepaliveTime = int32(neighborState.KeepaliveTime)

	received := bgpd.NewBgpCounters()
	received.Notification = int64(neighborState.Messages.Received.Notification)
	received.Update = int64(neighborState.Messages.Received.Update)
	sent := bgpd.NewBgpCounters()
	sent.Notification = int64(neighborState.Messages.Sent.Notification)
	sent.Update = int64(neighborState.Messages.Sent.Update)
	messages := bgpd.NewBGPMessages()
	messages.Received = received
	messages.Sent = sent
	bgpNeighborResponse.Messages = messages

	queues := bgpd.NewBGPQueues()
	queues.Input = int32(neighborState.Queues.Input)
	queues.Output = int32(neighborState.Queues.Output)
	bgpNeighborResponse.Queues = queues

	return bgpNeighborResponse
}

func (h *BGPHandler) GetBGPNeighbor(neighborAddress string) (*bgpd.BGPNeighborState, error) {
	bgpNeighborState := h.server.GetBGPNeighborState(neighborAddress)
	bgpNeighborResponse := h.convertToThriftNeighbor(bgpNeighborState)
	return bgpNeighborResponse, nil
}

func (h *BGPHandler) BulkGetBGPNeighbors(index int64, count int64) (*bgpd.BGPNeighborStateBulk, error) {
	nextIdx, currCount, bgpNeighbors := h.server.BulkGetBGPNeighbors(int(index), int(count))
	bgpNeighborsResponse := make([]*bgpd.BGPNeighborState, len(bgpNeighbors))
	for idx, item := range bgpNeighbors {
		bgpNeighborsResponse[idx] = h.convertToThriftNeighbor(item)
	}

	bgpNeighborStateBulk := bgpd.NewBGPNeighborStateBulk()
	bgpNeighborStateBulk.NextIndex = int64(nextIdx)
	bgpNeighborStateBulk.Count = int64(currCount)
	bgpNeighborStateBulk.More = (nextIdx != 0)
	bgpNeighborStateBulk.StateList = bgpNeighborsResponse

	return bgpNeighborStateBulk, nil
}

func (h *BGPHandler) UpdateBGPNeighbor(origN *bgpd.BGPNeighborConfig, updatedN *bgpd.BGPNeighborConfig, attrSet []bool) (bool, error) {
	h.logger.Info(fmt.Sprintln("Update peer attrs:", updatedN))
	return h.SendBGPNeighbor(origN, updatedN, attrSet), nil
}

func (h *BGPHandler) DeleteBGPNeighbor(neighborAddress string) (bool, error) {
	h.logger.Info(fmt.Sprintln("Delete BGP neighbor:", neighborAddress))
	ip := net.ParseIP(neighborAddress)
	if ip == nil {
		h.logger.Info(fmt.Sprintf("Can't delete BGP neighbor - IP[%s] not valid", neighborAddress))
		return false, nil
	}
	h.server.RemPeerCh <- neighborAddress
	return true, nil
}

func (h *BGPHandler) PeerCommand(in *PeerConfigCommands, out *bool) error {
	h.PeerCommandCh <- *in
	h.logger.Info(fmt.Sprintln("Good peer command:", in))
	*out = true
	return nil
}

func (h *BGPHandler) ValidateBGPPeerGroup(peerGroup *bgpd.BGPPeerGroup) (config.PeerGroupConfig, bool) {
	if peerGroup == nil {
		return config.PeerGroupConfig{}, true
	}

	group := config.PeerGroupConfig{
		BaseConfig: config.BaseConfig{
			PeerAS:                  uint32(peerGroup.PeerAS),
			LocalAS:                 uint32(peerGroup.LocalAS),
			AuthPassword:            peerGroup.AuthPassword,
			Description:             peerGroup.Description,
			RouteReflectorClusterId: uint32(peerGroup.RouteReflectorClusterId),
			RouteReflectorClient:    peerGroup.RouteReflectorClient,
			MultiHopEnable:          peerGroup.MultiHopEnable,
			MultiHopTTL:             uint8(peerGroup.MultiHopTTL),
			ConnectRetryTime:        uint32(peerGroup.ConnectRetryTime),
			HoldTime:                uint32(peerGroup.HoldTime),
			KeepaliveTime:           uint32(peerGroup.KeepaliveTime),
		},
		Name: peerGroup.Name,
	}
	return group, true
}

func (h *BGPHandler) SendBGPPeerGroup(oldGroup *bgpd.BGPPeerGroup, newGroup *bgpd.BGPPeerGroup, attrSet []bool) bool {
	oldGroupConf, err := h.ValidateBGPPeerGroup(oldGroup)
	if !err {
		return false
	}

	newGroupConf, err := h.ValidateBGPPeerGroup(newGroup)
	if !err {
		return false
	}

	h.server.AddPeerGroupCh <- server.PeerGroupUpdate{oldGroupConf, newGroupConf, attrSet}
	return true
}

func (h *BGPHandler) CreateBGPPeerGroup(peerGroup *bgpd.BGPPeerGroup) (bool, error) {
	h.logger.Info(fmt.Sprintln("Create BGP neighbor attrs:", peerGroup))
	return h.SendBGPPeerGroup(nil, peerGroup, make([]bool, 0)), nil
}

func (h *BGPHandler) UpdateBGPPeerGroup(origG *bgpd.BGPPeerGroup, updatedG *bgpd.BGPPeerGroup, attrSet []bool) (bool, error) {
	h.logger.Info(fmt.Sprintln("Update peer attrs:", updatedG))
	return h.SendBGPPeerGroup(origG, updatedG, attrSet), nil
}

func (h *BGPHandler) DeleteBGPPeerGroup(name string) (bool, error) {
	h.logger.Info(fmt.Sprintln("Delete BGP peer group:", name))
	h.server.RemPeerGroupCh <- name
	return true, nil
}

func (h *BGPHandler) GetBGPRoute(prefix string) ([]*bgpd.BGPRoute, error) {
	bgpRoutes := h.server.AdjRib.GetBGPRoutes(prefix)
	return bgpRoutes, nil
}

func (h *BGPHandler) BulkGetBGPRoutes(index int64, count int64) (*bgpd.BGPRouteBulk, error) {
	nextIdx, currCount, bgpRoutes := h.server.AdjRib.BulkGetBGPRoutes(int(index), int(count))

	bgpRoutesBulk := bgpd.NewBGPRouteBulk()
	bgpRoutesBulk.NextIndex = int64(nextIdx)
	bgpRoutesBulk.Count = int64(currCount)
	bgpRoutesBulk.More = (nextIdx != 0)
	bgpRoutesBulk.RouteList = bgpRoutes

	return bgpRoutesBulk, nil
}

func (h *BGPHandler) validateBGPAgg(bgpAgg *bgpd.BGPAggregate) (config.BGPAggregate, error) {
	if bgpAgg == nil {
		return config.BGPAggregate{}, nil
	}

	_, ipNet, err := net.ParseCIDR(bgpAgg.IPPrefix)
	if err != nil {
		h.logger.Info(fmt.Sprintln("SendBGPAggregate: ParseCIDR for IPPrefix", bgpAgg.IPPrefix, "failed with err", err))
		return config.BGPAggregate{}, err
	}

	ones, _ := ipNet.Mask.Size()
	ipPrefix := config.IPPrefix{
		Prefix: ipNet.IP,
		Length: uint8(ones),
	}

	agg := config.BGPAggregate{
		IPPrefix:        ipPrefix,
		GenerateASSet:   bgpAgg.GenerateASSet,
		SendSummaryOnly: bgpAgg.SendSummaryOnly,
	}
	return agg, nil
}

func (h *BGPHandler) SendBGPAggregate(oldAgg *bgpd.BGPAggregate, newAgg *bgpd.BGPAggregate, attrSet []bool) bool {
	oldBGPAgg, err := h.validateBGPAgg(oldAgg)
	if err != nil {
		return false
	}

	newBGPAgg, err := h.validateBGPAgg(newAgg)
	if err != nil {
		return false
	}

	h.server.AddAggCh <- server.AggUpdate{oldBGPAgg, newBGPAgg, attrSet}
	return true
}

func (h *BGPHandler) CreateBGPAggregate(bgpAgg *bgpd.BGPAggregate) (bool, error) {
	h.logger.Info(fmt.Sprintln("Create BGP aggregate attrs:", bgpAgg))
	return h.SendBGPAggregate(nil, bgpAgg, make([]bool, 0)), nil
}

func (h *BGPHandler) UpdateBGPAggregate(oldAgg *bgpd.BGPAggregate, newAgg *bgpd.BGPAggregate, attrSet []bool) (bool, error) {
	h.logger.Info(fmt.Sprintln("Update BGP aggregate attrs:", newAgg))
	return h.SendBGPAggregate(oldAgg, newAgg, attrSet), nil
}

func (h *BGPHandler) DeleteBGPAggregate(name string) (bool, error) {
	h.logger.Info(fmt.Sprintln("Delete BGP aggregate:", name))
	h.server.RemAggCh <- name
	return true, nil
}
