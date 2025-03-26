package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
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
	Users   map[string]*User `json:"-"`
	UsersMu *sync.Mutex      `json:"-"`
	// Rcon
	Address string     `json:"address"`
	Pass    string     `json:"pass"`
	Rcon    *rcon.Rcon `json:"-"`
	Retry   int        `json:"-"`
	// Volume
	Fadeout float64 `json:"fadeout"`
	Mute    float64 `json:"mute"`
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
				if server.Rcon == nil {
					time.Sleep(time.Duration(server.Retry*5) * time.Second)
					log.Printf("[Rcon/INFO]: server=\"%s\", message=\"try connecting\", retry=%d", serverName, server.Retry)
					r, err := rcon.Login(server.Address, server.Pass)
					if err != nil {
						log.Printf("[Rcon/ERROR]: server=\"%s\", message=\"filed connect: %s\"", serverName, err)
						server.Retry++
						continue
					}
					log.Printf("[Rcon/INFO]: server=\"%s\", message=\"connected\"", serverName)
					server.Retry = 0
					server.Rcon = r
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

	if strings.HasPrefix(filePath, "/api") {
		split := strings.Split(filePath, "/")
		if len(split) < 2 {
			returnRequest(w, http.StatusBadRequest, map[string]interface{}{
				"error": "function is required",
			})
			log.Printf("[Http/ERROR]: IP=\"%s\", method:\"%s\", request=\"%s\", message=\"function is required\"", r.RemoteAddr, r.Method, path)
			return
		}

		resource := split[2]
		switch resource {
		case "servers":
			switch r.Method {
			case http.MethodGet:
				server_list := map[string]Server{}

				for name, server := range servers {
					server_list[name] = Server{
						// Rcon
						Address: strings.Repeat("*", len(server.Address)),
						Pass:    strings.Repeat("*", len(server.Pass)),
						// Volume
						Fadeout: server.Fadeout,
						Mute:    server.Mute,
					}
				}

				w.Header().Set("Content-Type", "application/json")
				data, _ := json.Marshal(server_list)
				w.Write(data)
			default:
				returnRequest(w, http.StatusMethodNotAllowed, nil)
			}
		case "server":
			name := r.Header.Get("X-Name")
			pass := r.Header.Get("X-Password")

			server, ok := servers[name]

			switch r.Method {
			case http.MethodGet:
				if !ok {
					returnRequest(w, http.StatusNotFound, map[string]interface {
					}{
						"error": "server not found",
					})
					log.Printf("[Http/ERROR]: IP=\"%s\", method:\"%s\", request=\"%s\", message=\"server not found: %s\"", r.RemoteAddr, r.Method, path, name)
					return
				}
				if server.Pass != pass {
					returnRequest(w, http.StatusUnauthorized, nil)
					log.Printf("[Http/ERROR]: IP=\"%s\", method:\"%s\", request=\"%s\", message=\"password miss match: server=\\\"%s\\\", request=\\\"%s\\\"\"", r.RemoteAddr, r.Method, path, server.Pass, pass)
					return
				}
				result := Server{
					// Rcon
					Address: server.Address,
					Pass:    server.Pass,
					// Volume
					Fadeout: server.Fadeout,
					Mute:    server.Mute,
				}

				w.Header().Set("Content-Type", "application/json")
				data, _ := json.Marshal(result)
				w.Write(data)
			case http.MethodPut:
				if name == "" {
					returnRequest(w, http.StatusBadRequest, map[string]interface {
					}{
						"error": "missing server name",
					})
					log.Printf("[Http/ERROR]: IP=\"%s\", method:\"%s\", request=\"%s\", message=\"missing server name\"", r.RemoteAddr, r.Method, path)
					return
				}
				if ok {
					returnRequest(w, http.StatusBadRequest, map[string]interface {
					}{
						"error": "server already exists",
					})
					log.Printf("[Http/ERROR]: IP=\"%s\", method:\"%s\", request=\"%s\", message=\"server already exists: %s\"", r.RemoteAddr, r.Method, path, name)
					return
				}

				body, err := io.ReadAll(r.Body)
				if err != nil {
					returnRequest(w, http.StatusBadRequest, map[string]interface {
					}{
						"error": "failed body read",
					})
					log.Printf("[Http/ERROR]: IP=\"%s\", method:\"%s\", request=\"%s\", message=\"failed body read: %s\"", r.RemoteAddr, r.Method, path, err)
					return
				}
				var server Server
				err = json.Unmarshal(body, &server)
				if err != nil {
					returnRequest(w, http.StatusBadRequest, map[string]interface {
					}{
						"error": "failed to parse body",
					})
					log.Printf("[Http/ERROR]: IP=\"%s\", method:\"%s\", request=\"%s\", message=\"failed to parse body: %s\"", r.RemoteAddr, r.Method, path, err)
					return
				}

				serversMu.Lock()
				servers[name] = &Server{
					Users:   map[string]*User{},
					UsersMu: &sync.Mutex{},
					Address: server.Address,
					Pass:    server.Pass,
					Fadeout: server.Fadeout,
					Mute:    server.Mute,
				}
				serversMu.Unlock()

				w.WriteHeader(http.StatusCreated)
			case http.MethodDelete:
				if !ok {
					returnRequest(w, http.StatusNotFound, map[string]interface {
					}{
						"error": "server not found",
					})
					log.Printf("[Http/ERROR]: IP=\"%s\", method:\"%s\", request=\"%s\", message=\"server not found: %s\"", r.RemoteAddr, r.Method, path, name)
					return
				}
				if server.Pass != pass {
					returnRequest(w, http.StatusUnauthorized, nil)
					log.Printf("[Http/ERROR]: IP=\"%s\", method:\"%s\", request=\"%s\", message=\"password miss match: server=\\\"%s\\\", request=\\\"%s\\\"\"", r.RemoteAddr, r.Method, path, server.Pass, pass)
					return
				}

				serversMu.Lock()
				delete(servers, name)
				serversMu.Unlock()
				returnRequest(w, http.StatusNoContent, nil)
			default:
				returnRequest(w, http.StatusMethodNotAllowed, nil)
			}
		default:
			returnRequest(w, http.StatusNotFound, map[string]interface{}{
				"error": "function not found",
			})
		}
		log.Printf("[Http/INFO]: IP=\"%s\", method:\"%s\", API=\"%s\"", r.RemoteAddr, r.Method, resource)
		return
	}

	if strings.HasSuffix(filePath, "/") {
		filePath += "index"
	}
	if filepath.Ext(filePath) == "" {
		filePath += ".html"
	}
	filePath = filepath.Join(Root, filePath)
	log.Printf("[Http/INFO]: IP=\"%s\", method:\"%s\", request=\"%s\", resource=\"%s\"", r.RemoteAddr, r.Method, path, filePath)

	http.ServeFile(w, r, filePath)
}

func returnRequest(w http.ResponseWriter, status int, body map[string]interface{}) {
	w.WriteHeader(status)

	if body != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(body)
	}
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

	if _, ok := server.Users[meId]; ok {
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
	server.UsersMu.Lock()
	server.Users[meId] = me
	server.UsersMu.Unlock()

	defer func() {
		log.Printf("[Websocket/INFO]: server=\"%s\", client=\"%s\", IP=%s, message=\"disconnect\"", serverName, meId, ws.RemoteAddr())
		packet := packetBuilder(opDelete, me.Header, []byte{})
		for id, user := range server.Users {
			if id == meId {
				continue
			}

			err := websocket.Message.Send(user.Conn, packet)
			if err != nil {
				log.Printf("[Websocket/ERROR]: server=\"%s\", client=\"%s\", IP=%s, to:\"%s\" message=\"failed sent close message: %s\"", serverName, meId, ws.RemoteAddr(), id, err)
			}
		}

		isClose = true

		server.UsersMu.Lock()
		delete(server.Users, meId)
		server.UsersMu.Unlock()

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

			for id, user := range server.Users {
				if (id == meId || !me.isExist) && !isDebug {
					continue
				}

				var gain float64 = 0.0
				if user.Dimension == me.Dimension {
					x := (user.Pos[0] - me.Pos[0]) * (user.Pos[0] - me.Pos[0])
					y := (user.Pos[1] - me.Pos[1]) * (user.Pos[1] - me.Pos[1])
					z := (user.Pos[2] - me.Pos[2]) * (user.Pos[2] - me.Pos[2])
					d := math.Sqrt(x + y + z)

					if d <= server.Fadeout {
						gain = 1
					} else if d < server.Mute {
						d = d - server.Fadeout
						distanceRange := server.Mute - server.Fadeout
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

		if len(message)%4 != 0 || len(message)/4 < 10 || (!me.isExist && !isDebug) {
			continue
		}

		packet := packetBuilder(opPCM, me.Header, message)
		for id, user := range server.Users {
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
	for id, user := range s.Users {
		// is exist player
		result, err := s.Rcon.SendCommand(fmt.Sprintf("execute if entity %s", id))
		if err != nil {
			isCancel = true
			break
		}
		isExist = strings.Contains(string(result.Body), "1")

		// get Pos
		for i := 0; i < 3; i++ {
			result, err := s.Rcon.SendCommand(fmt.Sprintf("data get entity %s Pos[%d]", id, i))
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
		result, err = s.Rcon.SendCommand(fmt.Sprintf("data get entity %s Dimension", id))
		if err != nil {
			isCancel = true
			break
		}
		dimension := strings.Split(string(result.Body), " ")

		s.UsersMu.Lock()
		user.isExist = isExist
		user.Pos = pos
		user.Dimension = dimension[len(dimension)-1]
		s.UsersMu.Unlock()
	}

	if isCancel && s.Rcon != nil {
		s.Rcon.Close()
		s.Rcon = nil
	}
}
