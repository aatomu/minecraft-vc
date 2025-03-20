# minecraft-chat

クライアント/サーバー本体 にインストールせずに使える Minecraft 内 ボイスチャット

## 使い方

`go run main.go -address="localhost:25575" -pass="0000"`

# 引数/コンフィグ

- `listen=1031`
  - Listen port \
    クライアントのアクセス先
- \*`address="localhost:25575"`
  - Minecraft の Rcon アドレス
- \*`pass="0000"`
  - Minecraft の Rcon パスワード
- `update=1000`
  - Minecraft 内でプレイヤーの位置を確認する間隔 (millisecond) \
    大きい値にすると音量への反映が遅くなる
- `fadeout=3.0`
  - 音量が小さくなり始める距離
- `mute=15.0`
  - 音量が 0 になる距離

初期値は `=` の後に書かれています
`*`から始まる引数は設定が必要です