package main

import (
	"flag"
	"fmt"
	"github.com/hemtjanst/hemtjanst/device"
	"github.com/hemtjanst/hemtjanst/messaging"
	"github.com/hemtjanst/hemtjanst/messaging/flagmqtt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	timeout      = flag.Int("timeout", 120, "Minutes after which we think the robot is done")
	startTopic   = flag.String("start.topic", "remote/robovac/KEY_AUTO", "LIRCd topic to start the vacuum")
	startPress   = flag.String("start.press", "200", "Milliseconds to hold down the start button")
	stopTopic    = flag.String("stop.topic", "remote/robovac/KEY_HOME", "LIRCd topic to stop the vacuum")
	stopPress    = flag.String("stop.press", "5000", "Milliseconds to hold down the stop button")
	manufacturer = flag.String("robot.manufacturer", "Eufy", "Vacuum manufacturer")
	name         = flag.String("robot.name", "RoboVac", "Vacuum name")
	model        = flag.String("robot.model", "11", "Vacuum model")
	serial       = flag.String("robot.serial-number", "undefined", "Vacuum serial number")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Parameters:\n\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n")
	}
	flag.Parse()

	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	id := flagmqtt.NewUniqueIdentifier()
	conf := flagmqtt.ClientConfig{
		ClientID:    "dammsugare",
		WillTopic:   "leave",
		WillPayload: id,
		WillRetain:  false,
		WillQoS:     0,
	}
	c, err := flagmqtt.NewPersistentMqtt(conf)
	if err != nil {
		log.Fatal("Could not configure the MQTT client: ", err)
	}

	if token := c.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal("Failed to establish connection with broker: ", token.Error())
	}

	m := messaging.NewMQTTMessenger(c)

	robot := device.NewDevice("robot/vacuum", m)
	robot.Manufacturer = *manufacturer
	robot.Name = *name
	robot.Model = *model
	robot.SerialNumber = *serial
	robot.Type = "switch"
	robot.LastWillID = id
	robot.AddFeature("on", &device.Feature{})

	m.Subscribe("discover", 1, func(msg messaging.Message) {
		robot.PublishMeta()
	})

	log.Printf("Announced %s to Hemtjänst", *name)

	on, _ := robot.GetFeature("on")
	on.OnSet(func(msg messaging.Message) {
		if string(msg.Payload()) == "1" {
			m.Publish(*startTopic, []byte(*startPress), 1, false)
			on.Update("1")
			go func() {
				<-time.After(time.Duration(*timeout) * time.Minute)
				on.Update("0")
				log.Print("Timeout expired, setting switch to off")
			}()
			log.Print("Turned on robot")
		} else {
			m.Publish(*stopTopic, []byte(*stopPress), 1, false)
			on.Update("0")
			log.Print("Turned off robot")
		}
	})

loop:
	for {
		select {
		case sig := <-quit:
			log.Printf("Received signal: %s, proceeding to shutdown", sig)
			break loop
		}
	}

	c.Disconnect(250)
	log.Print("Disconnected from broker. Bye!")
	os.Exit(0)
}
