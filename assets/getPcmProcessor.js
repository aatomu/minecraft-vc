class GetPcmProcessor extends AudioWorkletProcessor {
  process(inputs, outputs, parameters) {
    const input = inputs[0]
    if (input.length > 0) {
      const voice = input[0]
      this.port.postMessage(voice)
    }

    outputs = inputs
    return true
  }
}

registerProcessor("get-pcm-processor", GetPcmProcessor);