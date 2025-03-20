package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/aatomu/aatomlib/rcon"
	"github.com/aatomu/aatomlib/utils"
	"golang.org/x/net/websocket"
)

var (
	// Flags
	Listen            = flag.String("listen", "1031", "http server listen port")
	RconAddress       = flag.String("address", "localhost:25575", "minecraft rcon listening port")
	RconPass          = flag.String("pass", "0000", "minecraft rcon login password")
	PosUpdateInterval = flag.Int("update", 1000, "check player position interval")
	DistanceFadeout   = flag.Float64("fadeout", 3.0, "Voice fadeout distance in minecraft")
	DistanceMute      = flag.Float64("mute", 15.0, "Voice mute distance in minecraft")

	// Resource
	Root       string = "./assets"
	Users             = map[string]*User{}
	UsersMutex sync.Mutex
)

type User struct {
	Conn      *websocket.Conn
	Header    []byte
	isExist   bool
	Pos       [3]float64
	Dimension string
}

type opCode uint8

const (
	opPCM opCode = iota
	opGain
	opDelete
	opMessage
)

func main() {
	_, file, _, _ := runtime.Caller(0)
	goDir := filepath.Dir(file) + "/"
	os.Chdir(goDir)

	flag.Parse()

	// Http request handler
	http.HandleFunc("/", HttpResponse)
	http.Handle("/websocket", websocket.Handler(WebSocketResponse))
	// Boot http server
	go func() {
		log.Println("Http Server Boot")
		err := http.ListenAndServe(":"+*Listen, nil)
		if err != nil {
			log.Println("Failed Listen:", err)
			return
		}
	}()
	// Rcon connection to server
	go func() {
		retry := 0

		ticker := time.NewTicker(time.Duration(*PosUpdateInterval) * time.Millisecond)

		// user data cache
		posRegexp := regexp.MustCompile(`(-?[0-9]+\.[0-9]+)d`)
		pos := [3]float64{0, 0, 0}
		var isExist bool

		for {
			time.Sleep(time.Duration(retry) * time.Second)
			log.Printf("Rcon connecting retry=%d", retry)
			r, err := rcon.Login(*RconAddress, *RconPass)
			if err != nil {
				log.Printf("Rcon connect err because=\"%s\"", err)
				retry++
				continue
			}

			log.Printf("Rcon connected")
			retry = 0

			shouldUpdate := true
			for shouldUpdate {
				<-ticker.C
				for id, user := range Users {
					// is exist player
					result, err := r.SendCommand(fmt.Sprintf("execute if entity %s", id))
					if err != nil {
						shouldUpdate = false
						break
					}
					isExist = strings.Contains(string(result.Body), "1")

					// get Pos
					for i := 0; i < 3; i++ {
						result, err := r.SendCommand(fmt.Sprintf("data get entity %s Pos[%d]", id, i))
						if err != nil {
							shouldUpdate = false
							break
						}
						match := posRegexp.FindAllSubmatch(result.Body, 1)
						if len(match) != 1 {
							continue
						}

						fmt.Sscanf(string(match[0][1]), "%f", &pos[i])
					}
					// get Dimension
					result, err = r.SendCommand(fmt.Sprintf("data get entity %s Dimension", id))
					if err != nil {
						shouldUpdate = false
						break
					}
					dimension := strings.Split(string(result.Body), " ")

					UsersMutex.Lock()
					user.isExist = isExist
					user.Pos = pos
					user.Dimension = dimension[len(dimension)-1]
					UsersMutex.Unlock()
				}
			}
		}
	}()
	<-utils.BreakSignal()
}

// ページ表示
func HttpResponse(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	log.Println("Access:", r.RemoteAddr, "Path:", path)

	filePath := filepath.Join(Root, filepath.Clean(path))
	http.ServeFile(w, r, filePath)
}

// ウェブソケット処理
func WebSocketResponse(ws *websocket.Conn) {
	meId := ws.Request().URL.Query().Get("id")

	header := make([]byte, 0, 4+16)
	header = binary.LittleEndian.AppendUint16(header, uint16(len(meId)))
	header = append(header, []byte(meId)...)

	if _, ok := Users[meId]; ok {
		log.Printf("Websocket connect cancel id=%s, because=\"Multi login not allowed.\"", meId)
		websocket.Message.Send(ws, packetBuilder(opMessage, header, []byte("Connection cancel: multi login is not allowed.")))
		ws.Close()
		return
	}

	log.Printf("Websocket connect id=%s, IP=%s", meId, ws.RemoteAddr())
	isClose := false

	me := &User{
		Conn:   ws,
		Header: header,
		Pos:    [3]float64{0, 0, 0},
	}
	UsersMutex.Lock()
	Users[meId] = me
	UsersMutex.Unlock()

	defer func() {
		log.Printf("Websocket disconnect id=%s, IP=%s", meId, ws.RemoteAddr())
		packet := packetBuilder(opDelete, me.Header, []byte{})
		for id, user := range Users {
			if id == meId {
				continue
			}

			err := websocket.Message.Send(user.Conn, packet)
			if err != nil {
				log.Printf("Websocket err srcId=%s, destId=%s location=\"sent close message\", err=%s", meId, id, err)
			}
		}

		isClose = true

		UsersMutex.Lock()
		delete(Users, meId)
		UsersMutex.Unlock()

		ws.Close()
	}()

	// Gain control
	go func() {
		ticker := time.NewTicker(time.Duration(*PosUpdateInterval) * time.Millisecond)
		gainBytes := make([]byte, 4)
		for {
			<-ticker.C
			if isClose {
				break
			}
			for id, user := range Users {
				if id == meId || !me.isExist {
					continue
				}

				var gain float64 = 0.0
				if user.Dimension == me.Dimension {
					x := (user.Pos[0] - me.Pos[0]) * (user.Pos[0] - me.Pos[0])
					y := (user.Pos[1] - me.Pos[1]) * (user.Pos[1] - me.Pos[1])
					z := (user.Pos[2] - me.Pos[2]) * (user.Pos[2] - me.Pos[2])
					d := math.Sqrt(x + y + z)

					if d <= *DistanceFadeout {
						gain = 1
					} else if d <= *DistanceMute {
						d = d - *DistanceFadeout
						distanceRange := *DistanceMute - *DistanceFadeout
						gain = 1 - (d / distanceRange)

						if gain < 0 {
							gain = 0
						}
					}
				}
				binary.Encode(gainBytes, binary.LittleEndian, float32(gain))

				err := websocket.Message.Send(user.Conn, packetBuilder(opGain, me.Header, gainBytes))
				if err != nil {
					log.Printf("Websocket err srcId=%s, destId=%s location=\"sent pcm message\", err=%s", meId, id, err)
				}
			}
		}
	}()

	// Receive/Multicast PCM
	var err error
	var message []byte
	for {
		err = websocket.Message.Receive(ws, &message)
		if err != nil {
			return
		}

		if len(message)%4 != 0 || len(message)/4 < 10 || !me.isExist {
			continue
		}

		packet := packetBuilder(opPCM, me.Header, message)
		for id, user := range Users {
			if id == meId || !user.isExist {
				continue
			}

			err := websocket.Message.Send(user.Conn, packet)
			if err != nil {
				log.Printf("Websocket err srcId=%s, destId=%s location=\"sent pcm message\", err=%s", meId, id, err)
			}
		}
	}

}

func packetBuilder(op opCode, header, body []byte) (packet []byte) {
	packet = []byte{uint8(op)}
	packet = append(packet, header...)
	packet = append(packet, body...)

	return packet
}
