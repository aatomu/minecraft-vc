// @ts-check
///<reference path="./script.js">

// Global variables
/** @type {HTMLInputElement} */
// @ts-expect-error
const BUTTON = document.getElementById("button")

BUTTON.addEventListener("click", clickButton)

function clickButton() {
  console.log(`Click button:`)

  window.location.href = `${REQUEST_URL}/client?server=${SERVER.value}&id=${MCID.value}`
}