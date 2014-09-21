package main

import (
	serial "github.com/tarm/goserial"
	"log"
	"fmt"
	"io"
	"net/http"
	"github.com/go-home/zigbee/handler"
	"os"
	"os/signal"
)

func ParseResponse(buf []byte) {
	sof := buf[0]
	if sof != handler.SOF {
		log.Println("Invalid Start of Frame Indicator")
		return
	}

	frameEnd := int(buf[1])+4
	cmd0 := buf[2]
	cmd1 := buf[3]
	data := buf[4:frameEnd]

	fcs := buf[frameEnd]

	if !handler.VerifyFCS(buf[1:frameEnd], fcs) {
		return
	}

	frame := new (handler.IncomingFrame)
	frame.Cmd0 = cmd0
	frame.Cmd1 = cmd1
	frame.Data = data

	responseHandler.HandleResponse(*frame)
}

func StartReadFromSerial(s io.ReadWriteCloser) {
	for {
		serialContent := make([]byte, 256)
		_, err := s.Read(serialContent)
		if err != nil {
			log.Fatal(err)
		}
		ParseResponse(serialContent)
	}
}
var responseHandler *handler.ResponseHandler

func main() {
	c := &serial.Config{ Name: "/dev/tty.usbmodem14141", Baud: 115200 }
	s, err := serial.OpenPort(c)
	if err != nil {
		log.Fatal(err)
	}

	responseHandler = handler.NewResponseHandler(s)
	go StartReadFromSerial(s)

	sigchan := make(chan os.Signal, 10)
	signal.Notify(sigchan, os.Interrupt)
	<-sigchan
	log.Println("Coordinator Stopped")
	os.Exit(0)
}
