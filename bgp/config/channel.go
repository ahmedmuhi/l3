// conn.go
package config

type ReachabilityInfo struct {
	IP          string
	ReachableCh chan bool
}

type Operation int

const (
	NOTIFY_ROUTE_CREATED Operation = 1
	NOTIFY_ROUTE_DELETED Operation = 2
	BFD_STATE_VALID      Operation = 3
	BFD_STATE_INVALID    Operation = 4
	INTF_STATE_DOWN      Operation = 5
	INTF_STATE_UP        Operation = 6
)

type BfdInfo struct {
	Oper   Operation
	DestIp string
	State  bool
}

type IntfStateInfo struct {
	Idx    int32
	Ipaddr string
	State  Operation
}
