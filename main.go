package main

import (
	"encoding/binary"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/aatomu/aatomlib/utils"
	"golang.org/x/net/websocket"
)

var (
	Listen     = ":1031"
	Root       = "./assets"
	Users      = map[string]*websocket.Conn{}
	UsersMutex sync.Mutex
)

func main() {
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
		log.Printf("Websocket connect cancel id=%s", meId)
		ws.WriteClose(400)
		return
	}
	log.Printf("Websocket connect id=%s, IP=%s", meId, ws.RemoteAddr())

	UsersMutex.Lock()
	Users[meId] = ws
	UsersMutex.Unlock()

	defer func() {
		log.Printf("Websocket disconnect id=%s, IP=%s", meId, ws.RemoteAddr())

		UsersMutex.Lock()
		delete(Users, meId)
		UsersMutex.Unlock()

		ws.Close()
	}()

	header := make([]byte, 0, 4+16)
	header = binary.LittleEndian.AppendUint16(header, uint16(len(meId)))
	header = append(header, []byte(meId)...)
	gainBytes := make([]byte, 4)
	// idLen+id+raw_pcm
	var sentMessage []byte

	var err error
	var message []byte
	var gain float32 = 1.0
	for {
		err = websocket.Message.Receive(ws, &message)
		if err != nil {
			return
		}

		if len(message)%4 != 0 || len(message)/4 < 10 {
			continue
		}

		for id, user := range Users {
			if id == meId && false {
				// if id == meId {
				continue
			}

			// Gain control
			gain = 10
			binary.Encode(gainBytes, binary.LittleEndian, gain)

			sentMessage = append(header, gainBytes...)
			sentMessage = append(sentMessage, message...)
			err = websocket.Message.Send(user, sentMessage)
			if err != nil {
				log.Printf("Websocket err id=%s, location=\"sent message\", err=%s", meId, err)
			}
		}
	}
}
