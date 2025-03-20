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
	"sync"
	"time"

	"github.com/aatomu/aatomlib/rcon"
	"github.com/aatomu/aatomlib/utils"
	"golang.org/x/net/websocket"
)

const (
	Listen            = ":1031"
	Root              = "./assets"
	PosUpdateInterval = 1000
)

var (
	Users       = map[string]*User{}
	UsersMutex  sync.Mutex
	RconAddress = flag.String("address", "", "")
)

type User struct {
	Conn   *websocket.Conn
	Header []byte
	Pos    [3]float64
}

type opCode uint8

const (
	opPCM opCode = iota
	opGain
	opDelete
)

func main() {
	flag.Parse()
	// 移動
	_, file, _, _ := runtime.Caller(0)
	goDir := filepath.Dir(file) + "/"
	os.Chdir(goDir)

	// アクセス先
	http.HandleFunc("/", HttpResponse)
	http.Handle("/websocket", websocket.Handler(WebSocketResponse))
	// Web鯖 起動
	go func() {
		log.Println("Http Server Boot")
		err := http.ListenAndServe(Listen, nil)
		if err != nil {
			log.Println("Failed Listen:", err)
			return
		}
	}()
	// Rcon 接続を確立
	go func() {
		ticker := time.NewTicker(time.Duration(PosUpdateInterval) * time.Millisecond)
		retry := 0
		posRegexp := regexp.MustCompile(`(-?[0-9]+\.[0-9]+)d`)
		pos := [3]float64{}
		for {
			time.Sleep(time.Duration(retry) * time.Second)
			log.Printf("Rcon connecting retry=%d", retry)
			r, err := rcon.Login(*RconAddress, "00128")
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

					UsersMutex.Lock()
					user.Pos = pos
					UsersMutex.Unlock()
				}
			}
		}
	}()
	<-utils.BreakSignal()
}

// ページ表示
func HttpResponse(w http.ResponseWriter, r *http.Request) {
	log.Println("Access:", r.RemoteAddr, "Path:", r.URL.Path, "File:", filepath.Join(Root, r.URL.Path))

	bytes, _ := os.ReadFile(filepath.Join(Root, r.URL.Path))
	switch filepath.Ext(r.URL.Path) {
	case "html":
		w.Header().Set("content-type", "text/html")
	case ".js":
		w.Header().Set("content-type", "text/javascript")
	}
	w.Write(bytes)
}

// ウェブソケット処理
func WebSocketResponse(ws *websocket.Conn) {
	meId := ws.Request().URL.Query().Get("id")

	if _, ok := Users[meId]; ok {
		log.Printf("Websocket connect cancel id=%s, because=\"Multi login not allowed.\"", meId)
		ws.WriteClose(400)
		return
	}

	log.Printf("Websocket connect id=%s, IP=%s", meId, ws.RemoteAddr())
	isClose := false

	header := make([]byte, 0, 4+16)
	header = binary.LittleEndian.AppendUint16(header, uint16(len(meId)))
	header = append(header, []byte(meId)...)

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
		packet := packetBuilder(opPCM, me.Header, []byte{})
		for id, user := range Users {
			if id == meId && false {
				// if id == meId {
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

	// Update Gain
	go func() {
		ticker := time.NewTicker(time.Duration(PosUpdateInterval) * time.Millisecond)
		gainBytes := make([]byte, 4)
		for !isClose {
			<-ticker.C
			log.Println(me.Pos)
			for id, user := range Users {
				if id == meId && false {
					// if id == meId {
					continue
				}

				x := (user.Pos[0] - me.Pos[0]) * (user.Pos[0] - me.Pos[0])
				y := (user.Pos[1] - me.Pos[1]) * (user.Pos[1] - me.Pos[1])
				z := (user.Pos[2] - me.Pos[2]) * (user.Pos[2] - me.Pos[2])
				d := math.Sqrt(x + y + z)

				gain := 1 - (d * 0.1)
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

		if len(message)%4 != 0 || len(message)/4 < 10 {
			continue
		}

		packet := packetBuilder(opPCM, me.Header, message)
		for id, user := range Users {
			if id == meId && false {
				// if id == meId {
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
