// @ts-check

const REQUEST_URL = new URL(window.location.href)
const URL_PARAMS = new URLSearchParams(REQUEST_URL.searchParams)

const API_ENTRY_POINT = REQUEST_URL.origin + "/api"