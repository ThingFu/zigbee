package handler

import (
	"io"
	"log"
	"fmt"
)

/*
### UART Transport frame

    # Frame Format
    |0xFE|GFF|fcs|

    # General Format Frame (GFF)
    |data_length|cmd0|cmd1|data|
 */

const (
	SOF = 0xFE	// Start of Frame Indicator

	// Command Types
	CMDTYPE_SREQ = 0x20
	CMDTYPE_AREQ = 0x40

	// Subsystems
	SUB_SYS = 0x01
	SUB_AF = 0x04
	SUB_ZB = 0x06

	// Commands with no data
	DATALENGTH_ZERO = 0x00

	// Commands
	SYS_PING = 0x01
	SYS_VERSION = 0x02

	ZB_START_REQUEST = 0x00
	ZB_WRITE_CONFIGURATION = 0x05
)

type IncomingFrame struct {
	Cmd0	byte
	Cmd1	byte
	Data	[]byte
}

type ResponseHandler struct {
	serial		io.ReadWriteCloser
}

func NewResponseHandler(s io.ReadWriteCloser) *ResponseHandler{
	h := new (ResponseHandler)
	h.serial = s

	return h
}

func (r *ResponseHandler) SendRequest(length byte, cmd0 byte, cmd1 byte, data []byte) {
	frame := []byte { SOF, length, cmd0, cmd1 }
	frame = append(frame, data...)
	frame = append(frame, CalculateFCS(frame[1:]))

	r.serial.Write(frame)
}

func (r *ResponseHandler) HandleResponse(f IncomingFrame) {
	cmd0 := f.Cmd0
	cmd1 := f.Cmd1
	data := f.Data

	switch {
	case cmd0 == 0x41 && cmd1 == 0x80:	// SYS_RESET_IND
		reason := f.Data[0]
		r.handlePowerUp(reason, data)

	case cmd0 == 0x66 && cmd1 == 0x00:	// ZB_START_REQUEST
		r.handleStartupRequest()

	case cmd0 == 0x45 && cmd1 == 0xC0:
		state := data[0]
		r.handleStateChange(state)

	case cmd0 == 0x66 && cmd1 == 0x05:
		status := data[0]
		r.handleWriteConfiguration(status)

	default:
		log.Println(fmt.Sprintf("Unknown Command [Cmd0: 0x%x, Cmd1: 0x%x]", cmd0, cmd1))
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
		log.Println("Started As Zigbee Co-ordinator")

	case state == 0x0A:
		log.Println("DEV_NWK_ORPHAN")

	default:
		log.Println("Unknown State")
	}
}

func (r *ResponseHandler) handleStartupRequest () {

	// ZB_START_CONFIRM

	// ZDO_STARTUP_FROM_APP
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

func (r *ResponseHandler) startUp() {
	// ZCD_NV_LOGICAL_TYPE
	r.SendRequest(byte(0x03), byte((CMDTYPE_SREQ |  SUB_ZB)), byte(ZB_WRITE_CONFIGURATION), []byte{ 0x0087, 0x01, 0x00 })

	// ZCD_NV_PAN_ID
	r.SendRequest(byte(0x04), byte((CMDTYPE_SREQ |  SUB_ZB)), byte(ZB_WRITE_CONFIGURATION), []byte { 0x0083, 0x02, 0xFF, 0xFF })

	// ZCD_NV_CHANLIST
	r.SendRequest(byte(0x06), byte((CMDTYPE_SREQ |  SUB_ZB)), byte(ZB_WRITE_CONFIGURATION), []byte{ 0x0084, 0x04, 0x00, 0x00, 0x08, 0x00 })


	// ZB_APP_REGISTER_REQUEST ?

	//  ZB_START_REQUEST
	r.SendRequest(byte(0x00), byte((CMDTYPE_SREQ |  SUB_ZB)), byte(ZB_START_REQUEST), []byte{})
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

