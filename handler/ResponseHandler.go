package handler

import (
	"io"
	"log"
	"fmt"
	"strings"
	"github.com/go-home/hub/utils"
)

/*
### UART Transport frame

    # Frame Format
    |0xFE|GFF|FCS|

    # General Format Frame (GFF)
    |data_length|cmd0|cmd1|data|
 */

const (
	SOF = 0xFE	// Start of Frame Indicator

	// Command Types
	CMDTYPE_SREQ = 0x20
	CMDTYPE_AREQ = 0x40

	// Subsystems
	SUBCOMMAND_SYS = 0x01
	SUBCOMMAND_AF = 0x04
	SUBCOMMAND_ZDO = 0x05
	SUBCOMMAND_ZB = 0x06
	SUBCOMMAND_UTIL = 0x07


	// Commands with no data
	DATALENGTH_ZERO = 0x00

	// Commands
	SYS_PING = 0x01
	SYS_VERSION = 0x02
	SYS_RESET_REQ = 0x00

	ZB_START_REQUEST byte = 0x00
	ZB_WRITE_CONFIGURATION = 0x05
	ZB_PERMIT_JOINING_REQUEST = 0x08

	ZDO_STARTUP_FROM_APP = 0x40
	ZDO_MSG_CB_REGISTER = 0xFF
	ZDO_MATCH_DESC_REQ = 0x06
	ZDO_ACTIVE_EP_REQ = 0x05

	UTIL_ASSOC_COUNT = 0x48
	UTIL_ASSOC_FIND_DEVICE = 0x49
	UTIL_ADDRMGR_NWK_ADDR_LOOKUP = 0x41

	ZCD_NV_LOGICAL_TYPE = 0x0087
	ZCD_NV_PANID = 0x0083
	ZCD_NV_CHANLIST = 0x0084

	AF_REGISTER = 0x00

	CMDTYPE_SREQ_ZB byte = (CMDTYPE_SREQ |  SUBCOMMAND_ZB)
	CMDTYPE_SREQ_AF byte = (CMDTYPE_SREQ |  SUBCOMMAND_AF)
	CMDTYPE_SREQ_UTIL byte = (CMDTYPE_SREQ |  SUBCOMMAND_UTIL)
	CMDTYPE_SREQ_ZDO byte = (CMDTYPE_SREQ |  SUBCOMMAND_ZDO)
)

type IncomingFrame struct {
	Cmd0	byte
	Cmd1	byte
	Data	[]byte
}

type Callback struct {
	Fn		func(data []byte, params interface {})
	Param	interface {}
}

func NewCallback(fn func(data []byte, params interface {}), param interface {}) Callback {
	cb := new (Callback)
	cb.Fn = fn
	cb.Param = param

	return *cb
}

type ResponseHandler struct {
	serial		io.ReadWriteCloser
	callbacks	map[string]Callback
}

func NewResponseHandler(s io.ReadWriteCloser) *ResponseHandler{
	h := new (ResponseHandler)
	h.callbacks = make(map[string]Callback)
	h.serial = s

	return h
}

func (r *ResponseHandler) HandleResponse(f IncomingFrame) {
	cmd0 := f.Cmd0
	cmd1 := f.Cmd1
	data := f.Data
	key := ""
	switch {
	case cmd0 == 0x45:
		switch {
		case cmd1 == 0xC9:	// ZDO_LEAVE_IND

		case cmd1 == 0xC1:	// ZDO_END_DEVICE_ANNCE_IND

		case cmd1 == 0xC0:	// ZDO_STATE_CHANGE_IND
			state := data[0]
			r.handleStateChange(state)
		}

	case cmd0 == 0x41:
		switch {
		case cmd1 == 0x80: // SYS_RESET_IND
			reason := f.Data[0]
			r.handlePowerUp(reason, data)

		}

	case cmd0 == 0x64:
		switch {
			case cmd1 == 0x00:
		}

	case cmd0 == 0x65:
		switch {
		case cmd1 == 0x05:
			key = "ZDO_ACTIVE_EP_REQ"
		}

	case cmd0 == 0x66:
		switch {
		case cmd1 == 0x00: // ZB_START_REQUEST
			r.handleStartupRequest()

		case cmd1 == 0x08:
			r.handlePermitJoiningRequest()

		case cmd1 == 0x05:
			status := data[0]
			r.handleWriteConfiguration(status)
		}

	case cmd0 == 0x67:
		switch {
		case cmd1 == 0x41: // UTIL_ADDRMGR_NWK_ADDR_LOOKUP
			key = "UTIL_ADDRMGR_NWK_ADDR_LOOKUP"
			addr := ""
			for i := len(data) - 1; i >= 0; i-- {
				addr += fmt.Sprintf("%x", data[i])
			}
			log.Println("Discovered New Device: IEEE " + strings.ToUpper(addr))

		case cmd1 == 0x48: // UTIL_ASSOC_COUNT
			count := int(data[0]) | int (data[1]) << 8
			for i:=0; i < count; i++ {
				r.SendRequest(CMDTYPE_SREQ_UTIL, UTIL_ASSOC_FIND_DEVICE, []byte{ byte(i-1) })
			}

		case cmd1 == 0x49: // UTIL_ASSOC_FIND_DEVICE
			addr := []byte{ data[0], data[1] }

			cb1 := NewCallback(func(d []byte, param interface {}) {

				payload := []byte{ data[0], data[1], data[0], data[1], 0x04, 0x01, 0x00, 0x00 }

				cb2 := NewCallback(func(d []byte, param interface {}) {
					fmt.Println("ZDO_ACTIVE_EP_REQ")
					fmt.Println(d)
				}, []byte {data[0], data[1], data[0], data[1]})
				r.SendRequestWithCallback(CMDTYPE_SREQ_ZDO, ZDO_ACTIVE_EP_REQ, payload, "ZDO_ACTIVE_EP_REQ", &cb2)
			}, addr)

			r.SendRequestWithCallback(CMDTYPE_SREQ_UTIL, UTIL_ADDRMGR_NWK_ADDR_LOOKUP, addr, "UTIL_ADDRMGR_NWK_ADDR_LOOKUP", &cb1)
		}

	default:
		log.Println(fmt.Sprintf("Unknown Command [Cmd0: 0x%x, Cmd1: 0x%x]", cmd0, cmd1))
		fmt.Println(data)
	}

	for k, v := range r.callbacks {
		if strings.HasPrefix(k, key) {
			v.Fn(data, v.Param)
			delete (r.callbacks, k)
		}
	}
}

func (r *ResponseHandler) handleWriteConfiguration(status byte) {
	if status == 0x00 {
		log.Println("Write Configuration OK")
	} else {
		log.Println("Uh oh..Write Configuration did not go so well")
	}
}

func (r *ResponseHandler) handleStateChange(state byte) {
	switch {
	case state == 0x00:
		log.Println("DEV_HOLD")

	case state == 0x01:
		log.Println("DEV_INIT")

	case state == 0x02:
		log.Println("DEV_NWK_DISC")

	case state == 0x03:
		log.Println("DEV_NWK_JOINING")

	case state == 0x04:
		log.Println("DEV_NWK_REJOIN")

	case state == 0x05:
		log.Println("DEV_END_DEVICE_UNAUTH")

	case state == 0x06:
		log.Println("DEV_END_DEVICE")

	case state == 0x07:
		log.Println("DEV_ROUTER")

	case state == 0x08:
		log.Println("DEV_COORD_STARTING")

	case state == 0x09:
		r.handleStartedAsCoordinator()

	case state == 0x0A:
		log.Println("DEV_NWK_ORPHAN")

	default:
		log.Println("Unknown State")
	}
}


func (r *ResponseHandler) handlePermitJoiningRequest() {
	r.SendRequest(CMDTYPE_SREQ_UTIL, UTIL_ASSOC_COUNT, []byte{ 0x00, 0x06 })
	r.SendRequest(CMDTYPE_SREQ_UTIL, UTIL_ASSOC_FIND_DEVICE, []byte{ 0x00 })
}

func (r *ResponseHandler) handleStartedAsCoordinator() {
	log.Println("Started As Zigbee Co-ordinator")
}

func (r *ResponseHandler) StartComm() {
	r.SendRequest(CMDTYPE_SREQ_ZB, SYS_RESET_REQ, []byte{ 0x00 })
}

func (r *ResponseHandler) handleStartupRequest () {

	r.SendRequest(CMDTYPE_SREQ_AF, AF_REGISTER, []byte{ 0x01, 0x04, 0x01, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x01, 0x00, 0x05 })
	r.SendRequest(CMDTYPE_SREQ_ZDO, ZDO_STARTUP_FROM_APP, []byte{ 0x00, 0x00 })
	r.SendRequest(CMDTYPE_SREQ_ZDO, ZDO_MSG_CB_REGISTER, []byte{ 0x00, 0x05 })
	r.SendRequest(CMDTYPE_SREQ_ZB, ZB_PERMIT_JOINING_REQUEST, []byte{ 0xfc, 0xff, 0x00 })
	r.SendRequest(CMDTYPE_SREQ_ZB, ZB_PERMIT_JOINING_REQUEST, []byte{ 0xfc, 0xff, 0x3c })
}

func (r *ResponseHandler) handlePowerUp (reason byte, data []byte) {
	if reason == 0x00 {
		log.Println("Zigbee CC2531 ZNP Powered Up")
	} else
	if reason == 0x01 {
		log.Println("Reset by External")
	} else
	if reason == 0x02 {
		log.Println("Reset by Watchdog")
	}

	log.Println("Transport Revision: " + fmt.Sprintf("%d", data[1]))
	log.Println("Product ID: " + fmt.Sprintf("%d", data[2]))
	log.Println("Product Version: " + fmt.Sprintf("%d.%d.%d", data[3],data[4],data[5]))

	r.startUp()
}

func (r *ResponseHandler) SendRequest(cmd0 byte, cmd1 byte, data []byte) {
	length := byte(len(data))

	frame := []byte { SOF, length, cmd0, cmd1 }
	frame = append(frame, data...)
	frame = append(frame, CalculateFCS(frame[1:]))

	r.serial.Write(frame)
}

func (r *ResponseHandler) SendRequestWithCallback(cmd0 byte, cmd1 byte, data []byte, key string, cb *Callback) {
	id := utils.RandomString(7)
	r.callbacks[key + "-" + id] = *cb
	length := byte(len(data))

	frame := []byte { SOF, length, cmd0, cmd1 }
	frame = append(frame, data...)
	frame = append(frame, CalculateFCS(frame[1:]))

	r.serial.Write(frame)
}


func (r *ResponseHandler) startUp() {
	r.SendRequest(CMDTYPE_SREQ_ZB, ZB_WRITE_CONFIGURATION, []byte{ 0x03, 0x01, 0x00 })
	r.SendRequest(CMDTYPE_SREQ_ZB, ZB_WRITE_CONFIGURATION, []byte{ 0x8f, 0x01, 0x01 })
	r.SendRequest(CMDTYPE_SREQ_ZB, ZB_WRITE_CONFIGURATION, []byte{ 0x62, 0x10, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f })
	r.SendRequest(CMDTYPE_SREQ_ZB, ZB_WRITE_CONFIGURATION, []byte{ 0x63, 0x01, 0x00 })
	r.SendRequest(CMDTYPE_SREQ_ZB, ZB_WRITE_CONFIGURATION, []byte{ 0x64, 0x01, 0x01 })
	r.SendRequest(CMDTYPE_SREQ_ZB, ZB_WRITE_CONFIGURATION, []byte{ 0x87, 0x01, 0x00 })
	r.SendRequest(CMDTYPE_SREQ_ZB, ZB_WRITE_CONFIGURATION, []byte{ 0x84, 0x04, 0x00, 0x08, 0x00, 0x00 })

	// r.SendRequest(byte(0x03), byte((CMDTYPE_SREQ |  SUB_ZB)), byte(ZB_WRITE_CONFIGURATION), []byte{ ZCD_NV_LOGICAL_TYPE, 0x01, 0x00 })
	// r.SendRequest(byte(0x04), byte((CMDTYPE_SREQ |  SUB_ZB)), byte(ZB_WRITE_CONFIGURATION), []byte { ZCD_NV_PANID, 0x02, 0xFF, 0xFF })
	// r.SendRequest(byte(0x06), byte((CMDTYPE_SREQ |  SUB_ZB)), byte(ZB_WRITE_CONFIGURATION), []byte{ ZCD_NV_CHANLIST, 0x04, 0x00, 0x00, 0x08, 0x00 })

	r.SendRequest(CMDTYPE_SREQ_ZB, ZB_START_REQUEST, []byte{})
}

// Verify Frame Check Sequence
func VerifyFCS(payload []byte, fcs byte) bool {
	calculatedFcs := CalculateFCS(payload)

	if calculatedFcs != fcs {
		return false
	}
	return true
}

// Calculate Frame Check Sequence
func CalculateFCS(buf []byte) byte {
	var fcs byte = 0

	for i:=0; i < len(buf); i++ {
		b := buf[i]

		fcs = fcs ^ b
	}
	return fcs
}

func (r *ResponseHandler) CreateFrame(payload []byte) []byte {
	frame := []byte { SOF }
	frame = append(frame, payload...)
	frame = append(frame, CalculateFCS(payload))

	return frame
}

