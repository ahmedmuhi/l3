// lahandler
package rpc

import (
	"database/sql"
	"errors"
	"fmt"
	"git.apache.org/thrift.git/lib/go/thrift"
	_ "github.com/mattn/go-sqlite3"
	vxlan "l3/tunnel/vxlan/protocol"
	"utils/logging"
	"vxland"
)

const DBName string = "UsrConfDb.db"

type VXLANDServiceHandler struct {
	server *vxlan.VXLANServer
	logger *logging.Writer
}

// look up the various other daemons based on c string
func GetClientPort(paramsFile string, c string) int {
	var clientsList []ClientJson

	bytes, err := ioutil.ReadFile(paramsFile)
	if err != nil {
		return 0
	}

	err = json.Unmarshal(bytes, &clientsList)
	if err != nil {
		return 0
	}

	for _, client := range clientsList {
		if client.Name == c {
			return client.Port
		}
	}
	return 0
}

func NewVXLANDServiceHandler(server *vxlan.VXLANServer, logger *logging.Writer) *VXLANDServiceHandler {
	//lacp.LacpStartTime = time.Now()
	// link up/down events for now
	//startEvtHandler()
	handler := &VXLANDServiceHandler{
		server: server,
		logger: logger,
	}

	// lets read the current config and re-play the config
	handler.ReadConfigFromDB()

	return handler
}

func (v *VXLANDServiceHandler) StartThriftServer() {

	var transport thrift.TServerTransport
	var err error

	fileName := v.server.Paramspath + "clients.json"
	port := GetClientPort(fileName, "vxland")
	if port != 0 {
		addr := fmt.Sprintf("localhost:%d", port)
		transport, err = thrift.NewTServerSocket(addr)
		if err != nil {
			panic(fmt.Sprintf("Failed to create Socket with:", addr))
		}

		processor := vxland.NewVXLANDServicesProcessor(v)
		transportFactory := thrift.NewTBufferedTransportFactory(8192)
		protocolFactory := thrift.NewTBinaryProtocolFactoryDefault()
		thriftserver := thrift.NewTSimpleServer4(processor, transport, transportFactory, protocolFactory)

		err = thriftserver.Serve()
		panic(err)
	}
	panic(errors.New("Unable to find vxland port"))
}

func (v *VXLANDServiceHandler) CreateVxlanInstance(config *vxland.VxlanInstance) (bool, error) {
	v.logger.Info(fmt.Sprintf("CreateVxlanConfigInstance %#v", config))

	c, err := vxlan.ConvertVxlanInstanceToVxlanConfig(config)
	if err == nil {
		v.server.Configchans.Vxlancreate <- *c
		return true, nil
	}
	return false, err
}

func (v *VXLANDServiceHandler) DeleteVxlanInstance(config *vxland.VxlanInstance) (bool, error) {
	v.logger.Info(fmt.Sprintf("DeleteVxlanConfigInstance %#v", config))
	c, err := vxlan.ConvertVxlanInstanceToVxlanConfig(config)
	if err == nil {
		v.server.Configchans.Vxlandelete <- *c
		return true, nil
	}
	return false, err
}

func (v *VXLANDServiceHandler) UpdateVxlanInstance(origconfig *vxland.VxlanInstance, newconfig *vxland.VxlanInstance, attrset []bool) (bool, error) {
	v.logger.Info(fmt.Sprintf("UpdateVxlanConfigInstance orig[%#v] new[%#v]", origconfig, newconfig))
	oc, _ := vxlan.ConvertVxlanInstanceToVxlanConfig(origconfig)
	nc, err := vxlan.ConvertVxlanInstanceToVxlanConfig(newconfig)
	if err == nil {
		update := vxlan.VxlanUpdate{
			Oldconfig: *oc,
			Newconfig: *nc,
			Attr:      attrset,
		}
		v.server.Configchans.Vxlanupdate <- update
		return true, nil
	}
	return false, err
}

func (v *VXLANDServiceHandler) CreateVxlanVtepInstance(config *vxland.VxlanVtepInstance) (bool, error) {
	v.logger.Info(fmt.Sprintf("CreateVxlanVtepInstance %#v", config))
	c, err := vxlan.ConvertVxlanVtepInstanceToVtepConfig(config)
	if err == nil {
		v.server.Configchans.Vtepcreate <- *c
		return true, err
	}
	return false, err
}

func (v *VXLANDServiceHandler) DeleteVxlanVtepInstance(config *vxland.VxlanVtepInstance) (bool, error) {
	v.logger.Info(fmt.Sprintf("DeleteVxlanVtepInstance %#v", config))
	c, err := vxlan.ConvertVxlanVtepInstanceToVtepConfig(config)
	if err == nil {
		v.server.Configchans.Vtepdelete <- *c
		return true, nil
	}
	return false, err
}

func (v *VXLANDServiceHandler) UpdateVxlanVtepInstance(origconfig *vxland.VxlanVtepInstance, newconfig *vxland.VxlanVtepInstance, attrset []bool) (bool, error) {
	v.logger.Info(fmt.Sprintf("UpdateVxlanVtepInstance orig[%#v] new[%#v]", origconfig, newconfig))
	oc, _ := vxlan.ConvertVxlanVtepInstanceToVtepConfig(origconfig)
	nc, err := vxlan.ConvertVxlanVtepInstanceToVtepConfig(newconfig)
	if err == nil {
		update := vxlan.VtepUpdate{
			Oldconfig: *oc,
			Newconfig: *nc,
			Attr:      attrset,
		}
		v.server.Configchans.Vtepupdate <- update
		return true, nil
	}

	return false, err
}

func (v *VXLANDServiceHandler) HandleDbReadVxlanInstance(dbHdl *sql.DB) error {
	dbCmd := "select * from VxlanInstance"
	rows, err := dbHdl.Query(dbCmd)
	if err != nil {
		fmt.Println(fmt.Sprintf("DB method Query failed for 'VxlanInstance' with error ", dbCmd, err))
		return err
	}

	defer rows.Close()

	for rows.Next() {

		object := new(vxland.VxlanInstance)
		if err = rows.Scan(&object.Vni, &object.VlanId); err != nil {
			fmt.Println("Db method Scan failed when interating over VxlanInstance")
		}
		_, err = v.CreateVxlanInstance(object)
		if err != nil {
			return err
		}
	}
	return nil
}

func (v *VXLANDServiceHandler) HandleDbReadVxlanVtepInstance(dbHdl *sql.DB) error {
	dbCmd := "select * from VxlanVtepInstance"
	rows, err := dbHdl.Query(dbCmd)
	if err != nil {
		fmt.Println(fmt.Sprintf("DB method Query failed for 'VxlanVtepInstance' with error ", dbCmd, err))
		return err
	}

	defer rows.Close()

	for rows.Next() {

		object := new(vxland.VxlanVtepInstance)
		if err = rows.Scan(&object.Intf, &object.IntfRef, &object.DstUDP, &object.TTL, &object.TOS, &object.InnerVlanHandlingMode, &object.Vni, &object.DstIp, &object.SrcIp, &object.VlanId, &object.Mtu); err != nil {

			fmt.Println("Db method Scan failed when interating over VxlanVtepInstance")
		}
		_, err = v.CreateVxlanVtepInstance(object)
		if err != nil {
			return err
		}
	}
	return nil
}

func (v *VXLANDServiceHandler) ReadConfigFromDB() error {
	var dbPath string = v.server.Paramspath + DBName

	dbHdl, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		//h.logger.Err(fmt.Sprintf("Failed to open the DB at %s with error %s", dbPath, err))
		//stp.StpLogger("ERROR", fmt.Sprintf("Failed to open the DB at %s with error %s", dbPath, err))
		return err
	}

	defer dbHdl.Close()

	if err := v.HandleDbReadVxlanInstance(dbHdl); err != nil {
		//stp.StpLogger("ERROR", "Error getting All VxlanInstance objects")
		return err
	}

	if err = v.HandleDbReadVxlanVtepInstance(dbHdl); err != nil {
		//stp.StpLogger("ERROR", "Error getting All VxlanVtepInstance objects")
		return err
	}

	return nil
}
