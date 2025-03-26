# minecraft-chat

クライアント/サーバー本体 にインストールせずに使える Minecraft 内 ボイスチャット

## 使い方

### サーバーソフトの起動

1. 以下を実行して、サーバーを起動します。

```sh
go run .
```

### Minecraft サーバーの設定

`server.properties`の以下の設定を変更します。

```properties
enable-rcon=true
rcon.password=<Your Original Password>
rcon.port=<Your Use Port>
```

### サーバーの設定

1. 以下の URL を開きます。

```sh
http://localhost:1031/
```

2. F12 を開き Dev Tool を開きます。
3. 以下のコードを実行します。

```js
ServerPut("server", "address", "pass", fadeout, mute);
```

※ 必要に応じて、各項目を変更し Enter を押してください。

| Required | Flag Name | Description                            | Default           |
| :------: | :-------- | :------------------------------------- | :---------------- |
|   Yes    | `server`  | クライアントが接続する際に使用する名前 | `example`         |
|   Yes    | `address` | Minecraft の Rcon アドレス             | `localhost:25575` |
|   Yes    | `pass`    | Minecraft の Rcon パスワード           | `0000`            |
|    No    | `fadeout` | 音量が小さくなり始める距離             | `3.0`             |
|    No    | `mute`    | 音量が 0 になる距離                    | `15.0`            |

### クライアントのアクセス

1. 以下の URL を開きます。

```sh
http://localhost/
```

2. サーバー管理者から聞いた`Server Name`を入力します。
3. 自分の`MCID`を入力します。
4. `Connect`をクリックします。この際 ページ遷移が発生します
5. 画面上部の`Connect to VC`をクリックすると、ボイスチャットが利用できます

# 引数/コンフィグ

### サーバーソフトの起動

| Required | Flag Name | Description                       | Default |
| :------: | :-------- | :-------------------------------- | :------ |
|    No    | `listen`  | クライアントがアクセスするポート  | `1031`  |
|    No    | `update`  | プレイヤー位置の確認間隔 (ミリ秒) | `1000`  |

> `update`の値を小さくすると音量への反映が早くなりますが、Minecraft サーバーやクライアントが重く可能性があります

### Web API のコマンド

#### `ServersGet()`

登録されているサーバー一覧を表示

- 引数:
  - なし

#### `ServerGet(name, pass)`

登録されているサーバーを詳細表示

- 引数
  - `name`: 登録名
  - `pass`: Rcon password

#### `ServerPut(name, address, pass, fadeout, mute)`

新しくサーバーを登録する

- 引数
  - `name`: 新規登録する名前
  - `address`: Rcon Address
  - `pass`: Rcon password
  - `fadeout`: 小さくなり始める距離
  - `mute`: 無音になる距離

#### `ServerDelete(name, password)`

登録されているサーバーを削除

- 引数
  - `name`: 登録名
  - `pass`: Rcon password
