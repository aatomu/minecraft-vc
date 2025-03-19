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
	Users      = map[uint32]*websocket.Conn{}
	UsersMutex sync.Mutex
	UsersId    uint32 = 0
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
	meId := UsersId
	UsersId++
	log.Printf("Websocket connect id=%d, IP=%s", meId, ws.RemoteAddr())

	UsersMutex.Lock()
	Users[meId] = ws
	UsersMutex.Unlock()

	defer func() {
		log.Printf("Websocket disconnect id=%d, IP=%s", meId, ws.RemoteAddr())

		UsersMutex.Lock()
		delete(Users, meId)
		UsersMutex.Unlock()

		ws.Close()
	}()

	idBinary := make([]byte, 4)
	binary.LittleEndian.PutUint32(idBinary, meId)
	var err error
	var message []byte
	for {
		err = websocket.Message.Receive(ws, &message)
		if err != nil {
			return
		}

		for id, user := range Users {
			if id == meId && false {
				// if id == meId {
				continue
			}

			err = websocket.Message.Send(user, append(idBinary, message...))
			if err != nil {
				log.Printf("Websocket err id=%d, location=\"sent message\", err=%s", meId, err)
			}
		}
	}
}
