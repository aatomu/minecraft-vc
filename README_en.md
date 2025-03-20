# minecraft-chat

A reproduction of SimpleVoiceChat that requires no installation

## How to use

`go run main.go -address="localhost:25575" -pass="0000"`

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
