// @ts-check
///<reference path="./script.js">

// Global variables
/** @type {HTMLInputElement} */
// @ts-expect-error
const BUTTON = document.getElementById("button")
let buttonState = 0
let isMute = false
/** @type {HTMLDivElement} */
// @ts-expect-error
const VOLUME_LIST = document.getElementById("volumes")
/** @type {AudioContext} */
// @ts-expect-error
let audioCtx = undefined
const SAMPLE_RATE = 44100
const INPUT_BUFFER_TIME = 100
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
const server = URL_PARAMS.get("server")
if (!server) {
  updateButton(true, "Please reload", "")
  updateMessage("Error: required server parameter.")
}
const id = URL_PARAMS.get("id")
if (!id) {
  updateButton(true, "Please reload", "")
  updateMessage("Error: required MCID parameter.")
}

BUTTON.addEventListener("click", clickButton)

function clickButton() {
  console.log(`Click button: state=${buttonState}`)
  switch (buttonState) {
    case 0: {
      NewConnection()
      isMute = false

      updateButton(false, "Mic to mute", "green")
      buttonState = 1
      break
    }
    case 1: {
      isMute = true

      updateButton(false, "Mic to unmute", "red")
      buttonState = 2
      break
    }
    case 2: {
      isMute = false

      updateButton(false, "Mic to mute", "green")
      buttonState = 1
      break
    }
  }
}

async function NewConnection() {
  // Websocket initialize
  console.log("Websocket initialize")
  const ws = new WebSocket(`./websocket?server=${server}&id=${id}`)
  ws.binaryType = "arraybuffer"
  let isClosed = false
  ws.addEventListener("open", () => {
    console.log("Websocket: open")
    updateMessage("connected to server")

    setInterval(() => {
      if (isClosed) return

      if (!isMute) {
        ws.send(new Float32Array(buffer))
      }
      buffer = []

    }, INPUT_BUFFER_TIME)
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
      case 0x03: { // opMessage
        const message = new TextDecoder("utf-8").decode(arr)
        console.log(`Server control: message id=${id} value=${message}`)
        updateMessage(message)
      }
    }
  })
  ws.addEventListener("error", (e) => {
    console.log(`Websocket: error`, e)
  })
  ws.addEventListener("close", (e) => {
    console.log("Websocket: close", e)
    if (e.code != 1000) {
      updateMessage(`Connection close: code=${e.code}`)
    }

    if (audioCtx) {
      audioCtx.close()
    }
    isClosed = true
  })

  // Audio API initialize
  console.log("Audio API initialize")
  users = {}
  // @ts-expect-error
  audioCtx = new (window.AudioContext || window.webkitAudioContext)({ sampleRate: SAMPLE_RATE });
  await audioCtx.audioWorklet.addModule(`./getPcmProcessor.js?t=${new Date()}`)
  const getPcmNode = new AudioWorkletNode(audioCtx, "get-pcm-processor")
  getPcmNode.port.onmessage = (e) => {
    if (isClosed) return

    buffer.push(...Array.from(e.data))
  }

  console.log("Get Voice stream")
  const media = await navigator.mediaDevices.getUserMedia({
    audio: {
      sampleRate: SAMPLE_RATE,
    },
    video: true,
  })
  console.log("Media:", media)
  const track = audioCtx.createMediaStreamSource(media)
  console.log("Track:", track)
  track.connect(getPcmNode)
  console.log("Connected track => getPcmNode")
}

/**
 * @param {string} id
 * @param {Float32Array} pcm
 */
function playAudioStream(id, pcm) {
  const buffer = audioCtx.createBuffer(1, pcm.length, SAMPLE_RATE)
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
  group.classList.add("volume-group")

  const name = document.createElement("div")
  name.classList.add("volume-name")
  name.innerText = `${id}:`
  group.append(name)

  const volume = document.createElement("input")
  volume.id = `${id}-input`
  volume.classList.add("volume-input")
  volume.type = "range"
  volume.setAttribute("min", "0")
  volume.setAttribute("max", "1")
  volume.setAttribute("step", "0.01")
  volume.value = "1"
  group.append(volume)

  const volumeValue = document.createElement("div")
  volumeValue.id = `${id}-value`
  volumeValue.classList.add("volume-value")
  volumeValue.innerText = `(100%)`
  group.append(volumeValue)

  if (VOLUME_LIST.children.length > 0) {
    let isPlaced = false
    for (let i = 0; i < VOLUME_LIST.children.length; i++) {
      /** @type {HTMLSpanElement} */
      //@ts-expect-error
      const childrenName = VOLUME_LIST.children[i].querySelector(".volume-name")
      VOLUME_LIST.children[0].querySelector(".volume-name")

      if (id.localeCompare(childrenName.innerText) < 0) {
        VOLUME_LIST.children[i].before(group)
        isPlaced = true
        break
      }
    }
    if (!isPlaced) {
      VOLUME_LIST.append(group)
    }
  } else {
    VOLUME_LIST.append(group)
  }

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
  volumeValue.innerText = `(${Math.floor(value * 100).toString().padStart(3, "0")}%)`

  users[id].clientGainNode.gain.value = value * 30
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

/**
 * @param {string} text
 */
function updateMessage(text) {
  /** @type {HTMLSpanElement} */
  // @ts-expect-error
  const message = document.getElementById("message")

  message.innerText = text
}

/**
 * @param {boolean} isDisable
 * @param {string} text
 * @param {""|"green"|"red"} color
 */
function updateButton(isDisable, text, color) {
  if (isDisable) {
    BUTTON.setAttribute("disabled", "true")
    BUTTON.classList.add("button-disable")
  } else {
    BUTTON.removeAttribute("disabled")
    BUTTON.classList.remove("button-disable")
  }

  /** @type {HTMLSpanElement} */
  // @ts-expect-error
  const buttonMessage = document.getElementById("button-message")
  buttonMessage.innerText = text

  switch (color) {
    case "green": {
      buttonMessage.classList.add("button-green")
      buttonMessage.classList.remove("button-red")
      buttonMessage.classList.remove("button-disable")
      break
    }
    case "red": {
      buttonMessage.classList.add("button-red")
      buttonMessage.classList.remove("button-green")
      buttonMessage.classList.remove("button-disable")
      break
    }
    default: {
      buttonMessage.classList.remove("button-green")
      buttonMessage.classList.remove("button-red")
      buttonMessage.classList.remove("button-disable")
      break
    }
  }
}