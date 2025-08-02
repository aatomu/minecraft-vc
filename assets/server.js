// @ts-check
const API_ENTRY_POINT = new URL(window.location.href).origin + "/api"

/**
 * @typedef Server
 * @property {string} address
 * @property {string} pass
 * @property {number} fadeout
 * @property {number} mute
 */


async function ServersGet() {
  /** @type {Object.<string,Server>} */
  const SERVER_LIST = await fetch(`${API_ENTRY_POINT}/servers`, {
    method: "GET",
  }).then(res => {
    return res.json()
  })

  Object.keys(SERVER_LIST).forEach((SERVER_NAME) => {
    const SERVER = SERVER_LIST[SERVER_NAME]
    console.log(`Name: ${SERVER_NAME}\n`, SERVER)
  })
}

/**
 * @param {string} name
 * @param {string} password
 */
async function ServerGet(name, password) {
  /** @type {Server} */
  const SERVER = await fetch(`${API_ENTRY_POINT}/server`, {
    method: "GET",
    headers: {
      "X-Name": name,
      "X-Password": password,
    }
  }).then(res => {
    return res.json()
  })

  console.log(`Name: ${name}\n`, SERVER)
}

/**
 * @param {string} name
 * @param {string} address
 * @param {string} pass
 * @param {number} fadeout
 * @param {number} mute
 */
async function ServerPut(name = "example", address = "localhost:25575", pass = "0000", fadeout = 3.0, mute = 15.0) {
  console.log(await fetch(`${API_ENTRY_POINT}/server`, {
    method: "PUT",
    headers: {
      "X-Name": name,
      "Content-Type": "application/json"
    },
    body: JSON.stringify({
      address: address,
      pass: pass,
      fadeout: fadeout,
      mute: mute
    })
  }))
}

/**
 * @param {string} name
 * @param {string} password
 */
async function ServerDelete(name, password) {
  /** @type {Server} */
  console.log(await fetch(`${API_ENTRY_POINT}/server`, {
    method: "DELETE",
    headers: {
      "X-Name": name,
      "X-Password": password,
    }
  }))
}
