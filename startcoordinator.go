package main

import (
	"fmt"
	"github.com/thingfu/zigbee/handler"
	serial "github.com/tarm/goserial"
	"io"
	"log"
	"os"
	"os/signal"
)

const (
	MAX_FRAME_SIZE = 256
)

func ParseResponse(buf []byte) {
	// fmt.Println("Incoming Buffer: ")
	// fmt.Println(buf)

	sof := buf[0]
	if sof != handler.SOF {
		log.Println("Invalid Start of Frame Indicator")
		return
	}

	frameEnd := int(buf[1]) + 4
	cmd0 := buf[2]
	cmd1 := buf[3]
	data := buf[4:frameEnd]

	fcs := buf[frameEnd]

	if !handler.VerifyFCS(buf[1:frameEnd], fcs) {
		fmt.Println("Failed VerifyFCS")
		return
	}

	frame := new(handler.IncomingFrame)
	frame.Cmd0 = cmd0
	frame.Cmd1 = cmd1
	frame.Data = data

	responseHandler.HandleResponse(*frame)
}

func StartReadFromSerial(s io.ReadWriteCloser) {
	for {
		serialContent := make([]byte, MAX_FRAME_SIZE)
		_, err := s.Read(serialContent)
		if err != nil {
			log.Fatal(err)
		}
		ParseResponse(serialContent)
	}
}

var responseHandler *handler.ResponseHandler

func main() {
	c := &serial.Config{Name: "/dev/tty.usbmodem14121", Baud: 115200}
	s, err := serial.OpenPort(c)
	if err != nil {
		log.Fatal(err)
	}

	responseHandler = handler.NewResponseHandler(s)
	go StartReadFromSerial(s)

	// SYS_RESET_REQ

	responseHandler.StartComm()

	sig_chan := make(chan os.Signal, 10)
	signal.Notify(sig_chan, os.Interrupt)
	<-sig_chan
	log.Println("Coordinator Stopped")
	os.Exit(0)
}

/*
	2014/09/22 02:15:31 Unknown Command [Cmd0: 0x45, Cmd1: 0xca]
*/
