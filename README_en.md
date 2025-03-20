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

- `listen=1031`
  - Listen port \
    The client needs access to this port.
- \*`address="localhost:25575"`
  - Minecraft rcon address
- \*`pass="0000"`
  - Minecraft rcon password
- `update=1000`
  - Interval(millisecond) for checking player position in Minecraft \
    The higher the value, the slower the volume updates.
- `fadeout=3.0`
  - Distance at which the volume starts to decrease
- `mute=15.0`
  - Distance at which the volume reaches zero

Default values are shown after '=' \
Arguments marked with '\*' are required
