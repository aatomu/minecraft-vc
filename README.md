# minecraft-chat

クライアント/サーバー本体 にインストールせずに使える Minecraft 内 ボイスチャット

## 使い方

### サーバーの起動

以下を実行して、サーバーを起動します。

```sh
go run main.go -address="localhost:25575" -pass="0000"
```

### クライアントのアクセス

ブラウザで以下の URL を開きます。

```sh
http://example.com/?id=<MCID>
```

※ `<MCID>` を自分の Minecraft ID に置き換えてください。

# 引数/コンフィグ

| Required | Flag Name | Description                       | Default           |
| :------: | :-------- | :-------------------------------- | :---------------- |
|    No    | `listen`  | クライアントがアクセスするポート  | `1031`            |
|   Yes    | `address` | Minecraft の Rcon アドレス        | `localhost:25575` |
|   Yes    | `pass`    | Minecraft の Rcon パスワード      | `0000`            |
|    No    | `update`  | プレイヤー位置の確認間隔 (ミリ秒) | `1000`            |
|    No    | `fadeout` | 音量が小さくなり始める距離        | `3.0`             |
|    No    | `mute`    | 音量が 0 になる距離               | `15.0`            |

> `update`の値を小さくすると音量への反映が早くなりますが、Minecraft サーバーやクライアントが重く可能性があります
