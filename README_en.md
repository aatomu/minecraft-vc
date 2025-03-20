# minecraft-chat

A reproduction of SimpleVoiceChat that requires no installation

## How to use

### Starting the Server

Run the following command to start the server:

```sh
go run main.go -address="localhost:25575" -pass="0000"
```

### Accessing the Client

Open the following URL in your browser:

```sh
http://example.com/?id=<MCID>
```

※ Replace <MCID> with your own Minecraft ID.

# Argument/Configuration

| Required | Flag Name | Description                                          | Default           |
| :------: | :-------- | :--------------------------------------------------- | :---------------- |
|    No    | `listen`  | Port for client access                               | `1031`            |
|   Yes    | `address` | Minecraft Rcon address                               | `localhost:25575` |
|   Yes    | `pass`    | Minecraft Rcon password                              | `0000`            |
|    No    | `update`  | Interval for checking player position (milliseconds) | `1000`            |
|    No    | `fadeout` | Distance at which the volume starts to decrease      | `3.0`             |
|    No    | `mute`    | Distance at which the volume reaches zero            | `15.0`            |

> Reducing the update value makes volume adjustments faster, but may increase the load on the Minecraft server and client.