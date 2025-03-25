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
	// Other
	isDebug = true
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

	serversMu.Lock()
	servers["test"] = &Server{
		users:   map[string]*User{},
		address: "localhost:25575",
		pass:    "0000",
		fadeout: 3.0,
		mute:    15.0,
	}
	serversMu.Unlock()

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
					time.Sleep(time.Duration(server.retry*5) * time.Second)
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
	log.Printf("[Http/INFO]: IP=\"%s\", method:\"%s\", request=\"%s\", resource=\"%s\"", r.RemoteAddr, r.Method, path, filePath)

	http.ServeFile(w, r, filePath)
}

// ウェブソケット処理
func WebSocketResponse(ws *websocket.Conn) {
	serverName := ws.Request().URL.Query().Get("server")
	meId := ws.Request().URL.Query().Get("id")

	header := make([]byte, 0, 4+16)
	header = binary.LittleEndian.AppendUint16(header, uint16(len(meId)))
	header = append(header, []byte(meId)...)

	server, ok := servers[serverName]
	if !ok {
		log.Printf("[Websocket/INFO]: server=\"%s\", client=\"%s\", IP=%s, message=\"cancel connect: server not found\"", serverName, meId, ws.RemoteAddr())
		websocket.Message.Send(ws, packetBuilder(opMessage, header, []byte("Connection cancel: server not found.")))
		ws.Close()
		return
	}

	if _, ok := server.users[meId]; ok {
		log.Printf("[Websocket/INFO]: server=\"%s\", client=\"%s\", IP=%s, message=\"cancel connect: multi login not allowed.\"", serverName, meId, ws.RemoteAddr())
		websocket.Message.Send(ws, packetBuilder(opMessage, header, []byte("Connection cancel: multi login is not allowed.")))
		ws.Close()
		return
	}

	log.Printf("[Websocket/INFO]: server=\"%s\", client=\"%s\", IP=%s, message=\"new connect\"", serverName, meId, ws.RemoteAddr())
	isClose := false

	me := &User{
		Conn:   ws,
		Header: header,
		Pos:    [3]float64{0, 0, 0},
	}
	server.usersMu.Lock()
	server.users[meId] = me
	server.usersMu.Unlock()

	defer func() {
		log.Printf("[Websocket/INFO]: server=\"%s\", client=\"%s\", IP=%s, message=\"disconnect\"", serverName, meId, ws.RemoteAddr())
		packet := packetBuilder(opDelete, me.Header, []byte{})
		for id, user := range server.users {
			if id == meId {
				continue
			}

			err := websocket.Message.Send(user.Conn, packet)
			if err != nil {
				log.Printf("[Websocket/ERROR]: server=\"%s\", client=\"%s\", IP=%s, to:\"%s\" message=\"failed sent close message: %s\"", serverName, meId, ws.RemoteAddr(), id, err)
			}
		}

		isClose = true

		server.usersMu.Lock()
		delete(server.users, meId)
		server.usersMu.Unlock()

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

			for id, user := range server.users {
				if (id == meId || !me.isExist) && !isDebug {
					continue
				}

				var gain float64 = 0.0
				if user.Dimension == me.Dimension {
					x := (user.Pos[0] - me.Pos[0]) * (user.Pos[0] - me.Pos[0])
					y := (user.Pos[1] - me.Pos[1]) * (user.Pos[1] - me.Pos[1])
					z := (user.Pos[2] - me.Pos[2]) * (user.Pos[2] - me.Pos[2])
					d := math.Sqrt(x + y + z)

					if d <= server.fadeout {
						gain = 1
					} else if d <= server.mute {
						d = d - server.fadeout
						distanceRange := server.mute - server.fadeout
						gain = 1 - (d / distanceRange)

						if gain < 0 {
							gain = 0
						}
					}
				}
				binary.Encode(gainBytes, binary.LittleEndian, float32(gain))

				err := websocket.Message.Send(user.Conn, packetBuilder(opGain, me.Header, gainBytes))
				if err != nil {
					log.Printf("[Websocket/ERROR]: server=\"%s\", client=\"%s\", IP=%s, to:\"%s\" message=\"failed sent gain message: %s\"", serverName, meId, ws.RemoteAddr(), id, err)
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

		if (len(message)%4 != 0 || len(message)/4 < 10 || !me.isExist) && !isDebug {
			continue
		}

		packet := packetBuilder(opPCM, me.Header, message)
		for id, user := range server.users {
			if (id == meId || !user.isExist) && !isDebug {
				continue
			}

			err := websocket.Message.Send(user.Conn, packet)
			if err != nil {
				log.Printf("[Websocket/ERROR]: server=\"%s\", client=\"%s\", IP=%s, to:\"%s\" message=\"failed sent pcm message: %s\"", serverName, meId, ws.RemoteAddr(), id, err)
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

	if isCancel && s.rcon != nil {
		s.rcon.Close()
		s.rcon = nil
	}
}
