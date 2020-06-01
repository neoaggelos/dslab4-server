package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	mrand "math/rand"
	"net"
	"net/url"
	"regexp"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var bind string
var mqttServer string
var done chan struct{}
var lastMessage = binMarshal(&v3Message{
	UplinkMessage: UplinkMessage{
		ReceivedAt: time.Now(),
	},
})
var re *regexp.Regexp
var tryAgain = []byte("try again\n")
var invalidCode = []byte("invalid code\n")

// UplinkMessage is an uplink message
type UplinkMessage struct {
	ReceivedAt     time.Time `json:"received_at"`
	FPort          int       `json:"f_port"`
	DecodedPayload struct {
		Event       string  `json:"event"`
		Light       int     `json:"light"`
		Temperature float32 `json:"temperature"`
	} `json:"decoded_payload"`
}

type v3Message struct {
	UplinkMessage UplinkMessage `json:"uplink_message"`
}

func binMarshal(v3msg *v3Message) []byte {
	if v3msg == nil {
		return []byte{}
	}
	return []byte(fmt.Sprintf("%d %03d %04d %d\n",
		v3msg.UplinkMessage.FPort,
		v3msg.UplinkMessage.DecodedPayload.Light,
		int(v3msg.UplinkMessage.DecodedPayload.Temperature*100),
		v3msg.UplinkMessage.ReceivedAt.Unix(),
	))
}

// listen connects to the MQTT server and updates the lastMessage
func listen() {
	uri, err := url.Parse(mqttServer)
	if err != nil {
		panic(err)
	}
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s", uri.Host))
	opts.SetUsername(uri.User.Username())
	password, _ := uri.User.Password()
	opts.SetPassword(password)
	client := mqtt.NewClient(opts)
	token := client.Connect()
	for !token.WaitTimeout(3 * time.Second) {
	}
	if err := token.Error(); err != nil {
		log.Fatal(err)
	}
	client.Subscribe("#", 0, func(client mqtt.Client, msg mqtt.Message) {
		v3msg := &v3Message{}
		if err := json.Unmarshal(msg.Payload(), &v3msg); err == nil {
			fmt.Printf("[MQ] New message: %v\n", v3msg)
			lastMessage = binMarshal(v3msg)
		}
	})
}

// connections listens on a TCP port for new clients
func connections() {
	l, err := net.Listen("tcp", bind)
	if err != nil {
		panic(err)
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Printf("[LI] Could not accept new connection\n")
			continue
		}

		log.Printf("[LI] New client: %s\n", conn.RemoteAddr().String())
		go handleConn(conn)
	}
}

// handleConn handles a TCP connection
func handleConn(conn net.Conn) {
	addr := conn.RemoteAddr().String()
	buffer := make([]byte, 100)
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			log.Printf("[HL] Could not read from %s\n", addr)
			conn.Close()
			return
		}

		msg := string(buffer[:n])

		// Get latest sensor data
		if len(msg) >= 3 && msg[:3] == "get" {
			conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
			n, err = conn.Write(lastMessage)
			if err != nil || n != len(lastMessage) {
				log.Printf("[HL] Could not write to %s\n", addr)
				conn.Close()
				return
			}
		} else if req := re.FindString(msg); req != "" {
			count := 5 + mrand.Intn(5)
			_buf := make([]byte, count)
			n, err := rand.Read(_buf)
			buf := []byte(hex.EncodeToString(_buf))
			bufWithNewline := []byte(hex.EncodeToString(_buf))

			if err != nil {
				conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
				n, err = conn.Write(tryAgain)
				if err != nil || n != len(tryAgain) {
					log.Printf("[HL] Could not write to %s\n", addr)
					conn.Close()
					return
				}
			} else {
				conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
				n, err = conn.Write(bufWithNewline)
				if err != nil || n != len(bufWithNewline) {
					log.Printf("[HL] Could not write to %s\n", addr)
					conn.Close()
					return
				}

				n, err := conn.Read(buffer)
				if err != nil {
					log.Printf("[HL] Could not read from %s\n", addr)
					conn.Close()
					return
				}

				fmt.Printf("BUFFER IS '%s'\n", string(buffer))

				if n >= len(buf)-1 && string(buffer[:len(buf)-1]) == string(buf[:len(buf)-1]) {
					msg := []byte(fmt.Sprintf("ACK %s\n", msg))
					conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
					n, err = conn.Write(msg)
					if err != nil || n != len(msg) {
						log.Printf("[HL] Could not write to %s\n", addr)
						conn.Close()
						return
					}
				} else {
					conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
					n, err = conn.Write(invalidCode)
					if err != nil || n != len(invalidCode) {
						log.Printf("[HL] Could not write to %s\n", addr)
						conn.Close()
					}
				}
			}
		} else {
			conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
			n, err = conn.Write(tryAgain)
			if err != nil || n != len(tryAgain) {
				log.Printf("[HL] Could not write to %s\n", addr)
				conn.Close()
			}
		}
	}
}

func main() {
	flag.StringVar(&bind, "bind", ":8080", "Address to bind to")
	flag.StringVar(&mqttServer, "mqtt", "", "Mqtt Address to connect to")
	flag.Parse()

	re = regexp.MustCompile(`\d \w+ \w+ .+`)

	go listen()
	go connections()

	done = make(chan struct{}, 1)
	<-done
}
