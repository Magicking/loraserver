package mqttpubsub

import (
	"bytes"
	"encoding/gob"
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/brocaar/loraserver"
	"github.com/eclipse/paho.mqtt.golang"
)

// Backend implements a MQTT pub-sub backend.
type Backend struct {
	conn         *mqtt.Client
	rxPacketChan chan loraserver.RXPacket
}

// NewBackend creates a new Backend.
func NewBackend(server, username, password string) (loraserver.GatewayBackend, error) {
	b := Backend{
		rxPacketChan: make(chan loraserver.RXPacket),
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(server)
	opts.SetUsername(username)
	opts.SetPassword(password)

	log.WithField("server", server).Info("gateway/mqttpubsub: connecting to mqtt server")
	b.conn = mqtt.NewClient(opts)
	if token := b.conn.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	log.WithField("topic", "gateway/+/rx").Info("gateway/mqttpubsub: subscribing to rx topic")
	if token := b.conn.Subscribe("gateway/+/rx", 0, b.rxPacketHandler); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	return &b, nil
}

// Close closes the backend.
func (b *Backend) Close() error {
	b.conn.Disconnect(250) // wait 250 milisec to complete pending operations
	return nil
}

// RXPacketChan returns the RXPacket channel.
func (b *Backend) RXPacketChan() chan loraserver.RXPacket {
	return b.rxPacketChan
}

// Send sends the given TXPacket to the gateway.
func (b *Backend) Send(txPacket loraserver.TXPacket) error {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(txPacket); err != nil {
		return err
	}
	topic := fmt.Sprintf("gateway/%s/tx", txPacket.TXInfo.MAC)
	log.WithField("topic", topic).Info("gateway/mqttpubsub: publishing message")
	if token := b.conn.Publish(topic, 0, false, buf.Bytes()); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}

func (b *Backend) rxPacketHandler(c *mqtt.Client, msg mqtt.Message) {
	var rxPacket loraserver.RXPacket
	dec := gob.NewDecoder(bytes.NewReader(msg.Payload()))
	if err := dec.Decode(&rxPacket); err != nil {
		log.Errorf("gateway/mqttpubsub: could not decode RXPacket: %s", err)
		return
	}
	b.rxPacketChan <- rxPacket
}
