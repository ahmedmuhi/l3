package rpc

import (
	"fmt"
	"ospfd"
)

func (h *OSPFHandler) GetOspfGlobalState(routerId string) (*ospfd.OspfGlobalState, error) {
	h.logger.Info(fmt.Sprintln("Get global attrs"))
	ospfGlobalResponse := ospfd.NewOspfGlobalState()
	return ospfGlobalResponse, nil
}

func (h *OSPFHandler) GetOspfAreaEntryState(areaId string) (*ospfd.OspfAreaEntryState, error) {
	h.logger.Info(fmt.Sprintln("Get Area attrs"))
	ospfAreaResponse := ospfd.NewOspfAreaEntryState()
	return ospfAreaResponse, nil
}

/*
func (h *OSPFHandler) GetOspfStubAreaEntryState(stubAreaId string, stubTOS int32) (*ospfd.OspfStubAreaState, error) {
    h.logger.Info(fmt.Sprintln("Get Area Stub attrs"))
    ospfStubAreaResponse := ospfd.NewOspfStubAreaState()
    return ospfStubAreaResponse, nil
}
*/

func (h *OSPFHandler) GetOspfLsdbEntryState(lsdbType int32, lsdbLsid string, lsdbAreaId string, lsdbRouterId string) (*ospfd.OspfLsdbEntryState, error) {
	h.logger.Info(fmt.Sprintln("Get Link State Database attrs"))
	ospfLsdbResponse := ospfd.NewOspfLsdbEntryState()
	return ospfLsdbResponse, nil
}

/*
func (h *OSPFHandler) GetOspfAreaRangeEntryState(rangeAreaId string, areaRangeNet string) (*ospfd.OspfAreaRangeState, error) {
    h.logger.Info(fmt.Sprintln("Get Address range attrs"))
    ospfAreaRangeResponse := ospfd.NewOspfAreaRangeState()
    return ospfAreaRangeResponse, nil
}
*/

func (h *OSPFHandler) GetOspfIfEntryState(ifIpAddress string, addressLessIf int32) (*ospfd.OspfIfEntryState, error) {
	h.logger.Info(fmt.Sprintln("Get Interface attrs"))
	ospfIfResponse := ospfd.NewOspfIfEntryState()
	return ospfIfResponse, nil
}

/*
func (h *OSPFHandler) GetOspfIfMetricState(ifMetricIpAddress string, ifMetricAddressLessIf int32, ifMetricTOS int32) (*ospfd.OspfIfMetricState, error) {
    h.logger.Info(fmt.Sprintln("Get Interface Metric attrs"))
    ospfIfMetricResponse := ospfd.NewOspfIfMetricState()
    return ospfIfMetricResponse, nil
}

func (h *OSPFHandler) GetOspfVirtIfState(virtIfAreaId string, virtIfNeighbor string) (*ospfd.OspfVirtIfState, error) {
    h.logger.Info(fmt.Sprintln("Get Virtual Interface attrs"))
    ospfVirtIfResponse := ospfd.NewOspfVirtIfState()
    return ospfVirtIfResponse, nil
}
*/

func (h *OSPFHandler) GetOspfNbrEntryState(nbrIpAddr string, nbrAddressLessIndex int32) (*ospfd.OspfNbrEntryState, error) {
	h.logger.Info(fmt.Sprintln("Get Neighbor attrs"))
	ospfNbrResponse := ospfd.NewOspfNbrEntryState()
	return ospfNbrResponse, nil
}

func (h *OSPFHandler) GetOspfVirtNbrEntryState(virtNbrRtrId string, virtNbrArea string) (*ospfd.OspfVirtNbrEntryState, error) {
	h.logger.Info(fmt.Sprintln("Get Virtual Neighbor attrs"))
	ospfVirtNbrResponse := ospfd.NewOspfVirtNbrEntryState()
	return ospfVirtNbrResponse, nil
}

