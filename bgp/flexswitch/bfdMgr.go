package FSMgr

import (
	"bfdd"
	"encoding/json"
	"errors"
	"fmt"
	nanomsg "github.com/op/go-nanomsg"
	"l3/bfd/bfddCommonDefs"
	"l3/bgp/api"
	"l3/bgp/config"
	"l3/bgp/rpc"
	"utils/logging"
)

/*  Init bfd manager with bfd client as its core
 */
func NewFSBfdMgr(logger *logging.Writer, fileName string) (*FSBfdMgr, error) {
	var bfddClient *bfdd.BFDDServicesClient = nil
	bfddClientChan := make(chan *bfdd.BFDDServicesClient)

	logger.Info("Connecting to BFDd")
	go rpc.StartBfddClient(logger, fileName, bfddClientChan)
	bfddClient = <-bfddClientChan
	if bfddClient == nil {
		logger.Err("Failed to connect to BFDd\n")
		return nil, errors.New("Failed to connect to BFDd")
	} else {
		logger.Info("Connected to BFDd")
	}
	mgr := &FSBfdMgr{
		plugin:     "ovsdb",
		logger:     logger,
		bfddClient: bfddClient,
	}

	return mgr, nil
}

/*  Do any necessary init. Called from server..
 */
func (mgr *FSBfdMgr) Init() {
	// create bfd sub socket listener
	mgr.bfdSubSocket, _ = mgr.SetupSubSocket(bfddCommonDefs.PUB_SOCKET_ADDR)
	go mgr.listenForBFDNotifications()
}

/*  Listen for any BFD notifications
 */
func (mgr *FSBfdMgr) listenForBFDNotifications() {
	for {
		mgr.logger.Info("Read on BFD subscriber socket...")
		rxBuf, err := mgr.bfdSubSocket.Recv(0)
		if err != nil {
			mgr.logger.Err(fmt.Sprintln("Recv on BFD subscriber socket failed with error:",
				err))
			continue
		}
		mgr.logger.Info(fmt.Sprintln("BFD subscriber recv returned:", rxBuf))
		mgr.handleBfdNotifications(rxBuf)
	}
}

func (mgr *FSBfdMgr) handleBfdNotifications(rxBuf []byte) {
	bfd := bfddCommonDefs.BfddNotifyMsg{}
	err := json.Unmarshal(rxBuf, &bfd)
	if err != nil {
		mgr.logger.Err(fmt.Sprintf("Unmarshal BFD notification failed with err %s", err))
		return
	}

	if bfd.State {
		api.SendBfdNotification(bfd.DestIp, bfd.State,
			config.BFD_STATE_VALID)
	} else {
		api.SendBfdNotification(bfd.DestIp, bfd.State,
			config.BFD_STATE_INVALID)
	}
}

func (mgr *FSBfdMgr) CreateBfdSession(ipAddr string) (bool, error) {
	bfdSession := bfdd.NewBfdSession()
	bfdSession.IpAddr = ipAddr
	bfdSession.Owner = "bgp"
	mgr.logger.Info(fmt.Sprintln("Creating BFD Session: ", bfdSession))
	ret, err := mgr.bfddClient.CreateBfdSession(bfdSession)
	return ret, err
}

func (mgr *FSBfdMgr) DeleteBfdSession(ipAddr string) (bool, error) {
	bfdSession := bfdd.NewBfdSession()
	bfdSession.IpAddr = ipAddr
	bfdSession.Owner = "bgp"
	mgr.logger.Info(fmt.Sprintln("Deleting BFD Session: ", bfdSession))
	ret, err := mgr.bfddClient.CreateBfdSession(bfdSession)
	return ret, err
}

func (mgr *FSBfdMgr) SetupSubSocket(address string) (*nanomsg.SubSocket, error) {
	var err error
	var socket *nanomsg.SubSocket
	if socket, err = nanomsg.NewSubSocket(); err != nil {
		mgr.logger.Err(fmt.Sprintf("Failed to create subscribe socket %s, error:%s", address, err))
		return nil, err
	}

	if err = socket.Subscribe(""); err != nil {
		mgr.logger.Err(fmt.Sprintf("Failed to subscribe to \"\" on subscribe socket %s, error:%s",
			address, err))
		return nil, err
	}

	if _, err = socket.Connect(address); err != nil {
		mgr.logger.Err(fmt.Sprintf("Failed to connect to publisher socket %s, error:%s", address, err))
		return nil, err
	}

	mgr.logger.Info(fmt.Sprintf("Connected to publisher socker %s", address))
	if err = socket.SetRecvBuffer(1024 * 1024); err != nil {
		mgr.logger.Err(fmt.Sprintln("Failed to set the buffer size for subsriber socket %s, error:",
			address, err))
		return nil, err
	}
	return socket, nil
}