package rpc

import (
	"errors"
	"fmt"
	"l3/ospf/config"
	"ospfd"
	//    "l3/ospf/server"
	//    "utils/logging"
	//    "net"
)

func (h *OSPFHandler) SendOspfGlobal(ospfGlobalConf *ospfd.OspfGlobal) error {
	gConf := config.GlobalConf{
		RouterId:                 config.RouterId(ospfGlobalConf.RouterId),
		ASBdrRtrStatus:           ospfGlobalConf.ASBdrRtrStatus,
		TOSSupport:               ospfGlobalConf.TOSSupport,
		RestartSupport:           config.RestartSupport(ospfGlobalConf.RestartSupport),
		RestartInterval:          ospfGlobalConf.RestartInterval,
	}
	h.server.GlobalConfigCh <- gConf
	retMsg := <-h.server.GlobalConfigRetCh
	return retMsg
}

func (h *OSPFHandler) SendOspfIfConf(ospfIfConf *ospfd.OspfIfEntry) error {
	ifConf := config.InterfaceConf{
		IfIpAddress:           config.IpAddress(ospfIfConf.IfIpAddress),
		AddressLessIf:         config.InterfaceIndexOrZero(ospfIfConf.AddressLessIf),
		IfAreaId:              config.AreaId(ospfIfConf.IfAreaId),
		IfType:                config.IfType(ospfIfConf.IfType),
		IfRtrPriority:         config.DesignatedRouterPriority(ospfIfConf.IfRtrPriority),
		IfTransitDelay:        config.UpToMaxAge(ospfIfConf.IfTransitDelay),
		IfRetransInterval:     config.UpToMaxAge(ospfIfConf.IfRetransInterval),
		IfHelloInterval:       config.HelloRange(ospfIfConf.IfHelloInterval),
		IfRtrDeadInterval:     config.PositiveInteger(ospfIfConf.IfRtrDeadInterval),
		IfPollInterval:        config.PositiveInteger(ospfIfConf.IfPollInterval),
		IfAuthKey:             ospfIfConf.IfAuthKey,
		IfAuthType:            config.AuthType(ospfIfConf.IfAuthType),
	}

	h.server.IntfConfigCh <- ifConf
	retMsg := <-h.server.IntfConfigRetCh
	return retMsg
}

func (h *OSPFHandler) SendOspfAreaConf(ospfAreaConf *ospfd.OspfAreaEntry) error {
	areaConf := config.AreaConf{
		AreaId:                              config.AreaId(ospfAreaConf.AreaId),
		AuthType:                            config.AuthType(ospfAreaConf.AuthType),
		ImportAsExtern:                      config.ImportAsExtern(ospfAreaConf.ImportAsExtern),
		AreaSummary:                         config.AreaSummary(ospfAreaConf.AreaSummary),
		AreaNssaTranslatorRole:              config.NssaTranslatorRole(ospfAreaConf.AreaNssaTranslatorRole),
	}

	h.server.AreaConfigCh <- areaConf
	retMsg := <-h.server.AreaConfigRetCh
	return retMsg
}

func (h *OSPFHandler) CreateOspfGlobal(ospfGlobalConf *ospfd.OspfGlobal) (bool, error) {
	if ospfGlobalConf == nil {
		err := errors.New("Invalid Global Configuration")
		return false, err
	}
	h.logger.Info(fmt.Sprintln("Create global config attrs:", ospfGlobalConf))
	err := h.SendOspfGlobal(ospfGlobalConf)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (h *OSPFHandler) CreateOspfAreaEntry(ospfAreaConf *ospfd.OspfAreaEntry) (bool, error) {
	if ospfAreaConf == nil {
		err := errors.New("Invalid Area Configuration")
		return false, err
	}
	h.logger.Info(fmt.Sprintln("Create Area config attrs:", ospfAreaConf))
	err := h.SendOspfAreaConf(ospfAreaConf)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (h *OSPFHandler) CreateOspfStubAreaEntry(ospfStubAreaConf *ospfd.OspfStubAreaEntry) (bool, error) {
	h.logger.Info(fmt.Sprintln("Create Stub Area config attrs:", ospfStubAreaConf))
	return true, nil
}

func (h *OSPFHandler) CreateOspfIfEntry(ospfIfConf *ospfd.OspfIfEntry) (bool, error) {
	if ospfIfConf == nil {
		err := errors.New("Invalid Interface Configuration")
		return false, err
	}
	h.logger.Info(fmt.Sprintln("Create interface config attrs:", ospfIfConf))
	err := h.SendOspfIfConf(ospfIfConf)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (h *OSPFHandler) CreateOspfIfMetricEntry(ospfIfMetricConf *ospfd.OspfIfMetricEntry) (bool, error) {
	h.logger.Info(fmt.Sprintln("Create interface metric config attrs:", ospfIfMetricConf))
	return true, nil
}

func (h *OSPFHandler) CreateOspfVirtIfEntry(ospfVirtIfConf *ospfd.OspfVirtIfEntry) (bool, error) {
	h.logger.Info(fmt.Sprintln("Create virtual interface config attrs:", ospfVirtIfConf))
	return true, nil
}

