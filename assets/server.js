// @ts-check
///<reference path="./script.js">

/**
 * @typedef Server
 * @property {string} address
 * @property {string} pass
 * @property {number} fadeout
 * @property {number} mute
 */


async function ServerGetList() {
  /** @type {Object.<string,Server>} */
  const SERVER_LIST = await fetch(`${API_ENTRY_POINT}/server`).then(res => {
    return res.json()
  })
  
  Object.keys(SERVER_LIST).forEach((SERVER_NAME) => {
    const SERVER = SERVER_LIST[SERVER_NAME]
    console.log(SERVER)
  })
}