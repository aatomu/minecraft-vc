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
	PosUpdateInterval = flag.Int("update", 1000, "check player position interval")
	// Resource
	Root      string = "./assets"
	servers          = map[string]*Server{}
	serversMu sync.Mutex
	Debug     = true
)

type Server struct {
	// User
	users   map[string]*User
	usersMu sync.Mutex
	// Rcon
	address string
	pass    string
	rcon    *rcon.Rcon
	retry   int
	// Volume
	fadeout float64
	mute    float64
}

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

	servers["test"] = &Server{
		users:   map[string]*User{},
		address: "localhost:25575",
		pass:    "0000",
		fadeout: 3.0,
		mute:    15.0,
	}
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

	go func() {
		ticker := time.NewTicker(time.Duration(*PosUpdateInterval) * time.Millisecond)

		for {
			<-ticker.C
			for serverName, server := range servers {
				// New rcon client
				if server.rcon == nil {
					time.Sleep(time.Duration(server.retry) * time.Second)
					log.Printf("[Rcon/INFO]: server=\"%s\", message=\"try connecting\", retry=%d", serverName, server.retry)
					r, err := rcon.Login(server.address, server.pass)
					if err != nil {
						log.Printf("[Rcon/ERROR]: server=\"%s\", message=\"filed connect: %s\"", serverName, err)
						server.retry++
						continue
					}
					log.Printf("[Rcon/INFO]: server=\"%s\", message=\"connected\"", serverName)
					server.retry = 0
					server.rcon = r
				}

				// Update player position
				go updatePosition(server)
			}
		}
	}()
	<-utils.BreakSignal()
}

// ページ表示
func HttpResponse(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	filePath := filepath.Clean(path)
	if strings.HasSuffix(path, "/") {
		filePath += "index"
	}
	if filepath.Ext(filePath) == "" {
		filePath += ".html"
	}
	filePath = filepath.Join(Root, filePath)
	log.Printf("HTTP: method=\"%s\", IP=\"%s\", request=\"%s\", resource=\"%s\"", r.Method, r.RemoteAddr, path, filePath)

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
				if (id == meId || !me.isExist) && !Debug {
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

		if (len(message)%4 != 0 || len(message)/4 < 10 || !me.isExist) && !Debug {
			continue
		}

		packet := packetBuilder(opPCM, me.Header, message)
		for id, user := range Users {
			if (id == meId || !user.isExist) && !Debug {
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

func updatePosition(s *Server) {
	// user data cache
	posRegexp := regexp.MustCompile(`(-?[0-9]+\.[0-9]+)d`)
	pos := [3]float64{0, 0, 0}
	var isExist bool

	isCancel := false
	for !isCancel {
		for id, user := range s.users {
			// is exist player
			result, err := s.rcon.SendCommand(fmt.Sprintf("execute if entity %s", id))
			if err != nil {
				isCancel = true
				break
			}
			isExist = strings.Contains(string(result.Body), "1")

			// get Pos
			for i := 0; i < 3; i++ {
				result, err := s.rcon.SendCommand(fmt.Sprintf("data get entity %s Pos[%d]", id, i))
				if err != nil {
					isCancel = true
					break
				}
				match := posRegexp.FindAllSubmatch(result.Body, 1)
				if len(match) != 1 {
					continue
				}

				fmt.Sscanf(string(match[0][1]), "%f", &pos[i])
			}
			// get Dimension
			result, err = s.rcon.SendCommand(fmt.Sprintf("data get entity %s Dimension", id))
			if err != nil {
				isCancel = true
				break
			}
			dimension := strings.Split(string(result.Body), " ")

			s.usersMu.Lock()
			user.isExist = isExist
			user.Pos = pos
			user.Dimension = dimension[len(dimension)-1]
			s.usersMu.Unlock()
		}
	}

	if isCancel {
		s.rcon.Close()
		s.rcon = nil
	}
}
