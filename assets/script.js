// @ts-check

// Global variables
/** @type {HTMLInputElement} */
// @ts-expect-error
const start = document.getElementById("start")
/** @type {HTMLSpanElement} */
// @ts-expect-error
const message = document.getElementById("message")
/** @type {HTMLDivElement} */
// @ts-expect-error
const volumeArea = document.getElementById("volume_area")
/** @type {AudioContext} */
// @ts-expect-error
let audioCtx = undefined
const sampleRate = 44100
const inputBufferTime = 100
let buffer = []
/** @type {Object.<string,User>} */
let users = {}

/**
 * @typedef User
 * @property {GainNode} serverGainNode
 * @property {GainNode} clientGainNode
 * @property {Number} schedule
 */

// Websocket initialize
console.log("Global initialize")
const url = new URL(window.location.href)
const params = new URLSearchParams(url.searchParams)
const id = params.get("id")
if (!id) {
  start.setAttribute("disabled", "true")
  message.innerText = "Required MCID parameter."
}


async function NewConnection() {
  console.log("Click new connection")

  // Websocket initialize
  console.log("Websocket initialize")
  const ws = new WebSocket(`./websocket?id=${id}`)
  ws.binaryType = "arraybuffer"
  let isClosed = false
  ws.addEventListener("open", () => {
    console.log("Websocket: open")
    setInterval(() => {
      if (isClosed) return

      ws.send(new Float32Array(buffer))
      buffer = []

    }, inputBufferTime)
  })
  ws.addEventListener("message", (e) => {
    if (!e.data) return
    let arr = e.data
    // OpCode
    const opCode = new Uint8Array(arr.slice(0, 1))[0]
    arr = arr.slice(1)
    // ID
    const idLen = new Uint16Array(arr.slice(0, 2))[0]
    arr = arr.slice(2)
    const id = new TextDecoder("utf-8").decode(arr.slice(0, idLen))
    arr = arr.slice(idLen)

    // User Initialize 
    if (!(id in users)) {
      console.log(`User(${id}) initialize`)
      users[id] = {
        serverGainNode: audioCtx.createGain(),
        clientGainNode: audioCtx.createGain(),
        schedule: audioCtx.currentTime,
      }
      users[id].serverGainNode.gain.value = 0
      users[id].serverGainNode.connect(users[id].clientGainNode)
      users[id].clientGainNode.connect(audioCtx.destination)

      newVolume(id)
    }

    switch (opCode) {
      case 0x00: { // opPCM
        playAudioStream(id, new Float32Array(arr))
        break
      }
      case 0x01: { // opGain
        const gain = new Float32Array(arr)[0]
        console.log(`Server control: gain id=${id}, value=${gain}`)
        users[id].serverGainNode.gain.linearRampToValueAtTime(gain, audioCtx.currentTime + 1.000)
        break
      }
      case 0x02: { // opDelete
        console.log(`Server control: delete id=${id}`)
        delete users[id]
        document.getElementById(`${id}-group`)?.remove()
        break
      }
    }
  })
  ws.addEventListener("error", (e) => {
    console.log(`Websocket: error`, e)
  })
  ws.addEventListener("close", (e) => {
    console.log("Websocket: close", e)
    if (e.code == 400) {
      message.innerText = "Connection close: multi login is not allowed."
    } else {
      message.innerText = `Connection close: ${e.reason}(code:${e.code})`
    }
    isClosed = true
  })

  // Audio API initialize
  console.log("Audio API initialize")
  users = {}
  // @ts-expect-error
  audioCtx = new (window.AudioContext || window.webkitAudioContext)({ sampleRate: sampleRate });
  const inputGainNode = audioCtx.createGain()
  inputGainNode.gain.value = 3
  await audioCtx.audioWorklet.addModule(`./getPcmProcessor.js?t=${new Date()}`)
  const getPcmNode = new AudioWorkletNode(audioCtx, "get-pcm-processor")
  getPcmNode.port.onmessage = (e) => {
    if (isClosed) return

    buffer.push(...Array.from(e.data))
  }

  console.log("Get Voice stream")
  const media = await navigator.mediaDevices.getUserMedia({
    audio: {
      sampleRate: sampleRate,
    },
    video: true,
  })
  console.log("Media:", media)
  const track = audioCtx.createMediaStreamSource(media)
  console.log("Track:", track)
  track.connect(inputGainNode)
  inputGainNode.connect(getPcmNode)
  console.log("Connected track => getPcmNode")
}

start.addEventListener("click", NewConnection)

/**
 * @param {string} id
 * @param {Float32Array} pcm
 */
function playAudioStream(id, pcm) {
  const buffer = audioCtx.createBuffer(1, pcm.length, sampleRate)
  const source = audioCtx.createBufferSource()
  const currentTime = audioCtx.currentTime;

  buffer.getChannelData(0).set(pcm);

  source.buffer = buffer;
  source.connect(users[id].serverGainNode);

  if (currentTime < users[id].schedule) {
    source.start(users[id].schedule)
    users[id].schedule += buffer.duration;
  } else {
    source.start(users[id].schedule)
    users[id].schedule = currentTime + buffer.duration;
  }
}

/**
 * @param {string} id
 */
function newVolume(id) {
  const group = document.createElement("div")
  group.id = `${id}-group`

  const name = document.createElement("span")
  name.innerText = `${id}:`
  group.append(name)
  const volume = document.createElement("input")
  volume.id = `${id}-input`
  volume.type = "range"
  volume.setAttribute("min", "0")
  volume.setAttribute("max", "5")
  volume.setAttribute("step", "0.1")
  volume.value = "1"
  group.append(volume)
  const volumeValue = document.createElement("span")
  volumeValue.id = `${id}-value`
  volumeValue.innerText = `(100%)`
  group.append(volumeValue)

  volumeArea?.append(group)

  volume.addEventListener("input", () => {
    const value = volume.value
    updateVolume(id)
  })

  const value = getCookie(id)
  if (value) {
    volume.value = value
  }
  updateVolume(id)
}

function updateVolume(id) {
  /** @type {HTMLInputElement} */
  // @ts-expect-error
  const volume = document.getElementById(`${id}-input`)
  const value = Number(volume.value) ?? 0
  /** @type {HTMLSpanElement} */
  // @ts-expect-error
  const volumeValue = document.getElementById(`${id}-value`)
  volumeValue.innerText = `(${Math.floor(value * 100)}%)`

  users[id].clientGainNode.gain.value = value
  setCookie(id, String(value))
}

/**
 * @param {string} key
 * @return {string|undefined}
 */
function getCookie(key) {
  return document.cookie.
    split("; ").
    find((row) => row.startsWith(`${key}=`))?.
    split("=")[1]

}

/**
 * @param {string} key
 * @param {string} value
 */
function setCookie(key, value) {
  document.cookie = `${key}=${value}`
}